package main

import (
	"context"
	"errors"
	"github.com/Xstly1014/Talko/internal/alertbridge"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	u, e := url.Parse(os.Getenv("FEISHU_WEBHOOK"))
	secret, token := os.Getenv("FEISHU_SECRET"), os.Getenv("ALERT_TOKEN")
	if path := os.Getenv("ALERT_TOKEN_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			slog.Error("alert token file unavailable")
			os.Exit(1)
		}
		token = strings.TrimSpace(string(b))
	}
	if e != nil || u.Scheme != "https" || u.Host != "open.feishu.cn" || u.User != nil || u.RawQuery != "" || !strings.HasPrefix(u.Path, "/open-apis/bot/v2/hook/") || len(strings.TrimPrefix(u.Path, "/open-apis/bot/v2/hook/")) < 1 || secret == "" || len(token) < 32 {
		slog.Error("valid Feishu webhook, signing secret and 32+ byte alert token required")
		os.Exit(1)
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	h := alertbridge.Handler{Token: token, Sender: alertbridge.Sender{URL: u.String(), Secret: secret, Client: client}}
	m := http.NewServeMux()
	m.Handle("/alerts", h)
	m.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	srv := &http.Server{Addr: "0.0.0.0:8082", Handler: m, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	if e = srv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		slog.Error("alert bridge stopped")
		os.Exit(1)
	}
}
