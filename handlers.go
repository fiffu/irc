package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Nicks must start with a letter or underscore, as IRC does not allow a leading digit or dash.
var nickRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,15}$`)

type server struct {
	cfg   config
	now   func() time.Time
	gamja http.Handler
	ergo  *httputil.ReverseProxy
	conns ergoConns
}

func newServer(cfg config) *server {
	return &server{
		cfg:   cfg,
		now:   time.Now,
		gamja: http.StripPrefix("/client", http.FileServer(http.Dir(cfg.gamjaDir))),
		ergo:  newErgoProxy(cfg.ergoURL),
		conns: ergoConns{conns: map[string]*ergoConn{}},
	}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/client", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/client/", http.StatusMovedPermanently)
	})
	mux.Handle("/client/", s.requireSession(http.HandlerFunc(s.handleClient)))
	mux.Handle("/ws", s.requireSession(http.HandlerFunc(s.handleWS)))
	mux.Handle("/invite", s.requireSession(http.HandlerFunc(s.handleInvite)))
	mux.HandleFunc("/favicon.png", handleFavicon)
	mux.HandleFunc("/favicon.ico", handleFavicon)
	// gamja-tabs.js sends older chat tabs here.
	mux.HandleFunc("/disconnected", func(w http.ResponseWriter, r *http.Request) {
		render(w, http.StatusOK, pageData{Title: "IRC disconnected", Message: "The chat is open in another tab.", Link: "/", LinkText: "Use this tab instead"})
	})
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// The invite page URL holds the invite key, so never send a Referer.
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.validSession(r) {
			render(w, http.StatusUnauthorized, pageData{Message: "Scan the invite QR code to join."})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	k := r.URL.Query().Get("k")
	if k == "" {
		if !s.validSession(r) {
			render(w, http.StatusUnauthorized, pageData{Message: "Scan the invite QR code to join."})
			return
		}
		// The session stands in for the invite key, so the user can change their nick.
		render(w, http.StatusOK, pageData{Form: true, Nick: s.savedNick(r)})
		return
	}
	if !s.validInvite(k) {
		render(w, http.StatusForbidden, pageData{Message: "This invite is not valid."})
		return
	}
	render(w, http.StatusOK, pageData{Form: true, Invite: k})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	k := r.PostForm.Get("k")
	if !s.validInvite(k) && !s.validSession(r) {
		render(w, http.StatusForbidden, pageData{Message: "This invite is not valid."})
		return
	}
	nick := strings.TrimSpace(r.PostForm.Get("nick"))
	if !nickRe.MatchString(nick) {
		render(w, http.StatusBadRequest, pageData{
			Form:   true,
			Invite: k,
			Nick:   nick,
			Error:  "Use 1 to 16 letters, digits, _ or -. Start with a letter or _.",
		})
		return
	}
	s.setSession(w, nick)
	http.Redirect(w, r, "/client/?nick="+url.QueryEscape(nick), http.StatusSeeOther)
}

// handleClient puts the saved nick back into gamja's URL. gamja reads the nick only from ?nick=,
// so a page refresh would otherwise connect as "user-<random>". The browser keeps the URL
// fragment (gamja's "#/#lobby" route) across the redirect.
func (s *server) handleClient(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/client/" && !r.URL.Query().Has("nick") {
		target := "/"
		if nick := s.savedNick(r); nick != "" {
			target = "/client/?nick=" + url.QueryEscape(nick)
		}
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	s.gamja.ServeHTTP(w, r)
}

func (s *server) savedNick(r *http.Request) string {
	if c, err := r.Cookie(nickCookie); err == nil && nickRe.MatchString(c.Value) {
		return c.Value
	}
	return ""
}
