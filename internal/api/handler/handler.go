package handler

import (
	"github.com/pramaan-oss/cold-email/internal/app/admin"
	"github.com/pramaan-oss/cold-email/internal/app/admindevice"
	"github.com/pramaan-oss/cold-email/internal/app/adminoutreach"
	"github.com/pramaan-oss/cold-email/internal/app/advanced"
	"github.com/pramaan-oss/cold-email/internal/app/advisor"
	"github.com/pramaan-oss/cold-email/internal/app/aiagent"
	"github.com/pramaan-oss/cold-email/internal/app/aitools"
	"github.com/pramaan-oss/cold-email/internal/app/analytics"
	"github.com/pramaan-oss/cold-email/internal/app/apikey"
	"github.com/pramaan-oss/cold-email/internal/app/appdirectory"
	"github.com/pramaan-oss/cold-email/internal/app/audit"
	"github.com/pramaan-oss/cold-email/internal/app/auth"
	"github.com/pramaan-oss/cold-email/internal/app/behavior"
	"github.com/pramaan-oss/cold-email/internal/app/billing"
	"github.com/pramaan-oss/cold-email/internal/app/bootstrap"
	"github.com/pramaan-oss/cold-email/internal/app/campaign"
	"github.com/pramaan-oss/cold-email/internal/app/cliauth"
	"github.com/pramaan-oss/cold-email/internal/app/compose"
	"github.com/pramaan-oss/cold-email/internal/app/contact"
	"github.com/pramaan-oss/cold-email/internal/app/contactimport"
	"github.com/pramaan-oss/cold-email/internal/app/credits"
	"github.com/pramaan-oss/cold-email/internal/app/crm"
	"github.com/pramaan-oss/cold-email/internal/app/dangerzone"
	"github.com/pramaan-oss/cold-email/internal/app/discount"
	"github.com/pramaan-oss/cold-email/internal/app/email"
	"github.com/pramaan-oss/cold-email/internal/app/emailsend"
	emailverifyapp "github.com/pramaan-oss/cold-email/internal/app/emailverify"
	"github.com/pramaan-oss/cold-email/internal/app/enterpriseauth"
	"github.com/pramaan-oss/cold-email/internal/app/feature"
	"github.com/pramaan-oss/cold-email/internal/app/fleetnode"
	"github.com/pramaan-oss/cold-email/internal/app/form"
	"github.com/pramaan-oss/cold-email/internal/app/group"
	"github.com/pramaan-oss/cold-email/internal/app/instancecheck"
	"github.com/pramaan-oss/cold-email/internal/app/instanceconfig"
	"github.com/pramaan-oss/cold-email/internal/app/instancesettings"
	"github.com/pramaan-oss/cold-email/internal/app/integration"
	"github.com/pramaan-oss/cold-email/internal/app/leadsync"
	"github.com/pramaan-oss/cold-email/internal/app/mailboximport"
	"github.com/pramaan-oss/cold-email/internal/app/mcp"
	"github.com/pramaan-oss/cold-email/internal/app/monitoring"
	"github.com/pramaan-oss/cold-email/internal/app/nodelogs"
	"github.com/pramaan-oss/cold-email/internal/app/notification"
	"github.com/pramaan-oss/cold-email/internal/app/oauth"
	"github.com/pramaan-oss/cold-email/internal/app/opsnotify"
	"github.com/pramaan-oss/cold-email/internal/app/organization"
	"github.com/pramaan-oss/cold-email/internal/app/orgrisk"
	"github.com/pramaan-oss/cold-email/internal/app/orgtransfer"
	"github.com/pramaan-oss/cold-email/internal/app/passkey"
	"github.com/pramaan-oss/cold-email/internal/app/placement"
	"github.com/pramaan-oss/cold-email/internal/app/ratelimit"
	"github.com/pramaan-oss/cold-email/internal/app/referral"
	"github.com/pramaan-oss/cold-email/internal/app/research"
	"github.com/pramaan-oss/cold-email/internal/app/segment"
	"github.com/pramaan-oss/cold-email/internal/app/sendingdomain"
	"github.com/pramaan-oss/cold-email/internal/app/sequence"
	"github.com/pramaan-oss/cold-email/internal/app/skills"
	"github.com/pramaan-oss/cold-email/internal/app/socket"
	"github.com/pramaan-oss/cold-email/internal/app/subscription"
	"github.com/pramaan-oss/cold-email/internal/app/sysstatus"
	"github.com/pramaan-oss/cold-email/internal/app/team"
	"github.com/pramaan-oss/cold-email/internal/app/template"
	"github.com/pramaan-oss/cold-email/internal/app/token"
	"github.com/pramaan-oss/cold-email/internal/app/trial"
	"github.com/pramaan-oss/cold-email/internal/app/twofa"
	"github.com/pramaan-oss/cold-email/internal/app/tz"
	"github.com/pramaan-oss/cold-email/internal/app/unibox"
	"github.com/pramaan-oss/cold-email/internal/app/unsublink"
	"github.com/pramaan-oss/cold-email/internal/app/updates"
	"github.com/pramaan-oss/cold-email/internal/app/user"
	"github.com/pramaan-oss/cold-email/internal/app/viewprefs"
	"github.com/pramaan-oss/cold-email/internal/app/warmup"
	"github.com/pramaan-oss/cold-email/internal/app/warmupcontent"
	"github.com/pramaan-oss/cold-email/internal/app/webhook"
	"github.com/pramaan-oss/cold-email/internal/app/websitetracking"
	"github.com/pramaan-oss/cold-email/internal/app/worker"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/eventbus"
	"github.com/pramaan-oss/cold-email/internal/pkg/generation"
	"github.com/pramaan-oss/cold-email/internal/pkg/textcheck"
	"github.com/pramaan-oss/cold-email/internal/pramaan"

	"github.com/pramaan-oss/cold-email/internal/infrastructure/encryptedkeys"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/kms"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/pubsub"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/storage"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/notify"
	"github.com/pramaan-oss/cold-email/internal/repository"
	"github.com/pramaan-oss/cold-email/internal/tasks"
)

