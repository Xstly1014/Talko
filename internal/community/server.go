package community

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Xstly1014/Talko/internal/store"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type bucket struct {
	count int
	until time.Time
}
type Server struct {
	rateMu   sync.Mutex
	rates    map[string]bucket
	DB       *sql.DB
	Origin   string
	Secure   bool
	Requests atomic.Int64
	Failures atomic.Int64
	Mail     func(string, string) error
}
type apiError struct {
	Status        int
	Code, Message string
}

func (e *apiError) Error() string { return e.Message }
func bad(s string) error          { return &apiError{400, "validation", s} }
func conflict(s string) error     { return &apiError{409, "conflict", s} }
func missing() error              { return &apiError{404, "not_found", "内容不存在或不可访问"} }

type handler func(http.ResponseWriter, *http.Request) error

func (s *Server) wrap(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.Requests.Add(1)
		if r.Method != "GET" && !s.allow(r) {
			writeJSON(w, 429, map[string]string{"code": "rate_limited", "message": "操作过于频繁，请稍后再试"})
			return
		}
		rid := store.ID()
		w.Header().Set("X-Request-ID", rid)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if r.Method != "GET" && r.Method != "HEAD" {
			origin := r.Header.Get("Origin")
			if origin != "" && origin != s.Origin {
				writeJSON(w, 403, map[string]string{"code": "origin_denied", "message": "请求来源不受信任", "request_id": rid})
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
		start := time.Now()
		if e := h(w, r); e != nil {
			status := 500
			code := "internal"
			msg := "服务暂时不可用，请稍后重试"
			var ae *apiError
			if errors.As(e, &ae) {
				status = ae.Status
				code = ae.Code
				msg = ae.Message
			} else if errors.Is(e, sql.ErrNoRows) {
				status = 404
				code = "not_found"
				msg = "记录不存在"
			}
			if status >= 500 {
				s.Failures.Add(1)
				slog.Error("request failed", "request_id", rid, "error", e.Error())
			}
			writeJSON(w, status, map[string]string{"code": code, "message": msg, "request_id": rid})
		}
		slog.Info("request", "request_id", rid, "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	}
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return bad("请求字段或 JSON 格式错误")
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return bad("只允许一个 JSON 对象")
	}
	return nil
}
func (s *Server) Routes() http.Handler {
	m := http.NewServeMux()
	routes := map[string]handler{
		"GET /healthz": s.health, "GET /readyz": s.ready, "GET /metrics": s.metrics,
		"POST /api/v1/auth/email-code": s.emailCode, "POST /api/v1/auth/register": s.register, "POST /api/v1/auth/login": s.login, "POST /api/v1/auth/logout": s.logout,
		"GET /api/v1/me": s.me, "PATCH /api/v1/me/profile": s.profile, "GET /api/v1/users/{id}": s.userProfile,
		"POST /api/v1/media": s.upload, "GET /api/v1/media/{id}": s.media,
		"POST /api/v1/posts": s.createPost, "PATCH /api/v1/posts/{id}/draft": s.editPost, "POST /api/v1/posts/{id}/submit": s.submitPost, "POST /api/v1/posts/{id}/withdraw": s.withdraw, "DELETE /api/v1/posts/{id}": s.deletePost,
		"GET /api/v1/posts/{id}": s.getPost, "GET /api/v1/feed": s.feed, "GET /api/v1/me/posts": s.myPosts,
		"GET /api/v1/posts/{id}/comments": s.comments, "POST /api/v1/posts/{id}/comments": s.addComment, "DELETE /api/v1/comments/{id}": s.deleteComment,
		"PUT /api/v1/posts/{id}/reactions/{kind}": s.react, "DELETE /api/v1/posts/{id}/reactions/{kind}": s.react,
		"PUT /api/v1/users/{id}/follow": s.follow, "DELETE /api/v1/users/{id}/follow": s.follow,
		"POST /api/v1/posts/{id}/reports": s.report, "GET /api/v1/me/reports": s.reports, "POST /api/v1/appeals": s.appeal, "GET /api/v1/me/appeals": s.appeals,
		"GET /api/v1/notifications": s.notifications, "POST /api/v1/notifications/{id}/read": s.readNotification,
	}
	for p, h := range routes {
		m.HandleFunc(p, s.wrap(h))
	}
	return m
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) error {
	writeJSON(w, 200, map[string]string{"status": "ok"})
	return nil
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) error {
	ctx, c := context.WithTimeout(r.Context(), time.Second)
	defer c()
	if e := s.DB.PingContext(ctx); e != nil {
		return e
	}
	writeJSON(w, 200, map[string]string{"status": "ready"})
	return nil
}
func (s *Server) metrics(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	var n int
	_ = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM outbox WHERE processed_at IS NULL").Scan(&n)
	fmt.Fprintf(w, "talko_http_requests_total %d\ntalko_http_failures_total %d\ntalko_outbox_pending %d\n", s.Requests.Load(), s.Failures.Load(), n)
	return nil
}
func (s *Server) auth(r *http.Request) (string, error) {
	c, e := r.Cookie("talko_session")
	if e != nil {
		return "", &apiError{401, "unauthenticated", "请先登录"}
	}
	var id string
	e = s.DB.QueryRowContext(r.Context(), "SELECT user_id FROM sessions WHERE token_hash=? AND expires_at>UTC_TIMESTAMP(6)", store.Hash(c.Value)).Scan(&id)
	if e != nil {
		return "", &apiError{401, "unauthenticated", "登录已过期，请重新登录"}
	}
	return id, nil
}
func (s *Server) optional(r *http.Request) string { id, _ := s.auth(r); return id }
func (s *Server) setSession(w http.ResponseWriter, r *http.Request, id string) error {
	token := store.ID() + store.ID()
	_, e := s.DB.ExecContext(r.Context(), "INSERT INTO sessions(token_hash,user_id,expires_at) VALUES(?,?,?)", store.Hash(token), id, time.Now().UTC().Add(7*24*time.Hour))
	if e != nil {
		return e
	}
	http.SetCookie(w, &http.Cookie{Name: "talko_session", Value: token, Path: "/", HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 604800})
	return nil
}
func (s *Server) emailCode(w http.ResponseWriter, r *http.Request) error {
	var in struct{ Email string }
	if e := decode(r, &in); e != nil {
		return e
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	a, e := mail.ParseAddress(in.Email)
	if e != nil || a.Address != in.Email || len(in.Email) > 254 {
		return bad("请输入有效邮箱")
	}
	n, e := rand.Int(rand.Reader, big.NewInt(1000000))
	if e != nil {
		return e
	}
	code := fmt.Sprintf("%06d", n.Int64())
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(r.Context(), "INSERT IGNORE INTO email_codes(email,code_hash,expires_at,sent_at) VALUES(?,?,UTC_TIMESTAMP(6),DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 DAY))", in.Email, "")
	if e != nil {
		return e
	}
	var sent time.Time
	e = tx.QueryRowContext(r.Context(), "SELECT sent_at FROM email_codes WHERE email=? FOR UPDATE", in.Email).Scan(&sent)
	if e != nil {
		return e
	}
	if time.Since(sent) < 60*time.Second {
		return &apiError{429, "rate_limited", "请 60 秒后重试"}
	}
	_, e = tx.ExecContext(r.Context(), "UPDATE email_codes SET code_hash=?,expires_at=?,attempts=0,sent_at=UTC_TIMESTAMP(6) WHERE email=?", store.Hash(in.Email+":"+code), time.Now().UTC().Add(10*time.Minute), in.Email)
	if e != nil {
		return e
	}
	if s.Mail == nil {
		return fmt.Errorf("mail transport not configured")
	}
	if e = s.Mail(in.Email, code); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	writeJSON(w, 202, map[string]string{"message": "验证码已发送，有效期 10 分钟"})
	return nil
}
func (s *Server) register(w http.ResponseWriter, r *http.Request) error {
	var in struct{ Email, Code, Password, Nickname string }
	if e := decode(r, &in); e != nil {
		return e
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if len(in.Password) < 12 || len(in.Password) > 72 || len([]rune(strings.TrimSpace(in.Nickname))) < 1 || len([]rune(in.Nickname)) > 30 {
		return bad("密码须为 12–72 字节，昵称须为 1–30 字")
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var expected string
	var exp time.Time
	var attempts int
	e = tx.QueryRowContext(r.Context(), "SELECT code_hash,expires_at,attempts FROM email_codes WHERE email=? FOR UPDATE", in.Email).Scan(&expected, &exp, &attempts)
	if e != nil || time.Now().After(exp) || attempts >= 5 {
		return bad("验证码无效或过期")
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(store.Hash(in.Email+":"+in.Code))) != 1 {
		_, e = tx.ExecContext(r.Context(), "UPDATE email_codes SET attempts=attempts+1 WHERE email=?", in.Email)
		if e != nil {
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
		return bad("验证码错误")
	}
	id := store.ID()
	_, e = tx.ExecContext(r.Context(), "INSERT INTO users(id,email,nickname,password_hash) VALUES(?,?,?,?)", id, in.Email, in.Nickname, hash)
	if e != nil {
		return conflict("此邮箱已注册")
	}
	_, e = tx.ExecContext(r.Context(), "DELETE FROM email_codes WHERE email=?", in.Email)
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	if e = s.setSession(w, r, id); e != nil {
		return e
	}
	writeJSON(w, 201, map[string]string{"id": id})
	return nil
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var in struct{ Email, Password string }
	if e := decode(r, &in); e != nil {
		return e
	}
	var id string
	var hash []byte
	e := s.DB.QueryRowContext(r.Context(), "SELECT id,password_hash FROM users WHERE email=?", strings.ToLower(strings.TrimSpace(in.Email))).Scan(&id, &hash)
	if e != nil || bcrypt.CompareHashAndPassword(hash, []byte(in.Password)) != nil {
		return &apiError{401, "invalid_credentials", "邮箱或密码错误"}
	}
	if e = s.setSession(w, r, id); e != nil {
		return e
	}
	writeJSON(w, 200, map[string]string{"id": id})
	return nil
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if c, e := r.Cookie("talko_session"); e == nil {
		if _, e = s.DB.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=?", store.Hash(c.Value)); e != nil {
			return e
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "talko_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode})
	writeJSON(w, 200, map[string]bool{"ok": true})
	return nil
}
func SMTP(addr, from string) func(string, string) error {
	return func(to, code string) error {
		body := "From: " + from + "\r\nTo: " + to + "\r\nSubject: Talko email verification\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nTalko 验证码：" + code + "。10 分钟内有效，请勿转发。"
		return smtp.SendMail(addr, nil, from, []string{to}, []byte(body))
	}
}
func Config(db *sql.DB) *Server {
	return &Server{DB: db, Origin: os.Getenv("APP_ORIGIN"), Secure: os.Getenv("COOKIE_SECURE") == "true", Mail: SMTP(os.Getenv("SMTP_ADDR"), os.Getenv("SMTP_FROM"))}
}

func (s *Server) allow(r *http.Request) bool {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	if s.rates == nil {
		s.rates = map[string]bucket{}
	}
	now := time.Now()
	if len(s.rates) > 10000 {
		for k, v := range s.rates {
			if now.After(v.until) {
				delete(s.rates, k)
			}
		}
		if len(s.rates) > 10000 {
			return false
		}
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	key := ip + "/write"
	limit := 120
	if strings.Contains(r.URL.Path, "/auth/") {
		key = ip + "/auth"
		limit = 15
	}
	b := s.rates[key]
	if now.After(b.until) {
		b = bucket{until: now.Add(time.Minute)}
	}
	b.count++
	s.rates[key] = b
	return b.count <= limit
}
