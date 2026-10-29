package moderation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Xstly1014/Talko/internal/store"
)

// Assessor is the boundary for a future model provider. Failure must route to manual review.
type Assessment struct {
	Risk, Reason, ModelVersion string
	RequiresManual             bool
}
type Assessor interface {
	Assess(context.Context, string, []string) (Assessment, error)
}
type ManualOnly struct{}

func (ManualOnly) Assess(context.Context, string, []string) (Assessment, error) {
	return Assessment{Risk: "unknown", Reason: "模型尚未配置，等待人工审核", RequiresManual: true}, nil
}
func Decide(ctx context.Context, db *sql.DB, task, actor, decision, reason string) error {
	if actor == "" || len(actor) > 254 || reason == "" || len(reason) > 1000 || decision != "approve" && decision != "reject" {
		return errors.New("审核员、结论和原因必填")
	}
	var kind, target, author string
	e := db.QueryRowContext(ctx, "SELECT kind,target_id,author_id FROM review_tasks WHERE id=?", task).Scan(&kind, &target, &author)
	if e != nil {
		return e
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var postID, postStatus string
	var draft sql.NullString
	if kind == "post" {
		if e = tx.QueryRowContext(ctx, "SELECT post_id FROM revisions WHERE id=?", target).Scan(&postID); e != nil {
			return e
		}
		if e = tx.QueryRowContext(ctx, "SELECT status,draft_revision_id FROM posts WHERE id=? FOR UPDATE", postID).Scan(&postStatus, &draft); e != nil {
			return e
		}
	} else {
		if e = tx.QueryRowContext(ctx, "SELECT post_id FROM comments WHERE id=?", target).Scan(&postID); e != nil {
			return e
		}
		if e = tx.QueryRowContext(ctx, "SELECT status FROM posts WHERE id=? FOR UPDATE", postID).Scan(&postStatus); e != nil {
			return e
		}
	}
	var status string
	var previous sql.NullString
	if e = tx.QueryRowContext(ctx, "SELECT status,decision FROM review_tasks WHERE id=? FOR UPDATE", task).Scan(&status, &previous); e != nil {
		return e
	}
	if status == "decided" && previous.String == decision {
		return nil
	}
	if status != "pending" {
		return errors.New("任务已处理或撤回")
	}
	var email string
	if e = tx.QueryRowContext(ctx, "SELECT email FROM users WHERE id=?", author).Scan(&email); e != nil {
		return e
	}
	if actor == email {
		return errors.New("不能审核自己的内容")
	}
	if postStatus == "deleted" || postStatus == "hidden" {
		return errors.New("内容已删除或下架")
	}
	if kind == "post" {
		if draft.String != target {
			return errors.New("旧版本不可发布")
		}
		state := "rejected"
		if decision == "approve" {
			state = "published"
		}
		_, e = tx.ExecContext(ctx, "UPDATE revisions SET state=?,reason=? WHERE id=? AND state='pending'", state, reason, target)
		if e != nil {
			return e
		}
		if decision == "approve" {
			_, e = tx.ExecContext(ctx, "UPDATE posts SET published_revision_id=?,status='published',published_at=COALESCE(published_at,UTC_TIMESTAMP(6)) WHERE id=?", target, postID)
			if e != nil {
				return e
			}
			if e = store.Event(ctx, tx, "post_published", postID, map[string]string{"author_id": author}); e != nil {
				return e
			}
		}
	} else {
		state := "rejected"
		if decision == "approve" {
			state = "published"
		}
		res, err := tx.ExecContext(ctx, "UPDATE comments SET status=? WHERE id=? AND status='pending'", state, target)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return errors.New("评论已删除或状态变化")
		}
	}
	_, e = tx.ExecContext(ctx, "UPDATE review_tasks SET status='decided',decision=?,reason=?,reviewer=?,decided_at=UTC_TIMESTAMP(6) WHERE id=?", decision, reason, actor, task)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO audit_log(id,actor,action,target_id,reason) VALUES(?,?,'review_decision',?,?)", store.ID(), actor, task, decision+": "+reason)
	if e != nil {
		return e
	}
	if e = store.Event(ctx, tx, "review_decided", postID, map[string]string{"author_id": author, "message": "审核结果：" + decision + "；" + reason}); e != nil {
		return e
	}
	return tx.Commit()
}
func ResolveReport(ctx context.Context, db *sql.DB, id, actor, action, reason string) error {
	if actor == "" || reason == "" || action != "dismiss" && action != "hide" {
		return errors.New("需要审核员、原因及 hide/dismiss")
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var post, user, status string
	if e = tx.QueryRowContext(ctx, "SELECT post_id,user_id,status FROM reports WHERE id=? FOR UPDATE", id).Scan(&post, &user, &status); e != nil {
		return e
	}
	if status != "pending" {
		return errors.New("举报已处理")
	}
	if action == "hide" {
		_, e = tx.ExecContext(ctx, "UPDATE posts SET status='hidden' WHERE id=? AND status<>'deleted'", post)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, "UPDATE revisions SET state='hidden',reason=? WHERE post_id=? AND state='published'", reason, post)
		if e != nil {
			return e
		}
		if e = store.Event(ctx, tx, "post_hidden", post, map[string]string{}); e != nil {
			return e
		}
	}
	_, e = tx.ExecContext(ctx, "UPDATE reports SET status='resolved',result=? WHERE id=?", reason, id)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO audit_log(id,actor,action,target_id,reason) VALUES(?,?,'resolve_report',?,?)", store.ID(), actor, id, action+": "+reason)
	if e != nil {
		return e
	}
	if e = store.Event(ctx, tx, "review_decided", post, map[string]string{"author_id": user, "message": "举报已处理：" + reason}); e != nil {
		return e
	}
	return tx.Commit()
}
func ResolveAppeal(ctx context.Context, db *sql.DB, id, actor, result string) error {
	if actor == "" || result == "" {
		return errors.New("复核员和处理结果必填")
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var user, status, task string
	if e = tx.QueryRowContext(ctx, "SELECT user_id,status,task_id FROM appeals WHERE id=? FOR UPDATE", id).Scan(&user, &status, &task); e != nil {
		return e
	}
	if status != "pending" {
		return errors.New("申诉已处理")
	}
	_, e = tx.ExecContext(ctx, "UPDATE appeals SET status='resolved' WHERE id=?", id)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO audit_log(id,actor,action,target_id,reason) VALUES(?,?,'resolve_appeal',?,?)", store.ID(), actor, id, result)
	if e != nil {
		return e
	}
	if e = store.Event(ctx, tx, "review_decided", "", map[string]string{"author_id": user, "message": fmt.Sprintf("申诉 %s 复核结果：%s。若允许修改，请编辑后重新提交审核。", task, result)}); e != nil {
		return e
	}
	return tx.Commit()
}
