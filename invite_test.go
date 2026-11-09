package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInvitePage(t *testing.T) {
	s := newTestServer(t, "http://ergo")
	req := httptest.NewRequest(http.MethodGet, "/invite", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: s.signSession(time.Now().Add(time.Hour))})
	rec := do(t, s.routes(), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "https://irc.example/?k="+testInvite) {
		t.Error("invite link missing")
	}
	if !strings.Contains(body, "<svg") || !strings.Contains(body, `d="M`) {
		t.Error("QR code missing")
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control %q", cc)
	}
}
