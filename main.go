package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rsc.io/qr"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{
		Addr:              cfg.listenAddr,
		Handler:           newServer(cfg).routes(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: it would cut long-lived WebSocket connections.
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	printInvite(cfg)
	log.Printf("listening on %s", cfg.listenAddr)
	log.Printf("proxying /ws to %s", cfg.ergoURL)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// printInvite writes the invite link and QR code to stderr. They end up in the container
// logs, so anyone who can read those logs can join.
func printInvite(cfg config) {
	link := inviteURL(cfg)
	log.Printf("invite link: %s", link)
	code, err := qr.Encode(link, qr.M)
	if err != nil {
		log.Printf("cannot make invite QR code: %v", err)
		return
	}
	fmt.Fprint(os.Stderr, qrTerminal(code))
}
