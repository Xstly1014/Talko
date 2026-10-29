package worker

import (
	"context"
	"database/sql"
	"github.com/Xstly1014/Talko/internal/store"
	"os"
	"testing"
)

func TestPoisonIsolationAndIdempotency(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN required")
	}
	db, e := sql.Open("mysql", store.DSN(dsn))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	if e = store.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	bad, good, user := store.ID(), store.ID(), store.ID()
	must := func(q string, a ...any) {
		t.Helper()
		if _, e = db.Exec(q, a...); e != nil {
			t.Fatal(e)
		}
	}
	defer func() {
		db.Exec("DELETE FROM audit_log WHERE target_id=? AND action='outbox_replay'", bad)
		db.Exec("DELETE FROM notifications WHERE event_id=?", good)
		db.Exec("DELETE FROM outbox_failures WHERE event_id=?", bad)
		db.Exec("DELETE FROM outbox_delivery WHERE event_id=?", bad)
		db.Exec("DELETE FROM outbox WHERE id IN (?,?)", bad, good)
	}()
	must("INSERT INTO outbox(id,kind,aggregate_id,payload,created_at) VALUES(?,'unsupported','','{}','2000-01-01')", bad)
	must("INSERT INTO outbox(id,kind,aggregate_id,payload,created_at) VALUES(?,'review_decided','',JSON_OBJECT('author_id',?,'message','test'),'2001-01-01')", good, user)
	if _, e = Step(ctx, db); e == nil {
		t.Fatal("poison should fail")
	}
	var attempts int
	db.QueryRow("SELECT attempts FROM outbox_delivery WHERE event_id=?", bad).Scan(&attempts)
	if attempts != 1 {
		t.Fatal("failure not persisted")
	}
	if ok, e := Step(ctx, db); e != nil || !ok {
		t.Fatalf("poison blocked normal event: %v", e)
	}
	for n := 2; n <= 8; n++ {
		must("UPDATE outbox_delivery SET next_attempt_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE event_id=?", bad)
		if _, e = Step(ctx, db); e == nil {
			t.Fatal("expected poison failure")
		}
	}
	var dead int
	if e = db.QueryRow("SELECT COUNT(*) FROM outbox_delivery WHERE event_id=? AND attempts=8 AND dead_at IS NOT NULL", bad).Scan(&dead); e != nil || dead != 1 {
		t.Fatal("dead letter missing", e)
	}
	must("UPDATE outbox SET processed_at=NULL WHERE id=?", good)
	if _, e = Step(ctx, db); e != nil {
		t.Fatal(e)
	}

	if e = Replay(ctx, db, bad, "operator@example.test", "修复故障后重放"); e != nil {
		t.Fatal(e)
	}
	if e = Replay(ctx, db, bad, "operator@example.test", "重复重放应被拒绝"); e == nil {
		t.Fatal("duplicate replay accepted")
	}
	var audit, history int
	if e = db.QueryRow("SELECT COUNT(*) FROM audit_log WHERE action='outbox_replay' AND target_id=?", bad).Scan(&audit); e != nil || audit != 1 {
		t.Fatal("missing replay audit", e)
	}
	if e = db.QueryRow("SELECT COUNT(*) FROM outbox_failures WHERE event_id=?", bad).Scan(&history); e != nil || history != 8 {
		t.Fatal("failure history lost", e)
	}
	var count int
	db.QueryRow("SELECT COUNT(*) FROM notifications WHERE event_id=?", good).Scan(&count)
	if count != 1 {
		t.Fatal("duplicate notification", count)
	}
}
