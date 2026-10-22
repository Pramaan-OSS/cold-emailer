package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

type actionFixture struct {
	calls          []string
	input          map[string]any
	failInspection bool
}

func (f *actionFixture) Call(_ context.Context, id string, input, out any, _ ...pramaan.Option) error {
	f.calls = append(f.calls, id)
	if id == "apps.catalog.search" && f.failInspection {
		return errors.New("schema unavailable")
	}
	if id == "apps.catalog.search" {
		return json.Unmarshal([]byte(`{"results":[{"app":"fixture_app","action":"SELECTED_ACTION","inputSchema":{"type":"object","properties":{"email":{"type":"string"}}}}]}`), out)
	}
	if id == "apps.action.execute" {
		f.input = input.(map[string]any)
	}
	return json.Unmarshal([]byte(`{"status":"success","data":{}}`), out)
}
func TestConnectedActionRequiresSelectedHandle(t *testing.T) {
	f := &actionFixture{}
	s := &service{client: f}
	target := repository.DispatchTarget{Subscription: models.IntegrationEventSubscription{Config: []byte(`{}`)}}
	if s.execAction(context.Background(), target, nil) == nil || len(f.calls) != 0 {
		t.Fatal("missing selection made an outside call")
	}
}
func TestConnectedActionInspectsAndScopesOnlySelectedFields(t *testing.T) {
	f := &actionFixture{}
	s := &service{client: f}
	org := uuid.New()
	target := repository.DispatchTarget{Subscription: models.IntegrationEventSubscription{Config: []byte(`{"pramaan_action":"SELECTED_ACTION","pramaan_input":{"email":"{{.contact_email}}"}}`)}, Secrets: repository.ConnectionSecrets{Conn: models.IntegrationConnection{OrganizationID: org}}}
	err := s.execAction(context.Background(), target, map[string]any{"contact_email": "fixture@example.test", "private_note": "must stay local"})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 || f.calls[0] != "apps.catalog.search" || f.calls[1] != "apps.action.execute" {
		t.Fatal(f.calls)
	}
	if f.input["endUserRef"] != "org_"+org.String() {
		t.Fatal("wrong owner")
	}
	in := f.input["input"].(map[string]any)
	if len(in) != 1 || in["email"] != "fixture@example.test" {
		t.Fatal("unselected fields escaped")
	}
}
func TestConnectedActionInspectionFailurePreventsWrite(t *testing.T) {
	f := &actionFixture{failInspection: true}
	s := &service{client: f}
	target := repository.DispatchTarget{Subscription: models.IntegrationEventSubscription{Config: []byte(`{"pramaan_action":"SELECTED_ACTION"}`)}}
	if s.execAction(context.Background(), target, nil) == nil || len(f.calls) != 1 {
		t.Fatal("write escaped failed inspection")
	}
}
