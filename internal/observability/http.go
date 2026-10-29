package observability

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type httpKey struct {
	Method, Route string
	Status        int
}
type httpValue struct {
	Count   uint64
	Sum     float64
	Buckets [7]uint64
}

var bounds = [7]float64{.01, .05, .1, .25, .5, 1, 5}

type HTTPMetrics struct {
	mu     sync.Mutex
	values map[httpKey]httpValue
}
type response struct {
	http.ResponseWriter
	status int
}

func (w *response) WriteHeader(s int) {
	if w.status == 0 {
		w.status = s
		w.ResponseWriter.WriteHeader(s)
	}
}
func (w *response) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *response) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (m *HTTPMetrics) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rw := &response{ResponseWriter: w}
		defer func() {
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			method := r.Method
			if method != "GET" && method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" && method != "HEAD" && method != "OPTIONS" {
				method = "OTHER"
			}
			status := rw.status
			if status == 0 {
				status = 200
			}
			elapsed := time.Since(start).Seconds()
			m.mu.Lock()
			if m.values == nil {
				m.values = map[httpKey]httpValue{}
			}
			k := httpKey{method, route, status}
			v := m.values[k]
			v.Count++
			v.Sum += elapsed
			for i, b := range bounds {
				if elapsed <= b {
					v.Buckets[i]++
				}
			}
			m.values[k] = v
			m.mu.Unlock()
			slog.Info("http request", "request_id", rw.Header().Get("X-Request-ID"), "method", method, "route", route, "status", status, "duration_ms", elapsed*1000)
		}()
		next.ServeHTTP(rw, r)
	})
}
func (m *HTTPMetrics) Write(w http.ResponseWriter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fmt.Fprintln(w, "# TYPE talko_http_requests_total counter\n# TYPE talko_http_duration_seconds histogram")
	keys := make([]httpKey, 0, len(m.values))
	for k := range m.values {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
	for _, k := range keys {
		v := m.values[k]
		labels := fmt.Sprintf("method=%q,route=%q,status=%q", k.Method, strings.TrimSpace(k.Route), fmt.Sprint(k.Status))
		fmt.Fprintf(w, "talko_http_requests_total{%s} %d\n", labels, v.Count)
		for i, b := range bounds {
			fmt.Fprintf(w, "talko_http_duration_seconds_bucket{%s,le=%q} %d\n", labels, fmt.Sprint(b), v.Buckets[i])
		}
		fmt.Fprintf(w, "talko_http_duration_seconds_bucket{%s,le=\"+Inf\"} %d\ntalko_http_duration_seconds_count{%s} %d\ntalko_http_duration_seconds_sum{%s} %g\n", labels, v.Count, labels, v.Count, labels, v.Sum)
	}
}
