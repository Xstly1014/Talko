package moderation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/Xstly1014/Talko/internal/store"
	"time"
)

func independent(ctx context.Context, tx *sql.Tx, actor, author, reporter string) error {
	var n int
	if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE id IN (?,?) AND email=?", author, reporter, actor).Scan(&n); e != nil {
		return e
	}
	if n > 0 {
		return errors.New("作者或举报人不能处理自己的案件")
	}
	return nil
}
func recordResolution(ctx context.Context, tx *sql.Tx, id, kind, actor, action, reason, next string) error {
	_, e := tx.ExecContext(ctx, "INSERT INTO case_resolutions(case_id,kind,action,reason,reviewer,new_task_id) VALUES(?,?,?,?,?,NULLIF(?,''))", id, kind, action, reason, actor, next)
	return e
}

// ResolveAppeal preserves the original decision. A successful appeal creates a fresh task, never an automatic publication.
func ResolveAppeal(ctx context.Context, db *sql.DB, id, actor, result string, actions ...string) error {
	action := "uphold"
	if len(actions) > 0 {
		action = actions[0]
	}
	if actor == "" || len([]rune(result)) < 2 || len([]rune(result)) > 400 || (action != "uphold" && action != "reopen") {
		return errors.New("复核人员、有效结论与依据必填")
	}
	var task, kind, target, author string
	if e := db.QueryRowContext(ctx, "SELECT t.id,t.kind,t.target_id,t.author_id FROM appeals a JOIN review_tasks t ON t.id=a.task_id WHERE a.id=?", id).Scan(&task, &kind, &target, &author); e != nil {
		return e
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var post, postStatus string
	var draft sql.NullString
	if kind == "profile" {
		var lock string
		if e = tx.QueryRowContext(ctx, "SELECT id FROM users WHERE id=? FOR UPDATE", author).Scan(&lock); e != nil {
			return e
		}
	} else {
		if kind == "post" {
			e = tx.QueryRowContext(ctx, "SELECT post_id FROM revisions WHERE id=?", target).Scan(&post)
		} else {
			e = tx.QueryRowContext(ctx, "SELECT post_id FROM comments WHERE id=?", target).Scan(&post)
		}
		if e != nil {
			return e
		}
		if e = tx.QueryRowContext(ctx, "SELECT status,draft_revision_id FROM posts WHERE id=? FOR UPDATE", post).Scan(&postStatus, &draft); e != nil {
			return e
		}
	}
	var status, original, reviewer string
	if e = tx.QueryRowContext(ctx, "SELECT status FROM appeals WHERE id=? FOR UPDATE", id).Scan(&status); e != nil {
		return e
	}
	if status != "pending" {
		return errors.New("申诉已处理")
	}
	if e = tx.QueryRowContext(ctx, "SELECT COALESCE(decision,''),COALESCE(reviewer,'') FROM review_tasks WHERE id=? FOR UPDATE", task).Scan(&original, &reviewer); e != nil {
		return e
	}
	if original != "reject" {
		return errors.New("原决定并非驳回")
	}
	if actor == reviewer {
		return errors.New("原审核员不得复核自己的决定")
	}
	if e = independent(ctx, tx, actor, author, author); e != nil {
		return e
	}
	next := ""
	if action == "reopen" {
		var state string
		switch kind {
		case "post":
			var opp []byte
			if postStatus == "hidden" || postStatus == "deleted" || draft.String != target {
				return errors.New("内容已下架、删除或被新版本替代")
			}
			if e = tx.QueryRowContext(ctx, "SELECT state,opportunity FROM revisions WHERE id=?", target).Scan(&state, &opp); e != nil {
				return e
			}
			if state != "rejected" {
				return errors.New("内容版本已变化")
			}
			if len(opp) > 0 && string(opp) != "null" {
				var o map[string]json.RawMessage
				var expires time.Time
				if json.Unmarshal(opp, &o) != nil || json.Unmarshal(o["expires_at"], &expires) != nil || !expires.After(time.Now().UTC()) {
					return errors.New("机会已过期，不能重新送审")
				}
			}
			_, e = tx.ExecContext(ctx, "UPDATE revisions SET state='pending',reason='' WHERE id=?", target)
		case "comment":
			if postStatus != "published" {
				return errors.New("关联内容已不可见")
			}
			var visible int
			if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts p JOIN revisions r ON r.id=p.published_revision_id WHERE p.id=? AND r.state='published' AND (p.type<>'opportunity' OR JSON_UNQUOTE(JSON_EXTRACT(r.opportunity,'$.expires_at'))>DATE_FORMAT(UTC_TIMESTAMP(6),'%Y-%m-%dT%H:%i:%s.%fZ'))", post).Scan(&visible); e != nil {
				return e
			}
			if visible == 0 {
				return errors.New("关联内容已不可见或过期")
			}
			if e = tx.QueryRowContext(ctx, "SELECT status FROM comments WHERE id=? FOR UPDATE", target).Scan(&state); e != nil {
				return e
			}
			if state != "rejected" {
				return errors.New("评论状态已变化")
			}
			_, e = tx.ExecContext(ctx, "UPDATE comments SET status='pending' WHERE id=?", target)
		case "profile":
			var latest string
			if e = tx.QueryRowContext(ctx, "SELECT id,status FROM profile_changes WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT 1", author).Scan(&latest, &state); e != nil {
				return e
			}
			if latest != target || state != "rejected" {
				return errors.New("资料已被新版本替代或状态变化")
			}
			_, e = tx.ExecContext(ctx, "UPDATE profile_changes SET status='pending',reason='' WHERE id=?", target)
		default:
			return errors.New("不支持的审核类型")
		}
		if e != nil {
			return e
		}
		next = store.ID()
		if _, e = tx.ExecContext(ctx, "INSERT INTO review_tasks(id,kind,target_id,author_id) VALUES(?,?,?,?)", next, kind, target, author); e != nil {
			return e
		}
		if e = store.Event(ctx, tx, "review_requested", post, map[string]string{"task_id": next}); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, "UPDATE appeals SET status='resolved' WHERE id=?", id); e != nil {
		return e
	}
	if e = recordResolution(ctx, tx, id, "appeal", actor, action, result, next); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO audit_log(id,actor,action,target_id,reason) VALUES(?,?,'resolve_appeal',?,?)", store.ID(), actor, id, action+": "+result); e != nil {
		return e
	}
	message := "申诉复核维持原决定：" + result
	if action == "reopen" {
		message = "申诉复核已重新送审，待独立审核后公开：" + result
	}
	if e = store.Event(ctx, tx, "review_decided", post, map[string]string{"author_id": author, "message": message}); e != nil {
		return e
	}
	return tx.Commit()
}
