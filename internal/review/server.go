package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/Xstly1014/Talko/internal/moderation"
	"github.com/Xstly1014/Talko/internal/store"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Staff struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}
type Server struct {
	DB       *sql.DB
	Origin   string
	Secure   bool
	mu       sync.Mutex
	attempts map[string]int
	window   time.Time
}
type failure struct {
	status  int
	message string
}

func (e *failure) Error() string        { return e.message }
func fail(status int, msg string) error { return &failure{status, msg} }
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func parse(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return fail(400, "请求格式或字段不正确")
	}
	var x any
	if e := d.Decode(&x); e != io.EOF {
		return fail(400, "请求只允许一个 JSON 对象")
	}
	return nil
}

type handler func(http.ResponseWriter, *http.Request, *Staff) error

func (s *Server) wrap(role string, h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rid := store.ID()
		w.Header().Set("X-Request-ID", rid)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if r.Method != "GET" {
			if origin := r.Header.Get("Origin"); origin != "" && origin != s.Origin {
				reply(w, 403, map[string]string{"message": "请求来源不受信任", "request_id": rid})
				return
			}
		}
		var staff *Staff
		var e error
		if role != "public" {
			staff, e = s.auth(r)
			if e == nil && role == "risk" && staff.Role != "risk" && staff.Role != "admin" {
				e = fail(403, "需要风控或管理员权限")
			}
			if e == nil && role == "admin" && staff.Role != "admin" {
				e = fail(403, "需要管理员权限")
			}
		}
		if e == nil {
			e = h(w, r, staff)
		}
		if e != nil {
			status := 500
			msg := "服务异常，请稍后重试"
			var f *failure
			if errors.As(e, &f) {
				status = f.status
				msg = f.message
			} else if errors.Is(e, sql.ErrNoRows) {
				status = 404
				msg = "记录不存在"
			} else {
				slog.Error("review request failed", "request_id", rid, "error", e.Error())
			}
			reply(w, status, map[string]string{"message": msg, "request_id": rid})
		}
	}
}
func (s *Server) auth(r *http.Request) (*Staff, error) {
	c, e := r.Cookie("talko_review_session")
	if e != nil {
		return nil, fail(401, "请登录审核工作台")
	}
	u := &Staff{}
	e = s.DB.QueryRowContext(r.Context(), "SELECT u.id,u.email,u.role FROM staff_sessions ss JOIN staff_users u ON u.id=ss.staff_id WHERE ss.token_hash=? AND ss.expires_at>UTC_TIMESTAMP(6) AND u.disabled=FALSE", store.Hash(c.Value)).Scan(&u.ID, &u.Email, &u.Role)
	if e == sql.ErrNoRows {
		return nil, fail(401, "登录已过期或账号已停用")
	}
	if e != nil {
		return nil, e
	}
	return u, nil
}
func (s *Server) Routes() http.Handler {
	m := http.NewServeMux()
	add := func(p, role string, h handler) { m.HandleFunc(p, s.wrap(role, h)) }
	add("GET /readyz", "public", s.ready)
	add("POST /review-api/v1/auth/login", "public", s.login)
	add("POST /review-api/v1/auth/logout", "staff", s.logout)
	add("GET /review-api/v1/me", "staff", func(w http.ResponseWriter, r *http.Request, u *Staff) error { reply(w, 200, u); return nil })
	add("GET /review-api/v1/summary", "staff", s.summary)
	add("GET /review-api/v1/tasks", "staff", s.tasks)
	add("GET /review-api/v1/tasks/{id}", "staff", s.task)
	add("POST /review-api/v1/tasks/{id}/claim", "staff", s.claim)
	add("POST /review-api/v1/tasks/{id}/decision", "staff", s.decide)
	add("GET /review-api/v1/media/{id}", "staff", s.media)
	add("GET /review-api/v1/reports", "risk", s.reports)
	add("GET /review-api/v1/reports/{id}", "risk", s.reportDetail)
	add("POST /review-api/v1/reports/{id}/resolve", "risk", s.resolveReport)
	add("GET /review-api/v1/appeals", "risk", s.appeals)
	add("POST /review-api/v1/appeals/{id}/resolve", "risk", s.resolveAppeal)
	add("GET /review-api/v1/audit", "admin", s.audit)
	return m
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request, u *Staff) error {
	if e := s.DB.PingContext(r.Context()); e != nil {
		return e
	}
	reply(w, 200, map[string]string{"status": "ready"})
	return nil
}
func (s *Server) login(w http.ResponseWriter, r *http.Request, u *Staff) error {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	s.mu.Lock()
	if time.Since(s.window) > time.Minute {
		s.attempts = map[string]int{}
		s.window = time.Now()
	}
	if s.attempts == nil {
		s.attempts = map[string]int{}
	}
	if len(s.attempts) > 10000 {
		s.mu.Unlock()
		return fail(429, "登录请求过多")
	}
	s.attempts[ip]++
	n := s.attempts[ip]
	s.mu.Unlock()
	if n > 10 {
		return fail(429, "登录尝试过于频繁，请一分钟后重试")
	}
	var in struct{ Email, Password string }
	if e := parse(r, &in); e != nil {
		return e
	}
	if len(in.Password) > 72 {
		return fail(401, "账号或密码错误")
	}
	var id string
	var hash []byte
	e := s.DB.QueryRowContext(r.Context(), "SELECT id,password_hash FROM staff_users WHERE email=? AND disabled=FALSE", strings.ToLower(strings.TrimSpace(in.Email))).Scan(&id, &hash)
	if e != nil && e != sql.ErrNoRows {
		return e
	}
	if e == sql.ErrNoRows {
		hash = []byte("$2a$12$yTKgJjObF1VxRgC3jB3LcOg4IM9F0RVaTnDB.xF5u6esV77b4QoEa")
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(in.Password)) != nil || id == "" {
		return fail(401, "账号或密码错误")
	}
	token := store.ID() + store.ID()
	_, e = s.DB.ExecContext(r.Context(), "INSERT INTO staff_sessions(token_hash,staff_id,expires_at) VALUES(?,?,?)", store.Hash(token), id, time.Now().UTC().Add(8*time.Hour))
	if e != nil {
		return e
	}
	http.SetCookie(w, &http.Cookie{Name: "talko_review_session", Value: token, Path: "/", MaxAge: 28800, HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode})
	reply(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request, u *Staff) error {
	c, e := r.Cookie("talko_review_session")
	if e != nil {
		return e
	}
	_, e = s.DB.ExecContext(r.Context(), "DELETE FROM staff_sessions WHERE token_hash=?", store.Hash(c.Value))
	if e != nil {
		return e
	}
	http.SetCookie(w, &http.Cookie{Name: "talko_review_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode})
	reply(w, 200, map[string]bool{"ok": true})
	return nil
}
func CreateStaff(ctx context.Context, db *sql.DB, email, password, role string) error {
	if role != "admin" && role != "reviewer" && role != "risk" {
		return errors.New("role must be admin/reviewer/risk")
	}
	if len(password) < 16 || len(password) > 72 || !strings.Contains(email, "@") {
		return errors.New("email and 16-72 byte password required")
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(password), 12)
	if e != nil {
		return e
	}
	_, e = db.ExecContext(ctx, "INSERT INTO staff_users(id,email,password_hash,role) VALUES(?,?,?,?)", store.ID(), strings.ToLower(strings.TrimSpace(email)), hash, role)
	return e
}
func (s *Server) withTaskLock(r *http.Request, fn func() error) error {
	conn, e := s.DB.Conn(r.Context())
	if e != nil {
		return e
	}
	defer conn.Close()
	name := "review_" + store.Hash(r.PathValue("id"))[:40]
	var acquired int
	if e = conn.QueryRowContext(r.Context(), "SELECT GET_LOCK(?,3)", name).Scan(&acquired); e != nil {
		return e
	}
	if acquired != 1 {
		return fail(409, "任务正在被其他人处理")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", name)
	return fn()
}
func (s *Server) claim(w http.ResponseWriter, r *http.Request, u *Staff) error {
	return s.withTaskLock(r, func() error {
		var status string
		if e := s.DB.QueryRowContext(r.Context(), "SELECT status FROM review_tasks WHERE id=?", r.PathValue("id")).Scan(&status); e != nil {
			return e
		}
		if status != "pending" {
			return fail(409, "任务已处理或取消")
		}
		var owner string
		var exp time.Time
		e := s.DB.QueryRowContext(r.Context(), "SELECT staff_id,expires_at FROM review_assignments WHERE task_id=?", r.PathValue("id")).Scan(&owner, &exp)
		if e != nil && e != sql.ErrNoRows {
			return e
		}
		if e == nil && owner != u.ID && exp.After(time.Now()) {
			return fail(409, "任务已被其他审核员领取")
		}
		_, e = s.DB.ExecContext(r.Context(), "INSERT INTO review_assignments(task_id,staff_id,expires_at) VALUES(?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 15 MINUTE)) ON DUPLICATE KEY UPDATE staff_id=VALUES(staff_id),expires_at=VALUES(expires_at)", r.PathValue("id"), u.ID)
		if e != nil {
			return e
		}
		reply(w, 200, map[string]string{"message": "领取成功，15 分钟内处理有效，可重复领取续期"})
		return nil
	})
}
func (s *Server) decide(w http.ResponseWriter, r *http.Request, u *Staff) error {
	var in struct {
		Decision, Reason string
		Advertising      bool `json:"advertising"`
	}
	if e := parse(r, &in); e != nil {
		return e
	}
	if len([]rune(in.Reason)) < 2 || len([]rune(in.Reason)) > 400 || in.Decision != "approve" && in.Decision != "reject" {
		return fail(400, "请选择结论并填写 2–400 字原因")
	}
	return s.withTaskLock(r, func() error {
		var owner string
		var exp time.Time
		if e := s.DB.QueryRowContext(r.Context(), "SELECT staff_id,expires_at FROM review_assignments WHERE task_id=?", r.PathValue("id")).Scan(&owner, &exp); e != nil {
			return fail(409, "请先领取任务")
		}
		if owner != u.ID || !exp.After(time.Now()) {
			return fail(409, "任务领取已失效，请重新领取")
		}
		if e := moderation.Decide(r.Context(), s.DB, r.PathValue("id"), u.Email, in.Decision, in.Reason, in.Advertising); e != nil {
			return fail(409, e.Error())
		}
		reply(w, 200, map[string]bool{"ok": true})
		return nil
	})
}
func rows(db *sql.DB, r *http.Request, q string, args ...any) ([]map[string]any, error) {
	rr, e := db.QueryContext(r.Context(), q, args...)
	if e != nil {
		return nil, e
	}
	defer rr.Close()
	cols, e := rr.Columns()
	if e != nil {
		return nil, e
	}
	items := []map[string]any{}
	for rr.Next() {
		v := make([]any, len(cols))
		p := make([]any, len(cols))
		for i := range v {
			p[i] = &v[i]
		}
		if e = rr.Scan(p...); e != nil {
			return nil, e
		}
		m := map[string]any{}
		for i, k := range cols {
			if b, ok := v[i].([]byte); ok {
				if strings.HasSuffix(k, "media_ids") || k == "opportunity" {
					var data any
					if json.Unmarshal(b, &data) == nil {
						v[i] = data
					} else {
						v[i] = string(b)
					}
				} else {
					v[i] = string(b)
				}
			}
			m[k] = v[i]
		}
		items = append(items, m)
	}
	return items, rr.Err()
}
func (s *Server) summary(w http.ResponseWriter, r *http.Request, u *Staff) error {
	result := map[string]int{}
	for k, q := range map[string]string{"pending": "SELECT COUNT(*) FROM review_tasks WHERE status='pending'", "decided": "SELECT COUNT(*) FROM review_tasks WHERE status='decided'", "reports": "SELECT (SELECT COUNT(*) FROM reports WHERE status='pending')+(SELECT COUNT(*) FROM comment_reports WHERE status='pending')", "appeals": "SELECT COUNT(*) FROM appeals WHERE status='pending'"} {
		var n int
		if e := s.DB.QueryRowContext(r.Context(), q).Scan(&n); e != nil {
			return e
		}
		result[k] = n
	}
	reply(w, 200, result)
	return nil
}
func (s *Server) tasks(w http.ResponseWriter, r *http.Request, u *Staff) error {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	if status != "pending" && status != "decided" && status != "cancelled" {
		return fail(400, "状态无效")
	}
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "post" && kind != "comment" && kind != "profile" {
		return fail(400, "类型无效")
	}
	items, e := s.list(r, "SELECT t.id,t.kind,t.status,t.reason,t.created_at,t.reviewer,t.decision,COALESCE(rv.title,LEFT(c.body,60),pc.nickname,'') AS title,u.nickname AS author,a.staff_id AS assigned_to,a.expires_at AS claim_expires_at FROM review_tasks t JOIN users u ON u.id=t.author_id LEFT JOIN revisions rv ON t.kind='post' AND rv.id=t.target_id LEFT JOIN comments c ON t.kind='comment' AND c.id=t.target_id LEFT JOIN profile_changes pc ON t.kind='profile' AND pc.id=t.target_id LEFT JOIN review_assignments a ON a.task_id=t.id WHERE t.status=? AND (?='' OR t.kind=?) ORDER BY t.created_at,t.id", status, kind, kind)
	if e != nil {
		return e
	}
	reply(w, 200, items)
	return nil
}
func (s *Server) task(w http.ResponseWriter, r *http.Request, u *Staff) error {
	items, e := rows(s.DB, r, "SELECT t.id,t.kind,t.status,t.reason,t.decision,t.created_at,t.decided_at,t.reviewer,t.author_id,u.nickname AS author,rv.post_id,rv.title,rv.body,rv.category,rv.media_ids,rv.opportunity,c.body AS comment_body,c.post_id AS comment_post_id,pr.title AS parent_title,pr.body AS parent_body,pr.media_ids AS parent_media_ids,pc.nickname AS profile_nickname,pc.bio AS profile_bio,pc.avatar_id AS profile_avatar,a.staff_id AS assigned_to,a.expires_at AS claim_expires_at FROM review_tasks t JOIN users u ON u.id=t.author_id LEFT JOIN revisions rv ON t.kind='post' AND rv.id=t.target_id LEFT JOIN comments c ON t.kind='comment' AND c.id=t.target_id LEFT JOIN profile_changes pc ON t.kind='profile' AND pc.id=t.target_id LEFT JOIN posts cp ON cp.id=c.post_id LEFT JOIN revisions pr ON pr.id=cp.published_revision_id LEFT JOIN review_assignments a ON a.task_id=t.id WHERE t.id=?", r.PathValue("id"))
	if e != nil {
		return e
	}
	if len(items) == 0 {
		return fail(404, "任务不存在")
	}
	reply(w, 200, items[0])
	return nil
}
func (s *Server) media(w http.ResponseWriter, r *http.Request, u *Staff) error {
	var mime string
	var b []byte
	if e := s.DB.QueryRowContext(r.Context(), "SELECT mime,body FROM media WHERE id=?", r.PathValue("id")).Scan(&mime, &b); e != nil {
		return e
	}
	w.Header().Set("Content-Type", mime)
	_, e := w.Write(b)
	return e
}
func (s *Server) reports(w http.ResponseWriter, r *http.Request, u *Staff) error {
	items, e := s.list(r, "SELECT x.* FROM (SELECT id,'post' AS kind,post_id AS target_id,reason,status,result,created_at FROM reports UNION ALL SELECT id,'comment' AS kind,comment_id AS target_id,reason,status,result,created_at FROM comment_reports) x ORDER BY created_at DESC,id DESC")
	if e != nil {
		return e
	}
	reply(w, 200, items)
	return nil
}
func (s *Server) resolveReport(w http.ResponseWriter, r *http.Request, u *Staff) error {
	var in struct{ Kind, Action, Reason string }
	if e := parse(r, &in); e != nil {
		return e
	}
	if len([]rune(in.Reason)) < 2 || len([]rune(in.Reason)) > 400 {
		return fail(400, "请填写处理原因")
	}
	var e error
	if in.Kind == "comment" {
		e = moderation.ResolveCommentReport(r.Context(), s.DB, r.PathValue("id"), u.Email, in.Action, in.Reason)
	} else if in.Kind == "post" {
		e = moderation.ResolveReport(r.Context(), s.DB, r.PathValue("id"), u.Email, in.Action, in.Reason)
	} else {
		return fail(400, "举报类型无效")
	}
	if e != nil {
		return fail(409, e.Error())
	}
	reply(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) appeals(w http.ResponseWriter, r *http.Request, u *Staff) error {
	items, e := s.list(r, "SELECT a.id,a.task_id,a.reason,a.status,a.created_at,t.kind,t.reason AS original_reason,t.reviewer AS original_reviewer,u.nickname AS author,c.reason AS result,c.action,c.new_task_id,c.reviewer AS resolved_by FROM appeals a JOIN review_tasks t ON t.id=a.task_id JOIN users u ON u.id=a.user_id LEFT JOIN case_resolutions c ON c.case_id=a.id AND c.kind='appeal' ORDER BY a.created_at DESC,a.id DESC")
	if e != nil {
		return e
	}
	reply(w, 200, items)
	return nil
}
func (s *Server) resolveAppeal(w http.ResponseWriter, r *http.Request, u *Staff) error {
	var in struct{ Reason, Action string }
	if e := parse(r, &in); e != nil {
		return e
	}
	if len([]rune(in.Reason)) < 2 || len([]rune(in.Reason)) > 400 {
		return fail(400, "请填写复核原因")
	}
	var reviewer sql.NullString
	if e := s.DB.QueryRowContext(r.Context(), "SELECT t.reviewer FROM appeals a JOIN review_tasks t ON t.id=a.task_id WHERE a.id=?", r.PathValue("id")).Scan(&reviewer); e != nil {
		return e
	}
	if reviewer.String == u.Email {
		return fail(403, "原审核员不得复核自己的决定")
	}
	if e := moderation.ResolveAppeal(r.Context(), s.DB, r.PathValue("id"), u.Email, in.Reason, in.Action); e != nil {
		return fail(409, e.Error())
	}
	reply(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) audit(w http.ResponseWriter, r *http.Request, u *Staff) error {
	items, e := s.list(r, "SELECT id,actor,action,target_id,reason,created_at FROM audit_log ORDER BY created_at DESC,id DESC")
	if e != nil {
		return e
	}
	reply(w, 200, items)
	return nil
}

func (s *Server) list(r *http.Request, q string, args ...any) (map[string]any, error) {
	page := 1
	if v := r.URL.Query().Get("page"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 100000 {
			return nil, fail(400, "页码无效")
		}
		page = n
	}
	args = append(args, 41, (page-1)*40)
	items, e := rows(s.DB, r, q+" LIMIT ? OFFSET ?", args...)
	if e != nil {
		return nil, e
	}
	more := len(items) > 40
	if more {
		items = items[:40]
	}
	return map[string]any{"items": items, "page": page, "has_more": more}, nil
}
func (s *Server) reportDetail(w http.ResponseWriter, r *http.Request, u *Staff) error {
	var items []map[string]any
	var e error
	switch r.URL.Query().Get("kind") {
	case "post":
		items, e = rows(s.DB, r, "SELECT rp.id,rp.reason,rp.status,rp.result,rp.created_at,p.id AS post_id,p.status AS current_status,p.published_revision_id AS current_revision_id,ev.revision_id AS evidence_revision_id,rv.title,rv.body,rv.category,rv.media_ids,rv.opportunity,u.nickname AS author,cr.action,cr.reviewer AS resolved_by FROM reports rp JOIN posts p ON p.id=rp.post_id JOIN users u ON u.id=p.author_id LEFT JOIN report_evidence ev ON ev.report_id=rp.id LEFT JOIN revisions rv ON rv.id=ev.revision_id LEFT JOIN case_resolutions cr ON cr.case_id=rp.id AND cr.kind='post_report' WHERE rp.id=?", r.PathValue("id"))
	case "comment":
		items, e = rows(s.DB, r, "SELECT rp.id,rp.reason,rp.status,rp.result,rp.created_at,c.post_id,c.id AS comment_id,c.body AS comment_body,c.status AS current_status,rv.title AS parent_title,rv.body AS parent_body,rv.media_ids,u.nickname AS author,cr.action,cr.reviewer AS resolved_by FROM comment_reports rp JOIN comments c ON c.id=rp.comment_id JOIN users u ON u.id=c.author_id JOIN posts p ON p.id=c.post_id LEFT JOIN revisions rv ON rv.id=p.published_revision_id LEFT JOIN case_resolutions cr ON cr.case_id=rp.id AND cr.kind='comment_report' WHERE rp.id=?", r.PathValue("id"))
	default:
		return fail(400, "举报类型无效")
	}
	if e != nil {
		return e
	}
	if len(items) == 0 {
		return fail(404, "举报不存在")
	}
	reply(w, 200, items[0])
	return nil
}
