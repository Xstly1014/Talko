package observability

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"net/http/httptest"
	"testing"
)

func TestMetricsDatabaseFailure(t *testing.T) {
	db, e := sql.Open("mysql", "unused:unused@tcp(127.0.0.1:1)/none")
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	w := httptest.NewRecorder()
	Serve(db, w, httptest.NewRequest("GET", "/metrics", nil))
	if w.Code != 503 {
		t.Fatalf("database failure reported healthy: %d %s", w.Code, w.Body.String())
	}
}
