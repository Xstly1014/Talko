package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

func Step(ctx context.Context, db *sql.DB) (bool, error) {
	tx, e := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	var id, kind, aggregate string
	var raw []byte
	e = tx.QueryRowContext(ctx, "SELECT id,kind,aggregate_id,payload FROM outbox WHERE processed_at IS NULL ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED").Scan(&id, &kind, &aggregate, &raw)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	p := map[string]string{}
	if e = json.Unmarshal(raw, &p); e != nil {
		return false, e
	}
	switch kind {
	case "post_published":
		_, e = tx.ExecContext(ctx, "INSERT IGNORE INTO feed_items(user_id,post_id,published_at) SELECT f.follower_id,p.id,p.published_at FROM posts p JOIN follows f ON f.followee_id=p.author_id WHERE p.id=? AND p.status='published' AND f.created_at<=p.published_at", aggregate)
	case "post_hidden":
		_, e = tx.ExecContext(ctx, "DELETE FROM feed_items WHERE post_id=?", aggregate)
	case "review_decided":
		_, e = tx.ExecContext(ctx, "INSERT IGNORE INTO notifications(id,user_id,event_id,message,post_id) VALUES(?,?,?,?,NULLIF(?,''))", id, p["author_id"], id, p["message"], aggregate)
	case "review_requested", "report_submitted": // Durable review/report tables are the queue; no external model is configured.
	default:
		return false, fmt.Errorf("unknown event kind %s", kind)
	}
	if e != nil {
		return false, e
	}
	_, e = tx.ExecContext(ctx, "UPDATE outbox SET processed_at=UTC_TIMESTAMP(6) WHERE id=?", id)
	if e != nil {
		return false, e
	}
	if e = tx.Commit(); e != nil {
		return false, e
	}
	return true, nil
}
func Run(ctx context.Context, db *sql.DB) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		ok, e := Step(ctx, db)
		if e != nil {
			slog.Error("outbox dispatch failed", "error", e.Error())
		}
		if !ok || e != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}
}
