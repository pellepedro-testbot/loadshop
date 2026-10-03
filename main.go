// Command loadshop is an in-memory web shop built as a system-under-test
// for browser and protocol load testing.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"loadshop/internal/shop"
)

//go:embed all:web/dist
var webDist embed.FS

const defaultSecret = "loadshop-dev-secret-change-me"

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	defTTL, err := time.ParseDuration(envOr("LOADSHOP_TOKEN_TTL", "24h"))
	if err != nil {
		log.Fatalf("LOADSHOP_TOKEN_TTL: %v", err)
	}
	addr := flag.String("addr", envOr("LOADSHOP_ADDR", ":8000"), "listen address")
	secret := flag.String("secret", envOr("LOADSHOP_SECRET", defaultSecret), "HMAC secret for bearer tokens (env LOADSHOP_SECRET)")
	ttl := flag.Duration("token-ttl", defTTL, "bearer token lifetime (env LOADSHOP_TOKEN_TTL)")
	accessLog := flag.Bool("access-log", os.Getenv("LOADSHOP_ACCESS_LOG") == "1", "log every request (off by default for performance)")
	flag.Parse()

	ui, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatal(err)
	}
	app, err := shop.New(shop.Config{Secret: *secret, TokenTTL: *ttl, AccessLog: *accessLog, UI: ui})
	if err != nil {
		log.Fatal(err)
	}
	if *secret == defaultSecret {
		log.Printf("using the built-in dev token secret; set -secret or LOADSHOP_SECRET for shared environments")
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           app,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// ListenAndServe returns as soon as Shutdown starts, so main waits on drained
	// before exiting; otherwise in-flight requests are cut off mid-drain.
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("loadshop listening on %s (%d products, token ttl %s)", *addr, len(app.Catalog.Products), *ttl)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	<-drained
	log.Printf("loadshop stopped")
}
