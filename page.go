package main

import (
	_ "embed"
	"html/template"
	"log"
	"net/http"
)

type pageData struct {
	// Title defaults to "IRC".
	Title     string
	Message   string
	Link      string
	LinkText  string
	Form      bool
	Invite    string
	Nick      string
	Error     string
	InviteURL string
	QR        template.HTML
}

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="referrer" content="no-referrer">
<title>{{or .Title "IRC"}}</title>
<link rel="icon" href="/favicon.png">
<style>
:root { color-scheme: light dark; font-family: system-ui, sans-serif; --top: #fffbe8; --bottom: #fff; --text: #2b2b2b; --line: #888; }
@media (prefers-color-scheme: dark) { :root { --top: #1f1c10; --bottom: #000; --text: #d6d3ca; } }
html { min-height: 100%; background: var(--bottom) linear-gradient(var(--top), var(--bottom)) no-repeat; }
body { max-width: 24rem; margin: 15vh auto; padding: 0 1rem 2rem; line-height: 1.5; color: var(--text); }
input, button, .cta { font: inherit; padding: .6rem; width: 100%; box-sizing: border-box; margin-top: .5rem; border-radius: 8px; }
input { border: 1px solid var(--line); background: transparent; color: inherit; }
button, .cta { display: block; border: 0; background: #000; color: #d6d3ca; font-weight: 600; text-align: center; text-decoration: none; cursor: pointer; }
button:hover, .cta:hover { background: #1a1a1a; }
.error { color: #c00; }
.qr svg { display: block; width: 100%; max-width: 20rem; margin: 1rem auto; }
.link { word-break: break-all; font-family: ui-monospace, monospace; font-size: .9rem; }
footer { position: fixed; bottom: .5rem; left: 0; right: 0; text-align: center; font-size: .75rem; opacity: .5; }
footer a { color: inherit; }
</style>
</head>
<body>
{{if .QR}}
<p>Scan this code, or send the link. Anyone with it can join.</p>
<div class="qr">{{.QR}}</div>
<p class="link"><a href="{{.InviteURL}}">{{.InviteURL}}</a></p>
{{else if .Form}}
<form method="post" action="/login">
<input type="hidden" name="k" value="{{.Invite}}">
<label for="nick">Pick a name</label>
<input id="nick" name="nick" value="{{.Nick}}" required maxlength="16" pattern="[A-Za-z_][A-Za-z0-9_\-]{0,15}" autocomplete="nickname" autofocus>
{{if .Error}}<p class="error">{{.Error}}</p>{{end}}
<button type="submit">Join</button>
</form>
{{else}}
<p>{{.Message}}</p>
{{if .Link}}<a class="cta" href="{{.Link}}">{{.LinkText}}</a>{{end}}
{{end}}
<footer><a href="https://github.com/fiffu/irc">Built with Gamja</a></footer>
</body>
</html>
`))

func render(w http.ResponseWriter, status int, d pageData) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.WriteHeader(status)
	if err := page.Execute(w, d); err != nil {
		log.Printf("render: %v", err)
	}
}

// favicon.png is a 64x64 crop of logo.png.
//
//go:embed favicon.png
var favicon []byte

// handleFavicon also answers /favicon.ico, which the gamja page requests because it names no icon.
func handleFavicon(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "image/png")
	h.Set("Cache-Control", "public, max-age=86400")
	w.Write(favicon)
}
