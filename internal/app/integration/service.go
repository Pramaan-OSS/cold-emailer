package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pramaan-oss/cold-email/internal/pramaan"

	emailverifyapp "github.com/pramaan-oss/cold-email/internal/app/emailverify"
	"github.com/pramaan-oss/cold-email/internal/pkg/emailverify"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/pramaan-oss/cold-email/internal/app/cipher"
	"github.com/pramaan-oss/cold-email/internal/app/credits"
	"github.com/pramaan-oss/cold-email/internal/app/webhook"
	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/pubsub"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pkg/generation"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

// oauthStateTTL bounds how long a started OAuth handshake stays valid.
const oauthStateTTL = 15 * time.Minute

// ErrOAuthNotConfigured is returned when a provider's OAuth client credentials
// are not present in the environment.
var ErrOAuthNotConfigured = errors.New("oauth is not configured for this provider")

// ErrUseOAuth is returned when a caller tries to paste credentials for a
// provider that should be connected via the OAuth handshake instead.
var ErrUseOAuth = errors.New("this provider connects via OAuth; start the authorize flow instead")

// Service exposes the integration surface the dashboard and event pipeline
// talk to. Provider-specific behaviour (OAuth identity, event actions, inbound
// webhooks) lives in the per-provider files in this package.
type Service interface {
	// Catalog returns every built-in integration, ranked by popularity.
	Catalog(ctx context.Context) []models.IntegrationCatalogEntry
	ListConnections(ctx context.Context, orgID uuid.UUID) ([]models.IntegrationConnection, error)
	GetConnection(ctx context.Context, orgID, id uuid.UUID) (*models.IntegrationConnection, error)

	// Connect registers a credential-based connection (api-key / webhook-URL
	// providers). OAuth providers must use OAuthStart instead. The config map's
	// secret values are sealed with the connecting user's envelope DEK before
	// they touch the database.
	Connect(ctx context.Context, orgID, userID uuid.UUID, provider models.IntegrationProvider, label string, config map[string]any) (*models.IntegrationConnection, error)
	Disconnect(ctx context.Context, orgID, id uuid.UUID) error

	// OAuthStart returns the provider authorization URL for a one-click connect.
	// params carries provider options (connected: "environment", "domain").
	OAuthStart(ctx context.Context, orgID, userID uuid.UUID, provider models.IntegrationProvider, label string, params map[string]string) (*models.IntegrationOAuthStartResponse, error)
	OAuthReturnOrigin(ctx context.Context, state string) string
	// OAuthFinish completes the handshake: validates state, exchanges the code,
	// resolves the account identity, and persists encrypted tokens. authorize
	// runs against the state's organization before the code is exchanged.
	OAuthFinish(ctx context.Context, userID uuid.UUID, code, state string, authorize func(ctx context.Context, orgID uuid.UUID) error) (*models.IntegrationConnection, error)
	// Reauth starts a fresh OAuth handshake for an existing connection whose
	// token expired or was revoked.
	Reauth(ctx context.Context, orgID, userID, id uuid.UUID) (*models.IntegrationOAuthStartResponse, error)

	// RotateInboundSecret replaces a connected/connected connection's inbound URL
	// secret, so the old URL stops working at once, and returns the new URL.
	RotateInboundSecret(ctx context.Context, orgID, id uuid.UUID) (string, error)
	// SetInboundSigningKey stores (empty key: removes) the key connected/connected
	// deliveries must be signed with; InboundSigningKey reads it back.
	SetInboundSigningKey(ctx context.Context, orgID, connID uuid.UUID, key string) (*models.IntegrationConnection, error)
	InboundSigningKey(ctx context.Context, conn *models.IntegrationConnection) (string, error)

	// Event subscriptions wire a ColdEmail event to a provider action.
	ListEventSubscriptions(ctx context.Context, orgID, connID uuid.UUID) ([]models.IntegrationEventSubscription, error)
	CreateEventSubscription(ctx context.Context, orgID, connID uuid.UUID, eventType string, action models.IntegrationAction, config map[string]any, enabled bool) (*models.IntegrationEventSubscription, error)
	DeleteEventSubscription(ctx context.Context, orgID, id uuid.UUID) error

	// Automations: the visual flow builder. An automation is a trigger event +
	// action steps; steps persist as automation-tagged event-subscriptions and
	// run through the same dispatcher.
	ListAutomations(ctx context.Context, orgID uuid.UUID) ([]models.Automation, error)
	GetAutomation(ctx context.Context, orgID, id uuid.UUID) (*models.Automation, error)
	CreateAutomation(ctx context.Context, orgID uuid.UUID, w models.AutomationWrite) (*models.Automation, error)
	UpdateAutomation(ctx context.Context, orgID, id uuid.UUID, w models.AutomationWrite) (*models.Automation, error)
	// UpdateAutomationLayout persists only node canvas coordinates (drag-to-stick),
	// separate from a full graph save and without bumping updated_at.
	UpdateAutomationLayout(ctx context.Context, orgID, id uuid.UUID, positions []models.AutomationNodePosition) error
	DeleteAutomation(ctx context.Context, orgID, id uuid.UUID) error
	// RunAutomationByID executes one automation graph on demand (e.g. launched
	// from a campaign step), regardless of its configured trigger event. data is
	// the synthetic event payload used for condition + template evaluation.
	RunAutomationByID(ctx context.Context, orgID, automationID uuid.UUID, data map[string]any) error
	// DryRunAutomation walks the graph against sample/provided data WITHOUT side
	// effects, returning the path + per-action previews (the builder "Test").
	DryRunAutomation(ctx context.Context, orgID, id uuid.UUID, req models.DryRunRequest) (*models.DryRunResponse, error)
	// ListAutomationRuns returns recent run history for an automation.
	ListAutomationRuns(ctx context.Context, orgID, id uuid.UUID, limit int) ([]models.AutomationRun, error)
	// TriggerInboundAutomation runs the automation whose inbound-webhook token is
	// given, using body (JSON) as the event payload. Returns
	// ErrInboundAutomationNotFound for an unknown/non-inbound token.
	TriggerInboundAutomation(ctx context.Context, token string, body []byte) error
	// SetNativeActions wires the native CRM/contact action executor + the realtime
	// publisher post-construction (they depend on services built after this one).
	SetNativeActions(n NativeActions)
	SetPublisher(p *pubsub.StreamingPublisher)
	// SetAI wires the LLM provider + credit ledger the AI automation nodes
	// (ai_step / ai_switch) use. Both may be nil (AI steps then fail with a clear
	// "not available" error); the provider is nil when no AI_PROVIDER is
	// configured.
	SetAI(p generation.Provider, c credits.CreditService)
	// SetAISearch wires the optional web-search backend an ai-mode AI switch may
	// use to look up the event's company. Nil leaves web search unavailable (the
	// node runs without it). Mirrors tasks.Service.SetAISearch.
	SetAISearch(sc generation.SearchClient)

	// ListSyncRuns returns recent observability records for a connection.
	ListSyncRuns(ctx context.Context, orgID, connID uuid.UUID, limit int) ([]models.IntegrationSyncRun, error)

	// PushContacts upserts a batch of contacts into a connected CRM on demand.
	// Used by the contextual "push to CRM" action in the dashboard. Per-record
	// results are returned so the UI can report exactly what synced.
	PushContacts(ctx context.Context, orgID, connID uuid.UUID, contacts []PushContact) (*PushResult, error)

	// Field mappings drive how ColdEmail fields project onto provider fields.
	ListFieldMappings(ctx context.Context, orgID, connID uuid.UUID) ([]models.IntegrationFieldMapping, error)
	// ReplaceFieldMappings swaps the connection-default map for one object.
	ReplaceFieldMappings(ctx context.Context, orgID, connID uuid.UUID, object string, mappings []models.IntegrationFieldMapping) error
	// UpdateConnectionConfig persists the onboarding/capability snapshot + sync
	// direction for a connection.
	UpdateConnectionConfig(ctx context.Context, orgID, connID uuid.UUID, configCapabilities map[string]any, syncDirection string) (*models.IntegrationConnection, error)
	// SetConfigKey replaces one key of config_capabilities, keeping the rest.
	SetConfigKey(ctx context.Context, orgID, connID uuid.UUID, key string, value any) error

	// WebhookSigningSecret returns the HMAC signing secret for an automation
	// connection's outbound webhook deliveries, generating + persisting one on
	// first request so the user can configure signature verification on their end.
	WebhookSigningSecret(ctx context.Context, orgID, connID uuid.UUID) (string, error)
	// SendTestEvent delivers a synthetic event through the connection's
	// configured notify/webhook automations so the user can verify the wiring.
	// Returns how many automations it fired.
	SendTestEvent(ctx context.Context, orgID, connID uuid.UUID) (int, error)

	// MarkSynced records a successful/failed round-trip against a connection.
	MarkSynced(ctx context.Context, id uuid.UUID, status models.IntegrationStatus, displayFields map[string]any, errMsg string) error

	// --- Google Sheets read helpers (used by the lead-sync feature) ---------
	// These expose the existing google_sheets OAuth token + Sheets client to
	// the leadsync package without leaking secret handling out of this service.

	// GoogleConnection returns the org's google_sheets OAuth connection, or nil
	// if the org has not connected Google. Even though google_sheets is hidden
	// from the integrations catalog, the underlying OAuth connection still lives
	// in integration_connections — this is how lead-sync finds it.
	GoogleConnection(ctx context.Context, orgID uuid.UUID) (*models.IntegrationConnection, error)
	// SpreadsheetMeta returns the sheet's title + tabs using the connection's
	// (refreshed) Google token.
	SpreadsheetMeta(ctx context.Context, orgID, connID uuid.UUID, sheetID string) (*SheetMeta, error)
	// SpreadsheetValues reads an A1 range from the sheet using the connection's
	// (refreshed) Google token.
	SpreadsheetValues(ctx context.Context, orgID, connID uuid.UUID, sheetID, a1Range string) ([][]string, error)

	// Dispatch fans a platform event out to every matching event subscription,
	// executing each provider action. Best-effort: action failures are recorded
	// on the connection's health but never block the caller.
	Dispatch(ctx context.Context, orgID uuid.UUID, eventType models.WebhookEventType, data map[string]any)

	// DispatchAny is the loosely-typed adapter wired into the webhook fan-out
	// sink. It forwards only map-shaped payloads (the common event shape) to
	// Dispatch; struct payloads are ignored.
	DispatchAny(ctx context.Context, orgID uuid.UUID, eventType models.WebhookEventType, data any)

	// VerificationProviderFor and ReportVerificationProviderError implement
	// emailverify.ProviderSource: the org's paid verification backend, if any.
	VerificationProviderFor(ctx context.Context, orgID uuid.UUID) (*emailverifyapp.Provider, error)
	ReportVerificationProviderError(ctx context.Context, connectionID uuid.UUID, err error)
	ClearVerificationProviderError(ctx context.Context, connectionID uuid.UUID)

	// Repo exposes the underlying repository for the inbound webhook handlers.
	Repo() repository.IntegrationRepository

	// AccessToken returns a usable OAuth access token for one of the org's
	// connections, refreshing it when near expiry. Used by the CRM sync engine.
	AccessToken(ctx context.Context, orgID, connID uuid.UUID) (string, *models.IntegrationConnection, error)
	// MarkConnectionHealth records a provider call's outcome on the connection.
	MarkConnectionHealth(ctx context.Context, connID uuid.UUID, status models.IntegrationStatus, health models.IntegrationHealth, detail string)
	// MergeConnectionDisplay overlays non-secret facts (a connected company
	// domain) onto a connection's display fields.
	MergeConnectionDisplay(ctx context.Context, connID uuid.UUID, patch map[string]any) error
	// SetCRMModeCheck reports which CRM a workspace runs on, so the legacy
	// upsert action of that provider steps aside.
	SetCRMModeCheck(check func(ctx context.Context, orgID uuid.UUID) models.CRMProvider)
	// ProviderAccess returns a usable OAuth access token for an org-owned
	// connection, refreshing first when force is set or the token is expiring.
	ProviderAccess(ctx context.Context, orgID, connID uuid.UUID, force bool) (*ProviderAccess, error)
	// Setconnected routes connected pushes, upsert actions and new
	// connections through the native sync.
	// SetChatInstallHook is told who approved a Chat install, so the member
	// who connected Chat is linked without a separate step.
}

