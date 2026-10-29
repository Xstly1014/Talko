package alertbridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoveryAndRejections(t *testing.T) {
	message := ""
	status := 200
	result := map[string]any{"code": 0}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p struct{ Content struct{ Text string } }
		_ = json.NewDecoder(r.Body).Decode(&p)
		message = p.Content.Text
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer up.Close()
	h := Handler{Token: "token", Sender: Sender{URL: up.URL, Secret: "secret", Client: up.Client()}}
	payload, _ := json.Marshal(map[string]any{"alerts": []any{map[string]any{"status": "resolved", "labels": map[string]string{"alertname": "WorkerDown", "job": "worker"}, "annotations": map[string]string{"summary": "scrape recovered", "private": "never forward user data"}}}})
	call := func(b string) int {
		r := httptest.NewRequest("POST", "/alerts", strings.NewReader(b))
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if call(string(payload)) != 204 || !strings.Contains(message, "恢复") || strings.Contains(message, "never forward") {
		t.Fatal("recovery formatting or allowlist failed", message)
	}
	result = map[string]any{}
	if call(string(payload)) != 502 {
		t.Fatal("missing upstream code accepted")
	}
	status = 503
	if call(string(payload)) != 502 {
		t.Fatal("upstream HTTP failure accepted")
	}
	if call(string(payload)+"{}") == 204 {
		t.Fatal("trailing JSON accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if h.Sender.Send(ctx, "test") == nil {
		t.Fatal("cancelled request acknowledged")
	}
}

func TestSignatureVector(t *testing.T) {
	if Signature("fixture-secret", 1700000000) != "eSSBpPPdWfBl7avPl9BWSBfQOnLGZ91Xn8G5oZis0sc=" {
		t.Fatal("Feishu signing vector changed")
	}
}
