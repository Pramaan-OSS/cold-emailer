package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type memoryRegistry map[string]map[string]bool

func (m memoryRegistry) Owns(_ context.Context, owner, kind, handle string) (bool, error) {
	return m[owner][kind+":"+handle], nil
}
func (m memoryRegistry) Save(_ context.Context, owner, kind, handle string) error {
	if m[owner] == nil {
		m[owner] = map[string]bool{}
	}
	m[owner][kind+":"+handle] = true
	return nil
}
func (m memoryRegistry) List(_ context.Context, owner, kind string) ([]string, error) {
	return nil, nil
}

type responseCaller struct {
	calls    int
	input    map[string]any
	response any
}

func (f *responseCaller) Call(_ context.Context, _ string, in, out any, _ ...Option) error {
	f.calls++
	f.input = in.(map[string]any)
	b, _ := json.Marshal(f.response)
	if out == nil {
		return nil
	}
	return json.Unmarshal(b, out)
}

func TestGatewayNeverSendsFromAnotherWorkspace(t *testing.T) {
	f := &responseCaller{}
	g := Gateway{Client: f, Registry: memoryRegistry{"one": {"inbox:one@example.test": true}}}
	_, err := g.Call(context.Background(), "two", "email.message.send", map[string]any{"from": "one@example.test", "to": []string{"recipient@example.test"}}, "action")
	if err == nil || f.calls != 0 {
		t.Fatalf("unauthorized call: %v, %d", err, f.calls)
	}
}
func TestGatewayRejectsMixedOwnedAndUnownedIdentifiers(t *testing.T) {
	f := &responseCaller{}
	g := Gateway{Client: f, Registry: memoryRegistry{"one": {"message:owned": true}}}
	_, err := g.Call(context.Background(), "one", "email.message.get", map[string]any{"ref": "owned", "messageId": "unowned"}, "action")
	if err == nil || f.calls != 0 {
		t.Fatal("an owned handle must not authorize an unrelated identifier")
	}
}
func TestGatewayBindsConnectionsToAuthenticatedWorkspace(t *testing.T) {
	f := &responseCaller{response: map[string]any{"connectUrl": "https://example.test/connect"}}
	g := Gateway{Client: f, Registry: memoryRegistry{}, ReturnURL: "https://example.test/app/integrations"}
	_, err := g.Call(context.Background(), "one", "apps.connection.start", map[string]any{"app": "connected-app", "endUserRef": "org_two", "redirectUrl": "https://elsewhere.test"}, "action")
	if err != nil {
		t.Fatal(err)
	}
	if f.input["endUserRef"] != "org_one" || f.input["redirectUrl"] != g.ReturnURL {
		t.Fatal("browser selected another workspace or return address")
	}
}
func TestGatewayFiltersMailboxListing(t *testing.T) {
	f := &responseCaller{response: map[string]any{"results": []map[string]any{{"address": "one@example.test"}, {"address": "two@example.test"}}}}
	g := Gateway{Client: f, Registry: memoryRegistry{"one": {"inbox:one@example.test": true}}}
	out, err := g.Call(context.Background(), "one", "email.inboxes.list", nil, "read")
	if err != nil {
		t.Fatal(err)
	}
	rows := out.(map[string]any)["results"].([]map[string]any)
	if len(rows) != 1 || rows[0]["address"] != "one@example.test" {
		t.Fatal("listing leaked another workspace")
	}
}
func TestGatewayMissingFeaturesStayOff(t *testing.T) {
	f := &responseCaller{}
	g := Gateway{Client: f, Registry: memoryRegistry{}}
	_, err := g.Call(context.Background(), "one", "domains.registration.create", nil, "action")
	if !errors.Is(err, ErrUnavailable) || f.calls != 0 {
		t.Fatal("unregistered operation reached transport")
	}
}

func (m memoryRegistry) Remove(_ context.Context, owner, kind, handle string) error {
	delete(m[owner], kind+":"+handle)
	return nil
}
