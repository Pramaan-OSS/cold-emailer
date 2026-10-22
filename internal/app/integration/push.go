package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

// PushContact is the generic, contact-agnostic record the synchronous push
// action upserts into a connected CRM. The handler maps domain contacts onto
// this shape so the integration service stays decoupled from the contacts
// package.
type PushContact struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Company   string    `json:"company"`
	Phone     string    `json:"phone"`
}

// PushRecordResult is the per-record outcome returned to the dashboard so it can
// show exactly which contacts synced and which failed.
type PushRecordResult struct {
	ContactID uuid.UUID `json:"contact_id"`
	Email     string    `json:"email"`
	OK        bool      `json:"ok"`
	Error     string    `json:"error,omitempty"`
}

// PushResult is the aggregate outcome of a push.
type PushResult struct {
	Provider string             `json:"provider"`
	Pushed   int                `json:"pushed"`
	Failed   int                `json:"failed"`
	Results  []PushRecordResult `json:"results"`
}

// ErrPushUnsupported is returned when a push is requested against a provider
// that has no contact-upsert handler.
var ErrPushUnsupported = errors.New("this integration does not support pushing contacts")

// ErrPushReauth is returned when the connection's token can't be refreshed; the
// user must reconnect before pushing. The connection is already flipped to
// reauth_required by the token path.
var ErrPushReauth = errors.New("this integration needs to be reconnected before you can push")

// PushContacts upserts a batch of contacts into a connected CRM on demand. It is
// idempotent at the provider level (every CRM upsert is keyed by email), so
// retries and repeated pushes are naturally safe and the endpoint needs no
// Idempotency-Key bookkeeping. Failures are reported per record rather than
// aborting the batch, and a sync run records the outcome for observability.
func (s *service) PushContacts(ctx context.Context, orgID, connID uuid.UUID, contacts []PushContact) (*PushResult, error) {
	conn, err := s.repo.GetConnectionByID(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, errors.New("connection not found")
	}
	var options map[string]any
	_ = json.Unmarshal(conn.ConfigCapabilities, &options)
	action, _ := options["pramaan_action"].(string)
	if action == "" || s.client == nil {
		return nil, errors.New("Choose and save a contact action in Connections first.")
	}
	apps := pramaan.Apps{Client: s.client}
	if _, err := apps.Inspect(ctx, action); err != nil {
		return nil, err
	}
	if conn.Status != models.IntegrationStatusConnected && conn.Status != models.IntegrationStatusDegraded {
		return nil, fmt.Errorf("connection is not usable (status: %s)", conn.Status)
	}
	run := &models.IntegrationSyncRun{
		ConnectionID:   connID,
		OrganizationID: orgID,
		Kind:           "manual_push",
		Detail:         fmt.Sprintf("push %d contact(s) to %s", len(contacts), conn.Provider),
	}
	_ = s.repo.CreateSyncRun(ctx, run)

	result := &PushResult{Provider: string(conn.Provider), Results: make([]PushRecordResult, 0, len(contacts))}
	var lastErr error
	for _, ct := range contacts {
		rr := PushRecordResult{ContactID: ct.ID, Email: ct.Email, OK: true}
		if strings.TrimSpace(ct.Email) == "" {
			rr.OK = false
			rr.Error = "contact has no email"
			result.Failed++
			result.Results = append(result.Results, rr)
			continue
		}
		input, _ := options["pramaan_input"].(map[string]any)
		props := map[string]any{}
		for key, value := range input {
			if text, ok := value.(string); ok {
				props[key] = renderTemplate(text, contactSource(ct))
			} else {
				props[key] = value
			}
		}
		_, aerr := apps.Execute(ctx, "org_"+orgID.String(), action, props, uuid.NewString())
		if aerr != nil {
			rr.OK = false
			rr.Error = truncate(aerr.Error(), 240)
			lastErr = aerr
			result.Failed++
		} else {
			result.Pushed++
		}
		result.Results = append(result.Results, rr)
	}

	// Record the outcome on the sync run + connection health.
	switch {
	case result.Failed == 0:
		_ = s.repo.FinishSyncRun(ctx, run.ID, "success", "", result.Pushed)
		_ = s.repo.SetConnectionStatus(ctx, connID, models.IntegrationStatusConnected, models.IntegrationHealthHealthy, "")
	default:
		status := "error"
		if result.Pushed > 0 {
			status = "partial"
		}
		_ = s.repo.FinishSyncRun(ctx, run.ID, status, fmt.Sprintf("%d ok, %d failed", result.Pushed, result.Failed), result.Pushed)
		// Only degrade health when nothing landed — a partial batch (e.g. a few
		// contacts missing emails) shouldn't trip a connection-health alert.
		if result.Pushed == 0 && lastErr != nil {
			_ = s.repo.SetConnectionStatus(ctx, connID, models.IntegrationStatusConnected, models.IntegrationHealthDegraded, truncate(lastErr.Error(), 240))
		}
	}
	return result, nil
}

