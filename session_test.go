package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestSessionUnique(t *testing.T) {
	s := newTestServer(t, "http://ergo")
	expiry := time.Now().Add(time.Hour)
	a, b := s.signSession(expiry), s.signSession(expiry)
	if a == b {
		t.Errorf("two logins in the same second got the same cookie %q", a)
	}
	// A cookie in the format from before the nonce still works.
	exp := strconv.FormatInt(expiry.Unix(), 10)
	old := exp + "." + base64.RawURLEncoding.EncodeToString(s.mac(exp))
	for _, value := range []string{a, b, old} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: cookieName, Value: value})
		if !s.validSession(req) {
			t.Errorf("cookie %q not valid", value)
		}
	}
}
