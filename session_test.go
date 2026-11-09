package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSessionRequired(t *testing.T) {
	s := newTestServer(t, "http://ergo")
	valid := s.signSession(time.Now().Add(time.Hour))
	cases := map[string]string{
		"none":     "",
		"expired":  s.signSession(time.Now().Add(-time.Second)),
		"tampered": "9999999999" + valid[strings.Index(valid, "."):],
		"garbage":  "abc",
	}
	for name, value := range cases {
		for _, path := range []string{"/client/", "/ws", "/invite"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			if value != "" {
				req.AddCookie(&http.Cookie{Name: cookieName, Value: value})
			}
			if rec := do(t, s.routes(), req); rec.Code != http.StatusUnauthorized {
				t.Errorf("%s cookie, %s: status %d, want 401", name, path, rec.Code)
			}
		}
	}
}
