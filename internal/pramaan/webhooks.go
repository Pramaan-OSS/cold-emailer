package pramaan

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
)

type SigningPolicy struct {
	Ref    string `json:"ref"`
	Secret string `json:"secret"`
}

func CreateSigningPolicy(ctx context.Context, c Caller, name, key string) (SigningPolicy, error) {
	var out SigningPolicy
	if c == nil {
		return out, ErrNotConfigured
	}
	err := c.Call(ctx, "workflow.webhook.signing-policy.create", map[string]any{"name": name}, &out, Idempotency(key))
	if err == nil && (out.Ref == "" || out.Secret == "") {
		err = errors.New("Signing policy creation is unconfirmed. Check before trying again.")
	}
	return out, err
}
func DeleteSigningPolicy(ctx context.Context, c Caller, ref, key string) error {
	if c == nil {
		return ErrNotConfigured
	}
	var out struct {
		Deleted bool `json:"deleted"`
	}
	if err := c.Call(ctx, "workflow.webhook.signing-policy.delete", map[string]any{"ref": ref}, &out, Idempotency(key)); err != nil {
		return err
	}
	if !out.Deleted {
		return errors.New("Signing policy deletion is unconfirmed.")
	}
	return nil
}
func SignedWebhook(ctx context.Context, c Caller, ref, address, eventID string, payload map[string]any) error {
	if c == nil {
		return ErrNotConfigured
	}
	var out struct {
		Status  string `json:"status"`
		EventID string `json:"eventId"`
	}
	if err := c.Call(ctx, "workflow.webhook.signed-call", map[string]any{"signaturePolicyRef": ref, "url": address, "eventId": eventID, "payload": payload}, &out, Idempotency(fmt.Sprintf("webhook-%x", sha256.Sum256([]byte(ref+":"+eventID))))); err != nil {
		return err
	}
	if out.Status != "accepted" || out.EventID != eventID {
		return errors.New("Delivery is uncertain. Check the receiver before trying again.")
	}
	return nil
}
