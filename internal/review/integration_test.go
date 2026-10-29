package review_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Xstly1014/Talko/internal/community"
	"github.com/Xstly1014/Talko/internal/review"
	"github.com/Xstly1014/Talko/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReviewBoundaryAndGovernance(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN required")
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
	app := &review.Server{DB: db, Origin: "http://localhost:5174"}
	h := app.Routes()
	communityApp := (&community.Server{DB: db, Origin: "http://localhost:5173"}).Routes()
	counter := 0
	call := func(handler http.Handler, method, path string, body any, cookie string) (int, map[string]any, string) {
		t.Helper()
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		counter++
		r.RemoteAddr = fmt.Sprintf("10.0.0.%d:1234", counter%250)
		r.Header.Set("Idempotency-Key", store.ID())
		if cookie != "" {
			r.Header.Set("Cookie", cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		v := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		c := ""
		for _, cc := range w.Result().Cookies() {
			c = cc.Name + "=" + cc.Value
		}
		return w.Code, v, c
	}
	expect := func(want, got int, v any) {
		t.Helper()
		if want != got {
			t.Fatalf("want %d got %d: %v", want, got, v)
		}
	}
	makeStaff := func(role string) string {
		email := store.ID() + "@review.test"
		if e := review.CreateStaff(ctx, db, email, "test-staff-password-2026", role); e != nil {
			t.Fatal(e)
		}
		st, v, c := call(h, "POST", "/review-api/v1/auth/login", map[string]string{"email": email, "password": "test-staff-password-2026"}, "")
		expect(200, st, v)
		return c
	}
	admin := makeStaff("admin")
	first := makeStaff("reviewer")
	second := makeStaff("reviewer")
	uid := store.ID()
	_, e = db.Exec("INSERT INTO users(id,email,nickname,password_hash) VALUES(?,?,?,?)", uid, store.ID()+"@student.test", "原昵称", []byte("not-used"))
	if e != nil {
		t.Fatal(e)
	}
	token := store.ID()
	_, e = db.Exec("INSERT INTO sessions(token_hash,user_id,expires_at) VALUES(?,?,?)", store.Hash(token), uid, time.Now().UTC().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	cc := "talko_session=" + token
	var pid, task string
	t.Run("session_separation_and_role_authorization", func(t *testing.T) {
		st, v, _ := call(h, "GET", "/review-api/v1/tasks", nil, cc)
		expect(401, st, v)
		st, v, _ = call(h, "GET", "/review-api/v1/reports", nil, first)
		expect(403, st, v)
		st, v, _ = call(h, "GET", "/review-api/v1/audit", nil, first)
		expect(403, st, v)
		st, v, _ = call(h, "GET", "/review-api/v1/audit", nil, admin)
		expect(200, st, v)
	})
	t.Run("claim_conflict_and_audited_decision", func(t *testing.T) {
		st, v, _ := call(communityApp, "POST", "/api/v1/posts", map[string]any{"title": "审核服务验收", "body": "核查独立审核业务", "category": "校园生活", "type": "note", "media_ids": []string{}}, cc)
		expect(201, st, v)
		pid = v["id"].(string)
		st, v, _ = call(communityApp, "POST", "/api/v1/posts/"+pid+"/submit", map[string]string{"revision_id": v["revision_id"].(string)}, cc)
		expect(202, st, v)
		task = v["task_id"].(string)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+task+"/decision", map[string]any{"decision": "approve", "reason": "未领取任务测试"}, first)
		expect(409, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+task+"/claim", nil, first)
		expect(200, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+task+"/claim", nil, second)
		expect(409, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+task+"/decision", map[string]any{"decision": "approve", "reason": "核查通过", "advertising": true}, second)
		expect(409, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+task+"/decision", map[string]any{"decision": "approve", "reason": "核查通过", "advertising": true}, first)
		expect(200, st, v)
		st, v, _ = call(communityApp, "GET", "/api/v1/posts/"+pid, nil, "")
		expect(200, st, v)
		if v["advertising"] != true {
			t.Fatal("advertising disclosure not persisted")
		}
	})
	t.Run("profile_changes_not_public_until_approved", func(t *testing.T) {
		st, v, _ := call(communityApp, "PATCH", "/api/v1/me/profile", map[string]string{"nickname": "新昵称", "bio": "新简介", "avatar_id": ""}, cc)
		expect(202, st, v)
		pt := v["task_id"].(string)
		st, v, _ = call(communityApp, "GET", "/api/v1/users/"+uid, nil, "")
		expect(200, st, v)
		if v["nickname"] != "原昵称" {
			t.Fatal("pending profile leaked")
		}
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+pt+"/claim", nil, first)
		expect(200, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+pt+"/decision", map[string]any{"decision": "approve", "reason": "资料检查通过"}, first)
		expect(200, st, v)
		st, v, _ = call(communityApp, "GET", "/api/v1/users/"+uid, nil, "")
		expect(200, st, v)
		if v["nickname"] != "新昵称" {
			t.Fatal("approved profile missing")
		}
	})
	t.Run("superseded_profile_version_cannot_publish", func(t *testing.T) {
		st, v, _ := call(communityApp, "PATCH", "/api/v1/me/profile", map[string]string{"nickname": "版本一", "bio": "简介", "avatar_id": ""}, cc)
		expect(202, st, v)
		old := v["task_id"].(string)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+old+"/claim", nil, first)
		expect(200, st, v)
		st, v, _ = call(communityApp, "PATCH", "/api/v1/me/profile", map[string]string{"nickname": "版本二", "bio": "简介", "avatar_id": ""}, cc)
		expect(202, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+old+"/decision", map[string]any{"decision": "approve", "reason": "旧版本检查"}, first)
		expect(409, st, v)
	})
	t.Run("comment_report_to_hide_closed_loop", func(t *testing.T) {
		st, v, _ := call(communityApp, "POST", "/api/v1/posts/"+pid+"/comments", map[string]string{"body": "需要治理的评论内容"}, cc)
		expect(202, st, v)
		cid := v["id"].(string)
		var ct string
		if e = db.QueryRow("SELECT id FROM review_tasks WHERE target_id=?", cid).Scan(&ct); e != nil {
			t.Fatal(e)
		}
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+ct+"/claim", nil, first)
		expect(200, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+ct+"/decision", map[string]any{"decision": "approve", "reason": "评论检查通过"}, first)
		expect(200, st, v)
		st, v, _ = call(communityApp, "POST", "/api/v1/comments/"+cid+"/reports", map[string]string{"reason": "需要进一步核查评论"}, cc)
		expect(201, st, v)
		rid := v["id"].(string)
		st, v, _ = call(communityApp, "POST", "/api/v1/comments/"+cid+"/reports", map[string]string{"reason": "重复举报不得生效"}, cc)
		expect(409, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/reports/"+rid+"/resolve", map[string]string{"kind": "comment", "action": "hide", "reason": "核查后下架评论"}, admin)
		expect(200, st, v)
		st, v, _ = call(communityApp, "GET", "/api/v1/posts/"+pid+"/comments", nil, "")
		expect(200, st, v)
		if strings.Contains(fmt.Sprint(v), cid) {
			t.Fatal("hidden comment leaked")
		}
	})

	t.Run("report_evidence_is_immutable_revision", func(t *testing.T) {
		st, v, _ := call(communityApp, "POST", "/api/v1/posts/"+pid+"/reports", map[string]string{"reason": "举报时留存原内容版本"}, cc)
		expect(201, st, v)
		rid := v["id"].(string)
		var evidence string
		if e = db.QueryRow("SELECT revision_id FROM report_evidence WHERE report_id=?", rid).Scan(&evidence); e != nil {
			t.Fatal(e)
		}
		var previous string
		if e = db.QueryRow("SELECT draft_revision_id FROM posts WHERE id=?", pid).Scan(&previous); e != nil {
			t.Fatal(e)
		}
		st, v, _ = call(communityApp, "PATCH", "/api/v1/posts/"+pid+"/draft", map[string]any{"title": "举报后更新的内容版本", "body": "新内容不应覆盖原始举报证据", "category": "校园生活", "type": "note", "media_ids": []string{}, "revision_id": previous}, cc)
		expect(200, st, v)
		st, v, _ = call(communityApp, "POST", "/api/v1/posts/"+pid+"/submit", map[string]string{"revision_id": v["revision_id"].(string)}, cc)
		expect(202, st, v)
		updatedTask := v["task_id"].(string)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+updatedTask+"/claim", nil, first)
		expect(200, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+updatedTask+"/decision", map[string]string{"decision": "approve", "reason": "更新版本人工检查通过"}, first)
		expect(200, st, v)
		st, v, _ = call(h, "GET", "/review-api/v1/reports/"+rid+"?kind=post", nil, admin)
		expect(200, st, v)
		if v["evidence_revision_id"] != evidence || v["title"] != "审核服务验收" {
			t.Fatal("wrong report evidence", v)
		}
		st, v, _ = call(h, "POST", "/review-api/v1/reports/"+rid+"/resolve", map[string]string{"kind": "post", "action": "dismiss", "reason": "核查证据后不予下架"}, admin)
		expect(200, st, v)
	})
	t.Run("appeal_reopens_fresh_task_preserving_original_decision", func(t *testing.T) {
		st, v, _ := call(communityApp, "POST", "/api/v1/posts", map[string]any{"title": "申诉业务验收", "body": "保留原始决定与重新审核的完整链路", "category": "校园生活", "type": "note", "media_ids": []string{}}, cc)
		expect(201, st, v)
		postID := v["id"].(string)
		st, v, _ = call(communityApp, "POST", "/api/v1/posts/"+postID+"/submit", map[string]string{"revision_id": v["revision_id"].(string)}, cc)
		expect(202, st, v)
		oldTask := v["task_id"].(string)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+oldTask+"/claim", nil, admin)
		expect(200, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+oldTask+"/decision", map[string]string{"decision": "reject", "reason": "初审材料需要进一步核查"}, admin)
		expect(200, st, v)
		st, v, _ = call(communityApp, "POST", "/api/v1/appeals", map[string]string{"task_id": oldTask, "reason": "补充依据申请重新审核"}, cc)
		expect(201, st, v)
		aid := v["id"].(string)
		st, v, _ = call(h, "POST", "/review-api/v1/appeals/"+aid+"/resolve", map[string]string{"action": "reopen", "reason": "独立复核后重新送审"}, admin)
		expect(403, st, v)
		risk := makeStaff("risk")
		st, v, _ = call(h, "POST", "/review-api/v1/appeals/"+aid+"/resolve", map[string]string{"action": "reopen", "reason": "独立复核后重新送审"}, risk)
		expect(200, st, v)
		var next, original string
		if e = db.QueryRow("SELECT new_task_id FROM case_resolutions WHERE case_id=? AND kind='appeal'", aid).Scan(&next); e != nil {
			t.Fatal(e)
		}
		if next == oldTask {
			t.Fatal("overwrote original task")
		}
		if e = db.QueryRow("SELECT decision FROM review_tasks WHERE id=?", oldTask).Scan(&original); e != nil || original != "reject" {
			t.Fatal("lost original decision", e)
		}
		st, v, _ = call(communityApp, "GET", "/api/v1/posts/"+postID, nil, "")
		expect(404, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+next+"/claim", nil, first)
		expect(200, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+next+"/decision", map[string]string{"decision": "approve", "reason": "重新审核全部材料后通过"}, first)
		expect(200, st, v)
		st, v, _ = call(communityApp, "GET", "/api/v1/posts/"+postID, nil, "")
		expect(200, st, v)
		st, v, _ = call(communityApp, "GET", "/api/v1/me/appeals", nil, cc)
		expect(200, st, v)
		if !strings.Contains(fmt.Sprint(v), "独立复核后重新送审") {
			t.Fatal("user cannot see appeal outcome", v)
		}
		st, v, _ = call(h, "POST", "/review-api/v1/appeals/"+aid+"/resolve", map[string]string{"action": "uphold", "reason": "重复处置无效"}, risk)
		expect(409, st, v)
	})

	t.Run("superseded_content_cannot_reopen_on_appeal", func(t *testing.T) {
		st, v, _ := call(communityApp, "POST", "/api/v1/posts", map[string]any{"title": "旧版本申诉保护", "body": "旧版本不能替换新草稿", "category": "校园生活", "type": "note", "media_ids": []string{}}, cc)
		expect(201, st, v)
		postID := v["id"].(string)
		rev := v["revision_id"].(string)
		st, v, _ = call(communityApp, "POST", "/api/v1/posts/"+postID+"/submit", map[string]string{"revision_id": rev}, cc)
		expect(202, st, v)
		old := v["task_id"].(string)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+old+"/claim", nil, first)
		expect(200, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+old+"/decision", map[string]string{"decision": "reject", "reason": "初始版本驳回"}, first)
		expect(200, st, v)
		st, v, _ = call(communityApp, "PATCH", "/api/v1/posts/"+postID+"/draft", map[string]any{"title": "已修改后的新版本", "body": "新草稿不能被旧任务覆盖", "category": "校园生活", "type": "note", "media_ids": []string{}, "revision_id": rev}, cc)
		expect(200, st, v)
		st, v, _ = call(communityApp, "POST", "/api/v1/appeals", map[string]string{"task_id": old, "reason": "针对旧决定提出申诉"}, cc)
		expect(201, st, v)
		aid := v["id"].(string)
		st, v, _ = call(h, "POST", "/review-api/v1/appeals/"+aid+"/resolve", map[string]string{"action": "reopen", "reason": "旧版本应被保护阻断"}, admin)
		expect(409, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/appeals/"+aid+"/resolve", map[string]string{"action": "uphold", "reason": "原版本已替换，请提交新版本审核"}, admin)
		expect(200, st, v)
	})
	t.Run("expired_claim_and_deleted_comment_cannot_decide", func(t *testing.T) {
		st, v, _ := call(communityApp, "POST", "/api/v1/posts/"+pid+"/comments", map[string]string{"body": "用于核查任务领取过期和删除状态"}, cc)
		expect(202, st, v)
		cid := v["id"].(string)
		var ct string
		if e = db.QueryRow("SELECT id FROM review_tasks WHERE target_id=?", cid).Scan(&ct); e != nil {
			t.Fatal(e)
		}
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+ct+"/claim", nil, first)
		expect(200, st, v)
		if _, e = db.Exec("UPDATE review_assignments SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 MINUTE) WHERE task_id=?", ct); e != nil {
			t.Fatal(e)
		}
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+ct+"/decision", map[string]string{"decision": "approve", "reason": "过期领取不可处理"}, first)
		expect(409, st, v)
		st, v, _ = call(communityApp, "DELETE", "/api/v1/comments/"+cid, nil, cc)
		expect(200, st, v)
		st, v, _ = call(h, "GET", "/review-api/v1/tasks/"+ct, nil, first)
		expect(200, st, v)
		if v["status"] != "cancelled" {
			t.Fatal("deleted comment still pending", v)
		}
	})
	t.Run("bounded_list_pagination", func(t *testing.T) {
		st, v, _ := call(h, "GET", "/review-api/v1/tasks?status=decided&page=1", nil, admin)
		expect(200, st, v)
		if len(v["items"].([]any)) > 40 {
			t.Fatal("unbounded result")
		}
		st, v, _ = call(h, "GET", "/review-api/v1/tasks?page=0", nil, admin)
		expect(400, st, v)
		st, v, _ = call(communityApp, "GET", "/api/v1/notifications?page=0", nil, cc)
		expect(400, st, v)
	})

	t.Run("expired_opportunity_cannot_approve", func(t *testing.T) {
		st, v, _ := call(communityApp, "POST", "/api/v1/posts", map[string]any{"title": "机会过期审核保护", "body": "核查审核期间过期的机会不能发布", "category": "兼职家教", "type": "opportunity", "media_ids": []string{}, "opportunity": map[string]string{"publisher": "个人发布方", "city": "校园", "compensation": "每小时 50 元", "settlement": "按次结算", "contact": "站外邮箱联系", "expires_at": time.Now().Add(time.Hour).In(time.FixedZone("offset", 7200)).Format(time.RFC3339)}}, cc)
		expect(201, st, v)
		opid := v["id"].(string)
		rev := v["revision_id"].(string)
		var normalized string
		if e = db.QueryRow("SELECT JSON_UNQUOTE(JSON_EXTRACT(opportunity,'$.expires_at')) FROM revisions WHERE id=?", rev).Scan(&normalized); e != nil {
			t.Fatal(e)
		}
		if !strings.HasSuffix(normalized, "Z") {
			t.Fatal("opportunity time not normalized", normalized)
		}
		st, v, _ = call(communityApp, "POST", "/api/v1/posts/"+opid+"/submit", map[string]string{"revision_id": rev}, cc)
		expect(202, st, v)
		ot := v["task_id"].(string)
		if _, e = db.Exec("UPDATE revisions SET opportunity=JSON_SET(opportunity,'$.expires_at',?) WHERE id=?", time.Now().UTC().Add(-time.Hour).Format(time.RFC3339), rev); e != nil {
			t.Fatal(e)
		}
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+ot+"/claim", nil, first)
		expect(200, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+ot+"/decision", map[string]string{"decision": "approve", "reason": "过期机会不可通过"}, first)
		expect(409, st, v)
		st, v, _ = call(h, "POST", "/review-api/v1/tasks/"+ot+"/decision", map[string]string{"decision": "reject", "reason": "机会已超过有效期限，请更新后提交"}, first)
		expect(200, st, v)
	})
	t.Run("deleting_parent_cancels_pending_comment_tasks", func(t *testing.T) {
		st, v, _ := call(communityApp, "POST", "/api/v1/posts/"+pid+"/comments", map[string]string{"body": "父内容删除后应取消本评论审核"}, cc)
		expect(202, st, v)
		var ct string
		if e = db.QueryRow("SELECT id FROM review_tasks WHERE target_id=?", v["id"]).Scan(&ct); e != nil {
			t.Fatal(e)
		}
		st, v, _ = call(communityApp, "DELETE", "/api/v1/posts/"+pid, nil, cc)
		expect(200, st, v)
		st, v, _ = call(h, "GET", "/review-api/v1/tasks/"+ct, nil, first)
		expect(200, st, v)
		if v["status"] != "cancelled" {
			t.Fatal("parent deletion left pending comment task", v)
		}
	})
	t.Run("disabled_staff_session_revoked_immediately", func(t *testing.T) {
		st, v, _ := call(h, "GET", "/review-api/v1/me", nil, second)
		expect(200, st, v)
		_, e = db.Exec("UPDATE staff_users SET disabled=TRUE WHERE id=?", v["id"])
		if e != nil {
			t.Fatal(e)
		}
		st, v, _ = call(h, "GET", "/review-api/v1/tasks", nil, second)
		expect(401, st, v)
	})
	t.Run("foreign_origin_denied", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/review-api/v1/auth/login", strings.NewReader("{}"))
		r.Header.Set("Origin", "http://localhost:5173")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("community origin allowed review mutation")
		}
	})
}
