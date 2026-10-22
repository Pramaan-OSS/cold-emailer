package webhook

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/pramaan-oss/cold-email/internal/models"
)

// Two connected CRMs each keep their own record of every event, so wiring a
// second record sink must add it, not replace the first.
func TestWireRecordSinkKeepsEverySink(t *testing.T) {
	s := &service{}
	var got []string
	s.WireRecordSink(func(context.Context, uuid.UUID, models.WebhookEventType, any) { got = append(got, "connected") })
	s.WireRecordSink(func(context.Context, uuid.UUID, models.WebhookEventType, any) { got = append(got, "connected") })
	s.WireRecordSink(nil)
	for _, sink := range s.recordSinks {
		sink(context.Background(), uuid.New(), models.WebhookEventCampaignEmailSent, nil)
	}
	if len(got) != 2 || got[0] != "connected" || got[1] != "connected" {
		t.Fatalf("every sink sees the event once: %v", got)
	}
}
