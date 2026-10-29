package alertbridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestDeliveryContract(t *testing.T) {
	code := 0
	calls := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var p struct{ Timestamp, Sign string }
		if json.NewDecoder(r.Body).Decode(&p) != nil {
			t.Error("bad request")
		}
		ts, _ := strconv.ParseInt(p.Timestamp, 10, 64)
		if p.Sign != Signature("secret", ts) {
			t.Error("bad signature")
		}
		json.NewEncoder(w).Encode(map[string]int{"code": code})
	}))
	defer up.Close()
	h := Handler{Token: "internal-test-token", Sender: Sender{URL: up.URL, Secret: "secret", Client: up.Client()}}
	body := "{\"status\":\"firing\",\"alerts\":[{\"status\":\"firing\",\"labels\":{\"alertname\":\"OutboxDelayed\"},\"annotations\":{\"summary\":\"test\"}}]}"
	call := func(token string) int {
		r := httptest.NewRequest("POST", "/alerts", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if call("wrong") != 401 || calls != 0 {
		t.Fatal("unauthorized sent notification")
	}
	if call(h.Token) != 204 {
		t.Fatal("valid notification failed")
	}
	code = 19021
	if call(h.Token) != 502 {
		t.Fatal("business rejection falsely acknowledged")
	}
	body = "{}"
	if call(h.Token) != 400 {
		t.Fatal("invalid payload accepted")
	}
}
