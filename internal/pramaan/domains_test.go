package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

type domainFake struct {
	calls       []string
	inputs      []map[string]any
	keys        []string
	fail        bool
	unavailable bool
}

func (f *domainFake) Call(_ context.Context, id string, in, out any, opts ...Option) error {
	request := in.(map[string]any)
	f.calls = append(f.calls, id)
	f.inputs = append(f.inputs, request)
	o := callOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	f.keys = append(f.keys, o.key)
	if f.unavailable {
		return ErrDomainUnavailable
	}
	var value any
	switch id {
	case "domains.registration.search":
		value = map[string]any{"results": []map[string]any{{"domain": "outreach.example", "available": true, "purchaseMode": "registration", "years": 1}}}
	case "domains.registration.quote":
		value = map[string]any{"domain": "outreach.example", "action": "register", "years": 1, "price": map[string]any{"amount": 1500, "currency": "USD"}}
	case "domains.registration.create":
		if f.fail {
			return errors.New("response lost")
		}
		value = map[string]any{"domain": "outreach.example", "ref": "owned-ref", "state": "active"}
	case "domains.registration.list":
		value = map[string]any{"results": []map[string]any{{"domain": "outreach.example", "ref": "owned-ref"}, {"domain": "other.example", "ref": "unrelated-ref"}}}
	case "domains.registration.get":
		value = map[string]any{"domain": "outreach.example", "state": "active"}
	default:
		value = map[string]any{}
	}
	b, _ := json.Marshal(value)
	return json.Unmarshal(b, out)
}
func (m memoryRegistry) Claim(ctx context.Context, owner, kind, handle string) (bool, error) {
	for _, rows := range m {
		if rows[kind+":"+handle] {
			return false, nil
		}
	}
	return true, m.Save(ctx, owner, kind, handle)
}
func domainSetup() (*Gateway, *domainFake) {
	f := &domainFake{}
	return &Gateway{Client: f, Registry: memoryRegistry{}}, f
}
func quoteInput(t *testing.T, g *Gateway) map[string]any {
	t.Helper()
	q, e := g.DomainCall(context.Background(), "one", "person", "owner", "domains.registration.quote", map[string]any{"domain": "outreach.example", "action": "register", "years": 1}, uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	delete(q, "action")
	q["registrant"] = map[string]any{"email": "synthetic@example.test"}
	return q
}
func TestDomainSearchQuoteConfirmAndCreateOnce(t *testing.T) {
	g, f := domainSetup()
	ctx := context.Background()
	if _, e := g.DomainCall(ctx, "one", "person", "admin", "domains.registration.search", map[string]any{"keyword": "outreach"}, uuid.NewString()); e != nil {
		t.Fatal(e)
	}
	in := quoteInput(t, g)
	key := uuid.NewString()
	out, e := g.DomainCall(ctx, "one", "person", "owner", "domains.registration.create", in, key)
	if e != nil || out["ref"] != "owned-ref" {
		t.Fatalf("%v %v", out, e)
	}
	if f.keys[len(f.keys)-1] != key || f.inputs[len(f.inputs)-1]["confirmation"] != nil {
		t.Fatal("lost idempotency or leaked local confirmation")
	}
	before := len(f.calls)
	_, e = g.DomainCall(ctx, "one", "person", "owner", "domains.registration.create", in, uuid.NewString())
	if !errors.Is(e, ErrDomainUnknown) || len(f.calls) != before {
		t.Fatal("duplicate purchase sent")
	}
	if !g.owns(ctx, "one", "registration", "owned-ref") {
		t.Fatal("ownership not saved")
	}
}
func TestDomainRefusesRoleMissingConfirmationAndChangedQuote(t *testing.T) {
	g, f := domainSetup()
	in := quoteInput(t, g)
	before := len(f.calls)
	for _, role := range []string{"manager", "viewer", "", "Owner"} {
		if _, e := g.DomainCall(context.Background(), "one", "person", role, "domains.registration.create", in, uuid.NewString()); e == nil {
			t.Fatal("role allowed", role)
		}
	}
	missing := map[string]any{"domain": in["domain"], "price": in["price"], "years": in["years"]}
	if _, e := g.DomainCall(context.Background(), "one", "person", "owner", "domains.registration.create", missing, uuid.NewString()); e == nil {
		t.Fatal("confirmation bypassed")
	}
	in["price"] = map[string]any{"amount": 1, "currency": "USD"}
	if _, e := g.DomainCall(context.Background(), "one", "person", "owner", "domains.registration.create", in, uuid.NewString()); e == nil {
		t.Fatal("quote changed")
	}
	if len(f.calls) != before {
		t.Fatal("refused request reached platform")
	}
}
func TestDomainUnknownResultRecoversByListThenGetNeverResends(t *testing.T) {
	g, f := domainSetup()
	in := quoteInput(t, g)
	f.fail = true
	_, e := g.DomainCall(context.Background(), "one", "person", "owner", "domains.registration.create", in, uuid.NewString())
	if !errors.Is(e, ErrDomainUnknown) {
		t.Fatal(e)
	}
	_, e = g.DomainCall(context.Background(), "one", "person", "owner", "domains.registration.create", in, uuid.NewString())
	if !errors.Is(e, ErrDomainUnknown) {
		t.Fatal(e)
	}
	list, e := g.DomainCall(context.Background(), "one", "person", "owner", "domains.registration.list", map[string]any{}, uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	rows := list["results"].([]map[string]any)
	if len(rows) != 1 {
		t.Fatal("other workspace leaked", rows)
	}
	_, e = g.DomainCall(context.Background(), "one", "person", "owner", "domains.registration.get", map[string]any{"ref": "owned-ref"}, uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, id := range f.calls {
		if id == "domains.registration.create" {
			count++
		}
	}
	if count != 1 {
		t.Fatal("create resent", count)
	}
}
func TestDomainUnavailableAndForeignRefs(t *testing.T) {
	g, f := domainSetup()
	f.unavailable = true
	if _, e := g.DomainCall(context.Background(), "one", "person", "owner", "domains.registration.search", map[string]any{"keyword": "test"}, uuid.NewString()); !errors.Is(e, ErrDomainUnavailable) {
		t.Fatal(e)
	}
	before := len(f.calls)
	for _, id := range []string{"domains.registration.get", "domains.dns-records.list", "domains.dns-records.apply"} {
		if _, e := g.DomainCall(context.Background(), "two", "person", "owner", id, map[string]any{"ref": "foreign"}, uuid.NewString()); e == nil {
			t.Fatal("foreign ref accepted")
		}
	}
	if len(f.calls) != before {
		t.Fatal("foreign request sent")
	}
}
func TestDomainUnavailableTransportVsUncertainEffect(t *testing.T) {
	for _, effect := range []string{"none", "unknown"} {
		t.Run(effect, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(502)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "upstream_unavailable", "effect": effect}})
			}))
			defer s.Close()
			client, _ := New("fake", s.URL, nil)
			var out map[string]any
			e := client.Call(context.Background(), "domains.registration.create", map[string]any{}, &out)
			if calls != 1 {
				t.Fatal("retried")
			}
			if effect == "none" && !errors.Is(e, ErrDomainUnavailable) {
				t.Fatal(e)
			}
			if effect == "unknown" && errors.Is(e, ErrDomainUnavailable) {
				t.Fatal("unknown shown as unavailable")
			}
		})
	}
}

func TestDomainUnavailableCreateRequiresFreshQuoteBeforeAnotherAttempt(t *testing.T) {
	g, f := domainSetup()
	in := quoteInput(t, g)
	f.unavailable = true
	if _, e := g.DomainCall(context.Background(), "one", "person", "owner", "domains.registration.create", in, uuid.NewString()); !errors.Is(e, ErrDomainUnavailable) {
		t.Fatal(e)
	}
	f.unavailable = false
	before := len(f.calls)
	if _, e := g.DomainCall(context.Background(), "one", "person", "owner", "domains.registration.create", in, uuid.NewString()); !errors.Is(e, ErrDomainUnknown) {
		t.Fatal("confirmation replay", e)
	}
	if len(f.calls) != before {
		t.Fatal("old confirmation resent")
	}
	in = quoteInput(t, g)
	if _, e := g.DomainCall(context.Background(), "one", "person", "owner", "domains.registration.create", in, uuid.NewString()); e != nil {
		t.Fatal("fresh quote cannot purchase after known no-effect refusal", e)
	}
}
