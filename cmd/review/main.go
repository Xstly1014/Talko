package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/Xstly1014/Talko/internal/review"
	"github.com/Xstly1014/Talko/internal/store"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	email := flag.String("bootstrap-email", "", "create one staff account, password from stdin")
	role := flag.String("role", "reviewer", "admin/reviewer/risk")
	flag.Parse()
	db, e := store.Open()
	if e != nil {
		die(e)
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *email != "" {
		b, e := io.ReadAll(io.LimitReader(os.Stdin, 100))
		if e != nil {
			die(e)
		}
		if e = review.CreateStaff(ctx, db, *email, strings.TrimSpace(string(b)), *role); e != nil {
			die(e)
		}
		fmt.Println("staff account created")
		return
	}
	origin := os.Getenv("REVIEW_ORIGIN")
	if origin == "" {
		origin = "http://localhost:5174"
	}
	secure := os.Getenv("COOKIE_SECURE") == "true"
	if os.Getenv("APP_ENV") == "production" && (!secure || !strings.HasPrefix(origin, "https://")) {
		die(errors.New("production requires HTTPS and secure cookies"))
	}
	addr := os.Getenv("REVIEW_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8081"
	}
	app := &review.Server{DB: db, Origin: origin, Secure: secure}
	srv := &http.Server{Addr: addr, Handler: app.Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	slog.Info("review service starting", "addr", addr)
	if e = srv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		die(e)
	}
}
func die(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
