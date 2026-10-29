package community

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"github.com/Xstly1014/Talko/internal/store"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Opportunity struct {
	Publisher    string `json:"publisher"`
	City         string `json:"city"`
	Compensation string `json:"compensation"`
	Settlement   string `json:"settlement"`
	ExpiresAt    string `json:"expires_at"`
	Contact      string `json:"contact"`
}
type Draft struct {
	Title       string       `json:"title"`
	Body        string       `json:"body"`
	Category    string       `json:"category"`
	MediaIDs    []string     `json:"media_ids"`
	Type        string       `json:"type"`
	RevisionID  string       `json:"revision_id"`
	Opportunity *Opportunity `json:"opportunity"`
}
type Post struct {
	Advertising    bool         `json:"advertising"`
	ID             string       `json:"id"`
	AuthorID       string       `json:"author_id"`
	Author         string       `json:"author"`
	AvatarID       string       `json:"avatar_id"`
	RevisionID     string       `json:"revision_id"`
	Type           string       `json:"type"`
	Status         string       `json:"status"`
	Title          string       `json:"title"`
	Body           string       `json:"body"`
	Category       string       `json:"category"`
	MediaIDs       []string     `json:"media_ids"`
	Opportunity    *Opportunity `json:"opportunity,omitempty"`
	RiskDisclosure string       `json:"risk_disclosure,omitempty"`
	Reason         string       `json:"reason"`
	TaskID         string       `json:"task_id"`
	PublishedAt    *time.Time   `json:"published_at"`
	Likes          int          `json:"likes"`
	Favorites      int          `json:"favorites"`
	Liked          bool         `json:"liked"`
	Saved          bool         `json:"saved"`
	Following      bool         `json:"following"`
}

