package moderation

import (
	"context"
	"database/sql"
	"errors"
	"github.com/Xstly1014/Talko/internal/store"
)

func DecideProfile(ctx context.Context, db *sql.DB, task, target, author, actor, decision, reason string) error {
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var email string
	if e = tx.QueryRowContext(ctx, "SELECT email FROM users WHERE id=? FOR UPDATE", author).Scan(&email); e != nil {
		return e
	}
	if actor == email {
		return errors.New("不能审核自己的资料")
	}
	var status string
	var prior sql.NullString
	if e = tx.QueryRowContext(ctx, "SELECT status,decision FROM review_tasks WHERE id=? FOR UPDATE", task).Scan(&status, &prior); e != nil {
		return e
	}
	if status == "decided" && prior.String == decision {
		return nil
	}
	if status != "pending" {
		return errors.New("资料任务已处理或被新版本替代")
	}
	var nick, bio, state string
	var avatar sql.NullString
	if e = tx.QueryRowContext(ctx, "SELECT nickname,bio,avatar_id,status FROM profile_changes WHERE id=? AND user_id=?", target, author).Scan(&nick, &bio, &avatar, &state); e != nil {
		return e
	}
	if state != "pending" {
		return errors.New("资料版本已变化")
	}
	next := "rejected"
	if decision == "approve" {
		next = "approved"
		_, e = tx.ExecContext(ctx, "UPDATE users SET nickname=?,bio=?,avatar_id=? WHERE id=?", nick, bio, avatar, author)
		if e != nil {
			return e
		}
	}
	_, e = tx.ExecContext(ctx, "UPDATE profile_changes SET status=?,reason=? WHERE id=?", next, reason, target)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "UPDATE review_tasks SET status='decided',decision=?,reason=?,reviewer=?,decided_at=UTC_TIMESTAMP(6) WHERE id=?", decision, reason, actor, task)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO audit_log(id,actor,action,target_id,reason) VALUES(?,?,'profile_decision',?,?)", store.ID(), actor, task, decision+": "+reason)
	if e != nil {
		return e
	}
	if e = store.Event(ctx, tx, "review_decided", "", map[string]string{"author_id": author, "message": "资料审核：" + decision + "；" + reason}); e != nil {
		return e
	}
	return tx.Commit()
}
func ResolveCommentReport(ctx context.Context, db *sql.DB, id, actor, action, reason string) error {
	if actor == "" || reason == "" || action != "hide" && action != "dismiss" {
		return errors.New("处理人、原因和有效动作必填")
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var cid, user, status string
	if e = tx.QueryRowContext(ctx, "SELECT comment_id,user_id,status FROM comment_reports WHERE id=? FOR UPDATE", id).Scan(&cid, &user, &status); e != nil {
		return e
	}
	if status != "pending" {
		return errors.New("举报已处理")
	}
	var post string
	if e = tx.QueryRowContext(ctx, "SELECT post_id FROM comments WHERE id=?", cid).Scan(&post); e != nil {
		return e
	}
	var author string
	if e = tx.QueryRowContext(ctx, "SELECT author_id FROM comments WHERE id=?", cid).Scan(&author); e != nil {
		return e
	}
	if e = independent(ctx, tx, actor, author, user); e != nil {
		return e
	}
	if action == "hide" {
		_, e = tx.ExecContext(ctx, "UPDATE comments SET status='hidden' WHERE id=? AND status<>'deleted'", cid)
		if e != nil {
			return e
		}
	}
	_, e = tx.ExecContext(ctx, "UPDATE comment_reports SET status='resolved',result=? WHERE id=?", reason, id)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO audit_log(id,actor,action,target_id,reason) VALUES(?,?,'comment_report',?,?)", store.ID(), actor, id, action+": "+reason)
	if e != nil {
		return e
	}
	if e = recordResolution(ctx, tx, id, "comment_report", actor, action, reason, ""); e != nil {
		return e
	}
	if action == "hide" {
		if e = store.Event(ctx, tx, "review_decided", post, map[string]string{"author_id": author, "message": "你的评论因举报核查被下架：" + reason}); e != nil {
			return e
		}
	}
	if e = store.Event(ctx, tx, "review_decided", post, map[string]string{"author_id": user, "message": "评论举报处理结果：" + reason}); e != nil {
		return e
	}
	return tx.Commit()
}
