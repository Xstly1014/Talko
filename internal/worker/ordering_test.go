package worker

import (
	"context"
	"database/sql"
	"github.com/Xstly1014/Talko/internal/store"
	"os"
	"testing"
)

func TestDelayedLifecycleEventsRespectCurrentPublication(t *testing.T) {
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
	author, follower, post := store.ID(), store.ID(), store.ID()
	events := []string{}
	must := func(q string, a ...any) {
		t.Helper()
		if _, e := db.Exec(q, a...); e != nil {
			t.Fatal(e)
		}
	}
	defer func() {
		for _, id := range events {
			db.Exec("DELETE FROM outbox WHERE id=?", id)
		}
		db.Exec("DELETE FROM feed_items WHERE post_id=?", post)
		db.Exec("DELETE FROM follows WHERE follower_id=?", follower)
		db.Exec("DELETE FROM posts WHERE id=?", post)
		db.Exec("DELETE FROM users WHERE id IN (?,?)", author, follower)
	}()
	for _, id := range []string{author, follower} {
		must("INSERT INTO users(id,email,nickname,password_hash) VALUES(?,?,?,'')", id, id+"@ordering.test", "fixture")
	}
	must("INSERT INTO follows(follower_id,followee_id,created_at) VALUES(?,?,'2000-01-01')", follower, author)
	must("INSERT INTO posts(id,author_id,type,status,published_at) VALUES(?,?,'note','published','2002-01-01')", post, author)
	must("INSERT INTO feed_items(user_id,post_id,published_at) VALUES(?,?,'2001-01-01')", follower, post)
	dispatch := func(kind string) {
		t.Helper()
		id := store.ID()
		events = append(events, id)
		must("INSERT INTO outbox(id,kind,aggregate_id,payload,created_at) VALUES(?,?,?,'{}','1971-01-01')", id, kind, post)
		if ok, e := Step(ctx, db); !ok || e != nil {
			t.Fatal("dispatch", kind, e)
		}
	}
	count := func() int {
		t.Helper()
		var n int
		if e := db.QueryRow("SELECT COUNT(*) FROM feed_items WHERE post_id=?", post).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}
	dispatch("post_hidden")
	if count() != 1 {
		t.Fatal("late hide removed current publication")
	}
	dispatch("post_published")
	var year int
	if e := db.QueryRow("SELECT YEAR(published_at) FROM feed_items WHERE post_id=?", post).Scan(&year); e != nil || year != 2002 {
		t.Fatal("republished ordering not updated", year, e)
	}
	must("UPDATE posts SET status='hidden' WHERE id=?", post)
	dispatch("post_hidden")
	if count() != 0 {
		t.Fatal("hidden feed row retained")
	}
	dispatch("post_published")
	if count() != 0 {
		t.Fatal("late publication resurrected hidden content")
	}
}
