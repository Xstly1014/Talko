package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBoundedRouteAndStatus(t *testing.T) {
	m := &HTTPMetrics{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /posts/{id}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) })
	h := m.Wrap(mux)
	for _, id := range []string{"secret-one", "secret-two"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/posts/"+id, nil))
	}
	w := httptest.NewRecorder()
	m.Write(w)
	s := w.Body.String()
	if strings.Contains(s, "secret-") || !strings.Contains(s, "status=\"503\"} 2") || !strings.Contains(s, "le=\"+Inf\"} 2") {
		t.Fatal(s)
	}
}
