package alertbridge

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Sender struct {
	URL, Secret string
	Client      *http.Client
}

func Signature(secret string, ts int64) string {
	h := hmac.New(sha256.New, []byte(fmt.Sprintf("%d\n%s", ts, secret)))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
func (s Sender) Send(ctx context.Context, text string) error {
	ts := time.Now().Unix()
	body, _ := json.Marshal(map[string]any{"timestamp": fmt.Sprint(ts), "sign": Signature(s.Secret, ts), "msg_type": "text", "content": map[string]string{"text": text}})
	r, e := http.NewRequestWithContext(ctx, "POST", s.URL, bytes.NewReader(body))
	if e != nil {
		return fmt.Errorf("invalid upstream configuration")
	}
	r.Header.Set("Content-Type", "application/json")
	resp, e := s.Client.Do(r)
	if e != nil {
		return fmt.Errorf("upstream unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("upstream HTTP failure")
	}
	var result struct{ Code *int }
	if e = json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&result); e != nil || result.Code == nil || *result.Code != 0 {
		return fmt.Errorf("upstream rejected notification")
	}
	return nil
}

type Alert struct {
	Status      string
	Labels      map[string]string
	Annotations map[string]string
}
type Handler struct {
	Token  string
	Sender Sender
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || h.Token == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(h.Token)) != 1 {
		http.Error(w, "unauthorized", 401)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	var in struct {
		Status string
		Alerts []Alert
	}
	d := json.NewDecoder(r.Body)
	if d.Decode(&in) != nil || len(in.Alerts) == 0 || len(in.Alerts) > 100 {
		http.Error(w, "invalid alert payload", 400)
		return
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		http.Error(w, "invalid alert payload", 400)
		return
	}
	for _, a := range in.Alerts {
		if (a.Status != "firing" && a.Status != "resolved") || a.Labels["alertname"] == "" {
			http.Error(w, "invalid alert status or name", 400)
			return
		}
	}
	var b strings.Builder
	b.WriteString("Talko 运行告警\n")
	for _, a := range in.Alerts {
		status := "触发"
		if a.Status == "resolved" {
			status = "恢复"
		}
		fmt.Fprintf(&b, "[%s] %s\n服务：%s；级别：%s\n%s\n", status, a.Labels["alertname"], a.Labels["job"], a.Labels["severity"], a.Annotations["summary"])
	}
	if b.Len() > 12000 {
		http.Error(w, "alert message too large", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	if h.Sender.Send(ctx, b.String()) != nil {
		http.Error(w, "notification delivery failed", 502)
		return
	}
	w.WriteHeader(204)
}
