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
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
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
	conn, e := db.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	var locked int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK('talko_schema_migration',30)").Scan(&locked); e != nil {
		return e
	}
	if locked != 1 {
		return fmt.Errorf("migration lock not acquired")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('talko_schema_migration')")
	if _, e = conn.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(version INT PRIMARY KEY,applied_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6))"); e != nil {
		return e
	}
	files, e := schema.ReadDir("migrations")
	if e != nil {
		return e
	}
	for _, f := range files {
		version, e := strconv.Atoi(strings.SplitN(f.Name(), "_", 2)[0])
		if e != nil {
			return e
		}
		var n int
		if e = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version=?", version).Scan(&n); e != nil {
			return e
		}
		if n > 0 {
			continue
		}
		b, e := schema.ReadFile("migrations/" + f.Name())
		if e != nil {
			return e
		}
		if _, e = conn.ExecContext(ctx, string(b)); e != nil {
			return fmt.Errorf("migration %s: %w", f.Name(), e)
		}
		if _, e = conn.ExecContext(ctx, "INSERT IGNORE INTO schema_migrations(version) VALUES(?)", version); e != nil {
			return e
		}
	}
	return nil
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
