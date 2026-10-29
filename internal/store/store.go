package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"os"
	"time"
)

//go:embed schema.sql
var schema embed.FS

func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func Open() (*sql.DB, error) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("MYSQL_DSN is required")
	}
	db, e := sql.Open("mysql", DSN(dsn))
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, e
	}
	return db, nil
}
func Migrate(ctx context.Context, db *sql.DB) error {
	b, e := schema.ReadFile("schema.sql")
	if e != nil {
		return e
	}
	_, e = db.ExecContext(ctx, string(b))
	return e
}
func Event(ctx context.Context, tx *sql.Tx, kind, id string, p any) error {
	b, e := json.Marshal(p)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO outbox(id,kind,aggregate_id,payload) VALUES(?,?,?,?)", ID(), kind, id, b)
	return e
}

func DSN(raw string) string {
	cfg, e := mysql.ParseDSN(raw)
	if e != nil {
		return raw
	}
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	cfg.Params["time_zone"] = "'+00:00'"
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	return cfg.FormatDSN()
}
