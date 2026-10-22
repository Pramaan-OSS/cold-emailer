package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type connectorFake struct {
	calls     []string
	inputs    []map[string]any
	keys      []string
	tools     []map[string]any
	outcome   string
	callError error
}

func (f *connectorFake) Call(_ context.Context, id string, in, out any, opts ...Option) error {
	f.calls = append(f.calls, id)
	f.inputs = append(f.inputs, in.(map[string]any))
	o := callOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	f.keys = append(f.keys, o.key)
	var result any
	switch id {
	case "connector.connection.register":
		auth := in.(map[string]any)["auth"].(map[string]any)
		state := "ready"
		result = map[string]any{"connectorRef": "cn_0123456789abcdef0123456789abcdef", "state": state}
		if auth["kind"] == "oauth" {
			result = map[string]any{"connectorRef": "cn_0123456789abcdef0123456789abcdef", "state": "pending-auth", "connectUrl": "https://example.test/connect"}
		}
	case "connector.connections.list":
		result = map[string]any{"results": []map[string]any{{"connectorRef": "cn_0123456789abcdef0123456789abcdef", "state": "ready"}}}
	case "connector.tools.list":
		result = map[string]any{"results": f.tools}
	case "connector.tool.call":
		if f.callError != nil {
			return f.callError
		}
		status := f.outcome
		if status == "" {
			status = "succeeded"
		}
		result = map[string]any{"status": status, "output": map[string]any{"text": "Synthetic tool output"}}
	default:
		result = map[string]any{}
	}
	if out == nil {
		return nil
	}
	b, _ := json.Marshal(result)
	return json.Unmarshal(b, out)
}
func remoteFixture() (Connectors, *connectorFake) {
	f := &connectorFake{tools: []map[string]any{{"toolRef": "tl_synthetic_tool", "schemaDigest": "digest", "approvalRequired": false, "inputSchema": map[string]any{"type": "object"}}}}
	return Connectors{Client: f, ReturnURL: "https://app.example.test/app/settings/connections"}, f
}
func TestConnectorRegisterAndHostedSignInOwner(t *testing.T) {
	c, f := remoteFixture()
	for _, auth := range []string{"none", "bearer", "oauth"} {
		out, e := c.Register(context.Background(), "workspace", "person", "https://tools.example.test/mcp", "Synthetic tools", auth, "synthetic-token", uuid.NewString())
		if e != nil {
			t.Fatal(e)
		}
		in := f.inputs[len(f.inputs)-1]
		owner := in["owner"].(map[string]any)
		if owner["kind"] != "end-user" || owner["endUserRef"] != "org_workspace_user_person" {
			t.Fatal(owner)
		}
		payload := in["auth"].(map[string]any)
		if auth == "oauth" && (payload["returnUrl"] != c.ReturnURL || out["state"] != "pending-auth" || out["connectUrl"] == nil) {
			t.Fatal(out, payload)
		}
		if _, found := in["headers"]; found {
			t.Fatal("custom headers sent")
		}
		if out["token"] != nil || out["url"] != nil {
			t.Fatal("credentials returned")
		}
	}
}
func TestConnectorListAndApprovedFreshCall(t *testing.T) {
	c, f := remoteFixture()
	ctx := context.Background()
	if _, e := c.List(ctx, "workspace", "person"); e != nil {
		t.Fatal(e)
	}
	if _, e := c.Tools(ctx, "workspace", "person", "connection", false); e != nil {
		t.Fatal(e)
	}
	key := uuid.NewString()
	out, e := c.Call(ctx, "workspace", "person", "connection", "tl_synthetic_tool", map[string]any{"query": "test"}, true, false, key)
	if e != nil || out["status"] != "succeeded" {
		t.Fatal(out, e)
	}
	n := len(f.calls)
	if f.calls[n-2] != "connector.tools.list" || f.inputs[n-2]["refresh"] != true || f.calls[n-1] != "connector.tool.call" || f.keys[n-1] != key {
		t.Fatal(f.calls, f.inputs, f.keys)
	}
}
func TestConnectorRefusesApprovalReadOnlyAndChangedToolBeforeCall(t *testing.T) {
	c, f := remoteFixture()
	ctx := context.Background()
	if _, e := c.Call(ctx, "org", "user", "connection", "tl_synthetic_tool", nil, false, false, uuid.NewString()); !errors.Is(e, ErrConnectorApproval) {
		t.Fatal(e)
	}
	if _, e := c.Call(ctx, "org", "user", "connection", "tl_synthetic_tool", nil, true, true, uuid.NewString()); !errors.Is(e, ErrConnectorReadOnly) {
		t.Fatal(e)
	}
	if len(f.calls) != 0 {
		t.Fatal("refused before approval reached network")
	}
	f.tools = []map[string]any{}
	if _, e := c.Call(ctx, "org", "user", "connection", "tl_synthetic_tool", nil, true, false, uuid.NewString()); !errors.Is(e, ErrConnectorChanged) {
		t.Fatal(e)
	}
	if len(f.calls) != 1 || f.calls[0] != "connector.tools.list" {
		t.Fatal("missing tool called", f.calls)
	}
}
func TestConnectorUnknownResultAndLostResponseAreNotRetried(t *testing.T) {
	for _, transportError := range []bool{false, true} {
		c, f := remoteFixture()
		f.outcome = "unknown"
		if transportError {
			f.callError = errors.New("reply lost")
		}
		out, e := c.Call(context.Background(), "org", "user", "connection", "tl_synthetic_tool", nil, true, false, uuid.NewString())
		if e != nil || out["status"] != "unknown" {
			t.Fatal(out, e)
		}
		count := 0
		for _, id := range f.calls {
			if id == "connector.tool.call" {
				count++
			}
		}
		if count != 1 {
			t.Fatal("retried", f.calls)
		}
	}
}
func TestConnectorAllowListUpdateAlsoNeedsApproval(t *testing.T) {
	c, f := remoteFixture()
	if e := c.Update(context.Background(), "org", "user", "ref", map[string]any{"allowWithoutApproval": []string{"tl_synthetic_tool"}}, false); !errors.Is(e, ErrConnectorApproval) || len(f.calls) != 0 {
		t.Fatal(e, f.calls)
	}
	if e := c.Update(context.Background(), "org", "user", "ref", map[string]any{"allowWithoutApproval": []string{"tl_synthetic_tool"}}, true); e != nil {
		t.Fatal(e)
	}
	if e := c.Update(context.Background(), "org", "user", "ref", map[string]any{"headers": "forbidden"}, true); e == nil {
		t.Fatal("unsupported custom headers accepted")
	}
}
func TestConnectorRefusesUnsupportedTransportAndOAuthReturn(t *testing.T) {
	c, f := remoteFixture()
	for _, address := range []string{"http://tools.example.test", "https://127.0.0.1/mcp", "https://tools.example.test:444/mcp", "https://user:password@tools.example.test/mcp", "https://tools.example.test/mcp?token=secret"} {
		if _, e := c.Register(context.Background(), "org", "user", address, "name", "none", "", uuid.NewString()); e == nil {
			t.Fatal(address)
		}
	}
	c.ReturnURL = "http://localhost/return"
	if _, e := c.Register(context.Background(), "org", "user", "https://tools.example.test/mcp", "name", "oauth", "", uuid.NewString()); e == nil {
		t.Fatal("unsafe callback")
	}
	if len(f.calls) != 0 {
		t.Fatal("invalid settings reached network")
	}
}
