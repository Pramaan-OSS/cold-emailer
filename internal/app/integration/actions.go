package integration

import (
	"context"

	"github.com/pramaan-oss/cold-email/internal/pramaan"

	"github.com/google/uuid"
)

// automationEventPayload is the structured, versioned body delivered to generic
// automation webhooks (Zapier / Make / n8n). The legacy flat fields
// (content/email/subject/title) are kept so existing Zaps keep working; the
// `event` discriminator + `data` object are what automations actually branch on.
type automationEventPayload struct {
	ID        string         `json:"id"`
	Event     string         `json:"event"`
	Version   string         `json:"version"`
	CreatedAt string         `json:"created_at"`
	Data      map[string]any `json:"data"`
	Content   string         `json:"content"`
	Email     string         `json:"email,omitempty"`
	Subject   string         `json:"subject,omitempty"`
	Title     string         `json:"title,omitempty"`
}

// automationDeliver POSTs a structured event to a generic automation webhook,
// HMAC-signing it with the connection's signing secret (same scheme as our
// customer webhooks: `X-ColdEmail-Signature: t=<unix>,v1=<hmac>` over `t.body`)
// and retrying transient (5xx / network) failures with linear backoff. 4xx is
// treated as a permanent misconfiguration and not retried.
func automationDeliver(ctx context.Context, targetURL, secret, eventType string, payload automationEventPayload) error {
	return pramaan.Call(ctx, "workflow.webhook.call", map[string]any{"url": targetURL, "method": "POST", "body": map[string]any{"event": eventType, "payload": payload}}, nil, pramaan.Idempotency(payload.ID))
}

// newDeliveryID returns a fresh idempotency / delivery id for a webhook event.
func newDeliveryID() string { return uuid.New().String() }

// truncateRunes caps a string at max runes (provider embed/field limits), adding
// an ellipsis when it trims so a long custom template can't get rejected.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}