type Handler struct {
	ManagedState       pramaan.ManagedState
	ManagedClient      pramaan.Caller
	PramaanGateway     *pramaan.Gateway
	ConnectorReturnURL string
	AuthService        auth.AuthService
	EnterpriseService  *enterpriseauth.Service
	TokenService       token.TokenService
	PasskeyService     passkey.Service

	// Native-app social sign-in discovery (GET /auth/providers).
	ExternalAuthProviders models.ExternalAuthProviders

	// Deployment facts served by GET /auth/config so the login screen renders
	// what this backend can actually do instead of guessing. The sign-in
	// provider list is not here: it comes from the auth service, which is what
	// actually holds the configured flows.
	MailDelivers   bool
	PasskeysUsable bool
	MailTransport  string
	// MailTransportRef backs the admin mail diagnostics, which need Preflight
	// and so cannot go through the EmailNotificationService interface.
	MailTransportRef *notify.Transport
	// BootstrapService backs the first-run setup page.
	BootstrapService *bootstrap.Service
	UserService      user.UserService
	EmailService     email.EmailService
	// MailboxImportService runs mailbox imports from files and pasted lists.
	MailboxImportService *mailboximport.Service
	// ContactImportService runs contact file imports in the background.
	ContactImportService *contactimport.Service
	// DelegationService connects whole Google Workspace domains and Microsoft 365 tenants.
	// SendingDomainService manages tracking hosts and bare-domain redirects per sending domain.
	SendingDomainService *sendingdomain.Service
	CampaignService      campaign.CampaignService
	ContactService       contact.ContactService
	SegmentService       segment.Service
	FormService          form.Service
	SequenceService      sequence.SequenceService
	UniboxService        unibox.UniboxService

	FolderService   group.GroupService
	TagService      group.GroupService
	CategoryService group.GroupService

	TzService           tz.TzService
	SocketService       socket.SocketService
	TasksService        tasks.TasksService
	NotificationService notification.Service
	TwoFAService        twofa.Service
	// ViewPreferencesService keeps each member's saved list layouts.
	ViewPreferencesService viewprefs.Service

	// New services
	APIKeyService    apikey.APIKeyService
	AnalyticsService analytics.AnalyticsService
	AuditService     audit.AuditService
	RateLimitService ratelimit.RateLimitService

	// Subscription & billing
	SubscriptionService subscription.SubscriptionService
	BillingService      billing.BillingService
	DiscountService     discount.DiscountService
	ReferralService     referral.Service

	// Trial & feature gates
	TrialService            trial.TrialService
	FeatureGateService      feature.FeatureGateService
	WorkerAssignmentService worker.WorkerAssignmentService

	// Organization & IAM
	OrganizationService organization.OrganizationService
	OrgRiskService      orgrisk.Service

	// CRM
	CRMService crm.CRMService

	// Teams
	TeamService team.TeamService

	// Email send & templates
	TemplateService  template.TemplateService
	EmailSendService emailsend.EmailSendService

	// Compose mailbox picker: scores the org's mailboxes for a recipient
	// (affinity, budget, auth health) and backs auto selection.
	ComposeService compose.Service

	// Admin
	AdminService         admin.AdminService
	AdminOutreachService adminoutreach.Service

	// Fleet. Nodes enrol with the join token and pull everything else; the
	// control plane never reaches into a machine.
	FleetNodes        *fleetnode.Service
	NodeLogs          nodelogs.Store
	BrokerDiagnostics eventbus.EventBus
	WorkerRepo        repository.WorkerRepository
	// UpdatesService backs the admin panel's update indicator and button.
	UpdatesService *updates.Service

	// Notifications
	EmailNotificationService notify.EmailNotificationService

	// Advanced outreach controls
	AdvancedService advanced.Service
	// UnsubscribeLinks verifies the signed tokens on recipient unsubscribe
	// links. Nil when the instance has no public API URL to mint them on.
	UnsubscribeLinks *unsublink.Signer
	// UnsubscribeTickets resolves the short form of those links. Nil means
	// only the self-contained signed tokens are honoured.
	UnsubscribeTickets repository.UnsubscribeLinkRepository

	// Website tracking snippet: settings and the page-view ingest path.
	WebsiteTrackingService websitetracking.Service

	// Pre-send email verification (control-plane SMTP RCPT probe / pluggable
	// paid backend). Drops hard-bouncing addresses before a worker sends.
	EmailVerifyService emailverifyapp.Service

	// Per-mailbox human sending behaviour: the ranges a mailbox rolls its
	// workday from, and the plan it rolled for today. Nil disables the
	// behaviour endpoints (the schedulers then run every mailbox on its fixed
	// cap and gap).
	BehaviorService behavior.Service

	// Warmup health
	WarmupService warmup.Service

	// Warmup routing rules — customer-defined preferences for premium-pool
	// partner selection (e.g. Gmail recipients from Google Workspace senders).
	WarmupRoutingRepo repository.WarmupRoutingRepository

	// Warmup content bank + offline AI generator (admin control/visibility).
	WarmupContentRepo    repository.WarmupContentRepository
	WarmupContentService warmupcontent.Service

	// AI writing assistant + credit ledger.
	CreditService    credits.CreditService
	WritingGenerator generation.WritingGenerator

	// AI provider layer (RunAgent tool-loop backend) + pluggable web search,
	// shared by the dashboard agent, research, automation AI nodes, and the
	// inbox agent. Nil when no LLM provider is configured.
	AIProvider generation.Provider
	// TextCheck is the shared typed-judgment client. Nil when no key is
	// configured; every endpoint that reads it degrades to its rules-only
	// answer.
	TextCheck textcheck.Asker
	AISearch  generation.SearchClient

	// AITools is the shared tool registry the dashboard agent and MCP server
	// run on. Handlers bound to the invoking user's permissions.
	AITools *aitools.Registry

	// AIAgentService orchestrates the dashboard agent (sessions, SSE runs,
	// approvals, per-iteration credits). Nil when no LLM provider is configured.
	AIAgentService aiagent.Service

	// ResearchService runs the AI contact-research agent (sync + batch).
	ResearchService research.Service

	// SkillsService manages org AI skills (playbooks) and injects them into AI
	// prompts.
	SkillsService skills.Service

	// MCPService manages org-connected MCP servers (external tools).
	MCPService mcp.Service

	// AIDraftRepo stores inbox-agent reply drafts awaiting human review (M10).
	// The draft is created in the consumer; these list/approve/discard handlers
	// read + resolve it. Nil disables the review endpoints.
	AIDraftRepo repository.AIDraftRepository

	// Seed inbox-placement testing.
	PlacementRepo    repository.PlacementRepository
	PlacementService placement.Service

	// Advisor: continuously-evaluated recommendations about deliverability,
	// mailbox config, warmup, campaign performance, copy, and list hygiene.
	// The repository is held alongside the service so a read can cheaply check
	// how stale the findings are before deciding to re-evaluate inline.
	AdvisorService    advisor.Service
	AdvisorRepository repository.AdvisorRepository

	// Customer-facing webhooks (subscribe → HMAC-signed delivery).
	WebhookService webhook.Service

	// Third-party integrations (connected, connected, DMARC, Postmaster,
	// SNDS, Cloudflare, GoDaddy, Namecheap, Google Sheets).
	IntegrationService integration.Service
	ContactRepo        repository.ContactRepository

	// ChatService is the Chat app (request URLs and the dashboard's Chat
	// panel). Nil answers chat_not_configured.

	// connected runs a workspace's CRM on connected when it chooses so. Nil on
	// processes built without it.
	// connected runs a workspace's CRM on connected when it chooses so.
	// CRMModes routes the CRM mode endpoints to the CRM a workspace runs on.

	// OAuth 2.1 authorization server (third-party app registration + the
	// authorization-code-with-PKCE flow + bearer-token validation).
	OAuthService *oauth.Service

	// AppDirectoryService is the community app directory (published OAuth apps).
	AppDirectoryService *appdirectory.Service

	// Realtime publisher for handler paths that emit live dashboard events
	// directly (inbound meeting webhooks have no service layer of their own).
	// nil-safe: realtime is a nicety, not a requirement.
	StreamingPublisher *pubsub.StreamingPublisher

	// On-demand Google Sheets -> leads sync. Reuses the google_sheets OAuth
	// connection's token to read sheets and the contact import path to upsert.
	LeadSyncService leadsync.Service

	// connectedService is the native connected sync: settings, imports, the
	// activity log and the contact panel.

	// Public websocket URL used by frontend clients
	WebsocketURI string

	// Object storage for user-uploaded artifacts (avatars, etc.).
	Storage storage.Store

	// Encrypted-DEK store. Served to workers over HTTPS at
	// /api/v1/internal/dek/:userID so workers don't need direct Postgres
	// access. Backend processes use Postgres directly; workers use the
	// HTTP-proxy implementation.
	EncryptedKeys encryptedkeys.Store

	// The instance's root of trust, used by /api/v1/internal/dek/decrypt to
	// open a sealed key for a node that holds no KMS credential of its own.
	// Nothing else in the handler layer touches it: application crypto goes
	// through the cipher service.
	KMS kms.Provider

	// Worker messageId -> internal email map, served to workers over HTTPS at
	// /api/v1/internal/email-message-map for the same no-direct-Postgres reason
	// as EncryptedKeys. Backed by Postgres in the backend.
	EmailMessageMap repository.EmailMessageMapRepository

	// Mailbox sync state, read by the dashboard (GET /emails/:id/sync) and by
	// the worker's priority lane over /api/v1/internal/sync/own-conversation.
	EmailSyncState repository.EmailSyncStateRepository
	WarmupDispatch repository.WarmupDispatchRepository

	// Click-link store, served to the tracking service over HTTPS at
	// /api/v1/internal/tracked-links/:id (same no-direct-Postgres rule).
	TrackedLinks repository.TrackedLinkRepository

	// InboxTagRepo backs the automatic-tagging review page. Optional: nil when
	// the feature was never configured, and the endpoint says so rather than
	// failing.
	InboxTagRepo repository.InboxTagRepository

	// Verified custom tracking and forms domains, read by the on-demand TLS
	// gate at /tls/authorize so a reverse proxy can obtain a certificate for a
	// hostname that was not known when the instance was installed.
	CustomDomains repository.CustomDomainRepository

	// Direct repositories used by handlers that don't yet have a
	// service layer (avatars, etc.). Keep narrow and add a service
	// only when business logic accumulates.
	UserRepo           repository.UserRepository
	OrgRepo            repository.OrganizationRepository
	AttachmentRepo     repository.AttachmentRepository
	EmailImageRepo     repository.EmailImageRepository
	StorageBackendRepo repository.StorageBackendRepository
	FleetNodeRepo      repository.FleetNodeRepository
	FleetSettingsRepo  repository.FleetSettingsRepository

	// Danger zone (delayed deletions for orgs & user accounts)
	DangerZoneService dangerzone.Service

	// Workspace archives: export an organization to a portable file and import
	// one back, for moving between instances. Nil disables the endpoints.
	OrgTransferService orgtransfer.Service

	// Device-code sign-in for the `coldemail` CLI. Nil-safe: routes answer 501.
	CLIAuthService cliauth.Service
	AdminDevice    *admindevice.Service

	// Infrastructure liveness probes for the admin System Status page.
	// Wired in cmd/backend/main.go where the concrete clients live.
	SystemChecker *sysstatus.Checker
	Monitoring    *monitoring.Service

	// Admin operations pages: cross-workspace reads of mailbox sync, the send
	// outcome loop, fleet placement and abuse signals, plus the scheduled job
	// registry every background loop records to. Nil-safe: the endpoints
	// answer 501 when the repository is not wired.
	AdminSyncRepo    repository.AdminSyncRepository
	AdminSendsRepo   repository.AdminSendsRepository
	AdminFleetRepo   repository.AdminFleetRepository
	AdminInsightRepo repository.AdminInsightRepository
	JobRuns          repository.JobRunRepository
	// WebhookRepo backs the operator's "reclaim stuck deliveries" action.
	WebhookRepo repository.WebhookRepository

	// Operator visibility (admin panel, Instance section).
	//
	// InstanceRuntime carries the facts only boot knows (the resolved CORS
	// list, the WebAuthn RP, the OIDC redirect, the auth policy) so the
	// configuration page shows what the process actually derived. Nil is safe:
	// every resolver falls back to the environment.
	InstanceRuntime *instanceconfig.Runtime
	// InstanceChecks is the setup and health registry. Nil falls back to the
	// environment-only checks, so the page is never blank.
	InstanceChecks *instancecheck.Registry
	// InstanceSettings is the database-backed settings tier. It holds only
	// keys no environment variable owns.
	InstanceSettings instancesettings.Service
	// OpsNotifier delivers instance-wide operator alerts. Nil disables the
	// notification admin surface.
	OpsNotifier opsnotify.Notifier
}
