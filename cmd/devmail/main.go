// devmail is a loopback-only SMTP inbox for local development; never deploy publicly.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"
)

type Message struct {
	To   string    `json:"to"`
	Body string    `json:"body"`
	At   time.Time `json:"at"`
}

var mu sync.Mutex
var messages = []Message{}

func main() {
	ln, e := net.Listen("tcp", "127.0.0.1:1025")
	if e != nil {
		log.Fatal(e)
	}
	go func() {
		for {
			c, e := ln.Accept()
			if e != nil {
				return
			}
			go serve(c)
		}
	}()
	http.HandleFunc("/api/messages", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(messages)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><title>Talko 开发邮箱</title><h1>本地开发邮箱</h1><p>仅用于开发，不能接收真实外部邮件。</p><pre id="mail"></pre><script>fetch('/api/messages').then(r=>r.json()).then(m=>document.getElementById('mail').textContent=m.map(x=>x.to+'\n'+x.body).join('\n\n'))</script>`)
	})
	log.Println("Development inbox http://localhost:8025")
	log.Fatal(http.ListenAndServe("127.0.0.1:8025", nil))
}
func serve(c net.Conn) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(30 * time.Second))
	r := textproto.NewReader(bufio.NewReader(c))
	w := bufio.NewWriter(c)
	reply := func(s string) { fmt.Fprint(w, s+"\r\n"); _ = w.Flush() }
	reply("220 Talko development SMTP")
	to := ""
	for {
		line, e := r.ReadLine()
		if e != nil {
			return
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			reply("250 localhost")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			reply("250 OK")
		case strings.HasPrefix(upper, "RCPT TO:"):
			to = line[len("RCPT TO:"):]
			reply("250 OK")
		case upper == "DATA":
			reply("354 End with .")
			b, e := io.ReadAll(io.LimitReader(r.DotReader(), 100000))
			if e != nil {
				return
			}
			mu.Lock()
			messages = append([]Message{{To: to, Body: string(b), At: time.Now()}}, messages...)
			if len(messages) > 100 {
				messages = messages[:100]
			}
			mu.Unlock()
			reply("250 Queued")
		case upper == "QUIT":
			reply("221 Bye")
			return
		default:
			reply("250 OK")
		}
	}
}
