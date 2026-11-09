package main

import (
	"crypto/hmac"
	"crypto/rand"
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

// signSession returns "<expiry unix seconds>.<nonce>.<base64url HMAC-SHA256 of the part before it>".
// The random nonce makes each login's cookie unique. splash keeps one Ergo connection per cookie
// value, so two users who log in during the same second must not share one.
func (s *server) signSession(expiry time.Time) string {
	nonce := make([]byte, 16)
	rand.Read(nonce)
	payload := strconv.FormatInt(expiry.Unix(), 10) + "." + base64.RawURLEncoding.EncodeToString(nonce)
	return payload + "." + base64.RawURLEncoding.EncodeToString(s.mac(payload))
}

func (s *server) validSession(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	// Cookies from before the nonce have the form "<expiry>.<mac>". They stay valid until they expire.
	i := strings.LastIndexByte(c.Value, '.')
	if i < 0 {
		return false
	}
	payload, sig := c.Value[:i], c.Value[i+1:]
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, s.mac(payload)) {
		return false
	}
	exp, _, _ := strings.Cut(payload, ".")
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
