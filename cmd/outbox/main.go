package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Xstly1014/Talko/internal/store"
	"github.com/Xstly1014/Talko/internal/worker"
	"os"
	"time"
)

func main() {
	id := flag.String("id", "", "dead event id")
	actor := flag.String("actor", "", "operator identity")
	reason := flag.String("reason", "", "replay justification")
	flag.Parse()
	db, e := store.Open()
	if e != nil {
		fmt.Fprintln(os.Stderr, "database unavailable")
		os.Exit(1)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e = worker.Replay(ctx, db, *id, *actor, *reason); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Println("dead letter requeued; prior failure history retained")
}