func validDraft(d Draft) error {
	if len([]rune(strings.TrimSpace(d.Title))) < 1 || len([]rune(d.Title)) > 60 || len([]rune(strings.TrimSpace(d.Body))) < 1 || len([]rune(d.Body)) > 10000 {
		return bad("标题须为 1–60 字，正文须为 1–10000 字")
	}
	if len(d.MediaIDs) > 6 {
		return bad("最多上传 6 张图片")
	}
	if !strings.Contains("|实习就业|升学考试|出国交流|兼职家教|校园生活|", "|"+d.Category+"|") {
		return bad("请选择有效分类")
	}
	if d.Type != "note" && d.Type != "opportunity" {
		return bad("内容类型无效")
	}
	if d.Type == "opportunity" {
		if d.Opportunity == nil {
			return bad("请填写机会信息")
		}
		o := d.Opportunity
		if o.Publisher == "" || o.City == "" || o.Compensation == "" || o.Settlement == "" || o.Contact == "" || len([]rune(o.Publisher+o.City+o.Compensation+o.Settlement+o.Contact)) > 1000 {
			return bad("机会信息不完整或过长")
		}
		t, e := time.Parse(time.RFC3339, o.ExpiresAt)
		if e != nil || !t.After(time.Now()) || t.After(time.Now().AddDate(0, 3, 0)) {
			return bad("机会有效期须在未来 3 个月内")
		}
		o.ExpiresAt = t.UTC().Format(time.RFC3339)
	}
	return nil
}
func (s *Server) validateMedia(r *http.Request, tx *sql.Tx, user string, ids []string) error {
	for _, id := range ids {
		var n int
		if e := tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM media WHERE id=? AND user_id=?", id, user).Scan(&n); e != nil {
			return e
		}
		if n != 1 {
			return bad("图片不存在或不属于当前用户")
		}
	}
	return nil
}
func (s *Server) createPost(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	var d Draft
	if e = decode(r, &d); e != nil {
		return e
	}
	if e = validDraft(d); e != nil {
		return e
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 8 || len(key) > 80 {
		return bad("请提供 8–80 位 Idempotency-Key")
	}
	b, _ := json.Marshal(d)
	digest := store.Hash(string(b))
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var oldID, oldHash string
	e = tx.QueryRowContext(r.Context(), "SELECT resource_id,body_hash FROM idempotency WHERE user_id=? AND request_key=? AND operation='create_post'", user, key).Scan(&oldID, &oldHash)
	if e == nil {
		if oldHash != digest {
			return conflict("同一幂等键对应不同请求")
		}
		writeJSON(w, 200, map[string]string{"id": oldID})
		return nil
	}
	if e != sql.ErrNoRows {
		return e
	}
	if e = s.validateMedia(r, tx, user, d.MediaIDs); e != nil {
		return e
	}
	id, rev := store.ID(), store.ID()
	_, e = tx.ExecContext(r.Context(), "INSERT INTO posts(id,author_id,type,draft_revision_id) VALUES(?,?,?,?)", id, user, d.Type, rev)
	if e != nil {
		return e
	}
	if e = insertRevision(r, tx, id, rev, d); e != nil {
		return e
	}
	_, e = tx.ExecContext(r.Context(), "INSERT INTO idempotency(user_id,request_key,operation,body_hash,resource_id) VALUES(?,?,'create_post',?,?)", user, key, digest, id)
	if e != nil {
		return conflict("请求已提交，请重试读取结果")
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	writeJSON(w, 201, map[string]string{"id": id, "revision_id": rev})
	return nil
}
func insertRevision(r *http.Request, tx *sql.Tx, id, rev string, d Draft) error {
	if d.MediaIDs == nil {
		d.MediaIDs = []string{}
	}
	media, _ := json.Marshal(d.MediaIDs)
	var opp any
	if d.Opportunity != nil {
		b, _ := json.Marshal(d.Opportunity)
		opp = string(b)
	}
	_, e := tx.ExecContext(r.Context(), "INSERT INTO revisions(id,post_id,title,body,category,media_ids,opportunity) VALUES(?,?,?,?,?,?,?)", rev, id, d.Title, d.Body, d.Category, media, opp)
	return e
}
func (s *Server) editPost(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	var d Draft
	if e = decode(r, &d); e != nil {
		return e
	}
	if e = validDraft(d); e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var author, typ, status string
	var current sql.NullString
	e = tx.QueryRowContext(r.Context(), "SELECT author_id,type,status,draft_revision_id FROM posts WHERE id=? FOR UPDATE", r.PathValue("id")).Scan(&author, &typ, &status, &current)
	if e != nil {
		return e
	}
	if author != user {
		return missing()
	}
	if current.String != d.RevisionID {
		return conflict("内容已更新，请刷新后重试")
	}
	if status == "deleted" || status == "hidden" {
		return conflict("当前内容不可编辑")
	}
	if typ != d.Type {
		return bad("不可改变内容类型")
	}
	if current.Valid {
		var state string
		e = tx.QueryRowContext(r.Context(), "SELECT state FROM revisions WHERE id=?", current.String).Scan(&state)
		if e != nil {
			return e
		}
		if state == "pending" {
			return conflict("请先撤回审核再修改")
		}
	}
	if e = s.validateMedia(r, tx, user, d.MediaIDs); e != nil {
		return e
	}
	rev := store.ID()
	if e = insertRevision(r, tx, r.PathValue("id"), rev, d); e != nil {
		return e
	}
	_, e = tx.ExecContext(r.Context(), "UPDATE posts SET draft_revision_id=? WHERE id=?", rev, r.PathValue("id"))
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id"), "revision_id": rev})
	return nil
}
func (s *Server) submitPost(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	var in struct {
		RevisionID string `json:"revision_id"`
	}
	if e = decode(r, &in); e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var author, status string
	var rev sql.NullString
	e = tx.QueryRowContext(r.Context(), "SELECT author_id,status,draft_revision_id FROM posts WHERE id=? FOR UPDATE", r.PathValue("id")).Scan(&author, &status, &rev)
	if e != nil {
		return e
	}
	if author != user {
		return missing()
	}
	if rev.String != in.RevisionID || !rev.Valid || status == "deleted" || status == "hidden" {
		return conflict("内容版本或状态已变化")
	}
	var task, taskStatus string
	e = tx.QueryRowContext(r.Context(), "SELECT id,status FROM review_tasks WHERE target_id=? ORDER BY created_at DESC,id DESC LIMIT 1", rev.String).Scan(&task, &taskStatus)
	if e == nil {
		if taskStatus != "pending" {
			return conflict("该版本已处理，请修改后重新提交")
		}
		writeJSON(w, 202, map[string]string{"task_id": task})
		return nil
	}
	if e != sql.ErrNoRows {
		return e
	}
	task = store.ID()
	_, e = tx.ExecContext(r.Context(), "UPDATE revisions SET state='pending' WHERE id=? AND state='draft'", rev.String)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(r.Context(), "INSERT INTO review_tasks(id,kind,target_id,author_id) VALUES(?,'post',?,?)", task, rev.String, user)
	if e != nil {
		return e
	}
	if e = store.Event(r.Context(), tx, "review_requested", r.PathValue("id"), map[string]string{"task_id": task, "revision_id": rev.String}); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	writeJSON(w, 202, map[string]string{"task_id": task})
	return nil
}
func (s *Server) withdraw(w http.ResponseWriter, r *http.Request) error {
	return s.changePost(w, r, false)
}
func (s *Server) deletePost(w http.ResponseWriter, r *http.Request) error {
	return s.changePost(w, r, true)
}
func (s *Server) changePost(w http.ResponseWriter, r *http.Request, del bool) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var author, status string
	var rev sql.NullString
	e = tx.QueryRowContext(r.Context(), "SELECT author_id,status,draft_revision_id FROM posts WHERE id=? FOR UPDATE", r.PathValue("id")).Scan(&author, &status, &rev)
	if e != nil {
		return e
	}
	if author != user {
		return missing()
	}
	if !del {
		var state string
		if !rev.Valid {
			return conflict("没有待审核版本")
		}
		if e = tx.QueryRowContext(r.Context(), "SELECT state FROM revisions WHERE id=?", rev.String).Scan(&state); e != nil {
			return e
		}
		if state != "pending" {
			return conflict("当前版本不在审核中")
		}
	}
	if rev.Valid {
		_, e = tx.ExecContext(r.Context(), "UPDATE review_tasks SET status='cancelled' WHERE target_id=? AND status='pending'", rev.String)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(r.Context(), "UPDATE revisions SET state='withdrawn' WHERE id=? AND state='pending'", rev.String)
		if e != nil {
			return e
		}
	}
	if del {
		if _, e = tx.ExecContext(r.Context(), "UPDATE review_tasks t JOIN comments c ON c.id=t.target_id SET t.status='cancelled' WHERE t.kind='comment' AND c.post_id=? AND t.status='pending'", r.PathValue("id")); e != nil {
			return e
		}
		_, e = tx.ExecContext(r.Context(), "UPDATE posts SET status='deleted' WHERE id=?", r.PathValue("id"))
		if e != nil {
			return e
		}
		if e = store.Event(r.Context(), tx, "post_hidden", r.PathValue("id"), map[string]string{"author_id": user}); e != nil {
			return e
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
	return nil
}

const postSelect = "SELECT p.id,p.author_id,u.nickname,COALESCE(u.avatar_id,''),p.type,p.status,r.id,r.title,r.body,r.category,r.media_ids,r.opportunity,r.reason,p.published_at,COALESCE((SELECT id FROM review_tasks WHERE target_id=r.id ORDER BY created_at DESC,id DESC LIMIT 1),'') FROM posts p JOIN users u ON u.id=p.author_id JOIN revisions r ON r.id="

func scanPost(row interface{ Scan(...any) error }) (Post, error) {
	var p Post
	var media []byte
	var opp []byte
	e := row.Scan(&p.ID, &p.AuthorID, &p.Author, &p.AvatarID, &p.Type, &p.Status, &p.RevisionID, &p.Title, &p.Body, &p.Category, &media, &opp, &p.Reason, &p.PublishedAt, &p.TaskID)
	if e != nil {
		return p, e
	}
	if e = json.Unmarshal(media, &p.MediaIDs); e != nil {
		return p, e
	}
	if len(opp) > 0 && string(opp) != "null" {
		if e = json.Unmarshal(opp, &p.Opportunity); e != nil {
			return p, e
		}
		p.RiskDisclosure = "信息来源未核实，请谨慎甄别。请勿预付押金或泄露证件信息。"
	}
	return p, nil
}
func publicWhere() string {
	return "p.status='published' AND (r.opportunity IS NULL OR CAST(JSON_UNQUOTE(JSON_EXTRACT(r.opportunity,'$.expires_at')) AS CHAR)>DATE_FORMAT(UTC_TIMESTAMP(),'%Y-%m-%dT%H:%i:%sZ'))"
}
func (s *Server) getPost(w http.ResponseWriter, r *http.Request) error {
	user := s.optional(r)
	expr := "p.published_revision_id"
	owner := r.URL.Query().Get("owner") == "1"
	if owner {
		if user == "" {
			return missing()
		}
		expr = "p.draft_revision_id"
	}
	q := postSelect + expr + " WHERE p.id=? AND "
	args := []any{r.PathValue("id")}
	if owner {
		q += "p.author_id=? AND p.status<>'deleted'"
		args = append(args, user)
	} else {
		q += publicWhere()
	}
	p, e := scanPost(s.DB.QueryRowContext(r.Context(), q, args...))
	if e != nil {
		return e
	}
	if owner {
		var st string
		if e = s.DB.QueryRowContext(r.Context(), "SELECT state FROM revisions WHERE id=?", p.RevisionID).Scan(&st); e != nil {
			return e
		}
		p.Status = st
	}
	if e = s.enrich(r, &p, user); e != nil {
		return e
	}
	writeJSON(w, 200, p)
	return nil
}
func (s *Server) enrich(r *http.Request, p *Post, user string) error {
	if e := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM reactions WHERE post_id=? AND kind='like'", p.ID).Scan(&p.Likes); e != nil {
		return e
	}
	if e := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM reactions WHERE post_id=? AND kind='favorite'", p.ID).Scan(&p.Favorites); e != nil {
		return e
	}
	var advertising bool
	if e := s.DB.QueryRowContext(r.Context(), "SELECT COALESCE((SELECT is_advertising FROM content_disclosures WHERE post_id=?),FALSE)", p.ID).Scan(&advertising); e != nil {
		return e
	}
	p.Advertising = advertising
	if user != "" {
		var a, b, c int
		if e := s.DB.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM reactions WHERE user_id=? AND post_id=? AND kind='like'),EXISTS(SELECT 1 FROM reactions WHERE user_id=? AND post_id=? AND kind='favorite'),EXISTS(SELECT 1 FROM follows WHERE follower_id=? AND followee_id=?)", user, p.ID, user, p.ID, user, p.AuthorID).Scan(&a, &b, &c); e != nil {
			return e
		}
		p.Liked = a > 0
		p.Saved = b > 0
		p.Following = c > 0
	}
	return nil
}
func (s *Server) feed(w http.ResponseWriter, r *http.Request) error {
	user := s.optional(r)
	q := postSelect + "p.published_revision_id WHERE " + publicWhere()
	args := []any{}
	v := r.URL.Query()
	if v.Get("following") == "1" {
		if user == "" {
			return &apiError{401, "unauthenticated", "请先登录"}
		}
		q += " AND EXISTS(SELECT 1 FROM follows f JOIN feed_items fi ON fi.user_id=f.follower_id AND fi.post_id=p.id WHERE f.follower_id=? AND f.followee_id=p.author_id AND f.created_at<=p.published_at)"
		args = append(args, user)
	}
	if v.Get("saved") == "1" {
		if user == "" {
			return &apiError{401, "unauthenticated", "请先登录"}
		}
		q += " AND EXISTS(SELECT 1 FROM reactions x WHERE x.user_id=? AND x.post_id=p.id AND x.kind='favorite')"
		args = append(args, user)
	}
	for key, col := range map[string]string{"category": "r.category", "author": "p.author_id", "type": "p.type"} {
		if v.Get(key) != "" {
			q += " AND " + col + "=?"
			args = append(args, v.Get(key))
		}
	}
	if len(v.Get("q")) > 200 {
		return bad("搜索词过长")
	}
	if v.Get("q") != "" {
		q += " AND (r.title LIKE ? OR r.body LIKE ?)"
		args = append(args, "%"+v.Get("q")+"%", "%"+v.Get("q")+"%")
	}
	if v.Get("cursor") != "" {
		b, e := base64.RawURLEncoding.DecodeString(v.Get("cursor"))
		if e != nil {
			return bad("分页游标无效")
		}
		parts := strings.SplitN(string(b), "|", 2)
		if len(parts) != 2 {
			return bad("分页游标无效")
		}
		t, e := time.Parse(time.RFC3339Nano, parts[0])
		if e != nil {
			return bad("分页游标无效")
		}
		q += " AND (p.published_at<? OR (p.published_at=? AND p.id<?))"
		args = append(args, t, t, parts[1])
	}
	limit := 20
	if v.Get("limit") != "" {
		n, e := strconv.Atoi(v.Get("limit"))
		if e != nil || n < 1 || n > 50 {
			return bad("分页大小应在 1–50 之间")
		}
		limit = n
	}
	q += " ORDER BY p.published_at DESC,p.id DESC LIMIT ?"
	args = append(args, limit+1)
	rows, e := s.DB.QueryContext(r.Context(), q, args...)
	if e != nil {
		return e
	}
	items := []Post{}
	for rows.Next() {
		p, e := scanPost(rows)
		if e != nil {
			rows.Close()
			return e
		}
		items = append(items, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		p := items[len(items)-1]
		next = base64.RawURLEncoding.EncodeToString([]byte(p.PublishedAt.Format(time.RFC3339Nano) + "|" + p.ID))
	}
	for i := range items {
		if e = s.enrich(r, &items[i], user); e != nil {
			return e
		}
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
	return nil
}
func (s *Server) myPosts(w http.ResponseWriter, r *http.Request) error {
	user, e := s.auth(r)
	if e != nil {
		return e
	}
	rows, e := s.DB.QueryContext(r.Context(), postSelect+"p.draft_revision_id WHERE p.author_id=? AND p.status<>'deleted' ORDER BY p.created_at DESC LIMIT 100", user)
	if e != nil {
		return e
	}
	items := []Post{}
	for rows.Next() {
		p, e := scanPost(rows)
		if e != nil {
			rows.Close()
			return e
		}
		items = append(items, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for i := range items {
		if e = s.DB.QueryRowContext(r.Context(), "SELECT state FROM revisions WHERE id=?", items[i].RevisionID).Scan(&items[i].Status); e != nil {
			return e
		}
	}
	writeJSON(w, 200, map[string]any{"items": items})
	return nil
}
