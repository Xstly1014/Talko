package community

import (
	"github.com/Xstly1014/Talko/internal/store"
	"net/http"
)

func (s *Server) reportComment(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	var in struct{ Reason string }
	if e = decode(r, &in); e != nil {
		return e
	}
	if len([]rune(in.Reason)) < 5 || len([]rune(in.Reason)) > 500 {
		return bad("举报原因须为 5–500 字")
	}
	var post string
	e = s.DB.QueryRowContext(r.Context(), "SELECT post_id FROM comments WHERE id=? AND status='published'", r.PathValue("id")).Scan(&post)
	if e != nil {
		return missing()
	}
	if e = s.publicPost(r, post); e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var lock string
	if e = tx.QueryRowContext(r.Context(), "SELECT id FROM users WHERE id=? FOR UPDATE", user).Scan(&lock); e != nil {
		return e
	}
	var n int
	if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM comment_reports WHERE user_id=? AND comment_id=? AND status='pending'", user, r.PathValue("id")).Scan(&n); e != nil {
		return e
	}
	if n > 0 {
		return conflict("已有待处理举报")
	}
	id := store.ID()
	_, e = tx.ExecContext(r.Context(), "INSERT INTO comment_reports(id,user_id,comment_id,reason) VALUES(?,?,?,?)", id, user, r.PathValue("id"), in.Reason)
	if e != nil {
		return e
	}
	if e = store.Event(r.Context(), tx, "report_submitted", post, map[string]string{"comment_report_id": id}); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	writeJSON(w, 201, map[string]string{"id": id})
	return nil
}
func (s *Server) commentReports(w http.ResponseWriter, r *http.Request) error {
	return s.privateRows(w, r, "SELECT cr.id,c.post_id,cr.reason,cr.status,cr.result,cr.created_at FROM comment_reports cr JOIN comments c ON c.id=cr.comment_id WHERE cr.user_id=? ORDER BY cr.created_at DESC")
}

func (s *Server) myReviews(w http.ResponseWriter, r *http.Request) error {
	return s.privateRows(w, r, "SELECT id,id AS task_id,kind,status,decision,reason,created_at FROM review_tasks WHERE author_id=? ORDER BY created_at DESC,id DESC")
}
