package main

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"rsc.io/qr"
)

// quietZone is the blank border around a QR code, in modules. The QR spec asks for 4.
const quietZone = 4

func inviteURL(cfg config) string {
	return cfg.publicURL + "/?k=" + url.QueryEscape(string(cfg.inviteKey))
}

// handleInvite shows the invite link and its QR code. Anyone with a session already
// received the invite key, so this shows them nothing new.
func (s *server) handleInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	link := inviteURL(s.cfg)
	code, err := qr.Encode(link, qr.M)
	if err != nil {
		http.Error(w, "cannot make QR code", http.StatusInternalServerError)
		return
	}
	render(w, http.StatusOK, pageData{InviteURL: link, QR: qrSVG(code)})
}

// qrSVG draws the code as one SVG path. Inline SVG needs no img-src in the page CSP.
func qrSVG(c *qr.Code) template.HTML {
	n := c.Size + 2*quietZone
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges" role="img" aria-label="Invite QR code">`, n, n)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, n, n)
	for y := 0; y < c.Size; y++ {
		for x := 0; x < c.Size; x++ {
			if c.Black(x, y) {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+quietZone, y+quietZone)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	// Safe: the markup holds only numbers that this function wrote.
	return template.HTML(b.String())
}

// qrTerminal draws the code with half-block characters, two rows per line. It draws the
// light modules, so it scans on a dark terminal background.
func qrTerminal(c *qr.Code) string {
	n := c.Size + 2*quietZone
	light := func(x, y int) bool {
		x, y = x-quietZone, y-quietZone
		return x < 0 || y < 0 || x >= c.Size || y >= c.Size || !c.Black(x, y)
	}
	var b strings.Builder
	for y := 0; y < n; y += 2 {
		for x := 0; x < n; x++ {
			top, bottom := light(x, y), y+1 < n && light(x, y+1)
			switch {
			case top && bottom:
				b.WriteString("█")
			case top:
				b.WriteString("▀")
			case bottom:
				b.WriteString("▄")
			default:
				b.WriteString(" ")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}
