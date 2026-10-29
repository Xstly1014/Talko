package worker

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Xstly1014/Talko/internal/store"
	"strings"
)

// Replay requires trusted database access. Actor is an audit label, not authentication.
func Replay(ctx context.Context, db *sql.DB, id, actor, reason string) error {
	if len(id) != 32 || strings.TrimSpace(actor) == "" || len(actor) > 254 || len([]rune(strings.TrimSpace(reason))) < 5 || len([]rune(reason)) > 1000 {
		return fmt.Errorf("valid event id, actor and reason (5-1000 chars) required")
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var locked string
	if e = tx.QueryRowContext(ctx, "SELECT id FROM outbox WHERE id=? AND processed_at IS NULL FOR UPDATE", id).Scan(&locked); e != nil {
		return e
	}
	r, e := tx.ExecContext(ctx, "UPDATE outbox_delivery SET attempts=0,next_attempt_at=NULL,dead_at=NULL WHERE event_id=? AND dead_at IS NOT NULL", id)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return fmt.Errorf("event is not a pending dead letter")
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO audit_log(id,actor,action,target_id,reason) VALUES(?,?,'outbox_replay',?,?)", store.ID(), actor, id, reason); e != nil {
		return e
	}
	return tx.Commit()
}