// ListFieldMappings returns every field mapping for a connection.
func (s *service) ListFieldMappings(ctx context.Context, orgID, connID uuid.UUID) ([]models.IntegrationFieldMapping, error) {
	return s.repo.ListFieldMappings(ctx, orgID, connID)
}

// ReplaceFieldMappings swaps the connection-default push map for one object
// (defaulting the object to the provider's primary object when unspecified).
func (s *service) ReplaceFieldMappings(ctx context.Context, orgID, connID uuid.UUID, object string, mappings []models.IntegrationFieldMapping) error {
	conn, err := s.repo.GetConnectionByID(ctx, orgID, connID)
	if err != nil {
		return err
	}
	if conn == nil {
		return errors.New("connection not found")
	}
	if object == "" {
		object = defaultObject(conn.Provider)
	}
	return s.repo.ReplaceConnectionFieldMappings(ctx, orgID, connID, object, mappings)
}

// UpdateConnectionConfig persists the onboarding/capability snapshot + direction.
func (s *service) UpdateConnectionConfig(ctx context.Context, orgID, connID uuid.UUID, configCapabilities map[string]any, syncDirection string) (*models.IntegrationConnection, error) {
	conn, err := s.repo.GetConnectionByID(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, errors.New("connection not found")
	}
	switch models.SyncDirection(syncDirection) {
	case models.SyncDirectionPush, models.SyncDirectionPull, models.SyncDirectionBoth:
	case "":
		syncDirection = conn.SyncDirection
		if syncDirection == "" {
			syncDirection = string(models.SyncDirectionPush)
		}
	default:
		return nil, fmt.Errorf("invalid sync direction %q", syncDirection)
	}
	// The signing secret is server-held and never sent to the dashboard, so a
	// save carries the stored one over and ignores any value in the request.
	next := make(map[string]any, len(configCapabilities)+1)
	for k, v := range configCapabilities {
		if k != models.ConfigCapabilitiesSigningSecret {
			next[k] = v
		}
	}
	if stored := configString(conn.ConfigCapabilities, models.ConfigCapabilitiesSigningSecret); stored != "" {
		next[models.ConfigCapabilitiesSigningSecret] = stored
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateConnectionConfig(ctx, orgID, connID, raw, syncDirection); err != nil {
		return nil, err
	}
	return s.repo.GetConnectionByID(ctx, orgID, connID)
}

// providerSupportsPush reports whether a provider can be a push target, reading
// the same catalog metadata the dashboard uses to render the action.
func providerSupportsPush(p models.IntegrationProvider) bool {
	for _, e := range Catalog() {
		if e.Provider == p {
			return e.SupportsPush
		}
	}
	return false
}

// contactSource normalizes a PushContact into the ColdEmail source vocabulary the
// field map projects from.
func contactSource(ct PushContact) map[string]any {
	return map[string]any{
		"email":      ct.Email,
		"first_name": ct.FirstName,
		"last_name":  ct.LastName,
		"company":    ct.Company,
		"phone":      ct.Phone,
		"name":       strings.TrimSpace(ct.FirstName + " " + ct.LastName),
	}
}

// SetConfigKey replaces one key of a connection's config_capabilities, leaving
// the rest as stored. Used by provider settings that validate themselves.
func (s *service) SetConfigKey(ctx context.Context, orgID, connID uuid.UUID, key string, value any) error {
	conn, err := s.repo.GetConnectionByID(ctx, orgID, connID)
	if err != nil {
		return err
	}
	if conn == nil {
		return errors.New("connection not found")
	}
	cc := map[string]any{}
	if len(conn.ConfigCapabilities) > 0 {
		_ = json.Unmarshal(conn.ConfigCapabilities, &cc)
	}
	cc[key] = value
	raw, err := json.Marshal(cc)
	if err != nil {
		return err
	}
	dir := conn.SyncDirection
	if dir == "" {
		dir = string(models.SyncDirectionPush)
	}
	return s.repo.UpdateConnectionConfig(ctx, orgID, connID, raw, dir)
}
