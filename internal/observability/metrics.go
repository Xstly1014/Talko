package observability

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"runtime"
	"time"
)

func Serve(db *sql.DB, w http.ResponseWriter, r *http.Request) bool {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	var pending, dead, review int
	var age, reviewAge float64
	queries := []struct {
		q string
		v []any
	}{
		{"SELECT COUNT(*),COALESCE(MAX(TIMESTAMPDIFF(SECOND,o.created_at,UTC_TIMESTAMP(6))),0) FROM outbox o LEFT JOIN outbox_delivery d ON d.event_id=o.id WHERE o.processed_at IS NULL AND d.dead_at IS NULL", []any{&pending, &age}},
		{"SELECT COUNT(*) FROM outbox_delivery WHERE dead_at IS NOT NULL", []any{&dead}},
		{"SELECT COUNT(*),COALESCE(MAX(TIMESTAMPDIFF(SECOND,created_at,UTC_TIMESTAMP(6))),0) FROM review_tasks WHERE status='pending'", []any{&review, &reviewAge}}}
	for _, q := range queries {
		if e := db.QueryRowContext(ctx, q.q).Scan(q.v...); e != nil {
			http.Error(w, "metrics collection failed", http.StatusServiceUnavailable)
			return false
		}
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	st := db.Stats()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	fmt.Fprintf(w, "talko_outbox_pending %d\ntalko_outbox_oldest_seconds %g\ntalko_outbox_dead %d\ntalko_review_pending %d\ntalko_review_oldest_seconds %g\ntalko_db_connections_open %d\ntalko_db_connections_in_use %d\ntalko_db_wait_total %d\ntalko_db_wait_seconds_total %g\ntalko_go_goroutines %d\ntalko_go_heap_bytes %d\n", pending, age, dead, review, reviewAge, st.OpenConnections, st.InUse, st.WaitCount, st.WaitDuration.Seconds(), runtime.NumGoroutine(), mem.HeapAlloc)
	return true
}
