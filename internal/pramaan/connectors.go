package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/url"

	"github.com/google/uuid"
)

var ErrConnectorApproval = errors.New("Approve this tool call before running it.")
var ErrConnectorReadOnly = errors.New("Read-only mode blocks tools with unknown effects.")
var ErrConnectorChanged = errors.New("This tool is missing or changed. Refresh the tools and review it again.")

type Connectors struct {
	Client    Caller
	ReturnURL string
}

func ConnectorOwner(org, user string) map[string]any {
	return map[string]any{"kind": "end-user", "endUserRef": "org_" + org + "_user_" + user}
}
func connectorInput(org, user, ref string) map[string]any {
	return map[string]any{"owner": ConnectorOwner(org, user), "connectorRef": ref}
}
func ConnectorURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && len(raw) <= 2048 && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && (u.Port() == "" || u.Port() == "443") && net.ParseIP(u.Hostname()) == nil && u.Hostname() != "localhost"
}
func (c Connectors) Register(ctx context.Context, org, user, address, label, auth, token, key string) (map[string]any, error) {
	if c.Client == nil {
		return nil, ErrNotConfigured
	}
	if org == "" || user == "" || !ConnectorURL(address) {
		return nil, errors.New("Use a public HTTPS tool-server address on port 443, without credentials, query or fragment.")
	}
	if _, err := uuid.Parse(key); err != nil {
		return nil, errors.New("Registration needs its own request reference.")
	}
	a := map[string]any{"kind": auth}
	switch auth {
	case "none":
	case "bearer":
		if token == "" {
			return nil, errors.New("Enter the connector token.")
		}
		a["token"] = token
	case "oauth":
		if !ConnectorURL(c.ReturnURL) {
			return nil, errors.New("Hosted sign-in needs a public HTTPS app return address.")
		}
		a["returnUrl"] = c.ReturnURL
	default:
		return nil, errors.New("Only no sign-in, a token or hosted OAuth sign-in is supported. Custom headers and private OAuth clients are off.")
	}
	var out map[string]any
	e := c.Client.Call(ctx, "connector.connection.register", map[string]any{"owner": ConnectorOwner(org, user), "url": address, "label": label, "auth": a}, &out, Idempotency(key))
	return out, e
}
func (c Connectors) List(ctx context.Context, org, user string) ([]map[string]any, error) {
	if c.Client == nil {
		return nil, ErrNotConfigured
	}
	var rows []map[string]any
	cursor := ""
	for page := 0; page < 2; page++ {
		in := map[string]any{"owner": ConnectorOwner(org, user), "limit": 50}
		if cursor != "" {
			in["cursor"] = cursor
		}
		var out map[string]any
		if e := c.Client.Call(ctx, "connector.connections.list", in, &out); e != nil {
			return nil, e
		}
		rows = append(rows, resultRows(out)...)
		cursor = stringField(out, "nextCursor")
		if cursor == "" {
			break
		}
	}
	return rows, nil
}
func (c Connectors) Tools(ctx context.Context, org, user, ref string, refresh bool) ([]map[string]any, error) {
	if c.Client == nil {
		return nil, ErrNotConfigured
	}
	rows := []map[string]any{}
	cursor := ""
	for page := 0; page < 4; page++ {
		in := connectorInput(org, user, ref)
		in["limit"] = 50
		if page == 0 {
			in["refresh"] = refresh
		}
		if cursor != "" {
			in["cursor"] = cursor
		}
		var out map[string]any
		if e := c.Client.Call(ctx, "connector.tools.list", in, &out); e != nil {
			return nil, e
		}
		rows = append(rows, resultRows(out)...)
		cursor = stringField(out, "nextCursor")
		if cursor == "" {
			break
		}
	}
	return rows, nil
}
func (c Connectors) Update(ctx context.Context, org, user, ref string, patch map[string]any, approved bool) error {
	if c.Client == nil {
		return ErrNotConfigured
	}
	if !approved {
		return ErrConnectorApproval
	}
	in := connectorInput(org, user, ref)
	for k, v := range patch {
		if k != "label" && k != "allowWithoutApproval" {
			return errors.New("Changing credentials or transport settings is unsupported; disconnect and register again.")
		}
		in[k] = v
	}
	return c.Client.Call(ctx, "connector.connection.update", in, nil, Idempotency(uuid.NewString()))
}
func (c Connectors) Disconnect(ctx context.Context, org, user, ref string) error {
	if c.Client == nil {
		return ErrNotConfigured
	}
	in := connectorInput(org, user, ref)
	in["confirm"] = true
	return c.Client.Call(ctx, "connector.connection.disconnect", in, nil, Idempotency(uuid.NewString()))
}
func (c Connectors) Call(ctx context.Context, org, user, ref, tool string, input map[string]any, approved, readOnly bool, key string) (map[string]any, error) {
	if readOnly {
		return nil, ErrConnectorReadOnly
	}
	if !approved {
		return nil, ErrConnectorApproval
	}
	if _, e := uuid.Parse(key); e != nil {
		return nil, errors.New("This call needs its own request reference.")
	}
	if input == nil {
		input = map[string]any{}
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > 100000 {
		return nil, errors.New("Tool input must be valid JSON of at most 100 KB.")
	}
	tools, e := c.Tools(ctx, org, user, ref, true)
	if e != nil {
		return nil, e
	}
	found := false
	for _, row := range tools {
		if _, valid := row["inputSchema"].(map[string]any); stringField(row, "toolRef") == tool && valid {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrConnectorChanged
	}
	in := connectorInput(org, user, ref)
	in["toolRef"] = tool
	in["input"] = input
	var out map[string]any
	e = c.Client.Call(ctx, "connector.tool.call", in, &out, Idempotency(key))
	if e != nil {
		var he *HTTPError
		if errors.As(e, &he) && he.Effect == "none" {
			return nil, e
		}
		return map[string]any{"status": "unknown", "errorCode": "completion-unknown", "output": map[string]any{}}, nil
	}
	switch stringField(out, "status") {
	case "succeeded", "failed", "unknown":
		return out, nil
	}
	return map[string]any{"status": "unknown", "errorCode": "completion-unknown", "output": map[string]any{}}, nil
}
