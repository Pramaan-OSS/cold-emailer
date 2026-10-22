package pramaan

import (
	"context"
	"errors"
)

// CheckoutStatus reads an owned checkout. A redirect or completed session alone never proves payment.
func (g *Gateway) CheckoutStatus(ctx context.Context, owner, ref string) (map[string]any, error) {
	if g == nil || g.Client == nil || g.Registry == nil {
		return nil, ErrNotConfigured
	}
	if owner == "" || ref == "" || !g.owns(ctx, owner, "checkout", ref) {
		return nil, errors.New("Select a checkout from this workspace.")
	}
	var out map[string]any
	if e := g.Client.Call(ctx, "payments.checkout.get", map[string]any{"ref": ref}, &out); e != nil {
		return nil, e
	}
	confirmed := false
	if out["status"] == "paid" {
		payments, _ := out["payments"].([]any)
		for _, value := range payments {
			if payment, ok := value.(map[string]any); ok && payment["status"] == "captured" && stringField(payment, "ref") != "" {
				confirmed = true
			}
		}
	}
	status := stringField(out, "status")
	switch status {
	case "active", "unpaid", "paid", "cancelled", "completed", "expired", "pending", "refused", "unknown":
	default:
		status = "unknown"
	}
	return map[string]any{"status": status, "confirmed": confirmed}, nil
}
