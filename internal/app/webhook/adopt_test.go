package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"github.com/pramaan-oss/cold-email/internal/repository"
	"strings"
	"testing"
	"time"
)

type adoptedRepo struct {
	repository.WebhookRepository
	endpoint             *models.WebhookEndpoint
	stored               string
	abandoned, delivered int
}

func (r *adoptedRepo) GetEndpoint(_ context.Context, org, id uuid.UUID) (*models.WebhookEndpoint, error) {
	if r.endpoint.OrganizationID != org || r.endpoint.ID != id {
		return nil, nil
	}
	return r.endpoint, nil
}
func (r *adoptedRepo) GetEndpointSecret(context.Context, uuid.UUID) (string, error) {
	return r.stored, nil
}
func (r *adoptedRepo) MarkAbandoned(context.Context, uuid.UUID, string) error {
	r.abandoned++
	return nil
}
func (r *adoptedRepo) MarkDelivered(context.Context, uuid.UUID, int) error            { r.delivered++; return nil }
func (r *adoptedRepo) UpdateEndpointHealthOnSuccess(context.Context, uuid.UUID) error { return nil }

type signedFake struct {
	calls int
	fail  bool
	input map[string]any
}

func (f *signedFake) Call(_ context.Context, id string, in, out any, _ ...pramaan.Option) error {
	f.calls++
	f.input = in.(map[string]any)
	if f.fail {
		return errors.New("Delivery uncertain")
	}
	b, _ := json.Marshal(map[string]any{"status": "accepted", "eventId": f.input["eventId"]})
	return json.Unmarshal(b, out)
}
func TestSignedDeliveryAcceptsOrStopsWithoutRetry(t *testing.T) {
	for _, fail := range []bool{false, true} {
		org, id, event := uuid.New(), uuid.New(), uuid.New()
		now := time.Now()
		r := &adoptedRepo{endpoint: &models.WebhookEndpoint{ID: id, OrganizationID: org, Enabled: true, VerifiedAt: &now, URL: "https://receiver.example.test/events"}, stored: "pramaan-policy:owned-policy"}
		f := &signedFake{fail: fail}
		w := DeliveryWorker{repo: r, broker: f}
		w.deliver(context.Background(), &models.WebhookDelivery{ID: uuid.New(), EndpointID: id, OrganizationID: org, EventID: event, EventType: "message.received", Payload: json.RawMessage(`{"id":"synthetic"}`)})
		if f.calls != 1 || f.input["signaturePolicyRef"] != "owned-policy" || f.input["eventId"] != event.String() {
			t.Fatal(f)
		}
		if fail && r.abandoned != 1 || !fail && r.delivered != 1 {
			t.Fatalf("abandoned=%d delivered=%d", r.abandoned, r.delivered)
		}
		if strings.Contains(r.stored, "whsec_") {
			t.Fatal("stored receiver secret")
		}
	}
}
