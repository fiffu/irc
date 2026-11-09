package main

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"time"
)

type config struct {
	listenAddr string
	inviteKey  []byte
	cookieKey  []byte
	cookieTTL  time.Duration
	ergoURL    *url.URL
	gamjaDir   string
	// publicURL is the scheme and host that users open, with no trailing slash.
	publicURL string
	// insecureCookie drops the Secure flag. Only for local tests over plain HTTP.
	insecureCookie bool
}

func loadConfig() (config, error) {
	c := config{
		listenAddr: envOr("LISTEN_ADDR", ":8080"),
		inviteKey:  []byte(os.Getenv("INVITE_KEY")),
		cookieKey:  []byte(os.Getenv("COOKIE_KEY")),
		gamjaDir:   envOr("GAMJA_DIR", "/gamja"),
		publicURL:  strings.TrimSuffix(envOr("PUBLIC_URL", "https://irc.example.com"), "/"),
	}
	// An empty key would make the check pass for anyone: "" matches "", and anyone can sign with "".
	if len(c.inviteKey) == 0 || len(c.cookieKey) == 0 {
		return c, errors.New("INVITE_KEY and COOKIE_KEY must be set")
	}
	ttl, err := time.ParseDuration(envOr("COOKIE_TTL", "720h"))
	if err != nil || ttl <= 0 {
		return c, errors.New("COOKIE_TTL must be a positive duration, such as 720h")
	}
	c.cookieTTL = ttl
	u, err := url.Parse(envOr("ERGO_URL", "http://irc-ergo:8097"))
	if err != nil || u.Host == "" {
		return c, errors.New("ERGO_URL must be an absolute URL")
	}
	c.ergoURL = u
	c.insecureCookie = os.Getenv("INSECURE_COOKIE") == "1"
	return c, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
