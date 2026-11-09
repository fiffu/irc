package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

func newErgoProxy(target *url.URL) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = r.In.Host
			// Ergo trusts these headers from the proxy and uses them for IP limits, bans and
			// the secure flag. Overwrite them so that a client cannot inject its own.
			r.Out.Header.Set("X-Forwarded-For", clientIP(r.In))
			r.Out.Header.Set("X-Forwarded-Proto", "https")
			r.Out.Header.Del("Cookie")
		},
	}
}

func clientIP(r *http.Request) string {
	if ip := net.ParseIP(r.Header.Get("CF-Connecting-IP")); ip != nil {
		return ip.String()
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ergoConns keeps at most one Ergo connection per session cookie. On a page refresh, the old
// WebSocket can stay open at Ergo for a while, because the close can arrive late. gamja then
// gets "nickname in use" and gives up, so splash closes the old connection first.
type ergoConns struct {
	mu    sync.Mutex
	conns map[string]*ergoConn
}

type ergoConn struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// oldConnWait bounds how long a new connection waits for the old one to close.
const oldConnWait = 5 * time.Second

func (s *server) handleWS(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		http.Error(w, "no session", http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	me := &ergoConn{cancel: cancel, done: make(chan struct{})}

	s.conns.mu.Lock()
	old := s.conns.conns[c.Value]
	s.conns.conns[c.Value] = me
	s.conns.mu.Unlock()

	if old != nil {
		// Cancelling the request context makes ReverseProxy close the upgraded connection.
		old.cancel()
		select {
		case <-old.done:
		case <-time.After(oldConnWait):
		}
	}
	defer func() {
		cancel()
		s.conns.mu.Lock()
		if s.conns.conns[c.Value] == me {
			delete(s.conns.conns, c.Value)
		}
		s.conns.mu.Unlock()
		close(me.done)
	}()
	s.ergo.ServeHTTP(w, r.WithContext(ctx))
}