// ProviderAccess is a live credential for one connection.
type ProviderAccess struct {
	Token       string
	InstanceURL string
	Conn        *models.IntegrationConnection
}

type service struct {
	client     pramaan.Caller
	refresh    singleflight.Group
	crmMode    func(ctx context.Context, orgID uuid.UUID) models.CRMProvider
	repo       repository.IntegrationRepository
	cipher     cipher.CipherService
	native     NativeActions
	publisher  *pubsub.StreamingPublisher
	aiProvider generation.Provider
	credits    credits.CreditService
	aiSearch   generation.SearchClient

	popMu         sync.Mutex
	pop           map[models.IntegrationProvider]int
	popAt         time.Time
	popRefreshing bool
}

// NewService builds the integration service. cipherSvc seals provider secrets
// with the connecting user's envelope DEK; oauth drives the OAuth handshakes.
// Native actions + realtime publisher are wired post-construction (SetNativeActions
// / SetPublisher) since they depend on services built after this one.
func NewService(repo repository.IntegrationRepository, cipherSvc cipher.CipherService, clients ...pramaan.Caller) Service {
	var client pramaan.Caller
	if len(clients) > 0 {
		client = clients[0]
	}
	return &service{repo: repo, cipher: cipherSvc, client: client}
}

func (s *service) SetNativeActions(n NativeActions)          { s.native = n }
func (s *service) SetPublisher(p *pubsub.StreamingPublisher) { s.publisher = p }
func (s *service) SetAI(p generation.Provider, c credits.CreditService) {
	s.aiProvider = p
	s.credits = c
}
func (s *service) SetAISearch(sc generation.SearchClient) { s.aiSearch = sc }

