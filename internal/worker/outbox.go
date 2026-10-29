package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Xstly1014/Talko/internal/store"
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
	e = tx.QueryRowContext(ctx, "SELECT o.id,o.kind,o.aggregate_id,o.payload FROM outbox o LEFT JOIN outbox_delivery d ON d.event_id=o.id WHERE o.processed_at IS NULL AND d.dead_at IS NULL AND (d.next_attempt_at IS NULL OR d.next_attempt_at<=UTC_TIMESTAMP(6)) ORDER BY o.created_at,o.id LIMIT 1 FOR UPDATE SKIP LOCKED").Scan(&id, &kind, &aggregate, &raw)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if _, e = tx.ExecContext(ctx, "SAVEPOINT delivery"); e != nil {
		return false, e
	}
	fail := func(cause error) (bool, error) {
		if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT delivery"); err != nil {
			return false, err
		}
		var attempts int
		err := tx.QueryRowContext(ctx, "SELECT attempts FROM outbox_delivery WHERE event_id=?", id).Scan(&attempts)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}
		attempts++
		delay := 1 << min(attempts, 10)
		reason := []rune(cause.Error())
		if len(reason) > 2000 {
			reason = reason[:2000]
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO outbox_delivery(event_id,attempts,next_attempt_at,dead_at,last_error) VALUES(?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL ? SECOND),IF(? >= 8,UTC_TIMESTAMP(6),NULL),?) ON DUPLICATE KEY UPDATE attempts=VALUES(attempts),next_attempt_at=VALUES(next_attempt_at),dead_at=VALUES(dead_at),last_error=VALUES(last_error)", id, attempts, delay, attempts, string(reason))
		if err != nil {
			return false, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO outbox_failures(id,event_id,attempt,reason) VALUES(?,?,?,?)", store.ID(), id, attempts, string(reason)); err != nil {
			return false, err
		}
		if err = tx.Commit(); err != nil {
			return false, err
		}
		slog.Error("outbox delivery failed", "event_id", id, "kind", kind, "attempt", attempts, "dead", attempts >= 8)
		return true, cause
	}
	p := map[string]string{}
	if e = json.Unmarshal(raw, &p); e != nil {
		return fail(e)
	}
	switch kind {
	case "post_published":
		if len(aggregate) != 32 {
			return fail(fmt.Errorf("invalid publication aggregate"))
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO feed_items(user_id,post_id,published_at) SELECT f.follower_id,p.id,p.published_at FROM posts p JOIN follows f ON f.followee_id=p.author_id WHERE p.id=? AND p.status='published' AND f.created_at<=p.published_at ON DUPLICATE KEY UPDATE published_at=GREATEST(feed_items.published_at,VALUES(published_at))", aggregate)
	case "post_hidden":
		if len(aggregate) != 32 {
			return fail(fmt.Errorf("invalid hidden aggregate"))
		}
		_, e = tx.ExecContext(ctx, "DELETE fi FROM feed_items fi JOIN posts p ON p.id=fi.post_id WHERE fi.post_id=? AND p.status<>'published'", aggregate)
	case "review_decided":
		if len(p["author_id"]) != 32 || len([]rune(p["message"])) < 1 || len([]rune(p["message"])) > 500 {
			return fail(fmt.Errorf("invalid review notification payload"))
		}
		_, e = tx.ExecContext(ctx, "INSERT IGNORE INTO notifications(id,user_id,event_id,message,post_id) VALUES(?,?,?,?,NULLIF(?,''))", id, p["author_id"], id, p["message"], aggregate)
	case "review_requested", "report_submitted": // Durable review/report tables are the queue; no external model is configured.
	default:
		return fail(fmt.Errorf("unknown event kind %s", kind))
	}
	if e != nil {
		return fail(e)
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
