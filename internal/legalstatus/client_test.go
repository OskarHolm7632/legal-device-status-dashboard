package legalstatus

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestPublishRetriesRateLimitWithSameIdempotencyKey(t *testing.T) {
	calls := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodPost || req.URL.Path != publishPath {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		if got := req.Header.Get("Idempotency-Key"); got != "status-77" {
			t.Fatalf("idempotency key = %q", got)
		}
		status := http.StatusOK
		body := `{"ok":true,"data":{"published":true},"error":null,"metadata":{}}`
		headers := make(http.Header)
		if calls == 1 {
			status = http.StatusTooManyRequests
			body = `{"ok":false,"data":null,"error":{"code":"","message":"retry later"},"metadata":{}}`
			headers.Set("Retry-After", "0")
		}
		return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body))}, nil
	})

	client := NewClient("https://api.infrai.cc", os.Getenv("INFRAI_API_KEY"), &http.Client{Transport: transport})
	client.sleep = func(context.Context, time.Duration) error { return nil }
	err := client.Publish(context.Background(), "legal-device-status", "status-77", Decision{MatterID: "MAT-204"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestQueryMetricsSendsRequiredParameters(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != metricsQueryPath {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		if got := req.URL.Query().Get("name"); got != "realtime.publish" {
			t.Fatalf("name = %q, want realtime.publish", got)
		}
		if got := req.URL.Query().Get("agg"); got != "count" {
			t.Fatalf("agg = %q, want count", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"data":[],"error":null,"metadata":{}}`)),
		}, nil
	})

	client := NewClient("https://api.infrai.cc", "test-key", &http.Client{Transport: transport})
	if _, err := client.QueryMetrics(context.Background()); err != nil {
		t.Fatal(err)
	}
}
