package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdoptionTransportRefusalsAndAvailability(t *testing.T) {
	ids := []string{"email.message.append", "email.account.protocol-diagnose", "email.directory.tenant.resolve", "web.image.read", "workflow.webhook.signing-policy.create", "workflow.webhook.signing-policy.delete", "workflow.webhook.signed-call", "ai.judgment.calibrated"}
	for _, id := range ids {
		for _, mode := range []string{"success", "refused", "unavailable", "uncertain"} {
			t.Run(id+"/"+mode, func(t *testing.T) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Header.Get("Idempotency-Key") == "" {
						t.Error("missing action key")
					}
					if r.URL.Path != "/v1/capabilities/"+id {
						t.Error(r.URL.Path)
					}
					switch mode {
					case "refused":
						w.WriteHeader(400)
						_, _ = w.Write([]byte(`{"error":{"code":"invalid_request","effect":"none"}}`))
					case "unavailable":
						w.WriteHeader(404)
						_, _ = w.Write([]byte(`{"error":{"code":"not_found","effect":"none"}}`))
					case "uncertain":
						w.WriteHeader(502)
						_, _ = w.Write([]byte(`{"error":{"code":"upstream_unavailable","effect":"unknown"}}`))
					default:
						_ = json.NewEncoder(w).Encode(map[string]any{"status": "saved", "ref": "fixture"})
					}
				}))
				defer server.Close()
				c, _ := New("fake", server.URL, nil)
				var out map[string]any
				err := c.Call(context.Background(), id, map[string]any{}, &out, Idempotency("stable"))
				if (err == nil) != (mode == "success") || calls != 1 {
					t.Fatalf("mode=%s err=%v calls=%d", mode, err, calls)
				}
				if mode == "unavailable" && !errors.Is(err, ErrUnavailable) {
					t.Fatal(err)
				}
			})
		}
	}
}
func TestHeadersNeverDroppedOrReplayed(t *testing.T) {
	for _, headers := range []map[string]string{{"List-Unsubscribe-Post": "List-Unsubscribe=One-Click"}, {"X-One": "1", "X-Two": "2"}, {"X-One": "bad\r\nBcc: victim"}, {"Authorization": "secret"}} {
		if ValidateMailHeaders(headers) == nil {
			t.Fatal("accepted invalid headers")
		}
	}
	valid := map[string]string{"List-Unsubscribe": "<https://example.test/stop>", "List-Unsubscribe-Post": "List-Unsubscribe=One-Click", "X-Mailtrace-Verify": "nonce"}
	if err := ValidateMailHeaders(valid); err != nil {
		t.Fatal(err)
	}
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/capabilities/messaging.opt-out.check" {
			_, _ = w.Write([]byte(`{"blocked":false}`))
			return
		}
		count++
		var in struct {
			Headers map[string]string `json:"headers"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if len(in.Headers) != 3 {
			t.Fatal("headers dropped")
		}
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_request","effect":"none","message":"headers refused"}}`))
	}))
	defer server.Close()
	c, _ := New("fake", server.URL, nil)
	_, err := (MailService{Client: c}).Send(context.Background(), MailRequest{From: "a@example.test", To: []string{"synthetic@fixture.email"}, Text: "body unsubscribe link", Headers: valid}, "stable")
	if !errors.Is(err, ErrMailHeadersRefused) || count != 1 {
		t.Fatalf("%v calls=%d", err, count)
	}
}
func TestAdoptionOwnershipAndAdminGates(t *testing.T) {
	fake := &responseCaller{response: map[string]any{"status": "saved", "ref": "saved"}}
	registry := memoryRegistry{"one": {"email_folder:folder": true, "webhook_policy:policy": true}}
	g := Gateway{Client: fake, Registry: registry}
	ctx := context.Background()
	if _, err := g.AdoptionCall(ctx, "two", "owner", "email.message.append", map[string]any{"app": "gmail", "ref": "folder", "content": "QQ=="}, "attempt"); err == nil || fake.calls != 0 {
		t.Fatal("foreign folder accepted")
	}
	if _, err := g.AdoptionCall(ctx, "one", "member", "workflow.webhook.signing-policy.delete", map[string]any{"ref": "policy"}, "attempt"); err == nil || fake.calls != 0 {
		t.Fatal("member deleted policy")
	}
	if _, err := g.AdoptionCall(ctx, "one", "owner", "email.message.append", map[string]any{"app": "gmail", "ref": "folder", "content": "QQ==", "endUserRef": "foreign"}, "attempt"); err != nil {
		t.Fatal(err)
	}
	if fake.input["endUserRef"] != "org_one" || !g.owns(ctx, "one", "message", "saved") {
		t.Fatal("append not scoped or saved")
	}
	if _, err := g.AdoptionCall(ctx, "one", "owner", "web.image.read", map[string]any{"url": "https://example.test/image.png"}, "attempt"); err == nil || fake.calls != 1 {
		t.Fatal("image imported without consent")
	}
}
func TestJudgmentRejectsPartialAndInvalidDistribution(t *testing.T) {
	options := []JudgmentOption{{"yes", "Yes"}, {"no", "No"}}
	for _, results := range [][]JudgmentProbability{{{"yes", .8}}, {{"yes", .8}, {"no", .8}}, {{"no", .5}, {"yes", .5}}, {{"yes", .8}, {"no", .2}}} {
		f := &responseCaller{response: map[string]any{"results": results}}
		_, err := Judgment(context.Background(), f, "Question?", "facts", options)
		valid := len(results) == 2 && results[0].OptionID == "yes" && results[1].Probability == .2
		if (err == nil) != valid {
			t.Fatalf("results=%v err=%v", results, err)
		}
	}
}
func TestCompanyRoutesAndWorkerSelection(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/companies/mail/connect" {
			t.Error(r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"companyRef": "company"})
	}))
	defer server.Close()
	client, _ := New("fake", server.URL, nil)
	g := Gateway{Client: client, Registry: memoryRegistry{}}
	if _, err := g.CompanyCall(context.Background(), "one", "member", "connect", map[string]any{"kind": "workspace", "domain": "example.test"}, "stable"); err == nil || calls != 0 {
		t.Fatal("member connected company")
	}
	if _, err := g.CompanyCall(context.Background(), "one", "owner", "connect", map[string]any{"kind": "workspace", "domain": "example.test"}, "stable"); err != nil || calls != 1 {
		t.Fatal(err)
	}
	if !g.owns(context.Background(), "one", "company", "company") {
		t.Fatal("company ownership not saved")
	}
	f := &responseCaller{response: map[string]any{}}
	box := CompanyMailbox{Client: f, CompanyRef: "company", Mailbox: "a@example.test"}
	if err := box.Call(context.Background(), "email.messages.list", map[string]any{"inbox": "a@example.test"}, nil); err != nil {
		t.Fatal(err)
	}
	if f.input["companyRef"] != "company" || f.input["mailbox"] != "a@example.test" || f.input["inbox"] != nil {
		t.Fatal(f.input)
	}
}

func TestSendRequestKeyRefusalIsNotMislabelledAsMailHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_request","message":"Idempotency-Key header is required."}}`))
	}))
	defer server.Close()
	c, _ := New("fake", server.URL, nil)
	err := c.Call(context.Background(), "email.send", map[string]any{"to": []string{"synthetic@fixture.email"}}, nil)
	if !errors.Is(err, ErrIdempotencyRequired) {
		t.Fatal(err)
	}
}
