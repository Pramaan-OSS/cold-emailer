package pramaan

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
)

// ImageImport copies only after consent; no sender-controlled URL reaches the browser.
func ImageImport(ctx context.Context, c Caller, raw string, consent bool, key string) (map[string]any, error) {
	if !consent {
		return nil, errors.New("Allow image import first. The sender may learn that the image was requested.")
	}
	var file struct {
		FileID      string `json:"fileId"`
		URL         string `json:"url"`
		ContentType string `json:"contentType"`
		Size        int    `json:"sizeBytes"`
	}
	if err := c.Call(ctx, "web.image.read", map[string]any{"url": raw, "consent": true}, &file, Idempotency(key)); err != nil {
		return nil, err
	}
	reader, ok := c.(interface {
		ReadFile(context.Context, string) ([]byte, error)
	})
	if !ok {
		return nil, ErrUnavailable
	}
	if file.FileID == "" || file.Size <= 0 || file.Size > 10<<20 {
		return nil, errors.New("The imported image exceeds the limit or is incomplete.")
	}
	data, err := reader.ReadFile(ctx, file.URL)
	if err != nil {
		return nil, err
	}
	kind := http.DetectContentType(data)
	if len(data) > 10<<20 || len(data) != file.Size || kind != file.ContentType || !(kind == "image/jpeg" || kind == "image/png" || kind == "image/gif" || kind == "image/webp") {
		return nil, errors.New("Use a JPEG, PNG, GIF or WebP image of at most 10 MiB.")
	}
	return map[string]any{"fileId": file.FileID, "contentType": kind, "sizeBytes": len(data), "dataURL": "data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(data)}, nil
}

func AdoptionCapability(id string) bool {
	switch id {
	case "web.image.read", "email.message.append", "email.account.protocol-diagnose", "email.folders.list", "email.directory.tenant.resolve", "workflow.webhook.signing-policy.create", "workflow.webhook.signing-policy.delete", "workflow.webhook.signed-call":
		return true
	}
	return false
}
func (g *Gateway) AdoptionCall(ctx context.Context, owner, role, id string, in map[string]any, key string) (any, error) {
	if g == nil || g.Client == nil || g.Registry == nil {
		return nil, ErrNotConfigured
	}
	if owner == "" || key == "" {
		return nil, errors.New("Choose a workspace and request reference first.")
	}
	b, _ := json.Marshal(in)
	in = map[string]any{}
	_ = json.Unmarshal(b, &in)
	saveKind := ""
	switch id {
	case "web.image.read":
		return ImageImport(ctx, g.Client, stringField(in, "url"), in["consent"] == true, key)
	case "email.directory.tenant.resolve":
	case "email.account.protocol-diagnose", "email.folders.list", "email.message.append":
		if in["app"] != "gmail" && in["app"] != "outlook" {
			return nil, errors.New("Choose a connected mailbox type.")
		}
		in["endUserRef"] = "org_" + owner
		if id == "email.message.append" && !g.owns(ctx, owner, "email_folder", stringField(in, "ref")) {
			return nil, errors.New("Select a folder from this workspace first.")
		}
		if id == "email.message.append" {
			saveKind = "message"
		}
	case "workflow.webhook.signing-policy.create":
		if role != "owner" && role != "admin" {
			return nil, ErrEnterpriseApproval
		}
		saveKind = "webhook_policy"
	case "workflow.webhook.signing-policy.delete", "workflow.webhook.signed-call":
		if role != "owner" && role != "admin" {
			return nil, ErrEnterpriseApproval
		}
		field := "ref"
		if id == "workflow.webhook.signed-call" {
			field = "signaturePolicyRef"
			if in["approved"] != true {
				return nil, ErrEnterpriseApproval
			}
			delete(in, "approved")
		}
		if !g.owns(ctx, owner, "webhook_policy", stringField(in, field)) {
			return nil, errors.New("Select a signing policy from this workspace.")
		}
	default:
		return nil, ErrUnavailable
	}
	var out map[string]any
	if err := g.Client.Call(ctx, id, in, &out, Idempotency(key)); err != nil {
		return nil, err
	}
	if id == "email.account.protocol-diagnose" && (out["health"] != "connected" || out["protocol"] != "mailbox_api") {
		return nil, errors.New("Mailbox health was not confirmed.")
	}
	if id == "email.message.append" && (out["status"] != "saved" || stringField(out, "ref") == "") {
		return nil, errors.New("Saving is uncertain. Check the folder before trying again.")
	}
	if id == "workflow.webhook.signing-policy.create" && (stringField(out, "ref") == "" || stringField(out, "secret") == "") {
		return nil, errors.New("Policy creation is uncertain. Do not create it again automatically.")
	}
	if id == "workflow.webhook.signed-call" && (out["status"] != "accepted" || stringField(out, "eventId") != stringField(in, "eventId")) {
		return nil, errors.New("Webhook delivery is uncertain. Check the receiver before trying again.")
	}
	if saveKind != "" {
		if err := g.Registry.Save(ctx, owner, saveKind, stringField(out, "ref")); err != nil {
			return nil, err
		}
	}
	if id == "email.folders.list" {
		for _, row := range resultRows(out) {
			if err := g.Registry.Save(ctx, owner, "email_folder", stringField(row, "ref")); err != nil {
				return nil, err
			}
		}
	}
	if id == "workflow.webhook.signing-policy.delete" {
		if out["deleted"] != true {
			return nil, errors.New("Policy removal is unconfirmed.")
		}
		if err := g.Registry.Remove(ctx, owner, "webhook_policy", stringField(in, "ref")); err != nil {
			return nil, err
		}
	}
	return out, nil
}
