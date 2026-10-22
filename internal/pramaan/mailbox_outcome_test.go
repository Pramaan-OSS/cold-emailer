package pramaan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mailboxRegistry struct{ memoryRegistry }

func (m mailboxRegistry) List(_ context.Context, owner, kind string) ([]string, error) {
	refs := []string{}
	for key := range m.memoryRegistry[owner] {
		if strings.HasPrefix(key, kind+":") {
			refs = append(refs, strings.TrimPrefix(key, kind+":"))
		}
	}
	return refs, nil
}

func TestMailboxOutcomeScopesInputsAndSavesPartialProgress(t *testing.T) {
	ctx := context.Background()
	registry := mailboxRegistry{memoryRegistry{}}
	posts := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write([]byte(`{"available":true}`))
			return
		}
		var in OutcomeRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Error(err)
		}
		if in.Input["endUserRef"] != "org_workspace" {
			t.Errorf("unscoped input: %v", in.Input)
		}
		if in.Input["ref"] == nil && in.Input["returnUrl"] != "https://app.pramaan.dev/return" {
			t.Errorf("wrong return page: %v", in.Input)
		}
		if in.Input["ref"] != nil && in.Input["ref"] != "connection-one" {
			t.Errorf("wrong connection: %v", in.Input)
		}
		if r.URL.Path == "/v1/outcomes/mailbox.connect/estimate" {
			w.Write([]byte(`{"plan":"mail-plan"}`))
			return
		}
		posts++
		if r.Header.Get("Pramaan-Plan") != "mail-plan" {
			t.Error("missing approved plan")
		}
		// A failed read test must retain connected account progress.
		w.Write([]byte(`{"outcome":"mailbox.connect","status":"done","result":{"ref":"connection-one","status":"connected","accountRef":"account-one","inbox":"mailbox@pramaan.dev"},"completeness":"partial"}`))
	}))
	defer s.Close()
	client, _ := New("fake", s.URL, nil)
	g := Gateway{Client: client, Registry: registry, ReturnURL: "https://app.pramaan.dev/return"}
	for _, in := range []OutcomeRequest{{Input: map[string]any{"endUserRef": "other"}}, {Input: map[string]any{"ref": "unowned"}}, {Input: map[string]any{"returnUrl": "https://other.test"}}, {Input: map[string]any{}, Options: map[string]any{"override": true}}} {
		if _, err := g.OutcomeEstimate(ctx, "workspace", "member", "mailbox.connect", in); err == nil {
			t.Fatal("routing override accepted")
		}
	}
	start := OutcomeRequest{Input: map[string]any{}}
	if _, err := g.OutcomeEstimate(ctx, "workspace", "member", "mailbox.connect", start); err != nil {
		t.Fatal(err)
	}
	for _, actor := range []string{"member", "other"} {
		plan := ""
		if actor == "other" {
			plan = "mail-plan"
		}
		if _, _, err := g.OutcomeStart(ctx, "workspace", actor, "mailbox.connect", start, "attempt", plan); err == nil {
			t.Fatal("unapproved run")
		}
	}
	if posts != 0 {
		t.Fatal("ran before approval")
	}
	if _, _, err := g.OutcomeStart(ctx, "workspace", "member", "mailbox.connect", start, "attempt", "mail-plan"); err != nil {
		t.Fatal(err)
	}
	for kind, ref := range map[string]string{"mail_connection": "connection-one", "account": "account-one", "inbox": "mailbox@pramaan.dev"} {
		if !g.owns(ctx, "workspace", kind, ref) || g.owns(ctx, "other", kind, ref) {
			t.Fatal("ownership not preserved", kind)
		}
	}
	check := OutcomeRequest{Input: map[string]any{"check": true}}
	if _, err := g.OutcomeEstimate(ctx, "other", "member", "mailbox.connect", check); err == nil {
		t.Fatal("cross-workspace check")
	}
	if _, err := g.OutcomeEstimate(ctx, "workspace", "member", "mailbox.connect", check); err != nil {
		t.Fatal(err)
	}
	if _, _, err := g.OutcomeStart(ctx, "workspace", "member", "mailbox.connect", check, "check-attempt", "mail-plan"); err != nil {
		t.Fatal(err)
	}
	if posts != 2 {
		t.Fatal("unexpected submissions", posts)
	}
}

func TestMailboxOutcomeApprovalRequiredIsNotRetried(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(403)
		w.Write([]byte(`{"error":{"code":"forbidden","reason":"approval_required","effect":"none","plan":"new-plan","needs":["test"]}}`))
	}))
	defer s.Close()
	c, _ := New("fake", s.URL, nil)
	_, _, err := c.OutcomeStart(context.Background(), "mailbox.connect", OutcomeRequest{Input: map[string]any{}}, "one-attempt", "approved-plan")
	refusal, ok := err.(*HTTPError)
	if !ok || refusal.Status != 403 || refusal.Code != "forbidden" || calls != 1 {
		t.Fatal(err, calls)
	}
}

func TestMailboxOutcomeJobRetainsConnectionAndScopesPolling(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/jobs/mail-job":
			w.Write([]byte(`{"status":"done","result":{"outcome":"mailbox.connect","status":"done","result":{"ref":"saved-connection","status":"connected","accountRef":"saved-account","inbox":"mailbox@pramaan.dev","tested":true}}}`))
		case r.Method == "GET":
			w.Write([]byte(`{"available":true}`))
		case strings.HasSuffix(r.URL.Path, "/estimate"):
			w.Write([]byte(`{"plan":"mail-plan"}`))
		default:
			w.WriteHeader(202)
			w.Write([]byte(`{"job_id":"mail-job","status":"running"}`))
		}
	}))
	defer s.Close()
	c, _ := New("fake", s.URL, nil)
	g := Gateway{Client: c, Registry: memoryRegistry{}, ReturnURL: "https://app.pramaan.dev/return"}
	ctx := context.Background()
	in := OutcomeRequest{Input: map[string]any{}}
	if _, err := g.OutcomeEstimate(ctx, "one", "member", "mailbox.connect", in); err != nil {
		t.Fatal(err)
	}
	if status, _, err := g.OutcomeStart(ctx, "one", "member", "mailbox.connect", in, "attempt", "mail-plan"); err != nil || status != 202 {
		t.Fatal(status, err)
	}
	if _, err := g.OutcomeJob(ctx, "two", "member", "mail-job"); err == nil {
		t.Fatal("cross-workspace job read")
	}
	if _, err := g.OutcomeJob(ctx, "one", "other", "mail-job"); err == nil {
		t.Fatal("cross-member job read")
	}
	if _, err := g.OutcomeJob(ctx, "one", "member", "mail-job"); err != nil {
		t.Fatal(err)
	}
	if !g.owns(ctx, "one", "mail_connection", "saved-connection") || !g.owns(ctx, "one", "account", "saved-account") || !g.owns(ctx, "one", "inbox", "mailbox@pramaan.dev") {
		t.Fatal("job did not register mailbox ownership")
	}
}
