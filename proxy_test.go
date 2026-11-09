package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProxyHeaders(t *testing.T) {
	var got http.Header
	ergo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
	}))
	defer ergo.Close()
	s := newTestServer(t, ergo.URL)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: s.signSession(time.Now().Add(time.Hour))})
	req.Header.Set("CF-Connecting-IP", "203.0.113.7")
	req.Header.Set("X-Forwarded-For", "6.6.6.6")
	req.Header.Set("X-Forwarded-Proto", "http")
	if rec := do(t, s.routes(), req); rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if v := got.Get("X-Forwarded-For"); v != "203.0.113.7" {
		t.Errorf("X-Forwarded-For %q", v)
	}
	if v := got.Get("X-Forwarded-Proto"); v != "https" {
		t.Errorf("X-Forwarded-Proto %q", v)
	}
	if v := got.Get("Cookie"); v != "" {
		t.Errorf("cookie leaked to Ergo: %q", v)
	}
}

// A second WebSocket from the same session closes the first one before it reaches Ergo.
func TestOneErgoConnPerSession(t *testing.T) {
	events := make(chan string, 10)
	ergo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, rw, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		id := r.URL.Query().Get("id")
		events <- "open " + id
		rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		rw.Flush()
		conn.Read(make([]byte, 1)) // blocks until splash closes the connection
		events <- "close " + id
	}))
	defer ergo.Close()
	s := newTestServer(t, ergo.URL)
	splash := httptest.NewServer(s.routes())
	defer splash.Close()
	cookie := cookieName + "=" + s.signSession(time.Now().Add(time.Hour))

	dial := func(id string) net.Conn {
		conn, err := net.Dial("tcp", strings.TrimPrefix(splash.URL, "http://"))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(conn, "GET /ws?id=%s HTTP/1.1\r\nHost: x\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nCookie: %s\r\n\r\n", id, cookie)
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil || resp.StatusCode != http.StatusSwitchingProtocols {
			t.Fatalf("dial %s: %v %v", id, resp, err)
		}
		return conn
	}
	a := dial("a")
	defer a.Close()
	b := dial("b")
	defer b.Close()

	var got []string
	for len(got) < 3 {
		select {
		case e := <-events:
			got = append(got, e)
		case <-time.After(2 * time.Second):
			t.Fatalf("events so far: %v", got)
		}
	}
	if want := "open a,close a,open b"; strings.Join(got, ",") != want {
		t.Errorf("events %v, want %s", got, want)
	}
}
