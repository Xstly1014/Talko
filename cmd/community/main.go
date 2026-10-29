package main

import (
	"context"
	"errors"
	"github.com/Xstly1014/Talko/internal/community"
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
		slog.Error("database unavailable", "error", e.Error())
		os.Exit(1)
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if e = store.Migrate(ctx, db); e != nil {
			slog.Error("migration failed", "error", e.Error())
			os.Exit(1)
		}
		slog.Info("migration complete")
		return
	}
	if os.Getenv("APP_ENV") == "production" && (os.Getenv("COOKIE_SECURE") != "true" || os.Getenv("APP_ORIGIN") == "") {
		slog.Error("production requires secure cookies and APP_ORIGIN")
		os.Exit(1)
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	srv := &http.Server{Addr: addr, Handler: community.Config(db).Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}
	go worker.Run(ctx, db)
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	slog.Info("community starting", "addr", addr)
	if e = srv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		slog.Error("server failed", "error", e.Error())
		os.Exit(1)
	}
}
