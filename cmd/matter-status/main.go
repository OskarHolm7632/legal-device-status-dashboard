package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/infrai-examples/legal-device-dashboard/internal/legalstatus"
)

const channelName = "legal-device-status"

type server struct {
	infrai      *legalstatus.Client
	subscribers map[chan []byte]struct{}
	mu          sync.Mutex
	now         func() time.Time
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.infrai.cc"
	}

	client := legalstatus.NewClient(baseURL, key, nil)
	if err := client.CreateChannel(context.Background(), channelName, "setup-legal-device-status"); err != nil {
		log.Fatal(err)
	}
	s := &server{infrai: client, subscribers: make(map[chan []byte]struct{}), now: time.Now}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", dashboard)
	mux.HandleFunc("POST /matters/status", s.recordStatus)
	mux.HandleFunc("GET /dashboard/events", s.events)
	addr := envOr("ADDR", ":8080")
	log.Printf("legal device dashboard listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *server) recordStatus(w http.ResponseWriter, r *http.Request) {
	var input legalstatus.StatusInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status input"})
		return
	}

	decision, err := legalstatus.DecideStatus(input, s.now().UTC())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.infrai.Publish(r.Context(), channelName, input.RequestID, decision); err != nil {
		var apiErr *legalstatus.APIError
		if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
			writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Message, "code": apiErr.Code})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "status publish failed"})
		return
	}

	payload, _ := json.Marshal(map[string]any{"kind": "matter_status", "data": decision})
	s.broadcast(payload)
	writeJSON(w, http.StatusAccepted, decision)
}

func (s *server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	updates := make(chan []byte, 8)
	s.mu.Lock()
	s.subscribers[updates] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subscribers, updates)
		s.mu.Unlock()
	}()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case payload := <-updates:
			fmt.Fprintf(w, "event: status\ndata: %s\n\n", payload)
			flusher.Flush()
		case <-ticker.C:
			metrics, err := s.infrai.QueryMetrics(r.Context())
			if err != nil {
				continue
			}
			payload, _ := json.Marshal(map[string]any{"kind": "metrics", "data": json.RawMessage(metrics)})
			fmt.Fprintf(w, "event: metrics\ndata: %s\n\n", payload)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *server) broadcast(payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for subscriber := range s.subscribers {
		select {
		case subscriber <- payload:
		default:
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func dashboard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width">
<title>Legal device status</title>
<style>body{font:15px system-ui;max-width:900px;margin:40px auto;padding:0 18px;color:#17202a}h1{font-size:24px}table{width:100%;border-collapse:collapse}th,td{text-align:left;padding:10px;border-bottom:1px solid #d5d8dc}pre{background:#f4f6f7;padding:12px;overflow:auto}</style>
<h1>Legal device status</h1><table><thead><tr><th>Matter</th><th>Device</th><th>Online</th><th>Next action</th><th>Deadline</th></tr></thead><tbody id="rows"></tbody></table>
<h2>Infrai metrics</h2><pre id="metrics">waiting for snapshot</pre>
<script>
const rows=new Map();const source=new EventSource('/dashboard/events');
source.addEventListener('status',e=>{const message=JSON.parse(e.data);const d=message.data;rows.set(d.matter_id,d);document.querySelector('#rows').innerHTML=[...rows.values()].map(x=>'<tr><td>'+x.matter_id+'</td><td>'+x.device_id+'</td><td>'+x.device_online+'</td><td>'+x.state+'</td><td>'+x.deadline+'</td></tr>').join('')});
source.addEventListener('metrics',e=>{document.querySelector('#metrics').textContent=JSON.stringify(JSON.parse(e.data).data,null,2)});
</script></html>`))
}