func (s *service) Repo() repository.IntegrationRepository { return s.repo }

func (s *service) Catalog(ctx context.Context) []models.IntegrationCatalogEntry {
	if s.client == nil {
		return nil
	}
	result, err := (pramaan.Apps{Client: s.client}).Search(ctx, "connected app actions", "")
	if err != nil {
		return nil
	}
	rows, _ := result["results"].([]any)
	seen := map[string]bool{}
	entries := []models.IntegrationCatalogEntry{}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		app, _ := row["app"].(string)
		if app == "" || seen[app] {
			continue
		}
		seen[app] = true
		entries = append(entries, models.IntegrationCatalogEntry{Provider: models.IntegrationProvider(app), Name: app, Configured: true, ActionTypes: []string{string(models.IntegrationActionConnected)}})
	}
	return entries
}

func (s *service) ListConnections(ctx context.Context, orgID uuid.UUID) ([]models.IntegrationConnection, error) {
	if s.client == nil {
		return s.repo.ListConnections(ctx, orgID)
	}
	var result struct {
		Results []struct {
			App    string `json:"app"`
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := s.client.Call(ctx, "apps.connections.list", map[string]any{"endUserRef": "org_" + orgID.String()}, &result); err != nil {
		return nil, err
	}
	for _, row := range result.Results {
		if row.App == "" {
			continue
		}
		status := models.IntegrationStatusPending
		if row.Status == "active" {
			status = models.IntegrationStatusConnected
		} else if row.Status == "revoked" || row.Status == "disabled" || row.Status == "expired" {
			status = models.IntegrationStatusDisconnected
		}
		conn := &models.IntegrationConnection{OrganizationID: orgID, Provider: models.IntegrationProvider(row.App), Label: row.App, Status: status, AuthMethod: string(models.IntegrationAuthOAuth), DisplayFields: []byte("{}"), Health: string(models.IntegrationHealthUnknown)}
		if err := s.repo.UpsertConnection(ctx, &repository.ConnectionWrite{Conn: conn}); err != nil {
			return nil, err
		}
	}
	return s.repo.ListConnections(ctx, orgID)
}

func (s *service) GetConnection(ctx context.Context, orgID, id uuid.UUID) (*models.IntegrationConnection, error) {
	return s.repo.GetConnectionByID(ctx, orgID, id)
}

func (s *service) Connect(ctx context.Context, orgID, userID uuid.UUID, provider models.IntegrationProvider, label string, config map[string]any) (*models.IntegrationConnection, error) {
	return nil, errors.New("Connect your app in Connections; this endpoint does not accept outside credentials.")
}

func (s *service) Disconnect(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.DeleteConnection(ctx, orgID, id)
}

func (s *service) OAuthStart(ctx context.Context, orgID, userID uuid.UUID, provider models.IntegrationProvider, label string, params map[string]string) (*models.IntegrationOAuthStartResponse, error) {
	if s.client == nil {
		return nil, pramaan.ErrUnavailable
	}
	out, e := (pramaan.Apps{Client: s.client}).Connect(ctx, "org_"+orgID.String(), string(provider), strings.TrimRight(os.Getenv("APP_URL"), "/")+"/app/integrations", uuid.NewString())
	if e != nil {
		return nil, e
	}
	url, _ := out["connectUrl"].(string)
	return &models.IntegrationOAuthStartResponse{URL: url}, nil
}

func (s *service) OAuthReturnOrigin(ctx context.Context, state string) string {
	origin, err := s.repo.OAuthReturnOrigin(ctx, state)
	if err != nil {
		return ""
	}
	if origin == "" {
		return config.PrimaryDashboardOrigin()
	}
	return config.DashboardOrigin(origin)
}

func (s *service) Reauth(ctx context.Context, orgID, userID, id uuid.UUID) (*models.IntegrationOAuthStartResponse, error) {
	conn, err := s.repo.GetConnectionByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, errors.New("connection not found")
	}
	if conn.AuthMethod != string(models.IntegrationAuthOAuth) {
		return nil, ErrUseOAuth
	}
	_ = s.repo.SetConnectionStatus(ctx, id, models.IntegrationStatusAuthorizing, models.IntegrationHealthDegraded, "reauthorizing")
	var params map[string]string
	if host := configString(conn.DisplayFields, "login_host"); host != "" {
		params = map[string]string{"domain": host}
	}
	return s.OAuthStart(ctx, orgID, userID, conn.Provider, conn.Label, params)
}

func (s *service) RotateInboundSecret(ctx context.Context, orgID, id uuid.UUID) (string, error) {
	conn, err := s.repo.GetConnectionByID(ctx, orgID, id)
	if err != nil {
		return "", err
	}
	if conn == nil {
		return "", repository.ErrInboundConnectionNotFound
	}
	if !IsInboundProvider(conn.Provider) {
		return "", ErrNotInboundProvider
	}
	secret, err := generateInboundSecret(conn.Provider)
	if err != nil {
		return "", err
	}
	if err := s.repo.SetInboundSecret(ctx, orgID, id, secret); err != nil {
		return "", err
	}
	return BuildInboundURL(conn.Provider, secret), nil
}

func (s *service) ListEventSubscriptions(ctx context.Context, orgID, connID uuid.UUID) ([]models.IntegrationEventSubscription, error) {
	return s.repo.ListEventSubscriptions(ctx, orgID, connID)
}

func (s *service) CreateEventSubscription(ctx context.Context, orgID, connID uuid.UUID, eventType string, action models.IntegrationAction, config map[string]any, enabled bool) (*models.IntegrationEventSubscription, error) {
	conn, err := s.repo.GetConnectionByID(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, errors.New("connection not found")
	}
	if !models.IsValidWebhookEventType(eventType) {
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
	if !models.ProviderSupportsAction(conn.Provider, action) {
		return nil, ErrActionProviderMismatch
	}
	if err := s.validateConnectedConfig(ctx, config); err != nil {
		return nil, err
	}
	// SSRF guard for action configs that carry an outbound URL.
	if err := validateOutboundConfigURLs(config); err != nil {
		return nil, err
	}
	cfg, _ := json.Marshal(config)
	sub := &models.IntegrationEventSubscription{
		ConnectionID:   connID,
		OrganizationID: orgID,
		EventType:      eventType,
		Action:         action,
		Config:         cfg,
		Enabled:        enabled,
	}
	if err := s.repo.CreateEventSubscription(ctx, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *service) DeleteEventSubscription(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.DeleteEventSubscription(ctx, orgID, id)
}

// --- Automations ------------------------------------------------------------

func (s *service) ListAutomations(ctx context.Context, orgID uuid.UUID) ([]models.Automation, error) {
	list, err := s.repo.ListAutomations(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		decorateAutomation(&list[i])
	}
	return list, nil
}

func (s *service) GetAutomation(ctx context.Context, orgID, id uuid.UUID) (*models.Automation, error) {
	a, err := s.repo.GetAutomation(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	decorateAutomation(a)
	return a, nil
}

func (s *service) CreateAutomation(ctx context.Context, orgID uuid.UUID, w models.AutomationWrite) (*models.Automation, error) {
	a, err := s.buildAutomation(ctx, orgID, w)
	if err != nil {
		return nil, err
	}
	if isInboundTrigger(a.TriggerEvent) {
		tok, terr := generateAutomationInboundToken()
		if terr != nil {
			return nil, terr
		}
		a.InboundToken = tok
	}
	if err := s.keepFedCampaignsRunning(ctx, a); err != nil {
		return nil, err
	}
	if err := s.repo.CreateAutomation(ctx, a); err != nil {
		return nil, err
	}
	return s.GetAutomation(ctx, orgID, a.ID)
}

func (s *service) UpdateAutomation(ctx context.Context, orgID, id uuid.UUID, w models.AutomationWrite) (*models.Automation, error) {
	a, err := s.buildAutomation(ctx, orgID, w)
	if err != nil {
		return nil, err
	}
	a.ID = id
	// Preserve the inbound token across edits; mint one the first time a flow
	// becomes inbound; clear it when the trigger changes away (so the old URL
	// stops firing).
	if isInboundTrigger(a.TriggerEvent) {
		existing, _ := s.repo.GetAutomation(ctx, orgID, id)
		switch {
		case existing != nil && existing.InboundToken != "":
			a.InboundToken = existing.InboundToken
		default:
			tok, terr := generateAutomationInboundToken()
			if terr != nil {
				return nil, terr
			}
			a.InboundToken = tok
		}
	}
	if err := s.keepFedCampaignsRunning(ctx, a); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateAutomation(ctx, a); err != nil {
		return nil, err
	}
	return s.GetAutomation(ctx, orgID, id)
}

// UpdateAutomationLayout persists only the canvas coordinates of the given
// nodes. Cosmetic and high-churn (written as cards are dragged), so it stays out
// of the build/validate path of a full save and never bumps updated_at.
func (s *service) UpdateAutomationLayout(ctx context.Context, orgID, id uuid.UUID, positions []models.AutomationNodePosition) error {
	return s.repo.UpdateAutomationLayout(ctx, orgID, id, positions)
}

// isInboundTrigger reports whether a trigger event is the inbound webhook.
func isInboundTrigger(trigger string) bool {
	return trigger == string(models.WebhookEventInboundWebhook)
}

// decorateAutomation fills the computed, non-stored fields on an automation
// (today: the public InboundURL derived from its token).
func decorateAutomation(a *models.Automation) {
	if a == nil {
		return
	}
	a.InboundURL = BuildInboundAutomationURL(a.InboundToken)
}

func (s *service) DeleteAutomation(ctx context.Context, orgID, id uuid.UUID) error {
	// Referential integrity: refuse to orphan campaign "Run automation" steps that
	// point at this automation. A blocked delete names the campaigns so the user
	// knows where to remove the step first (a typed Conflict the handler maps to 409).
	used, err := s.repo.CampaignsUsingAutomation(ctx, orgID, id)
	if err != nil {
		return err
	}
	if len(used) > 0 {
		return errx.New(errx.Conflict, automationInUseMessage(used))
	}
	return s.repo.DeleteAutomation(ctx, orgID, id)
}

// automationInUseMessage builds a human, actionable conflict message naming up to
// three referencing campaigns.
func automationInUseMessage(names []string) string {
	shown, more := names, 0
	if len(shown) > 3 {
		shown, more = shown[:3], len(shown)-3
	}
	list := strings.Join(shown, ", ")
	if more > 0 {
		list = fmt.Sprintf("%s and %d more", list, more)
	}
	noun := "campaign"
	if len(names) != 1 {
		noun = "campaigns"
	}
	return fmt.Sprintf("This automation is still used by %d %s (%s). Remove the 'Run automation' step from those campaigns before deleting it.", len(names), noun, list)
}

// buildAutomation validates a write payload (trigger event, step connections,
// SSRF on step URLs) and merges the automation-level filter into each step's
// config so the existing subscription filter logic applies to every step.
func (s *service) buildAutomation(ctx context.Context, orgID uuid.UUID, w models.AutomationWrite) (*models.Automation, error) {
	name := strings.TrimSpace(w.Name)
	if name == "" {
		name = "Automation"
	}
	trigger := strings.TrimSpace(w.TriggerEvent)
	if !models.IsValidWebhookEventType(trigger) {
		return nil, fmt.Errorf("unknown trigger event: %s", trigger)
	}

	if err := s.validateAutomationGraph(ctx, orgID, w.Graph); err != nil {
		return nil, err
	}

	filter := w.Filter
	if len(filter) == 0 {
		filter = json.RawMessage("{}")
	}
	return &models.Automation{
		OrganizationID: orgID,
		Name:           name,
		Enabled:        w.Enabled,
		TriggerEvent:   trigger,
		Filter:         filter,
		Graph:          w.Graph,
	}, nil
}

// validateAutomationGraph checks node/edge integrity: a single trigger, edges
// referencing real nodes, action nodes pointing at org-owned connections (with
// an SSRF check on any outbound URL), valid condition fields/operators, and no
// cycles. An empty graph (just being drafted) is allowed.
func (s *service) validateAutomationGraph(ctx context.Context, orgID uuid.UUID, g models.AutomationGraph) error {
	byID := make(map[string]models.AutomationNode, len(g.Nodes))
	triggers := 0
	for _, n := range g.Nodes {
		if n.ID == "" {
			return errors.New("a node is missing its id")
		}
		if _, dup := byID[n.ID]; dup {
			return fmt.Errorf("duplicate node id: %s", n.ID)
		}
		byID[n.ID] = n
		switch n.Type {
		case models.AutomationNodeTrigger:
			triggers++
		case models.AutomationNodeStop:
			// A terminal marker: no action, no config, no outgoing edges. Accepted
			// so a path can explicitly end at a visible Stop node.
		case models.AutomationNodeAction:
			if strings.TrimSpace(string(n.Action)) == "" {
				return errors.New("an action node is missing its action")
			}
			// Native (ColdEmail-internal) actions run on the event's contact with no
			// external connection — validate their own config instead.
			if models.IsNativeAction(n.Action) {
				if err := validateNativeActionConfig(n.Action, n.Config); err != nil {
					return err
				}
				break
			}
			if n.ConnectionID == nil {
				return errors.New("an action node has no integration selected")
			}
			conn, err := s.repo.GetConnectionByID(ctx, orgID, *n.ConnectionID)
			if err != nil {
				return err
			}
			if conn == nil {
				return errors.New("an action node references an unknown integration")
			}
			if !models.ProviderSupportsAction(conn.Provider, n.Action) {
				return ErrActionProviderMismatch
			}
			cfg := map[string]any{}
			if len(n.Config) > 0 {
				_ = json.Unmarshal(n.Config, &cfg)
			}
			if err := s.validateConnectedConfig(ctx, cfg); err != nil {
				return err
			}
			if err := validateOutboundConfigURLs(cfg); err != nil {
				return err
			}
		case models.AutomationNodeCondition:
			if n.Condition == nil {
				return errors.New("a condition node has no condition set")
			}
			if !models.ValidAutomationConditionField(n.Condition.Field) {
				return fmt.Errorf("unknown condition field: %s", n.Condition.Field)
			}
			if n.Condition.Field == models.AutoCondExpression {
				// Free-form predicate: no operator; just validate it compiles.
				if err := ValidExpression(n.Condition.Expression); err != nil {
					return fmt.Errorf("invalid condition expression: %w", err)
				}
			} else if n.Condition.Field == models.AutoCondAI {
				// Ask-AI branch: no operator; just a bounded yes/no question.
				p := strings.TrimSpace(n.Condition.Prompt)
				if p == "" {
					return errors.New("an Ask AI branch needs a question")
				}
				if len(p) > maxAIConditionPrompt {
					return fmt.Errorf("an Ask AI question is limited to %d characters", maxAIConditionPrompt)
				}
			} else {
				if !models.ValidAutomationConditionOperator(n.Condition.Operator) {
					return fmt.Errorf("unknown condition operator: %s", n.Condition.Operator)
				}
				if n.Condition.Field == models.AutoCondField && strings.TrimSpace(n.Condition.Key) == "" {
					return errors.New("a condition is missing the field to test")
				}
			}
		default:
			return fmt.Errorf("unknown node type: %s", n.Type)
		}
	}
	if len(g.Nodes) > 0 && triggers != 1 {
		return errors.New("the flow must have exactly one trigger")
	}

	adj := map[string][]string{}
	for _, e := range g.Edges {
		src, ok := byID[e.Source]
		if !ok {
			return errors.New("an edge starts from a node that does not exist")
		}
		if _, ok := byID[e.Target]; !ok {
			return errors.New("an edge points to a node that does not exist")
		}
		if e.Source == e.Target {
			return errors.New("a node cannot connect to itself")
		}
		// Branch labels: a condition's edges are the yes/no paths; an action may
		// have a plain "then" edge plus an optional "on error" branch; an AI
		// classify action may additionally carry per-label branches
		// ("label:<x>", x from the node's configured labels) so the canvas
		// routes multi-way on the model's verdict; anything else is an
		// unconditional "then".
		switch {
		case src.Type == models.AutomationNodeCondition:
			if e.When != "true" && e.When != "false" {
				return errors.New("a condition's branches must be a yes or no path")
			}
		case src.Type == models.AutomationNodeAction:
			if lbl, ok := strings.CutPrefix(e.When, aiLabelEdgePrefix); ok {
				if !aiNodeRoutesByLabel(src) {
					return errors.New("only an AI switch can have per-case branches")
				}
				if !aiNodeHasBranch(src, lbl) {
					return fmt.Errorf("branch case %q is not one of the AI switch's cases", lbl)
				}
			} else if e.When != "" && e.When != "error" {
				return errors.New("an action edge must be a plain path or an on-error branch")
			}
		case e.When != "":
			return errors.New("only conditions and actions can have branch labels")
		}
		adj[e.Source] = append(adj[e.Source], e.Target)
	}
	if hasCycle(byID, adj) {
		return errors.New("the flow has a loop; remove the cycle")
	}
	return nil
}

// hasCycle does a DFS colour-walk over the node graph.
func hasCycle(nodes map[string]models.AutomationNode, adj map[string][]string) bool {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var visit func(id string) bool
	visit = func(id string) bool {
		color[id] = gray
		for _, m := range adj[id] {
			switch color[m] {
			case gray:
				return true
			case white:
				if visit(m) {
					return true
				}
			}
		}
		color[id] = black
		return false
	}
	for id := range nodes {
		if color[id] == white {
			if visit(id) {
				return true
			}
		}
	}
	return false
}

func (s *service) ListSyncRuns(ctx context.Context, orgID, connID uuid.UUID, limit int) ([]models.IntegrationSyncRun, error) {
	return s.repo.ListSyncRuns(ctx, orgID, connID, limit)
}

func (s *service) MarkSynced(ctx context.Context, id uuid.UUID, status models.IntegrationStatus, displayFields map[string]any, errMsg string) error {
	df, _ := json.Marshal(displayFields)
	return s.repo.MarkConnectionSynced(ctx, id, status, df, errMsg)
}

// --- Google Sheets read helpers ---------------------------------------------

// GoogleConnection returns the org's google_sheets OAuth connection (the most
// recent one), or nil when none exists. Lead-sync uses this connection's token
// to read sheets even though google_sheets is no longer surfaced as a catalog
// integration.
func (s *service) GoogleConnection(ctx context.Context, orgID uuid.UUID) (*models.IntegrationConnection, error) {
	conns, err := s.repo.ListConnections(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for i := range conns {
		if configString(conns[i].ConfigCapabilities, "rows_action") != "" {
			c := conns[i]
			return &c, nil
		}
	}
	return nil, nil
}

// googleSheetsClient resolves the (refreshed) Google access token for a
// connection and returns a ready Sheets client. The connection must belong to
// orgID and be the google_sheets provider.
func (s *service) googleSheetsClient(ctx context.Context, orgID, connID uuid.UUID) (*SheetsClient, error) {
	conn, err := s.repo.GetConnectionByID(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, errors.New("Connection not found.")
	}
	var cfg map[string]any
	_ = json.Unmarshal(conn.ConfigCapabilities, &cfg)
	str := func(k string) string { v, _ := cfg[k].(string); return v }
	return &SheetsClient{client: s.client, actor: "org_" + orgID.String(), metadata: str("metadata_action"), rows: str("rows_action"), append: str("append_action")}, nil
}

func (s *service) SpreadsheetMeta(ctx context.Context, orgID, connID uuid.UUID, sheetID string) (*SheetMeta, error) {
	client, err := s.googleSheetsClient(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	return client.GetSpreadsheet(ctx, sheetID)
}

func (s *service) SpreadsheetValues(ctx context.Context, orgID, connID uuid.UUID, sheetID, a1Range string) ([][]string, error) {
	client, err := s.googleSheetsClient(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	return client.ReadValues(ctx, sheetID, a1Range)
}

// --- encryption helpers -----------------------------------------------------

func (s *service) seal(ctx context.Context, orgID uuid.UUID, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if s.cipher == nil {
		return "", lowerLayer(errors.New("cipher service unavailable"))
	}
	c, err := s.cipher.Cipher(ctx, orgID)
	if err != nil {
		return "", lowerLayer(err)
	}
	out, err := c.Encrypt(ctx, plaintext)
	return out, lowerLayer(err)
}

func (s *service) open(ctx context.Context, orgID uuid.UUID, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if s.cipher == nil {
		return "", lowerLayer(errors.New("cipher service unavailable"))
	}
	c, err := s.cipher.Cipher(ctx, orgID)
	if err != nil {
		return "", lowerLayer(err)
	}
	out, err := c.Decrypt(ctx, ciphertext)
	return out, lowerLayer(err)
}

func (s *service) sealConfig(ctx context.Context, orgID uuid.UUID, config map[string]any) ([]byte, error) {
	if len(config) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	b64, err := s.seal(ctx, orgID, string(raw))
	if err != nil {
		return nil, err
	}
	return []byte(b64), nil
}

func (s *service) openConfig(ctx context.Context, sec *repository.ConnectionSecrets) (map[string]any, error) {
	if len(sec.ConfigEncrypted) == 0 {
		return map[string]any{}, nil
	}
	plain, err := s.open(ctx, sec.Conn.OrganizationID, string(sec.ConfigEncrypted))
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if plain == "" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(plain), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// accessTokenFor decrypts the connection's access token, refreshing via the
// stored refresh token when near expiry and persisting the refreshed pair.
// On an unrecoverable refresh failure it flips the connection to
// reauth_required and returns an error.
func (s *service) accessTokenFor(ctx context.Context, sec *repository.ConnectionSecrets) (string, error) {
	tok, _, err := s.accessToken(ctx, sec, false)
	return tok, err
}

// accessToken is accessTokenFor that can force a refresh, and also reports the
// API host when the provider has one per org.
func (s *service) accessToken(ctx context.Context, sec *repository.ConnectionSecrets, force bool) (token, instanceURL string, err error) {
	return "", "", pramaan.ErrUnavailable
}

func orFallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

// refreshRefused reports a refresh the provider rejected outright: a revoked
// or expired grant, or a client it no longer recognises.

// ProviderAccess returns a usable token for an org-owned OAuth connection.
func (s *service) ProviderAccess(ctx context.Context, orgID, connID uuid.UUID, force bool) (*ProviderAccess, error) {
	conn, err := s.repo.GetConnectionByID(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, errors.New("connection not found")
	}
	if conn.Status == models.IntegrationStatusReauthRequired || conn.Status == models.IntegrationStatusDisconnected {
		return nil, ErrPushReauth
	}
	sec, err := s.repo.GetConnectionSecrets(ctx, connID)
	if err != nil {
		return nil, err
	}
	if sec == nil || sec.Conn.OrganizationID != orgID {
		return nil, errors.New("connection not found")
	}
	// Concurrent callers share one refresh: with refresh-token rotation, two
	// parallel refreshes would leave one holding a dead token.
	// Its own keyspace: AccessToken shares the group with a different result type.
	key := "provider:" + connID.String()
	if force {
		key += ":force"
	}
	v, err, _ := s.refresh.Do(key, func() (any, error) {
		tok, instance, err := s.accessToken(ctx, sec, force)
		if err != nil {
			return nil, err
		}
		return [2]string{tok, instance}, nil
	})
	if err != nil {
		if false {
			return nil, fmt.Errorf("%w: %v", ErrPushReauth, err)
		}
		return nil, fmt.Errorf("could not refresh the access token: %w", err)
	}
	pair := v.([2]string)
	return &ProviderAccess{Token: pair[0], InstanceURL: pair[1], Conn: conn}, nil
}

// AccessToken loads, refreshes and returns a connection's OAuth token after
// checking the connection belongs to orgID. Concurrent callers share one refresh.
func (s *service) AccessToken(ctx context.Context, orgID, connID uuid.UUID) (string, *models.IntegrationConnection, error) {
	v, err, _ := s.refresh.Do(connID.String(), func() (any, error) {
		sec, err := s.repo.GetConnectionSecrets(ctx, connID)
		if err != nil {
			return nil, err
		}
		if sec == nil || sec.Conn.OrganizationID != orgID {
			return nil, errors.New("connection not found")
		}
		tok, err := s.accessTokenFor(ctx, sec)
		if err != nil {
			return nil, err
		}
		conn := sec.Conn
		return tokenResult{token: tok, conn: &conn}, nil
	})
	if err != nil {
		return "", nil, err
	}
	r := v.(tokenResult)
	return r.token, r.conn, nil
}

type tokenResult struct {
	token string
	conn  *models.IntegrationConnection
}

func (s *service) SetCRMModeCheck(check func(ctx context.Context, orgID uuid.UUID) models.CRMProvider) {
	s.crmMode = check
}

func (s *service) MergeConnectionDisplay(ctx context.Context, connID uuid.UUID, patch map[string]any) error {
	return s.repo.MergeDisplayFields(ctx, connID, patch)
}

func (s *service) MarkConnectionHealth(ctx context.Context, connID uuid.UUID, status models.IntegrationStatus, health models.IntegrationHealth, detail string) {
	_ = s.repo.SetConnectionStatus(ctx, connID, status, health, detail)
}

// --- shared helpers ---------------------------------------------------------

// validateOutboundConfigURLs enforces the SSRF/HTTPS policy on any config value
// under a url-bearing key (webhook_url / url) we will later POST to.
func validateOutboundConfigURLs(config map[string]any) error {
	for _, k := range []string{"webhook_url", "url"} {
		v, ok := config[k]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" {
			continue
		}
		// Only the action-level "url" is templatable (rendered + re-validated at
		// dispatch by renderOutboundURL), so defer its strict check when it holds
		// a {{ template. The connection's sealed "webhook_url" is NEVER templated,
		// so it must pass full validation here — never skip it (else a {{-laced
		// host could bypass the SSRF guard).
		if k == "url" && strings.Contains(s, "{{") {
			continue
		}
		if err := webhook.ValidateOutboundURL(s); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
	}
	return nil
}

func hasAnyCredential(config map[string]any) bool {
	for _, k := range []string{"api_token", "access_token", "webhook_url", "api_key"} {
		if v, ok := config[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return true
			}
		}
	}
	return false
}

func catalogAuthMethod(provider models.IntegrationProvider) string {
	for _, e := range Catalog() {
		if e.Provider == provider {
			return e.AuthMethod
		}
	}
	return string(models.IntegrationAuthAPIKey)
}

// isAutomationProvider reports whether a provider is a generic outbound-webhook
// automation tool (Zapier / Make / n8n).
func isAutomationProvider(p models.IntegrationProvider) bool {
	return false
}

// generateSigningSecret returns a Billing-style `whsec_`-prefixed 32-byte hex
// HMAC key for outbound webhook signatures.
func generateSigningSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(buf), nil
}

// signingSecretField is the sealed-config key of an automation connection's outbound HMAC secret.
const signingSecretField = "signing_secret"

// WebhookSigningSecret returns the connection's outbound-webhook HMAC secret,
// generating one on first request. It is kept in the connection's sealed config.
func (s *service) WebhookSigningSecret(ctx context.Context, orgID, connID uuid.UUID) (string, error) {
	sec, err := s.repo.GetConnectionSecrets(ctx, connID)
	if err != nil {
		return "", err
	}
	if sec == nil || sec.Conn.OrganizationID != orgID {
		return "", fmt.Errorf("connection not found")
	}
	cfg, err := s.openConfig(ctx, sec)
	if err != nil {
		return "", err
	}
	if existing := s.signingSecretFor(ctx, sec, cfg); existing != "" {
		return existing, nil
	}
	secret, err := generateSigningSecret()
	if err != nil {
		return "", err
	}
	cfg[signingSecretField] = secret
	sealed, err := s.sealConfig(ctx, orgID, cfg)
	if err != nil {
		return "", err
	}
	if err := s.repo.SetSigningSecretConfig(ctx, orgID, connID, sealed); err != nil {
		return "", err
	}
	return secret, nil
}

// signingSecretFor reads the signing secret from the opened config, moving one
// still held in config_capabilities into it (best effort; the value is used either way).
func (s *service) signingSecretFor(ctx context.Context, sec *repository.ConnectionSecrets, cfg map[string]any) string {
	if v := stringFromMap(cfg, signingSecretField); v != "" {
		return v
	}
	legacy := configString(sec.Conn.ConfigCapabilities, models.ConfigCapabilitiesSigningSecret)
	if legacy == "" {
		return ""
	}
	next := make(map[string]any, len(cfg)+1)
	for k, v := range cfg {
		next[k] = v
	}
	next[signingSecretField] = legacy
	if sealed, err := s.sealConfig(ctx, sec.Conn.OrganizationID, next); err == nil {
		if err := s.repo.SetSigningSecretConfig(ctx, sec.Conn.OrganizationID, sec.Conn.ID, sealed); err != nil {
			log.Warn().Err(err).Str("connection_id", sec.Conn.ID.String()).Msg("integration: sealing signing secret failed")
		}
	}
	return legacy
}

// StartSigningSecretMigration seals every signing secret still held in
// config_capabilities, in one paced pass per process.
func (s *service) StartSigningSecretMigration(ctx context.Context) {
	var cursor uuid.UUID
	for {
		ids, err := s.repo.ListPlaintextSigningSecrets(ctx, cursor, 100)
		if err != nil {
			log.Warn().Err(err).Msg("integration: listing signing secrets to seal failed")
			return
		}
		if len(ids) == 0 {
			return
		}
		for _, id := range ids {
			cursor = id
			sec, err := s.repo.GetConnectionSecrets(ctx, id)
			if err != nil || sec == nil {
				continue
			}
			cfg, err := s.openConfig(ctx, sec)
			if err != nil {
				continue
			}
			_ = s.signingSecretFor(ctx, sec, cfg)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// SendTestEvent fires a synthetic event through the connection's notify/webhook
// automations (Chat, Discord, generic webhook), reusing the real delivery path
// so signing + payload shape match production. CRM-upsert automations are
// skipped so a test never writes a junk record into the customer's CRM.
func (s *service) SendTestEvent(ctx context.Context, orgID, connID uuid.UUID) (int, error) {
	return 0, errors.New("Use Connections to inspect an action and run it explicitly; a test event could change outside records.")
}

// generateInboundSecret returns a prefixed 24-byte hex string.
func generateInboundSecret(provider models.IntegrationProvider) (string, error) {
	return "", errors.New("Use broker trigger deployments in Connections.")
}

// BuildInboundURL is exported so the routes file and handler tests can generate
// the same URL the dashboard surfaces.
func BuildInboundURL(provider models.IntegrationProvider, secret string) string {
	return ""
}

// ErrInboundAutomationNotFound means no enabled inbound-webhook automation owns
// the given token (unknown, or the automation's trigger is no longer inbound).
var ErrInboundAutomationNotFound = errors.New("inbound automation not found")

// generateAutomationInboundToken returns the high-entropy secret embedded in an
// automation's inbound-webhook URL (192 bits of randomness, prefixed).
func generateAutomationInboundToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "wmauto_" + hex.EncodeToString(buf), nil
}

// BuildInboundAutomationURL is the public POST path that fires an inbound-webhook
// automation. Empty token (non-inbound automation) yields an empty URL.
func BuildInboundAutomationURL(token string) string {
	if token == "" {
		return ""
	}
	return "/api/v1/integrations/inbound/automation/" + token
}

// parseInboundPayload turns an inbound webhook body into the event data map. A
// JSON object is used as-is (minus internal underscore keys a caller must not
// set); anything else is exposed verbatim under "body" so a template/condition
// can still read it.
func parseInboundPayload(body []byte) map[string]any {
	if len(body) > 0 {
		var m map[string]any
		if json.Unmarshal(body, &m) == nil && m != nil {
			for k := range m {
				if strings.HasPrefix(k, "_") {
					delete(m, k)
				}
			}
			return m
		}
	}
	out := map[string]any{}
	if len(body) > 0 {
		out["body"] = string(body)
	}
	return out
}

// TriggerInboundAutomation resolves the automation an inbound-webhook token
// points at and runs its graph (in the background) with the POSTed body as the
// event payload. Unknown/non-inbound token -> ErrInboundAutomationNotFound; a
// disabled automation is accepted as a no-op (it must not leak that it exists).
func (s *service) TriggerInboundAutomation(ctx context.Context, token string, body []byte) error {
	a, err := s.repo.GetAutomationByInboundToken(ctx, strings.TrimSpace(token))
	if err != nil {
		return err
	}
	if a == nil || !isInboundTrigger(a.TriggerEvent) {
		return ErrInboundAutomationNotFound
	}
	if !a.Enabled {
		return nil
	}
	data := parseInboundPayload(body)
	data["trigger"] = "inbound"
	go func(au models.Automation, d map[string]any) {
		bg, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.executeAutomationGraph(bg, au, string(models.WebhookEventInboundWebhook), d)
	}(*a, data)
	return nil
}

// buildDisplayFields extracts the public, non-secret bits of the config that
// the dashboard surfaces next to a connection card.
func buildDisplayFields(provider models.IntegrationProvider, config map[string]any) map[string]any {
	return map[string]any{}
}

// refuseSecondVerifier reports the verifier already connected to the workspace,
// so connecting another one is refused rather than silently taking over.
func (s *service) refuseSecondVerifier(ctx context.Context, orgID uuid.UUID) error {
	active, err := s.activeVerificationConnection(ctx, orgID)
	if err != nil || active == nil {
		return err
	}
	return fmt.Errorf("%s already verifies this workspace's addresses. Disconnect it first", ProviderLabel(active.Provider))
}

// activeVerificationConnection returns the connection that verifies this
// workspace's addresses, or nil when none does. Connect keeps there being at
// most one, so the first match is the answer. A disconnected or
// reauth-required connection does not count.
func (s *service) activeVerificationConnection(ctx context.Context, orgID uuid.UUID) (*models.IntegrationConnection, error) {
	conns, err := s.repo.ListConnections(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for _, c := range conns {
		if !slices.Contains(models.VerificationProviders, c.Provider) {
			continue
		}
		if c.Status != models.IntegrationStatusConnected && c.Status != models.IntegrationStatusDegraded {
			continue
		}
		return &c, nil
	}
	return nil, nil
}

// VerificationProviderFor returns the org's connected verification client, or
// nil when none is connected.
func (s *service) VerificationProviderFor(ctx context.Context, orgID uuid.UUID) (*emailverifyapp.Provider, error) {
	return nil, nil
}

// ReportVerificationProviderError records why the provider stopped being
// usable on the connection card.
func (s *service) ReportVerificationProviderError(ctx context.Context, connectionID uuid.UUID, err error) {
	if err == nil {
		return
	}
	status, health := models.IntegrationStatusDegraded, models.IntegrationHealthDegraded
	detail := err.Error()
	switch {
	case errors.Is(err, emailverify.ErrProviderUnconfirmed):
		status, health = models.IntegrationStatusReauthRequired, models.IntegrationHealthDown
		detail = "The account email address is not confirmed yet; confirm it, then reconnect"
	case errors.Is(err, emailverify.ErrProviderKey):
		status, health = models.IntegrationStatusReauthRequired, models.IntegrationHealthDown
		detail = "The API key was rejected; reconnect with a current key"
	case errors.Is(err, emailverify.ErrProviderCredits):
		detail = "No allowance or credits left; the built-in check is used until the account is topped up"
	}
	_ = s.repo.SetConnectionStatus(ctx, connectionID, status, health, detail)
}

// ClearVerificationProviderError puts the connection card back to healthy once
// the provider answers again, since nothing else ever withdrew the error.
func (s *service) ClearVerificationProviderError(ctx context.Context, connectionID uuid.UUID) {
	_ = s.repo.ClearConnectionHealth(ctx, connectionID)
}

func (s *service) OAuthFinish(ctx context.Context, userID uuid.UUID, code, state string, authorize func(context.Context, uuid.UUID) error) (*models.IntegrationConnection, error) {
	return nil, errors.New("Complete the connection in Pramaan Connections; local token exchange is unavailable.")
}

func (s *service) validateConnectedConfig(ctx context.Context, cfg map[string]any) error {
	action, _ := cfg["pramaan_action"].(string)
	if action == "" {
		return errors.New("Choose and inspect a connected action before enabling this automation.")
	}
	if _, ok := cfg["pramaan_input"].(map[string]any); !ok {
		return errors.New("Enter the selected action fields as an object.")
	}
	if s.client == nil {
		return pramaan.ErrUnavailable
	}
	_, err := (pramaan.Apps{Client: s.client}).Inspect(ctx, action)
	return err
}
