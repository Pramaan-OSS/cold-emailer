package pramaan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestActionHeadersAndAsyncPolling(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing authorization")
		}
		if r.Method == "POST" {
			if r.Header.Get("Idempotency-Key") != "fb2d00af-1f6e-48cc-9a38-4df705002f69" {
				t.Error("lost action key")
			}
			if r.Header.Get("Pramaan-Account") != "account-ref" {
				t.Error("lost account")
			}
			w.WriteHeader(202)
			w.Write([]byte(`{"job_id":"job-1","status":"running"}`))
			return
		}
		if r.URL.Path != "/v1/jobs/job-1" {
			t.Error(r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"job_id": "job-1", "status": "done", "result": map[string]any{"ok": true}})
	}))
	defer s.Close()
	c, e := New("test-key", s.URL, nil)
	if e != nil {
		t.Fatal(e)
	}
	c.PollInterval = time.Millisecond
	var out struct {
		OK bool `json:"ok"`
	}
	if e = c.Call(context.Background(), "email.message.send", map[string]any{"to": "recipient@pramaan.dev"}, &out, Idempotency("fb2d00af-1f6e-48cc-9a38-4df705002f69"), Account("account-ref")); e != nil {
		t.Fatal(e)
	}
	if !out.OK || calls != 2 {
		t.Fatalf("result=%v calls=%d", out, calls)
	}
}
func TestNoActionRetryAndNoProviderLeak(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"vendor-secret provider-name $99"}`))
	}))
	defer s.Close()
	c, _ := New("test", s.URL, nil)
	e := c.Call(context.Background(), "email.send", map[string]any{}, nil)
	if e == nil || calls != 1 || strings.Contains(e.Error(), "vendor") {
		t.Fatalf("%v calls=%d", e, calls)
	}
}
func TestRedirectNeverForwardsKey(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("followed redirect") }))
	defer target.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer s.Close()
	c, _ := New("test", s.URL, nil)
	if c.Call(context.Background(), "web.search", nil, nil) == nil {
		t.Fatal("accepted redirect")
	}
}
func TestCancellationStopsJob(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		w.Write([]byte(`{"job_id":"job-1","status":"running"}`))
	}))
	defer s.Close()
	c, _ := New("test", s.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if c.Call(ctx, "web.search", nil, nil) == nil {
		t.Fatal("ignored cancellation")
	}
}
