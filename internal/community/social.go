package community

import (
	"bytes"
	"database/sql"
	"github.com/Xstly1014/Talko/internal/store"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	id, e := s.auth(r)
	if e != nil {
		return e
	}
	return s.sendProfile(w, r, id, true)
}
func (s *Server) userProfile(w http.ResponseWriter, r *http.Request) error {
	return s.sendProfile(w, r, r.PathValue("id"), false)
}
func (s *Server) sendProfile(w http.ResponseWriter, r *http.Request, id string, private bool) error {
	var nick, bio, avatar, email string
	e := s.DB.QueryRowContext(r.Context(), "SELECT nickname,bio,COALESCE(avatar_id,''),email FROM users WHERE id=?", id).Scan(&nick, &bio, &avatar, &email)
	if e != nil {
		return e
	}
	if !private {
		email = ""
	}
	var followers, following int
	if e = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM follows WHERE followee_id=?", id).Scan(&followers); e != nil {
		return e
	}
	if e = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM follows WHERE follower_id=?", id).Scan(&following); e != nil {
		return e
	}
	result := map[string]any{"id": id, "nickname": nick, "bio": bio, "avatar_id": avatar, "email": email, "followers": followers, "following": following}
	if private {
		var cid, nickname, bio, avatar, status, reason string
		e = s.DB.QueryRowContext(r.Context(), "SELECT id,nickname,bio,COALESCE(avatar_id,''),status,reason FROM profile_changes WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT 1", id).Scan(&cid, &nickname, &bio, &avatar, &status, &reason)
		if e == nil {
			result["profile_change"] = map[string]string{"id": cid, "nickname": nickname, "bio": bio, "avatar_id": avatar, "status": status, "reason": reason}
		} else if e != sql.ErrNoRows {
			return e
		}
	}
	writeJSON(w, 200, result)
	return nil
}
func (s *Server) profile(w http.ResponseWriter, r *http.Request) error {
	id, e := s.auth(r)
	if e != nil {
		return e
	}
	var in struct {
		Nickname, Bio string
		AvatarID      string `json:"avatar_id"`
	}
	if e = decode(r, &in); e != nil {
		return e
	}
	if len([]rune(strings.TrimSpace(in.Nickname))) < 1 || len([]rune(in.Nickname)) > 30 || len([]rune(in.Bio)) > 150 {
		return bad("昵称为 1–30 字，简介不超过 150 字")
	}
	if in.AvatarID != "" {
		var n int
		if e = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM media WHERE id=? AND user_id=?", in.AvatarID, id).Scan(&n); e != nil {
			return e
		}
		if n == 0 {
			return bad("头像不存在或不属于当前用户")
		}
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var lock string
	if e = tx.QueryRowContext(r.Context(), "SELECT id FROM users WHERE id=? FOR UPDATE", id).Scan(&lock); e != nil {
		return e
	}
	_, e = tx.ExecContext(r.Context(), "UPDATE review_tasks t JOIN profile_changes p ON p.id=t.target_id SET t.status='cancelled' WHERE t.kind='profile' AND p.user_id=? AND t.status='pending'", id)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(r.Context(), "UPDATE profile_changes SET status='superseded' WHERE user_id=? AND status='pending'", id)
	if e != nil {
		return e
	}
	change, task := store.ID(), store.ID()
	_, e = tx.ExecContext(r.Context(), "INSERT INTO profile_changes(id,user_id,nickname,bio,avatar_id) VALUES(?,?,?,?,NULLIF(?,''))", change, id, in.Nickname, in.Bio, in.AvatarID)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(r.Context(), "INSERT INTO review_tasks(id,kind,target_id,author_id) VALUES(?,'profile',?,?)", task, change, id)
	if e != nil {
		return e
	}
	if e = store.Event(r.Context(), tx, "review_requested", id, map[string]string{"task_id": task}); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	writeJSON(w, 202, map[string]string{"id": change, "task_id": task, "status": "pending"})
	return nil
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, 5<<20+1))
	if e != nil {
		return e
	}
	if len(b) > 5<<20 {
		return bad("图片不得超过 5 MB")
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || format != "jpeg" && format != "png" || cfg.Width > 6000 || cfg.Height > 6000 || cfg.Width*cfg.Height > 16000000 {
		return bad("仅支持最大 1600 万像素的 JPEG / PNG 图片")
	}
	img, _, e := image.Decode(bytes.NewReader(b))
	if e != nil {
		return bad("图片无法解码")
	}
	var clean bytes.Buffer
	if e = jpeg.Encode(&clean, img, &jpeg.Options{Quality: 88}); e != nil {
		return e
	}
	id := store.ID()
	_, e = s.DB.ExecContext(r.Context(), "INSERT INTO media(id,user_id,mime,body) VALUES(?,?,'image/jpeg',?)", id, user, clean.Bytes())
	if e != nil {
		return e
	}
	writeJSON(w, 201, map[string]string{"id": id})
	return nil
}
func (s *Server) media(w http.ResponseWriter, r *http.Request) error {
	user := s.optional(r)
	var owner, mime string
	var b []byte
	e := s.DB.QueryRowContext(r.Context(), "SELECT user_id,mime,body FROM media WHERE id=?", r.PathValue("id")).Scan(&owner, &mime, &b)
	if e != nil {
		return e
	}
	if user != owner {
		var n int
		e = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM posts p JOIN revisions r ON r.id=p.published_revision_id WHERE "+publicWhere()+" AND JSON_CONTAINS(r.media_ids,JSON_QUOTE(?))", r.PathValue("id")).Scan(&n)
		if e != nil {
			return e
		}
		if n == 0 {
			e = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users WHERE avatar_id=?", r.PathValue("id")).Scan(&n)
			if e != nil {
				return e
			}
			if n == 0 {
				return missing()
			}
		}
	}
	w.Header().Set("Content-Type", mime)
	_, e = w.Write(b)
	return e
}
func (s *Server) publicPost(r *http.Request, id string) error {
	var n int
	e := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM posts p JOIN revisions r ON r.id=p.published_revision_id WHERE p.id=? AND "+publicWhere(), id).Scan(&n)
	if e != nil {
		return e
	}
	if n == 0 {
		return missing()
	}
	return nil
}
func (s *Server) addComment(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	if e = s.publicPost(r, r.PathValue("id")); e != nil {
		return e
	}
	var in struct{ Body string }
	if e = decode(r, &in); e != nil {
		return e
	}
	if len([]rune(strings.TrimSpace(in.Body))) < 1 || len([]rune(in.Body)) > 1000 {
		return bad("评论须为 1–1000 字")
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 8 || len(key) > 80 {
		return bad("评论需要幂等键")
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var old, digest string
	e = tx.QueryRowContext(r.Context(), "SELECT resource_id,body_hash FROM idempotency WHERE user_id=? AND request_key=? AND operation='comment'", user, key).Scan(&old, &digest)
	hash := store.Hash(r.PathValue("id") + in.Body)
	if e == nil {
		if digest != hash {
			return conflict("幂等键冲突")
		}
		writeJSON(w, 200, map[string]string{"id": old, "status": "pending"})
		return nil
	}
	if e != sql.ErrNoRows {
		return e
	}
	id, task := store.ID(), store.ID()
	_, e = tx.ExecContext(r.Context(), "INSERT INTO comments(id,post_id,author_id,body) VALUES(?,?,?,?)", id, r.PathValue("id"), user, in.Body)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(r.Context(), "INSERT INTO review_tasks(id,kind,target_id,author_id) VALUES(?,'comment',?,?)", task, id, user)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(r.Context(), "INSERT INTO idempotency(user_id,request_key,operation,body_hash,resource_id) VALUES(?,?,'comment',?,?)", user, key, hash, id)
	if e != nil {
		return conflict("评论已提交，请重试查询")
	}
	if e = store.Event(r.Context(), tx, "review_requested", id, map[string]string{"task_id": task}); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	writeJSON(w, 202, map[string]string{"id": id, "status": "pending"})
	return nil
}
func (s *Server) comments(w http.ResponseWriter, r *http.Request) error {
	if e := s.publicPost(r, r.PathValue("id")); e != nil {
		return e
	}
	user := s.optional(r)
	page := 1
	if v := r.URL.Query().Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100000 {
			return bad("页码无效")
		}
		page = n
	}
	rows, e := s.DB.QueryContext(r.Context(), "SELECT c.id,c.author_id,u.nickname,c.body,c.status,c.created_at FROM comments c JOIN users u ON u.id=c.author_id WHERE c.post_id=? AND (c.status='published' OR (c.author_id=? AND c.status='pending')) ORDER BY c.created_at DESC,c.id DESC LIMIT 41 OFFSET ?", r.PathValue("id"), user, (page-1)*40)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, author, nick, body, status string
		var created any
		if e = rows.Scan(&id, &author, &nick, &body, &status, &created); e != nil {
			return e
		}
		items = append(items, map[string]any{"id": id, "author_id": author, "author": nick, "body": body, "status": status, "created_at": created})
	}
	if e = rows.Err(); e != nil {
		return e
	}
	more := len(items) > 40
	if more {
		items = items[:40]
	}
	writeJSON(w, 200, map[string]any{"items": items, "page": page, "has_more": more})
	return nil
}
func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	var post string
	if e = s.DB.QueryRowContext(r.Context(), "SELECT post_id FROM comments WHERE id=? AND author_id=?", r.PathValue("id"), user).Scan(&post); e != nil {
		return missing()
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var lock string
	if e = tx.QueryRowContext(r.Context(), "SELECT id FROM posts WHERE id=? FOR UPDATE", post).Scan(&lock); e != nil {
		return e
	}
	if _, e = tx.ExecContext(r.Context(), "UPDATE comments SET status='deleted' WHERE id=? AND author_id=?", r.PathValue("id"), user); e != nil {
		return e
	}
	if _, e = tx.ExecContext(r.Context(), "UPDATE review_tasks SET status='cancelled' WHERE kind='comment' AND target_id=? AND status='pending'", r.PathValue("id")); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) react(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	kind := r.PathValue("kind")
	if kind != "like" && kind != "favorite" {
		return bad("互动类型无效")
	}
	if e = s.publicPost(r, r.PathValue("id")); e != nil {
		return e
	}
	if r.Method == "PUT" {
		_, e = s.DB.ExecContext(r.Context(), "INSERT IGNORE INTO reactions(user_id,post_id,kind) VALUES(?,?,?)", user, r.PathValue("id"), kind)
	} else {
		_, e = s.DB.ExecContext(r.Context(), "DELETE FROM reactions WHERE user_id=? AND post_id=? AND kind=?", user, r.PathValue("id"), kind)
	}
	if e != nil {
		return e
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) follow(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	if user == r.PathValue("id") {
		return bad("不能关注自己")
	}
	var n int
	if e = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users WHERE id=?", r.PathValue("id")).Scan(&n); e != nil {
		return e
	}
	if n == 0 {
		return missing()
	}
	if r.Method == "PUT" {
		_, e = s.DB.ExecContext(r.Context(), "INSERT IGNORE INTO follows(follower_id,followee_id) VALUES(?,?)", user, r.PathValue("id"))
	} else {
		_, e = s.DB.ExecContext(r.Context(), "DELETE FROM follows WHERE follower_id=? AND followee_id=?", user, r.PathValue("id"))
	}
	if e != nil {
		return e
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) report(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	var in struct{ Reason string }
	if e = decode(r, &in); e != nil {
		return e
	}
	if len([]rune(in.Reason)) < 5 || len([]rune(in.Reason)) > 500 {
		return bad("举报说明须为 5–500 字")
	}
	if e = s.publicPost(r, r.PathValue("id")); e != nil {
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
	if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM reports WHERE user_id=? AND post_id=? AND status='pending'", user, r.PathValue("id")).Scan(&n); e != nil {
		return e
	}
	if n > 0 {
		return conflict("已有待处理举报，请勿重复提交")
	}
	var revision string
	if e = tx.QueryRowContext(r.Context(), "SELECT r.id FROM posts p JOIN revisions r ON r.id=p.published_revision_id WHERE p.id=? AND "+publicWhere()+" FOR SHARE", r.PathValue("id")).Scan(&revision); e != nil {
		return missing()
	}
	id := store.ID()
	_, e = tx.ExecContext(r.Context(), "INSERT INTO reports(id,user_id,post_id,reason) VALUES(?,?,?,?)", id, user, r.PathValue("id"), in.Reason)
	if e != nil {
		return e
	}
	if _, e = tx.ExecContext(r.Context(), "INSERT INTO report_evidence(report_id,revision_id) VALUES(?,?)", id, revision); e != nil {
		return e
	}
	if e = store.Event(r.Context(), tx, "report_submitted", r.PathValue("id"), map[string]string{"report_id": id}); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	writeJSON(w, 201, map[string]string{"id": id})
	return nil
}
func (s *Server) reports(w http.ResponseWriter, r *http.Request) error {
	return s.privateRows(w, r, "SELECT id,post_id,reason,status,result,created_at FROM reports WHERE user_id=? ORDER BY created_at DESC")
}
func (s *Server) appeals(w http.ResponseWriter, r *http.Request) error {
	return s.privateRows(w, r, "SELECT a.id,a.task_id,a.reason,a.status,a.created_at,c.reason AS result,c.action FROM appeals a LEFT JOIN case_resolutions c ON c.case_id=a.id AND c.kind='appeal' WHERE a.user_id=? ORDER BY a.created_at DESC")
}
func (s *Server) notifications(w http.ResponseWriter, r *http.Request) error {
	return s.privateRows(w, r, "SELECT id,message,post_id,read_at,created_at FROM notifications WHERE user_id=? ORDER BY created_at DESC LIMIT 100")
}
func (s *Server) privateRows(w http.ResponseWriter, r *http.Request, q string) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	page := 1
	if v := r.URL.Query().Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100000 {
			return bad("页码无效")
		}
		page = n
	}
	rows, e := s.DB.QueryContext(r.Context(), q+" LIMIT ? OFFSET ?", user, 41, (page-1)*40)
	if e != nil {
		return e
	}
	defer rows.Close()
	cols, e := rows.Columns()
	if e != nil {
		return e
	}
	items := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptr := make([]any, len(cols))
		for i := range vals {
			ptr[i] = &vals[i]
		}
		if e = rows.Scan(ptr...); e != nil {
			return e
		}
		v := map[string]any{}
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				vals[i] = string(b)
			}
			v[c] = vals[i]
		}
		items = append(items, v)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	more := len(items) > 40
	if more {
		items = items[:40]
	}
	writeJSON(w, 200, map[string]any{"items": items, "page": page, "has_more": more})
	return nil
}
func (s *Server) readNotification(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	res, e := s.DB.ExecContext(r.Context(), "UPDATE notifications SET read_at=COALESCE(read_at,UTC_TIMESTAMP(6)) WHERE id=? AND user_id=?", r.PathValue("id"), user)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return missing()
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) appeal(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	var in struct {
		TaskID string `json:"task_id"`
		Reason string
	}
	if e = decode(r, &in); e != nil {
		return e
	}
	if len([]rune(in.Reason)) < 5 || len([]rune(in.Reason)) > 1000 {
		return bad("申诉说明须为 5–1000 字")
	}
	var n int
	if e = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM review_tasks WHERE id=? AND author_id=? AND decision='reject'", in.TaskID, user).Scan(&n); e != nil {
		return e
	}
	if n == 0 {
		return missing()
	}
	id := store.ID()
	_, e = s.DB.ExecContext(r.Context(), "INSERT INTO appeals(id,user_id,task_id,reason) VALUES(?,?,?,?)", id, user, in.TaskID, in.Reason)
	if e != nil {
		return conflict("该审核决定已提交申诉")
	}
	writeJSON(w, 201, map[string]string{"id": id})
	return nil
}
