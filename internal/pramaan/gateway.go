package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// Gateway is a workspace-scoped façade. Clients cannot select an end user or an unowned resource.
type Gateway struct {
	Client    Caller
	Registry  Registry
	ReturnURL string
	Project   string
}

var publicReads = map[string]bool{"apps.catalog.search": true, "apps.triggers.list": true, "apps.trigger.get": true, "companies.logo.get": true, "people.enrich": true, "people.enrich-bulk": true, "people.email.find": true, "people.email.verify": true, "people.email.verify-bulk": true, "companies.enrich": true, "companies.enrich-bulk": true, "companies.search": true, "companies.discovery-search": true, "companies.discovery-count": true, "companies.lookalike.search": true, "companies.similar": true, "companies.domain.find": true, "companies.profile.lookup": true, "companies.search-filtered": true, "companies.match-search": true, "web.domain-names.check": true, "web.search": true, "web.read": true, "llm.complete": true}

func (g *Gateway) Call(ctx context.Context, owner, id string, in map[string]any, key string) (any, error) {
	if g == nil || g.Client == nil || g.Registry == nil {
		return nil, ErrNotConfigured
	}
	if owner == "" {
		return nil, errors.New("Choose a workspace first.")
	}
	if in == nil {
		in = map[string]any{}
	}
	inCopy := map[string]any{}
	for k, v := range in {
		inCopy[k] = v
	}
	in = inCopy
	var out map[string]any
	saveKind := ""
	saveInput := ""
	switch {
	case id == "apps.catalog.search":
		if strings.TrimSpace(stringField(in, "query")) == "" {
			return nil, errors.New("Describe the action with a keyword first.")
		}
		in["limit"] = 50
		delete(in, "page")
	case publicReads[id]:
	case id == "document.parse":
		if !g.owns(ctx, owner, "connected_file", stringField(in, "fileId")) {
			return nil, errors.New("Select a downloaded file from this workspace.")
		}
		delete(in, "url")
		delete(in, "content")
	case id == "apps.connection.start":
		in["endUserRef"] = "org_" + owner
		in["redirectUrl"] = g.ReturnURL
	case id == "apps.connection.revoke":
		if !g.owns(ctx, owner, "app_connection", stringField(in, "app")) {
			return nil, errors.New("Select a connected app in this workspace.")
		}
		in["endUserRef"] = "org_" + owner
	case id == "apps.connections.list" || id == "apps.trigger-deployments.list":
		in["endUserRef"] = "org_" + owner
	case id == "apps.action.execute":
		if !g.owns(ctx, owner, "action", stringField(in, "action")) {
			return nil, errors.New("Discover and select an action first.")
		}
		in["endUserRef"] = "org_" + owner
	case id == "apps.trigger-deployment.get" || id == "apps.trigger-deployment.delete" || id == "apps.trigger-deployment-events.list":
		if !g.owns(ctx, owner, "deployment", stringField(in, "deploymentId")) {
			return nil, errors.New("Select a trigger in this workspace.")
		}
		in["endUserRef"] = "org_" + owner
	case id == "domains.add" || id == "domains.verify" || id == "domains.list":
		if g.Project == "" {
			return nil, errors.New("Set the app's hosting project before managing its web domains.")
		}
		in["project"] = g.Project
		if id != "domains.list" && !g.owns(ctx, owner, "domain", stringField(in, "domain")) {
			return nil, errors.New("Attach your owned domain first.")
		}
	case id == "email.account.connect.start":
		saveKind = "mail_connection"
		in["endUserRef"] = "org_" + owner
		in["returnUrl"] = g.ReturnURL
	case id == "email.account.connect.status":
		ref := stringField(in, "ref")
		if ref == "" {
			refs, e := g.Registry.List(ctx, owner, "mail_connection")
			if e != nil || len(refs) == 0 {
				return nil, errors.New("Connect your mailbox first.")
			}
			ref = refs[len(refs)-1]
		}
		if !g.owns(ctx, owner, "mail_connection", ref) {
			return nil, ErrUnavailable
		}
		in["ref"] = ref
	case id == "email.accounts.list":
		in["endUserRef"] = "org_" + owner
	case id == "email.inbox.create":
		saveKind = "inbox"
	case id == "email.domain.create":
		saveKind = "domain"
		saveInput = stringField(in, "domain")
		if saveInput == "" {
			return nil, errors.New("Enter a domain you own.")
		}
	case id == "email.inboxes.list":
	case id == "email.domains.list":
	case strings.HasPrefix(id, "email.inbox."):
		if !g.ownsEither(ctx, owner, "inbox", in, "ref", "address") {
			return nil, errors.New("Select a mailbox in this workspace.")
		}
	case strings.HasPrefix(id, "email.domain."):
		if !g.ownsEither(ctx, owner, "domain", in, "ref", "domain") {
			return nil, errors.New("Select a domain in this workspace.")
		}
	case id == "email.message.send":
		from := stringField(in, "from")
		if !g.owns(ctx, owner, "inbox", from) {
			return nil, errors.New("Select a sender in this workspace.")
		}
		b, _ := json.Marshal(in)
		var r MailRequest
		if json.Unmarshal(b, &r) != nil {
			return nil, errors.New("The message could not be read.")
		}
		result, e := (MailService{Client: g.Client}).Send(ctx, r, key)
		if e != nil {
			return nil, e
		}
		if e = g.Registry.Save(ctx, owner, "message", result.Ref); e != nil {
			return nil, e
		}
		if e = g.Registry.Save(ctx, owner, "message", result.MessageID); e != nil {
			return nil, e
		}
		return result, nil
	case id == "email.message.reply":
		if !g.owns(ctx, owner, "inbox", stringField(in, "inbox")) || !g.ownsEither(ctx, owner, "message", in, "ref", "messageId") {
			return nil, errors.New("Select a conversation in this workspace.")
		}
		// Reuse send's opt-out gate. The parent sender must be explicitly selected in the reply form.
		b, _ := json.Marshal(in)
		var r MailRequest
		json.Unmarshal(b, &r)
		r.From = stringField(in, "inbox")
		r.InReplyTo = stringField(in, "messageId")
		if r.InReplyTo == "" {
			return nil, errors.New("Choose the parent message before replying.")
		}
		result, e := (MailService{Client: g.Client}).Send(ctx, r, key)
		if e != nil {
			return nil, e
		}
		g.Registry.Save(ctx, owner, "message", result.Ref)
		g.Registry.Save(ctx, owner, "message", result.MessageID)
		return result, nil
	case id == "email.messages.search" || id == "email.messages.list" || id == "email.threads.list":
		if !g.owns(ctx, owner, "inbox", stringField(in, "inbox")) {
			return nil, errors.New("Select a mailbox in this workspace.")
		}
	case id == "email.status" || id == "email.message.get":
		if !g.ownsEither(ctx, owner, "message", in, "ref", "messageId") {
			return nil, errors.New("Select a message in this workspace.")
		}
	case id == "messaging.opt-out.check" || id == "messaging.opt-out.add" || id == "messaging.opt-out.remove":
		if !g.owns(ctx, owner, "contact", stringField(in, "address")) {
			return nil, errors.New("Select a contact in this workspace.")
		}
		in["channel"] = "email"
	default:
		return nil, ErrUnavailable
	}
	if e := g.Client.Call(ctx, id, in, &out, Idempotency(key)); e != nil {
		return nil, e
	}
	if id == "apps.action.execute" && stringField(out, "status") == "failed" {
		return nil, errors.New("The connected action failed. Check its fields and try again.")
	}
	if id == "apps.action.execute" {
		for _, ref := range ConnectedFileIDs(out) {
			if err := g.Registry.Save(ctx, owner, "connected_file", ref); err != nil {
				return nil, err
			}
		}
	}
	if id == "apps.connections.list" {
		for _, r := range resultRows(out) {
			if e := g.Registry.Save(ctx, owner, "app_connection", stringField(r, "app")); e != nil {
				return nil, e
			}
		}
	}
	if id == "apps.connection.revoke" {
		if e := g.Registry.Remove(ctx, owner, "app_connection", stringField(in, "app")); e != nil {
			return nil, e
		}
	}
	if id == "email.account.connect.status" && stringField(out, "status") == "connected" {
		if e := g.Registry.Save(ctx, owner, "account", stringField(out, "ref")); e != nil {
			return nil, e
		}
		if inbox := stringField(out, "inbox"); inbox != "" {
			if e := g.Registry.Save(ctx, owner, "inbox", inbox); e != nil {
				return nil, e
			}
		}
	}
	if id == "email.inbox.create" {
		if e := g.Registry.Save(ctx, owner, "inbox_native", stringField(out, "address")); e != nil {
			return nil, e
		}
	}
	if saveKind != "" {
		for _, value := range []string{saveInput, stringField(out, "ref"), stringField(out, "address"), stringField(out, "domain")} {
			if value != "" {
				if e := g.Registry.Save(ctx, owner, saveKind, value); e != nil {
					return nil, errors.New("The resource could not be saved to this workspace.")
				}
			}
		}
	}
	if id == "apps.catalog.search" {
		for _, r := range resultRows(out) {
			if a := stringField(r, "action"); a != "" {
				if e := g.Registry.Save(ctx, owner, "action", a); e != nil {
					return nil, e
				}
			}
		}
	}
	if id == "apps.trigger-deployments.list" {
		for _, r := range resultRows(out) {
			if a := stringField(r, "deploymentId"); a != "" {
				if e := g.Registry.Save(ctx, owner, "deployment", a); e != nil {
					return nil, e
				}
			}
		}
	}
	if id == "email.inboxes.list" || id == "email.domains.list" || id == "domains.list" {
		kind, field := "inbox", "address"
		if id == "email.domains.list" {
			kind, field = "domain", "domain"
		} else if id == "domains.list" {
			kind, field = "domain", "name"
		}
		filtered := []map[string]any{}
		for _, r := range resultRows(out) {
			if g.owns(ctx, owner, kind, stringField(r, field)) {
				filtered = append(filtered, r)
			}
		}
		out["results"] = filtered
	}
	if id == "email.messages.search" || id == "email.messages.list" {
		for _, r := range resultRows(out) {
			for _, field := range []string{"ref", "messageId"} {
				if value := stringField(r, field); value != "" {
					if e := g.Registry.Save(ctx, owner, "message", value); e != nil {
						return nil, e
					}
				}
			}
		}
	}
	return out, nil
}
func (g *Gateway) owns(ctx context.Context, owner, kind, ref string) bool {
	if ref == "" {
		return false
	}
	yes, e := g.Registry.Owns(ctx, owner, kind, ref)
	return e == nil && yes
}
func (g *Gateway) ownsEither(ctx context.Context, owner, kind string, in map[string]any, a, b string) bool {
	first, second := stringField(in, a), stringField(in, b)
	if first != "" && !g.owns(ctx, owner, kind, first) {
		return false
	}
	if second != "" && !g.owns(ctx, owner, kind, second) {
		return false
	}
	return first != "" || second != ""
}
func stringField(m map[string]any, key string) string { s, _ := m[key].(string); return s }
func resultRows(m map[string]any) []map[string]any {
	raw, _ := m["results"].([]any)
	out := []map[string]any{}
	for _, r := range raw {
		if row, ok := r.(map[string]any); ok {
			out = append(out, row)
		}
	}
	return out
}
