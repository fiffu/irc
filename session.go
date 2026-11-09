package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	cookieName = "session"
	// nickCookie holds the nick from the login form. It is not signed, because a nick grants nothing.
	nickCookie = "nick"
)

// validInvite compares hashes so that the comparison time does not depend on the key length.
func (s *server) validInvite(k string) bool {
	got := sha256.Sum256([]byte(k))
	want := sha256.Sum256(s.cfg.inviteKey)
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

func (s *server) setSession(w http.ResponseWriter, nick string) {
	expiry := s.now().Add(s.cfg.cookieTTL)
	for name, value := range map[string]string{cookieName: s.signSession(expiry), nickCookie: nick} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    value,
			Path:     "/",
			Expires:  expiry,
			HttpOnly: true,
			Secure:   !s.cfg.insecureCookie,
			// Lax, so that a chat link opened from another site still arrives logged in.
			SameSite: http.SameSiteLaxMode,
		})
	}
}

// signSession returns "<expiry unix seconds>.<base64url HMAC-SHA256 of the expiry>".
func (s *server) signSession(expiry time.Time) string {
	exp := strconv.FormatInt(expiry.Unix(), 10)
	return exp + "." + base64.RawURLEncoding.EncodeToString(s.mac(exp))
}

func (s *server) validSession(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	exp, sig, ok := strings.Cut(c.Value, ".")
	if !ok {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, s.mac(exp)) {
		return false
	}
	unix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return false
	}
	return s.now().Before(time.Unix(unix, 0))
}

func (s *server) mac(msg string) []byte {
	m := hmac.New(sha256.New, s.cfg.cookieKey)
	m.Write([]byte("session:" + msg))
	return m.Sum(nil)
}
