package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOutcomeTransportEstimateRunAndPolling(t *testing.T) {
	posts, polls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range []string{"Pramaan-Account", "Pramaan-End-User", "Pramaan-End-User-Group", "Pramaan-Max-Price-Micro"} {
			if r.Header.Get(h) != "" {
				t.Errorf("forbidden header %s", h)
			}
		}
		switch r.URL.Path {
		case "/v1/outcomes":
			w.Write([]byte(`{"results":[{"id":"lead.find","available":true}]}`))
		case "/v1/outcomes/lead.find/estimate":
			var b OutcomeRequest
			json.NewDecoder(r.Body).Decode(&b)
			if b.Input["description"] != "software companies" || b.Basis["ground"] != "consent" {
				t.Errorf("lost request: %+v", b)
			}
			w.Write([]byte(`{"plan":"plan-1","ready":true,"can_run":true,"price_max_micro":1200,"steps":[]}`))
		case "/v1/outcomes/lead.find":
			if r.Method == "GET" {
				w.Write([]byte(`{"id":"lead.find","available":true}`))
				return
			}
			posts++
			if r.Header.Get("Pramaan-Plan") != "plan-1" || r.Header.Get("Idempotency-Key") != "run-1" {
				t.Error("lost approval/key")
			}
			w.WriteHeader(202)
			w.Write([]byte(`{"job_id":"job-1","status":"running"}`))
		case "/v1/jobs/job-1":
			polls++
			if polls == 1 {
				w.Write([]byte(`{"status":"running"}`))
			} else {
				w.Write([]byte(`{"status":"done","result":{"status":"done","result":{"leads":[]},"steps":[],"cost":{"charged_micro":10}}}`))
			}
		default:
			t.Error(r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c, _ := New("fake", server.URL, nil)
	c.PollInterval = time.Millisecond
	ctx := context.Background()
	if _, e := c.OutcomesList(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e := c.OutcomeGet(ctx, "lead.find"); e != nil {
		t.Fatal(e)
	}
	in := OutcomeRequest{Input: map[string]any{"description": "software companies"}, Basis: map[string]any{"ground": "consent"}}
	estimate, e := c.OutcomeEstimate(ctx, "lead.find", in)
	if e != nil || estimate["plan"] != "plan-1" {
		t.Fatal(estimate, e)
	}
	out, e := c.OutcomeRun(ctx, "lead.find", in, "run-1", "plan-1")
	if e != nil || out["status"] != "done" || posts != 1 || polls != 2 {
		t.Fatal(out, e, posts, polls)
	}
}
func TestOutcomeRefusalsAndUncertainResultsAreNotRetried(t *testing.T) {
	for _, status := range []int{403, 409, 502, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(status)
				code := "internal"
				if status == 502 {
					code = "upstream_unavailable"
				}
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": "private-upstream-details"}})
			}))
			defer s.Close()
			c, _ := New("fake", s.URL, nil)
			_, e := c.OutcomeRun(context.Background(), "lead.find", OutcomeRequest{Input: map[string]any{}}, "key", "plan")
			if e == nil || calls != 1 {
				t.Fatal(e, calls)
			}
			if status == 502 && !errors.Is(e, ErrOutcomeUnavailable) {
				t.Fatal(e)
			}
			if status == 403 || status == 409 {
				var he *HTTPError
				if !errors.As(e, &he) || he.Status != status {
					t.Fatal(e)
				}
			}
		})
	}
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"status":"failed","steps":[{"effect":"unknown"}],"error":{"code":"internal"}}`))
	}))
	defer s.Close()
	c, _ := New("fake", s.URL, nil)
	out, e := c.OutcomeRun(context.Background(), "lead.find", OutcomeRequest{Input: map[string]any{}}, "key", "plan")
	if e != nil || out["status"] != "failed" || calls != 1 {
		t.Fatal(out, e, calls)
	}
}
func TestOutcomeGatewayBindsApprovalAndJobsToMemberAndInput(t *testing.T) {
	calls := 0
	available := true
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET":
			json.NewEncoder(w).Encode(map[string]any{"available": available})
		case r.URL.Path == "/v1/outcomes/lead.find/estimate":
			w.Write([]byte(`{"plan":"plan-1"}`))
		default:
			calls++
			w.WriteHeader(202)
			w.Write([]byte(`{"job_id":"job-1","status":"running"}`))
		}
	}))
	defer s.Close()
	c, _ := New("fake", s.URL, nil)
	g := Gateway{Client: c, Registry: memoryRegistry{}}
	ctx := context.Background()
	in := OutcomeRequest{Input: map[string]any{"description": "software companies"}}
	if _, e := g.OutcomeEstimate(ctx, "org-1", "member-1", "lead.find", in); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		owner, actor, plan string
		in                 OutcomeRequest
		status             int
	}{{"org-1", "member-1", "", in, 403}, {"org-2", "member-1", "plan-1", in, 409}, {"org-1", "member-2", "plan-1", in, 409}, {"org-1", "member-1", "plan-1", OutcomeRequest{Input: map[string]any{"description": "changed"}}, 409}} {
		status, _, e := g.OutcomeStart(ctx, tc.owner, tc.actor, "lead.find", tc.in, "key", tc.plan)
		if e == nil || status != tc.status {
			t.Fatal(status, e)
		}
	}
	available = false
	_, _, e := g.OutcomeStart(ctx, "org-1", "member-1", "lead.find", in, "key", "plan-1")
	if !errors.Is(e, ErrOutcomeUnavailable) || calls != 0 {
		t.Fatal(e, calls)
	}
	available = true
	status, _, e := g.OutcomeStart(ctx, "org-1", "member-1", "lead.find", in, "key", "plan-1")
	if e != nil || status != 202 || calls != 1 {
		t.Fatal(status, e, calls)
	}
	if _, e = g.OutcomeJob(ctx, "org-2", "member-1", "job-1"); e == nil {
		t.Fatal("cross-workspace job")
	}
	if _, e = g.OutcomeJob(ctx, "org-1", "member-2", "job-1"); e == nil {
		t.Fatal("cross-member job")
	}
}

func TestOutcomeCapacityAndCancellation(t *testing.T) {
	posts, polls := 0, 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/jobs/job-1/cancel" {
			w.Write([]byte(`{"job_id":"job-1","status":"cancelled"}`))
			return
		}
		if r.Method == "POST" {
			posts++
			w.WriteHeader(202)
			w.Write([]byte(`{"job_id":"job-1","status":"running"}`))
			return
		}
		polls++
		if polls == 1 {
			w.WriteHeader(429)
			w.Write([]byte(`{"error":{"code":"rate_limited","effect":"none"}}`))
			return
		}
		w.Write([]byte(`{"job_id":"job-1","status":"done","result":{"status":"done"}}`))
	}))
	defer s.Close()
	c, _ := New("fake", s.URL, nil)
	c.PollInterval = time.Millisecond
	if _, e := c.OutcomeRun(context.Background(), "lead.find", OutcomeRequest{Input: map[string]any{}}, "key", "plan"); e != nil || posts != 1 || polls != 2 {
		t.Fatal(e, posts, polls)
	}
	g := Gateway{Client: c, Registry: memoryRegistry{"one": {"outcome_job_member:job-1": true}}}
	if _, e := g.OutcomeCancel(context.Background(), "two", "member", "job-1"); e == nil {
		t.Fatal("cross-workspace cancellation")
	}
	out, e := g.OutcomeCancel(context.Background(), "one", "member", "job-1")
	if e != nil || out["status"] != "cancelled" {
		t.Fatal(out, e)
	}
}

func TestLongOutcomeTerminalReportIsPreserved(t *testing.T) {
	for _, terminal := range []string{"failed", "cancelled"} {
		t.Run(terminal, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					posts++
					w.WriteHeader(202)
					w.Write([]byte(`{"job_id":"long-1"}`))
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"status": terminal, "result": map[string]any{"status": terminal, "steps": []any{map[string]any{"effect": "unknown"}}}})
			}))
			defer server.Close()
			c, _ := New("fake", server.URL, nil)
			c.PollInterval = time.Millisecond
			report, err := c.OutcomeRun(context.Background(), "lead.find", OutcomeRequest{Input: map[string]any{}}, "one-attempt", "plan")
			if err != nil || report["status"] != terminal || posts != 1 {
				t.Fatal(report, err, posts)
			}
			if report["steps"].([]any)[0].(map[string]any)["effect"] != "unknown" {
				t.Fatal(report)
			}
		})
	}
}
func TestLongOutcomeUpstreamTimeoutIsNotResubmitted(t *testing.T) {
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts++
			w.WriteHeader(202)
			w.Write([]byte(`{"job_id":"long-1"}`))
			return
		}
		w.Write([]byte(`{"status":"failed","error":{"code":"upstream_timeout"}}`))
	}))
	defer server.Close()
	c, _ := New("fake", server.URL, nil)
	c.PollInterval = time.Millisecond
	_, err := c.OutcomeRun(context.Background(), "lead.find", OutcomeRequest{Input: map[string]any{}}, "one-attempt", "plan")
	var refused *HTTPError
	if !errors.As(err, &refused) || refused.Code != "upstream_timeout" || refused.Effect != "unknown" || posts != 1 {
		t.Fatal(err, posts)
	}
}
