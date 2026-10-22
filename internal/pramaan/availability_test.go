package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnavailableCapabilityGroupsNeverSucceedOrLeak(t *testing.T) {
	ids := []string{"email.inbox.create", "email.inbox.get", "email.inbox.update", "email.inbox.delete", "email.inboxes.list", "email.domain.create", "email.domain.get", "email.domain.verify", "email.domain.update", "email.domain.delete", "email.domains.list", "email.domain.zonefile.get", "email.domain.setuplink.get", "email.accounts.list", "email.account.connect.start", "email.account.connect.status", "email.messages.list", "email.messages.search", "email.message.get", "email.threads.list", "email.status", "email.message.send", "email.message.reply", "domains.registration.search", "domains.registration.quote", "domains.registration.create", "domains.registration.get", "domains.registration.list", "domains.registration.renew", "domains.registration.update", "domains.registration.contacts-set", "domains.registration.verification-resend", "domains.registration.transfer-out-prepare"}
	// Include every company capability the product exposes, even if only some are unavailable today.
	for id := range publicReads {
		if strings.HasPrefix(id, "companies.") {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			for _, effect := range []string{"none", "unknown"} {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(502)
					_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "upstream_unavailable", "effect": effect, "message": "private service details and stack trace"}})
				}))
				c, _ := New("fixture", server.URL, nil)
				var out map[string]any
				e := c.Call(context.Background(), id, map[string]any{}, &out)
				server.Close()
				if e == nil || out != nil || calls != 1 || !strings.Contains(e.Error(), "not available yet") || strings.Contains(e.Error(), "private") || strings.Contains(e.Error(), "stack trace") {
					t.Fatal(id, e, out, calls)
				}
				if strings.HasPrefix(id, "domains.registration.") && effect == "unknown" && errors.Is(e, ErrDomainUnavailable) {
					t.Fatal("uncertain purchase treated as no-effect")
				}
			}
		})
	}
	t.Logf("checked %d mailbox/domain/company capability IDs", len(ids))
}
func TestGenerationMessagesAndSmallTokenBudgets(t *testing.T) {
	for _, id := range []string{"llm.chat", "llm.complete"} {
		messages := []any{map[string]any{"role": "user", "content": "Synthetic classification"}}
		for _, budget := range []int{8, 16, 64, 512} {
			v, e := PrepareInput(id, map[string]any{"messages": messages, "maxTokens": budget})
			if e != nil {
				t.Fatal(e)
			}
			b, _ := json.Marshal(v)
			var out map[string]any
			_ = json.Unmarshal(b, &out)
			if out["maxTokens"].(float64) < 64 {
				t.Fatal(v)
			}
			if e = ValidateInput(id, v); e != nil {
				t.Fatal(e)
			}
			if budget < 64 && ValidateInput(id, map[string]any{"messages": messages, "maxTokens": budget}) == nil {
				t.Fatal("strict fake accepted small budget")
			}
		}
		if _, e := PrepareInput(id, map[string]any{"prompt": "not a transport field"}); e == nil {
			t.Fatal("accepted prompt instead of messages")
		}
		if _, e := PrepareInput(id, map[string]any{}); e == nil {
			t.Fatal("accepted missing messages")
		}
	}
}

func TestUnavailableAsyncJobNeverCompletes(t *testing.T) {
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts++
			w.WriteHeader(202)
			_, _ = w.Write([]byte(`{"job_id":"fixture","status":"queued"}`))
			return
		}
		_, _ = w.Write([]byte(`{"job_id":"fixture","status":"failed","error":{"code":"upstream_unavailable","effect":"none","message":"private service detail"}}`))
	}))
	defer server.Close()
	c, _ := New("fixture", server.URL, nil)
	c.PollInterval = 0
	e := c.Call(context.Background(), "companies.search", map[string]any{"query": "synthetic"}, nil)
	if !errors.Is(e, ErrUnavailable) || !strings.Contains(e.Error(), "not available yet") || posts != 1 {
		t.Fatal(e, posts)
	}
}
