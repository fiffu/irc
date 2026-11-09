package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testInvite = "invite-key-0123456789"

func newTestServer(t *testing.T, ergo string) *server {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("gamja"), 0o644); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(ergo)
	return newServer(config{
		inviteKey: []byte(testInvite),
		cookieKey: []byte(strings.Repeat("k", 32)),
		cookieTTL: time.Hour,
		ergoURL:   u,
		gamjaDir:  dir,
		publicURL: "https://irc.example",
	})
}

func do(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func login(t *testing.T, s *server, nick string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"k": {testInvite}, "nick": {nick}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(t, s.routes(), req)
}

func TestIndexInvite(t *testing.T) {
	s := newTestServer(t, "http://ergo")
	cases := []struct {
		target string
		status int
	}{
		{"/", http.StatusUnauthorized},
		{"/?k=wrong", http.StatusForbidden},
		{"/?k=" + testInvite, http.StatusOK},
		{"/nope", http.StatusNotFound},
	}
	for _, c := range cases {
		rec := do(t, s.routes(), httptest.NewRequest(http.MethodGet, c.target, nil))
		if rec.Code != c.status {
			t.Errorf("GET %s: status %d, want %d", c.target, rec.Code, c.status)
		}
	}
}

func TestLogin(t *testing.T) {
	s := newTestServer(t, "http://ergo")

	rec := login(t, s, "alice")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/client/?nick=alice" {
		t.Errorf("Location %q", loc)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("want session and nick cookies: %+v", cookies)
	}
	for _, c := range cookies {
		if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
			t.Errorf("bad cookie: %+v", c)
		}
	}

	get := func(target string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		return do(t, s.routes(), req)
	}
	if rec := get("/client/?nick=alice"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "gamja") {
		t.Errorf("client with cookie: status %d", rec.Code)
	}
	// A page refresh loses ?nick=, so splash puts it back.
	if rec := get("/client/"); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/client/?nick=alice" {
		t.Errorf("refresh: status %d, Location %q", rec.Code, rec.Header().Get("Location"))
	}
	// The index shows the form with the saved nick, so the user can change it.
	if rec := get("/"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `value="alice"`) {
		t.Errorf("index with session: status %d, want the form with the saved nick", rec.Code)
	}
}

// Sessions from before the nick cookie existed have no saved nick. They get the name form, with no
// invite key needed.
func TestSessionWithoutNick(t *testing.T) {
	s := newTestServer(t, "http://ergo")
	session := &http.Cookie{Name: cookieName, Value: s.signSession(time.Now().Add(time.Hour))}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(session)
	if rec := do(t, s.routes(), req); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `name="nick"`) {
		t.Errorf("index: status %d, want the name form", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/client/", nil)
	req.AddCookie(session)
	if rec := do(t, s.routes(), req); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Errorf("client without nick cookie: status %d, Location %q", rec.Code, rec.Header().Get("Location"))
	}

	form := url.Values{"nick": {"bob"}}
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(session)
	if rec := do(t, s.routes(), req); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/client/?nick=bob" {
		t.Errorf("login with session: status %d, Location %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestLoginRejects(t *testing.T) {
	s := newTestServer(t, "http://ergo")
	for _, nick := range []string{"", "1abc", "-abc", "a b", "abcdefghijklmnopq", "nick!"} {
		if rec := login(t, s, nick); rec.Code != http.StatusBadRequest {
			t.Errorf("nick %q: status %d, want 400", nick, rec.Code)
		}
	}

	form := url.Values{"k": {"wrong"}, "nick": {"alice"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if rec := do(t, s.routes(), req); rec.Code != http.StatusForbidden {
		t.Errorf("wrong invite: status %d, want 403", rec.Code)
	}
}

func TestDisconnected(t *testing.T) {
	s := newTestServer(t, "http://ergo")
	rec := do(t, s.routes(), httptest.NewRequest(http.MethodGet, "/disconnected", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `href="/">Use this tab instead`) || !strings.Contains(rec.Body.String(), "<title>IRC disconnected</title>") {
		t.Errorf("status %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestFavicon(t *testing.T) {
	s := newTestServer(t, "http://ergo")
	for _, path := range []string{"/favicon.png", "/favicon.ico"} {
		rec := do(t, s.routes(), httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || !strings.HasPrefix(rec.Body.String(), "\x89PNG") {
			t.Errorf("%s: status %d, type %q", path, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
}
