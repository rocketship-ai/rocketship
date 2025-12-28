package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rocketship-ai/rocketship/internal/controlplane"
)

func main() {
	cfg, err := controlplane.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("controlplane configuration error: %v", err)
	}

	srv, err := controlplane.NewServer(cfg)
	if err != nil {
		log.Fatalf("failed to initialise controlplane: %v", err)
	}
	defer func() {
		if err := srv.Close(); err != nil {
			log.Printf("controlplane close error: %v", err)
		}
	}()

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           loggingMiddleware(srv),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("controlplane shutdown error: %v", err)
		}
	}()

	log.Printf("rocketship controlplane listening on %s", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("controlplane server error: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lrw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, lrw.status, time.Since(start))
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (l *loggingResponseWriter) WriteHeader(statusCode int) {
	l.status = statusCode
	l.ResponseWriter.WriteHeader(statusCode)
}
