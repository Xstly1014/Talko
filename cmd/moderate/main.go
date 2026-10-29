package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Xstly1014/Talko/internal/moderation"
	"github.com/Xstly1014/Talko/internal/store"
	"os"
	"time"
)

func main() {
	action := flag.String("action", "list", "list, decide, reports, resolve-report, appeals, resolve-appeal")
	id := flag.String("id", "", "task/report/appeal id")
	actor := flag.String("actor", "", "audited operator identity")
	decision := flag.String("decision", "", "approve/reject or hide/dismiss")
	reason := flag.String("reason", "", "decision reason")
	flag.Parse()
	db, e := store.Open()
	if e != nil {
		fail(e)
	}
	defer db.Close()
	ctx, c := context.WithTimeout(context.Background(), 30*time.Second)
	defer c()
	switch *action {
	case "decide":
		e = moderation.Decide(ctx, db, *id, *actor, *decision, *reason)
	case "resolve-report":
		e = moderation.ResolveReport(ctx, db, *id, *actor, *decision, *reason)
	case "resolve-appeal":
		e = moderation.ResolveAppeal(ctx, db, *id, *actor, *reason)
	case "list", "reports", "appeals":
		q := "SELECT t.id,t.kind,t.target_id,t.status,r.title,r.body,c.body AS comment_body FROM review_tasks t LEFT JOIN revisions r ON t.kind='post' AND r.id=t.target_id LEFT JOIN comments c ON t.kind='comment' AND c.id=t.target_id WHERE t.status='pending' ORDER BY t.created_at"
		if *action == "reports" {
			q = "SELECT id,post_id,reason,status FROM reports WHERE status='pending'"
		}
		if *action == "appeals" {
			q = "SELECT id,task_id,reason,status FROM appeals WHERE status='pending'"
		}
		rows, err := db.QueryContext(ctx, q)
		if err != nil {
			fail(err)
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			fail(err)
		}
		for rows.Next() {
			v := make([]any, len(cols))
			p := make([]any, len(cols))
			for i := range v {
				p[i] = &v[i]
			}
			if err = rows.Scan(p...); err != nil {
				fail(err)
			}
			item := map[string]any{}
			for i, k := range cols {
				if b, ok := v[i].([]byte); ok {
					v[i] = string(b)
				}
				item[k] = v[i]
			}
			_ = json.NewEncoder(os.Stdout).Encode(item)
		}
		e = rows.Err()
	default:
		e = fmt.Errorf("unknown action")
	}
	if e != nil {
		fail(e)
	}
}
func fail(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
