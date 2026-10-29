package community

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Xstly1014/Talko/internal/moderation"
	"github.com/Xstly1014/Talko/internal/store"
	"github.com/Xstly1014/Talko/internal/worker"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLifecycleMySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN required for real MySQL integration")
	}
	db, e := sql.Open("mysql", store.DSN(dsn))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	ctx := context.Background()
	if e = store.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	codes := map[string]string{}
	s := &Server{DB: db, Origin: "http://localhost:5173", Mail: func(email, code string) error { codes[email] = code; return nil }}
	h := s.Routes()
	ip := 0
	call := func(method, path string, body any, cookie, key string) (int, map[string]any, string) {
		t.Helper()
		var b []byte
		if body != nil {
			b, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(b))
		ip++
		req.RemoteAddr = fmt.Sprintf("192.0.%d.%d:1234", ip/250, ip%250)
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		var v map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		c := ""
		for _, cc := range w.Result().Cookies() {
			if cc.Name == "talko_session" {
				c = cc.Name + "=" + cc.Value
			}
		}
		return w.Code, v, c
	}
	expect := func(want, got int, v any) {
		t.Helper()
		if want != got {
			t.Fatalf("want %d got %d: %v", want, got, v)
		}
	}
	register := func() (string, string, string) {
		email := store.ID() + "@example.test"
		st, v, _ := call("POST", "/api/v1/auth/email-code", map[string]string{"email": email}, "", "")
		expect(202, st, v)
		st, v, c := call("POST", "/api/v1/auth/register", map[string]string{"email": email, "code": codes[email], "password": "correct-horse-battery", "nickname": "测试用户"}, "", "")
		expect(201, st, v)
		return v["id"].(string), c, email
	}
	a, ac, ae := register()
	_, bc, _ := register()
	var post, rev, task string
	key := store.ID()
	draft := map[string]any{"title": "实习记录", "body": "真实业务验收内容", "type": "note", "category": "实习就业", "media_ids": []string{}}
	t.Run("create_idempotency_and_private_draft", func(t *testing.T) {
		st, v, _ := call("POST", "/api/v1/posts", draft, ac, key)
		expect(201, st, v)
		post = v["id"].(string)
		rev = v["revision_id"].(string)
		st, v, _ = call("POST", "/api/v1/posts", draft, ac, key)
		expect(200, st, v)
		if v["id"] != post {
			t.Fatal("idempotency returned different post")
		}
		st, v, _ = call("GET", "/api/v1/posts/"+post, nil, "", "")
		expect(404, st, v)
		st, v, _ = call("GET", "/api/v1/posts/"+post+"?owner=1", nil, bc, "")
		expect(404, st, v)
	})
	t.Run("follow_before_publication", func(t *testing.T) {
		st, v, _ := call("PUT", "/api/v1/users/"+a+"/follow", nil, bc, "")
		expect(200, st, v)
	})
	t.Run("submit_and_self_review_denied", func(t *testing.T) {
		st, v, _ := call("POST", "/api/v1/posts/"+post+"/submit", map[string]string{"revision_id": rev}, ac, "")
		expect(202, st, v)
		task = v["task_id"].(string)
		if e = moderation.Decide(ctx, db, task, ae, "approve", "通过"); e == nil {
			t.Fatal("self review allowed")
		}
		if e = moderation.Decide(ctx, db, task, "reviewer@example.test", "approve", "人工核验通过"); e != nil {
			t.Fatal(e)
		}
		if e = moderation.Decide(ctx, db, task, "reviewer@example.test", "approve", "重复"); e != nil {
			t.Fatal(e)
		}
		for i := 0; i < 1000; i++ {
			ok, e := worker.Step(ctx, db)
			if e != nil {
				t.Fatal(e)
			}
			if !ok {
				break
			}
		}
	})
	t.Run("public_and_distributed_feed", func(t *testing.T) {
		st, v, _ := call("GET", "/api/v1/posts/"+post, nil, "", "")
		expect(200, st, v)
		st, v, _ = call("GET", "/api/v1/feed?following=1", nil, bc, "")
		expect(200, st, v)
		if !strings.Contains(fmt.Sprint(v), post) {
			t.Fatal("published post missing from follower feed")
		}
	})
	var comment string
	t.Run("pending_comment_visibility_and_moderation", func(t *testing.T) {
		st, v, _ := call("POST", "/api/v1/posts/"+post+"/comments", map[string]string{"body": "这是一条待审核评论"}, bc, store.ID())
		expect(202, st, v)
		comment = v["id"].(string)
		st, v, _ = call("GET", "/api/v1/posts/"+post+"/comments", nil, "", "")
		expect(200, st, v)
		if strings.Contains(fmt.Sprint(v), comment) {
			t.Fatal("pending comment public")
		}
		var ct string
		if e = db.QueryRow("SELECT id FROM review_tasks WHERE target_id=?", comment).Scan(&ct); e != nil {
			t.Fatal(e)
		}
		if e = moderation.Decide(ctx, db, ct, "reviewer@example.test", "approve", "评论通过"); e != nil {
			t.Fatal(e)
		}
		st, v, _ = call("GET", "/api/v1/posts/"+post+"/comments", nil, "", "")
		expect(200, st, v)
		if !strings.Contains(fmt.Sprint(v), comment) {
			t.Fatal("approved comment missing")
		}
	})
	t.Run("withdraw_blocks_stale_decision", func(t *testing.T) {
		draft["revision_id"] = rev
		draft["title"] = "修改后的内容"
		st, v, _ := call("PATCH", "/api/v1/posts/"+post+"/draft", draft, ac, "")
		expect(200, st, v)
		rev = v["revision_id"].(string)
		st, v, _ = call("POST", "/api/v1/posts/"+post+"/submit", map[string]string{"revision_id": rev}, ac, "")
		expect(202, st, v)
		newTask := v["task_id"].(string)
		st, v, _ = call("POST", "/api/v1/posts/"+post+"/withdraw", nil, ac, "")
		expect(200, st, v)
		if e = moderation.Decide(ctx, db, newTask, "reviewer@example.test", "approve", "旧版本"); e == nil {
			t.Fatal("cancelled task approved")
		}
		st, v, _ = call("GET", "/api/v1/posts/"+post, nil, "", "")
		expect(200, st, v)
		if v["title"] != "实习记录" {
			t.Fatal("old approved revision overwritten")
		}
	})
	t.Run("report_hide_and_filter_every_public_read", func(t *testing.T) {
		st, v, _ := call("POST", "/api/v1/posts/"+post+"/reports", map[string]string{"reason": "需要核查信息来源"}, bc, "")
		expect(201, st, v)
		report := v["id"].(string)
		st, v, _ = call("POST", "/api/v1/posts/"+post+"/reports", map[string]string{"reason": "再次核查信息来源"}, bc, "")
		expect(409, st, v)
		if e = moderation.ResolveReport(ctx, db, report, "risk@example.test", "hide", "风险核查下架"); e != nil {
			t.Fatal(e)
		}
		st, v, _ = call("GET", "/api/v1/posts/"+post, nil, "", "")
		expect(404, st, v)
		st, v, _ = call("GET", "/api/v1/feed?following=1", nil, bc, "")
		expect(200, st, v)
		if strings.Contains(fmt.Sprint(v), post) {
			t.Fatal("hidden post leaked")
		}
		st, v, _ = call("GET", "/api/v1/posts/"+post+"/comments", nil, "", "")
		expect(404, st, v)
	})

	t.Run("media_private_until_review_and_hidden_after_removal", func(t *testing.T) {
		var raw bytes.Buffer
		_ = png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 2, 2)))
		req := httptest.NewRequest("POST", "/api/v1/media", &raw)
		req.Header.Set("Cookie", ac)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		expect(201, w.Code, w.Body.String())
		var result map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		mid := result["id"].(string)
		st, v, _ := call("GET", "/api/v1/media/"+mid, nil, "", "")
		expect(404, st, v)
		d := map[string]any{"title": "带图内容", "body": "测试媒体访问控制", "category": "校园生活", "type": "note", "media_ids": []string{mid}}
		st, v, _ = call("POST", "/api/v1/posts", d, ac, store.ID())
		expect(201, st, v)
		pid := v["id"].(string)
		rid := v["revision_id"].(string)
		st, v, _ = call("POST", "/api/v1/posts/"+pid+"/submit", map[string]string{"revision_id": rid}, ac, "")
		expect(202, st, v)
		if e = moderation.Decide(ctx, db, v["task_id"].(string), "reviewer@example.test", "approve", "图片人工检查通过"); e != nil {
			t.Fatal(e)
		}
		st, v, _ = call("GET", "/api/v1/media/"+mid, nil, "", "")
		expect(200, st, v)
		st, v, _ = call("DELETE", "/api/v1/posts/"+pid, nil, ac, "")
		expect(200, st, v)
		st, v, _ = call("GET", "/api/v1/media/"+mid, nil, "", "")
		expect(404, st, v)
	})
	t.Run("expired_opportunity_hidden_and_risk_disclosure_present", func(t *testing.T) {
		d := map[string]any{"title": "机会信息测试", "body": "兼职报酬信息", "category": "兼职家教", "type": "opportunity", "media_ids": []string{}, "opportunity": map[string]string{"publisher": "个人发布者", "city": "深圳", "compensation": "每小时 50 元", "settlement": "按日", "contact": "请在约定渠道核验", "expires_at": time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)}}
		st, v, _ := call("POST", "/api/v1/posts", d, ac, store.ID())
		expect(201, st, v)
		pid, rid := v["id"].(string), v["revision_id"].(string)
		st, v, _ = call("POST", "/api/v1/posts/"+pid+"/submit", map[string]string{"revision_id": rid}, ac, "")
		expect(202, st, v)
		if e = moderation.Decide(ctx, db, v["task_id"].(string), "reviewer@example.test", "approve", "人工检查"); e != nil {
			t.Fatal(e)
		}
		st, v, _ = call("GET", "/api/v1/posts/"+pid, nil, "", "")
		expect(200, st, v)
		if v["risk_disclosure"] == "" {
			t.Fatal("money risk disclosure missing")
		}
		_, e = db.Exec("UPDATE revisions SET opportunity=JSON_SET(opportunity,'$.expires_at','2000-01-01T00:00:00Z') WHERE id=?", rid)
		if e != nil {
			t.Fatal(e)
		}
		st, v, _ = call("GET", "/api/v1/posts/"+pid, nil, "", "")
		expect(404, st, v)
	})

	t.Run("origin_and_unknown_fields", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader("{}"))
		req.Header.Set("Origin", "https://evil.test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatal("origin check missing")
		}
		st, v, _ := call("POST", "/api/v1/auth/login", map[string]string{"unknown": "field"}, "", "")
		expect(400, st, v)
	})
	_ = time.Second
	_ = http.MethodGet
}
func TestDraftRules(t *testing.T) {
	d := Draft{Title: "test", Body: "body", Category: "校园生活", Type: "note"}
	if e := validDraft(d); e != nil {
		t.Fatal(e)
	}
	d.Type = "opportunity"
	if e := validDraft(d); e == nil {
		t.Fatal("missing opportunity accepted")
	}
}
