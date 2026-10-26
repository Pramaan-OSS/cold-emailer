package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

// CompanyMail owns the platform's company connection routes, not vendor endpoints.
func (c *Client) CompanyMail(ctx context.Context, action string, in map[string]any, key string) (map[string]any, error) {
	method, path := "POST", "/v1/companies/mail/connect"
	if action != "connect" {
		ref := stringField(in, "companyRef")
		if ref == "" {
			return nil, errors.New("Enter the company connection reference.")
		}
		path = "/v1/companies/mail/" + url.PathEscape(ref)
		method = "GET"
		if action == "revoke" {
			method = "DELETE"
		} else if action != "status" {
			return nil, ErrUnavailable
		}
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	if method != "POST" {
		b = nil
	}
	_, body, err := c.request(ctx, method, path, b, callOptions{key: key})
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if json.Unmarshal(body, &out) != nil {
		return nil, errors.New("The company connection result is incomplete.")
	}
	return out, nil
}
func (g *Gateway) CompanyCall(ctx context.Context, owner, role, action string, in map[string]any, key string) (map[string]any, error) {
	if g == nil || g.Registry == nil || g.Client == nil {
		return nil, ErrNotConfigured
	}
	if owner == "" || key == "" {
		return nil, errors.New("Choose a workspace first.")
	}
	if role != "owner" && role != "admin" {
		return nil, ErrEnterpriseApproval
	}
	client, ok := g.Client.(interface {
		CompanyMail(context.Context, string, map[string]any, string) (map[string]any, error)
	})
	if !ok {
		return nil, ErrUnavailable
	}
	ref := stringField(in, "companyRef")
	// A status read can claim a reference after hosted consent. The platform
	// verifies the builder organisation; our registry prevents cross-workspace use.
	if action == "revoke" && !g.owns(ctx, owner, "company", ref) {
		return nil, errors.New("Select this workspace's company connection.")
	}
	if action == "enroll" {
		if !g.owns(ctx, owner, "company", ref) {
			return nil, errors.New("Check this workspace's company connection first.")
		}
		mailbox := strings.ToLower(strings.TrimSpace(stringField(in, "mailbox")))
		if !strings.Contains(mailbox, "@") || strings.ContainsAny(mailbox, "\r\n ") {
			return nil, errors.New("Enter one mailbox address.")
		}
		var page map[string]any
		if err := g.Client.Call(ctx, "email.messages.list", map[string]any{"companyRef": ref, "mailbox": mailbox, "limit": 1}, &page, Idempotency(key)); err != nil {
			return nil, err
		}
		if err := g.Registry.Save(ctx, owner, "inbox", mailbox); err != nil {
			return nil, err
		}
		return map[string]any{"companyRef": ref, "mailbox": mailbox, "status": "connected"}, nil
	}
	body := map[string]any{}
	for _, name := range []string{"kind", "domain", "tenant", "mailbox", "companyRef"} {
		if v, ok := in[name]; ok {
			body[name] = v
		}
	}
	out, err := client.CompanyMail(ctx, action, body, key)
	if err != nil {
		return nil, err
	}
	if resultRef := stringField(out, "companyRef"); resultRef != "" {
		if err = g.Registry.Save(ctx, owner, "company", resultRef); err != nil {
			return nil, err
		}
	}
	if action == "revoke" && out["revoked"] == true {
		err = g.Registry.Remove(ctx, owner, "company", ref)
	}
	return out, err
}

// CompanyMailbox injects a persisted selector for worker operations. Unsupported
// company operations are refused by the platform, never sent as personal mail.
type CompanyMailbox struct {
	Client              Caller
	CompanyRef, Mailbox string
}

func (c CompanyMailbox) Call(ctx context.Context, id string, in, out any, opts ...Option) error {
	if !strings.HasPrefix(id, "email.") && !strings.HasPrefix(id, "calendar.") {
		return c.Client.Call(ctx, id, in, out, opts...)
	}
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	var body map[string]any
	if err = json.Unmarshal(b, &body); err != nil {
		return err
	}
	if from := stringField(body, "from"); from != "" && !strings.EqualFold(from, c.Mailbox) {
		return errors.New("Company mail sends only from the selected mailbox. Choose that mailbox instead of an alias.")
	}
	delete(body, "from")
	delete(body, "inbox")
	delete(body, "endUserRef")
	delete(body, "app")
	body["companyRef"] = c.CompanyRef
	body["mailbox"] = c.Mailbox
	return c.Client.Call(ctx, id, body, out, opts...)
}
func (c CompanyMailbox) Close() error {
	if closer, ok := c.Client.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
