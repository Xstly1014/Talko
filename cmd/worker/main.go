package main

import (
	"context"
	"errors"
	"github.com/Xstly1014/Talko/internal/observability"
	"github.com/Xstly1014/Talko/internal/store"
	"github.com/Xstly1014/Talko/internal/worker"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	db, e := store.Open()
	if e != nil {
		slog.Error("database unavailable", "error", e)
		os.Exit(1)
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	m := http.NewServeMux()
	m.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) { observability.Serve(db, w, r) })
	m.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		c, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if db.PingContext(c) != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		w.WriteHeader(204)
	})
	addr := os.Getenv("WORKER_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8085"
	}
	srv := &http.Server{Addr: addr, Handler: m, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	go func() {
		if e := srv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			slog.Error("worker metrics server failed")
			stop()
		}
	}()
	worker.Run(ctx, db)
	c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(c)
}
