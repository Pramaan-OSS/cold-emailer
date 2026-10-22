package instanceconfig

import (
	"os"
	"strconv"
	"strings"

	"github.com/pramaan-oss/cold-email/internal/config"
)

// Docs anchors. Kept as constants so a renamed section is one edit.
const (
	docsDeployment = "/development/configuration/#deployment"
	docsSecrets    = "/development/configuration/#secrets"
	docsAddresses  = "/development/configuration/#addresses"
	docsProxy      = "/development/configuration/#network-and-proxy"
	docsSignIn     = "/development/configuration/#authentication"
	docsMail       = "/development/configuration/#platform-mail"
	docsEncryption = "/development/configuration/#encryption"
	docsStorage    = "/development/configuration/#storage"
	docsEventBus   = "/development/configuration/#event-bus"
	docsDatabase   = "/development/configuration/#database"
	docsGeoIP      = "/development/configuration/#geoip"
	docsWorkers    = "/development/configuration/#workers"
	docsCaptcha    = "/development/configuration/#captcha"
	docsSSO        = "/development/accounts-and-access/#single-sign-on"
	docsFirstOwner = "/development/accounts-and-access/#first-owner"
	docsUpdates    = "/development/updates/"
	docsAI         = "/development/configuration/#ai-and-search"
	// The database-backed settings document, which is the one tier the
	// environment does not own.
	docsSettingsDoc = "/development/configuration/#settings-stored-in-the-database"
)

// table is the static inventory. Declaration order is display order.
var table = []Entry{
	// Deployment.
	{
		Key: "APP_ENV", Group: GroupDeployment, RuntimeChangeable: ChangeBootOnly,
		Effect:     "dev tolerates the published default secrets and turns on Gin debug logging. Set prod for anything other people can reach.",
		DocsAnchor: docsDeployment,
		Resolve:    envOr("APP_ENV", "dev"),
	},
	{
		Key: "DEPLOYMENT_MODE", Group: GroupDeployment, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Picks the auth defaults (login code, signup lockdown, email verification). Each stays individually overridable.",
		DocsAnchor: docsDeployment, WhenUnset: SourceDerived,
		Resolve: func(*Runtime) string {
			if v := trimmed("DEPLOYMENT_MODE"); v != "" {
				return v
			}
			if config.SelfHosted() {
				return "self_hosted"
			}
			return "cloud"
		},
	},
	{
		Key: "ALLOW_INSECURE_DEFAULTS", Group: GroupDeployment, RuntimeChangeable: ChangeBootOnly,
		Effect:     "true lets the backend start while a published default secret is still in use, instead of refusing.",
		DocsAnchor: docsSecrets,
		Resolve:    boolOr("ALLOW_INSECURE_DEFAULTS", false),
	},
	{
		Key: "GIN_MODE", Group: GroupDeployment, RuntimeChangeable: ChangeBootOnly,
		Effect:     "debug logs every route and request body; release is the production posture.",
		DocsAnchor: docsDeployment, WhenUnset: SourceDerived,
		Resolve: func(*Runtime) string {
			if v := trimmed("GIN_MODE"); v != "" {
				return v
			}
			if trimmed("APP_ENV") == "prod" {
				return "release"
			}
			return "debug"
		},
	},
	{
		Key: "ENV_LABEL", Group: GroupDeployment, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The badge the admin panel shows so a production panel is not mistaken for a local one.",
		DocsAnchor: docsDeployment,
		Resolve:    envValue("ENV_LABEL"),
	},
	{
		Key: "BILLING_PROVIDER", Group: GroupDeployment, RuntimeChangeable: ChangeBootOnly,
		Effect:     "none runs with no Billing, every feature unlocked and no trial expiry.",
		DocsAnchor: docsDeployment,
		Resolve:    func(*Runtime) string { return config.BillingProvider() },
	},
	{
		Key: "TASKS_PROVIDER", Group: GroupDeployment, RuntimeChangeable: ChangeBootOnly,
		Effect:     "local runs an in-process Postgres poller for campaign ticks and scheduled sends, with no external service.",
		DocsAnchor: docsDeployment,
		Resolve:    func(*Runtime) string { return config.TasksProvider() },
	},
	{
		Key: "TASKS_LOCAL_POLL_INTERVAL", Group: GroupDeployment, RuntimeChangeable: ChangeBootOnly,
		Effect:     "How often the local task poller looks for due work.",
		DocsAnchor: docsDeployment,
		Resolve:    envOr("TASKS_LOCAL_POLL_INTERVAL", "1s"),
	},
	{
		Key: "COLD_EMAIL_ALLOW_UNSAFE_WEBHOOK_URLS", Group: GroupDeployment, RuntimeChangeable: ChangePerRequest,
		Effect:     "true lets customer webhooks point at http:// and private addresses. Development only.",
		DocsAnchor: docsDeployment,
		Resolve:    boolOr("COLD_EMAIL_ALLOW_UNSAFE_WEBHOOK_URLS", false),
	},
	{
		Key: "INBOX_TAGGING_ENABLED", Group: GroupDeployment, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Enables TextCheck classification of inbound message content when its API key is configured.",
		DocsAnchor: docsAI,
		Resolve:    boolOr("INBOX_TAGGING_ENABLED", false),
	},

	// Addresses.
	{
		Key: "APP_URL", Aliases: []string{"FRONTEND_BASE_URL"}, Group: GroupAddresses, RuntimeChangeable: ChangePerRequest,
		Effect:     "The dashboard origin every emailed link is built from: password resets, invitations and the setup link.",
		DocsAnchor: docsAddresses,
		Resolve:    func(*Runtime) string { return config.AppBaseURL() },
	},
	{
		Key: "API_PUBLIC_URL", Group: GroupAddresses, RuntimeChangeable: ChangeBootOnly,
		Effect:     "This backend's public base. The OIDC redirect URL and the frontends' API base derive from it.",
		DocsAnchor: docsAddresses,
		Resolve:    envValue("API_PUBLIC_URL"),
	},
	{
		Key: "API_HOST", Group: GroupAddresses, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The address the API binds. 0.0.0.0:8080 already binds every interface.",
		DocsAnchor: docsAddresses,
		Resolve:    envOr("API_HOST", "0.0.0.0:8080"),
	},
	{
		Key: "APP_ORIGIN", Group: GroupAddresses, RuntimeChangeable: ChangePerRequest,
		Effect:     "The target origin the mailbox OAuth callback posts the authorization code back to. Empty means a wildcard target.",
		DocsAnchor: docsAddresses,
		Resolve:    envValue("APP_ORIGIN"),
	},
	{
		Key: "WEBSOCKET_URL", Group: GroupAddresses, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The realtime endpoint the dashboard connects to for live updates and presence.",
		DocsAnchor: docsAddresses,
		Resolve: func(rt *Runtime) string {
			if rt != nil && rt.WebsocketURL != "" {
				return rt.WebsocketURL
			}
			return os.Getenv("WEBSOCKET_URL")
		},
	},
	{
		Key: "CORS_ALLOW_ORIGINS", Group: GroupAddresses, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The browser origins allowed to call this API. When unset it derives from APP_URL, the websocket origin and, outside prod, the local dev ports.",
		DocsAnchor: docsAddresses, WhenUnset: SourceDerived,
		Resolve: func(rt *Runtime) string {
			if rt != nil && len(rt.CORSOrigins) > 0 {
				return strings.Join(rt.CORSOrigins, ", ")
			}
			return os.Getenv("CORS_ALLOW_ORIGINS")
		},
	},
	{
		Key: "TRUSTED_PROXIES", Group: GroupAddresses, RuntimeChangeable: ChangeBootOnly,
		Effect:     "CIDRs allowed to set X-Forwarded-For. Empty trusts nothing, which is correct for a directly exposed backend.",
		DocsAnchor: docsProxy,
		Resolve:    envValue("TRUSTED_PROXIES"),
	},

	// Database, cache and GeoIP.
	{
		Key: "PRIMARY_DB", Group: GroupDatabase, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The Postgres connection string. Migrations are applied against it at boot.",
		DocsAnchor: docsDatabase,
		Resolve:    envValue("PRIMARY_DB"),
	},
	{
		Key: "GEODB_PATH", Group: GroupDatabase, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Path to a GeoLite2 database. It must be set on the backend in every environment; a missing file at that path is tolerated and only costs city labels.",
		DocsAnchor: docsGeoIP,
		Resolve:    envValue("GEODB_PATH"),
	},
	{
		Key: "GEODB_URL", Group: GroupDatabase, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Where to download the database at GEODB_PATH from when that path holds no file. A file already there is never replaced.",
		DocsAnchor: docsGeoIP,
		Resolve:    envValue("GEODB_URL"),
	},
	{
		Key: "REDIS", Group: GroupCache, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The Redis connection string. Rate limits, the organization key cache, the setup token and the realtime bridge all live there.",
		DocsAnchor: docsDatabase,
		Resolve:    envValue("REDIS"),
	},

	// Platform mail.
	{
		Key: "MAIL_TRANSPORT", Group: GroupMail, RuntimeChangeable: ChangeBootOnly,
		Effect:     "pramaan delivers through the capability client; log writes local messages and delivers nothing.",
		DocsAnchor: docsMail, WhenUnset: SourceDerived,
		Resolve: func(rt *Runtime) string {
			if rt != nil && rt.MailTransportKind != "" {
				return rt.MailTransportKind
			}
			return config.MailTransport()
		},
	},
	{
		Key: "EMAIL_NAME", Group: GroupMail, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The display name on platform mail. The backend refuses to start without it; the consumer only warns and silently sends nothing.",
		DocsAnchor: docsMail,
		Resolve:    envValue("EMAIL_NAME"),
	},
	{
		Key: "EMAIL_ADDRESS", Group: GroupMail, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The From address on platform mail.",
		DocsAnchor: docsMail,
		Resolve:    envValue("EMAIL_ADDRESS"),
	},
	{
		Key: "NOTIFICATION_EMAIL_DAILY_CAP", Group: GroupMail, RuntimeChangeable: ChangePerRequest,
		Effect:     "How many notification digest emails one user can receive in a day.",
		DocsAnchor: docsMail,
		Resolve:    envValue("NOTIFICATION_EMAIL_DAILY_CAP"),
	},
	{
		Key: "EMAIL_BRAND_NAME", Group: GroupMail, RuntimeChangeable: ChangePerRequest,
		Effect:     "The brand platform mail is attributed to. A self-hosted install should not send mail attributed to another company.",
		DocsAnchor: docsMail,
		Resolve:    envValue("EMAIL_BRAND_NAME"),
	},

	// Authentication and access.
	{
		Key: "AUTH_SECRET", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Signs every session and JWT. It must be byte-identical to the realtime service's JWT_SECRET; compare the fingerprints.",
		DocsAnchor: docsSecrets,
		Resolve:    envValue("AUTH_SECRET"),
	},
	{
		Key: "TWOFA_SECRET", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Seals stored TOTP secrets. Falls back to AUTH_SECRET; rotating it invalidates every enrolled authenticator.",
		DocsAnchor: docsSecrets, WhenUnset: SourceDerived,
		Resolve: func(*Runtime) string {
			if v := trimmed("TWOFA_SECRET"); v != "" {
				return v
			}
			return os.Getenv("AUTH_SECRET")
		},
	},
	{
		Key: "AUTH_LOGIN_CODE", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "always emails a code on every login, new_device only on an unrecognised device, off never. A transport that does not deliver demotes always to new_device.",
		DocsAnchor: docsSignIn, WhenUnset: SourceDerived,
		Resolve: func(rt *Runtime) string { return policy(rt).LoginCode },
	},
	{
		Key: "DISABLE_REGISTRATION", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "false is open signup, invite_only accepts only invited addresses, true closes signup and invitations both.",
		DocsAnchor: docsSignIn, WhenUnset: SourceDerived,
		Resolve: func(rt *Runtime) string { return policy(rt).Registration },
	},
	{
		Key: "REQUIRE_EMAIL_VERIFICATION", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "true makes registration complete an emailed code before the account exists.",
		DocsAnchor: docsSignIn, WhenUnset: SourceDerived,
		Resolve: func(rt *Runtime) string { return yesNo(policy(rt).RequireEmailVerification) },
	},
	{
		Key: "DISABLE_PASSWORD_LOGIN", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "true turns off email and password entirely, for deployments that authenticate through OIDC only.",
		DocsAnchor: docsSignIn, WhenUnset: SourceDerived,
		Resolve: func(rt *Runtime) string { return yesNo(policy(rt).DisablePasswordLogin) },
	},
	{
		Key: "SSO_AUTO_PROVISION", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "true lets a verified identity-provider assertion create an account regardless of DISABLE_REGISTRATION.",
		DocsAnchor: docsSSO, WhenUnset: SourceDerived,
		Resolve: func(rt *Runtime) string { return yesNo(policy(rt).SSOAutoProvision) },
	},
	{
		Key: "AUTH_IP_RATE_LIMIT", Group: GroupAuth, RuntimeChangeable: ChangePerRequest,
		Effect:     "Unauthenticated auth requests allowed per IP per 15 minutes.",
		DocsAnchor: docsSignIn,
		Resolve:    envOr("AUTH_IP_RATE_LIMIT", "60"),
	},
	{
		Key: "WEBAUTHN_RP_ID", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The domain passkeys are cryptographically bound to. Changing it invalidates every enrolled passkey.",
		DocsAnchor: docsSignIn, WhenUnset: SourceDerived,
		Resolve: func(rt *Runtime) string {
			if rt != nil && rt.WebAuthnRPID != "" {
				return rt.WebAuthnRPID
			}
			return os.Getenv("WEBAUTHN_RP_ID")
		},
	},
	{
		Key: "WEBAUTHN_RP_ORIGINS", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The full origins allowed to run passkey ceremonies. Derived from CORS_ALLOW_ORIGINS or APP_URL when unset.",
		DocsAnchor: docsSignIn, WhenUnset: SourceDerived,
		Resolve: func(rt *Runtime) string {
			if rt != nil && len(rt.WebAuthnRPOrigins) > 0 {
				return strings.Join(rt.WebAuthnRPOrigins, ", ")
			}
			return os.Getenv("WEBAUTHN_RP_ORIGINS")
		},
	},
	{
		Key: "SOCIAL_BROWSER_CLIENT_ID", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The app-owned browser social sign-in client.",
		DocsAnchor: docsSignIn,
		Resolve:    envValue("SOCIAL_BROWSER_CLIENT_ID"),
	},
	{
		Key: "SOCIAL_BROWSER_CLIENT_SECRET", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The browser social sign-in client secret.",
		DocsAnchor: docsSignIn,
		Resolve:    envValue("SOCIAL_BROWSER_CLIENT_SECRET"),
	},
	{
		Key: "SOCIAL_BROWSER_REDIRECT_URI", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Where browser social sign-in returns. Served by the API, not the dashboard; derived from API_PUBLIC_URL when unset. This is the URI to register at the provider.",
		DocsAnchor: docsSignIn, WhenUnset: SourceDerived,
		Resolve: ssoRedirect("SOCIAL_BROWSER_REDIRECT_URI", "google"),
	},
	{
		Key: "SOCIAL_DEVICE_APP_ID", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The app-owned device social sign-in service identifier.",
		DocsAnchor: docsSignIn,
		Resolve:    envValue("SOCIAL_DEVICE_APP_ID"),
	},
	{
		Key: "SOCIAL_DEVICE_REDIRECT_URI", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Where device social sign-in returns. Must be https; derived from API_PUBLIC_URL when unset.",
		DocsAnchor: docsSignIn, WhenUnset: SourceDerived,
		Resolve: ssoRedirect("SOCIAL_DEVICE_REDIRECT_URI", "apple"),
	},
	{
		Key: "SOCIAL_DEVICE_TEAM_ID", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The social client team the key belongs to.",
		DocsAnchor: docsSignIn,
		Resolve:    envValue("SOCIAL_DEVICE_TEAM_ID"),
	},
	{
		Key: "SOCIAL_DEVICE_KEY_ID", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The social client private key identifier.",
		DocsAnchor: docsSignIn,
		Resolve:    envValue("SOCIAL_DEVICE_KEY_ID"),
	},
	{
		Key: "SOCIAL_DEVICE_KEY_SECRET", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The social client private key used to sign client assertions.",
		DocsAnchor: docsSignIn,
		Resolve:    envValue("SOCIAL_DEVICE_KEY_SECRET"),
	},
	{
		Key: "COLD_EMAIL_BOOTSTRAP_EMAIL", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Creates the first owner unattended. Read only while the users table is empty.",
		DocsAnchor: docsFirstOwner,
		Resolve:    envValue("COLD_EMAIL_BOOTSTRAP_EMAIL"),
	},
	{
		Key: "COLD_EMAIL_BOOTSTRAP_PASSWORD_HASH", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The argon2 PHC string for the first owner. Preferred over the plaintext variant.",
		DocsAnchor: docsFirstOwner,
		Resolve:    envValue("COLD_EMAIL_BOOTSTRAP_PASSWORD_HASH"),
	},
	{
		Key: "COLD_EMAIL_BOOTSTRAP_PASSWORD", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "A plaintext first-owner password. Remove it once the account exists: it is read only while the users table is empty.",
		DocsAnchor: docsFirstOwner,
		Resolve:    envValue("COLD_EMAIL_BOOTSTRAP_PASSWORD"),
	},
	{
		Key: "COLD_EMAIL_BOOTSTRAP_ORG", Group: GroupAuth, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The name of the organization created alongside the first owner.",
		DocsAnchor: docsFirstOwner,
		Resolve:    envValue("COLD_EMAIL_BOOTSTRAP_ORG"),
	},
	{
		Key: "COLD_EMAIL_SETTINGS_BOOTSTRAP", Group: GroupDeployment, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Seeds the database-backed settings document (sync budgets, retention windows) on an instance that has never saved one. A no-op from the first save in Instance settings onwards, so leaving it here cannot undo an edit made there.",
		DocsAnchor: docsSettingsDoc,
		Resolve:    envValue("COLD_EMAIL_SETTINGS_BOOTSTRAP"),
	},

	// Captcha.
	{
		Key: "CAPTCHA_PROVIDER", Group: GroupCaptcha, RuntimeChangeable: ChangeBootOnly,
		Effect:     "none disables captcha (default); pramaan verifies supplied tokens server-side. No hosted widget is available.",
		DocsAnchor: docsCaptcha, WhenUnset: SourceDerived,
		Resolve: func(*Runtime) string { return config.CaptchaProvider() },
	},

	// Encryption.
	{
		Key: "KMS_PROVIDER", Group: GroupEncryption, RuntimeChangeable: ChangeBootOnly,
		Effect:     "local wraps organization keys; brokered asks the owned backend to unwrap.",
		DocsAnchor: docsEncryption,
		Resolve:    func(*Runtime) string { return config.KMSProvider() },
	},
	{
		Key: "KMS_LOCAL_MASTER_KEY", Group: GroupEncryption, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The base64 32-byte root key every organization key is wrapped with. Back it up: losing it is unrecoverable.",
		DocsAnchor: docsEncryption,
		Resolve:    envValue("KMS_LOCAL_MASTER_KEY"),
	},
	{
		Key: "KMS_LOCAL_MASTER_KEY_FILE", Group: GroupEncryption, RuntimeChangeable: ChangeBootOnly,
		Effect:     "A file holding the root key, as an alternative to the inline value. Setting both is an error.",
		DocsAnchor: docsEncryption,
		Resolve:    envValue("KMS_LOCAL_MASTER_KEY_FILE"),
	},
	{
		Key: "CREDENTIALS_ENCRYPTION_KEY", Group: GroupEncryption, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Seals mailbox SMTP and IMAP passwords at rest. Empty stores them unsealed. Back it up: losing it makes connected mailboxes unrecoverable.",
		DocsAnchor: docsEncryption,
		Resolve:    envValue("CREDENTIALS_ENCRYPTION_KEY"),
	},
	{
		Key: "ENCRYPTED_KEYS_PROVIDER", Group: GroupEncryption, RuntimeChangeable: ChangeBootOnly,
		Effect:     "postgres for the backend and consumer, http for workers, which proxy through the backend internal API instead of opening SQL.",
		DocsAnchor: docsEncryption,
		Resolve:    envOr("ENCRYPTED_KEYS_PROVIDER", "postgres"),
	},

	// Storage.
	{
		Key: "BLOB_PROVIDER", Group: GroupStorage, RuntimeChangeable: ChangeBootOnly,
		Effect:     "filesystem keeps files locally; brokered storage uses the capability client.",
		DocsAnchor: docsStorage,
		Resolve:    func(*Runtime) string { return config.BlobProvider() },
	},
	{
		Key: "BLOB_FS_ROOT", Group: GroupStorage, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The directory the filesystem provider writes to. The backend, the consumer and every worker on this host must share it.",
		DocsAnchor: docsStorage,
		Resolve:    envValue("BLOB_FS_ROOT"),
	},
	{
		Key: "BLOB_BUCKET", Group: GroupStorage, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The logical brokered storage bucket.",
		DocsAnchor: docsStorage,
		Resolve:    envValue("BLOB_BUCKET"),
	},
	{
		Key: "BLOB_PUBLIC_BASE_URL", Group: GroupStorage, RuntimeChangeable: ChangePerRequest,
		Effect:     "The public URL base avatars and logos are served from.",
		DocsAnchor: docsStorage,
		Resolve:    envValue("BLOB_PUBLIC_BASE_URL"),
	},

	// Event bus.
	{
		Key: "EVENTBUS_PROVIDER", Group: GroupEventBus, RuntimeChangeable: ChangeBootOnly,
		Effect:     "nats runs one small JetStream binary; kafka needs the kafka build tag and the KAFKA_* settings.",
		DocsAnchor: docsEventBus,
		Resolve:    func(*Runtime) string { return config.EventBusProvider() },
	},
	{
		Key: "CODEC_PROVIDER", Group: GroupEventBus, RuntimeChangeable: ChangeBootOnly,
		Effect:     "json needs nothing; avro resolves every event against SCHEMA_REGISTRY_URL and is only compiled into the -kafka images. Producers and consumers have to agree, so it is changed by draining the bus, not in place.",
		DocsAnchor: docsEventBus,
		Resolve:    func(*Runtime) string { return config.CodecProvider() },
	},
	{
		Key: "NATS_URL", Group: GroupEventBus, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The NATS server campaign sends, mailbox syncs and tracking events flow through.",
		DocsAnchor: docsEventBus,
		Resolve:    envOr("NATS_URL", "nats://localhost:4222"),
	},
	{
		Key: "NATS_STREAM_NAME", Group: GroupEventBus, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The JetStream stream every subject is bound to.",
		DocsAnchor: docsEventBus,
		Resolve:    envOr("NATS_STREAM_NAME", "coldemail"),
	},
	{
		Key: "NATS_SUBJECT_PREFIX", Group: GroupEventBus, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The prefix every subject carries. It must match across the backend, the consumer and every worker.",
		DocsAnchor: docsEventBus,
		Resolve:    envOr("NATS_SUBJECT_PREFIX", "coldemail"),
	},
	{
		Key: "KAFKA_BOOTSTRAP_SERVERS", Group: GroupEventBus, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The Kafka brokers, when EVENTBUS_PROVIDER is kafka.",
		DocsAnchor: docsEventBus,
		Resolve:    envValue("KAFKA_BOOTSTRAP_SERVERS"),
	},
	{
		Key: "SCHEMA_REGISTRY_URL", Group: GroupEventBus, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The Avro schema registry. Only used with the kafka bus and the avro codec.",
		DocsAnchor: docsEventBus,
		Resolve:    envValue("SCHEMA_REGISTRY_URL"),
	},
	{
		Key: "EVENTBUS_HANDLER_TIMEOUT", Group: GroupEventBus, RuntimeChangeable: ChangeBootOnly,
		Effect:     "How long one consumer handler may run before the message is redelivered.",
		DocsAnchor: docsEventBus,
		Resolve:    envValue("EVENTBUS_HANDLER_TIMEOUT"),
	},

	// Workers.
	{
		Key: "INTERNAL_API_TOKEN", Group: GroupWorkers, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The token on the internal routes the tracking and forms services call. Fleet nodes use it too unless NODE_BROKER_TOKEN is set.",
		DocsAnchor: docsWorkers,
		Resolve:    envValue("INTERNAL_API_TOKEN"),
	},
	{
		Key: "NODE_BROKER_TOKEN", Group: GroupWorkers, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The token on the internal routes only fleet nodes call (keys, blob signing, sync, heartbeat). Falls back to INTERNAL_API_TOKEN.",
		DocsAnchor: docsWorkers, WhenUnset: SourceDerived,
		Resolve: func(*Runtime) string {
			if v := trimmed("NODE_BROKER_TOKEN"); v != "" {
				return v
			}
			return trimmed("INTERNAL_API_TOKEN")
		},
	},
	{
		Key: "ENCRYPTED_KEYS_BACKEND_URL", Group: GroupWorkers, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The backend base a worker reaches organization keys through. Empty makes the worker start normally and never register.",
		DocsAnchor: docsWorkers,
		Resolve:    envValue("ENCRYPTED_KEYS_BACKEND_URL"),
	},
	{
		Key: "ENCRYPTED_KEYS_WORKER_TOKEN", Group: GroupWorkers, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The worker's copy of the node token (NODE_BROKER_TOKEN, else INTERNAL_API_TOKEN). It must match the backend's value exactly.",
		DocsAnchor: docsWorkers,
		Resolve:    envValue("ENCRYPTED_KEYS_WORKER_TOKEN"),
	},
	{
		Key: "WORKER_ID", Group: GroupWorkers, RuntimeChangeable: ChangeBootOnly,
		Effect:     "A worker's stable identity. Derived from the hostname when unset, which changes on every container recreate.",
		DocsAnchor: docsWorkers,
		Resolve:    envValue("WORKER_ID"),
	},
	{
		Key: "COLD_EMAIL_NODE_REGION", Group: GroupWorkers, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Where this node egresses from, as a free-form label. Placement prefers a worker near where a mailbox's provider expects sign-ins; unset scores neutral.",
		DocsAnchor: docsWorkers,
		Resolve:    envValue("COLD_EMAIL_NODE_REGION"),
	},
	{
		Key: "MAIL_TLS_INSECURE", Group: GroupWorkers, RuntimeChangeable: ChangeBootOnly,
		Effect:     "true disables certificate verification on customer mailbox connections.",
		DocsAnchor: docsWorkers,
		Resolve:    boolOr("MAIL_TLS_INSECURE", false),
	},

	// Tracking.
	{
		Key: "FORMS_DOMAIN", Group: GroupAddresses, RuntimeChangeable: ChangePerRequest,
		Effect:     "The host hosted form pages and embeds are served on (forms.example.com, routed to the forms service). Share links and embed codes point here; unset leaves forms without a public URL.",
		DocsAnchor: docsAddresses,
		Resolve:    envValue("FORMS_DOMAIN"),
	},
	{
		Key: "FORM_IP_RATE_LIMIT", Group: GroupAddresses, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Public form submissions allowed per source IP per 10 minutes, enforced by the forms service. Default 30.",
		DocsAnchor: docsAddresses, WhenUnset: SourceDerived,
		Resolve: envValue("FORM_IP_RATE_LIMIT"),
	},
	{
		Key: "TRACKING_DOMAIN", Group: GroupTracking, RuntimeChangeable: ChangePerRequest,
		Effect:     "The host open pixels and click links point at. Recipients still receive mail when it is wrong, but nothing is recorded.",
		DocsAnchor: docsAddresses,
		Resolve:    envValue("TRACKING_DOMAIN"),
	},
	{
		Key: "TRACKING_SERVICE_URL", Group: GroupTracking, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Where the backend probes the tracking service. Unset means the System status page cannot report on it.",
		DocsAnchor: docsAddresses,
		Resolve:    envValue("TRACKING_SERVICE_URL"),
	},
	{
		Key: "BACKEND_INTERNAL_URL", Group: GroupTracking, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The backend base the tracking service resolves opaque click tickets through.",
		DocsAnchor: docsWorkers,
		Resolve:    envValue("BACKEND_INTERNAL_URL"),
	},
	{
		Key: "TRACKING_RATE_LIMIT_PER_MIN", Group: GroupTracking, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Tracking events counted per source address per minute. Over-budget pixels are still served, just not counted.",
		DocsAnchor: docsAddresses,
		Resolve:    envOr("TRACKING_RATE_LIMIT_PER_MIN", "300"),
	},
	{
		Key: "TRACKING_SCANNER_BUILTINS", Group: GroupTracking, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Loads the known-scanner network catalogue shipped with ColdEmail. Off leaves only the networks you name.",
		DocsAnchor: docsAddresses,
		Resolve:    boolOr("TRACKING_SCANNER_BUILTINS", true),
	},
	{
		Key: "TRACKING_SCANNER_NETWORKS", Group: GroupTracking, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Extra scanner sources whose opens and clicks are both recorded as automated.",
		DocsAnchor: docsAddresses,
		Resolve:    envValue("TRACKING_SCANNER_NETWORKS"),
	},
	{
		Key: "TRACKING_SCANNER_CLICK_NETWORKS", Group: GroupTracking, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Extra scanner sources judged on clicks only, for networks that also proxy a mail client's image fetches.",
		DocsAnchor: docsAddresses,
		Resolve:    envValue("TRACKING_SCANNER_CLICK_NETWORKS"),
	},
	{
		Key: "TRACKING_SCANNER_ASN_HEADER", Group: GroupTracking, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Header a trusted proxy sets with the source ASN. Where it is set it wins over the database below.",
		DocsAnchor: docsAddresses,
		Resolve:    envValue("TRACKING_SCANNER_ASN_HEADER"),
	},
	{
		Key: "TRACKING_SCANNER_ASN_DB", Group: GroupTracking, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Path to a GeoLite2-ASN database on the tracking service, which makes asn: entries match with no ASN header and no edge transform rule. It reads the client address, so behind a reverse proxy it still needs TRACKING_TRUSTED_PROXIES. A separate file from GEODB_PATH; missing is tolerated and only costs ASN matching.",
		DocsAnchor: docsGeoIP,
		Resolve:    envValue("TRACKING_SCANNER_ASN_DB"),
	},

	// Observability.

	// Updates.
	{
		Key: "UPDATE_CHECK_ENABLED", Group: GroupUpdates, RuntimeChangeable: ChangeBootOnly,
		Effect:     "Polls GitHub Releases for a newer ColdEmail and shows it in the admin panel's top bar and on Setup and health.",
		DocsAnchor: docsUpdates,
		Resolve:    boolOr("UPDATE_CHECK_ENABLED", true),
	},
	{
		Key: "UPDATE_CHECK_INTERVAL", Group: GroupUpdates, RuntimeChangeable: ChangeBootOnly,
		Effect:     "How often the release check runs. Minimum 5m.",
		DocsAnchor: docsUpdates,
		Resolve:    envOr("UPDATE_CHECK_INTERVAL", "30m"),
	},
	{
		Key: "UPDATE_CHANNEL", Group: GroupUpdates, RuntimeChangeable: ChangeBootOnly,
		Effect:     "stable follows releases; dev also offers prereleases.",
		DocsAnchor: docsUpdates,
		Resolve:    envOr("UPDATE_CHANNEL", "stable"),
	},
	{
		Key: "UPDATER_URL", Group: GroupUpdates, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The host-side updater that applies an update (pull, rebuild, restart). Unset leaves the panel report-only.",
		DocsAnchor: docsUpdates,
		Resolve:    envValue("UPDATER_URL"),
	},
	{
		Key: "UPDATER_TOKEN", Group: GroupUpdates, RuntimeChangeable: ChangeBootOnly,
		Effect:     "The bearer token the backend presents to the updater. Falls back to INTERNAL_API_TOKEN.",
		DocsAnchor: docsUpdates, WhenUnset: SourceDerived,
		Resolve: func(*Runtime) string {
			if v := trimmed("UPDATER_TOKEN"); v != "" {
				return v
			}
			return trimmed("INTERNAL_API_TOKEN")
		},
	},
}

// ssoRedirect mirrors the backend's own derivation, so the configuration page
// shows the URI that has to be registered at the provider rather than an empty
// cell whenever the operator left the override unset.
func ssoRedirect(key, provider string) func(*Runtime) string {
	return func(*Runtime) string {
		if v := trimmed(key); v != "" {
			return v
		}
		if base := strings.TrimRight(trimmed("API_PUBLIC_URL"), "/"); base != "" {
			return base + "/v1/auth/" + provider + "/callback"
		}
		return ""
	}
}

func envValue(key string) func(*Runtime) string {
	return func(*Runtime) string { return os.Getenv(key) }
}

func envOr(key, def string) func(*Runtime) string {
	return func(*Runtime) string {
		if v := trimmed(key); v != "" {
			return v
		}
		return def
	}
}

func boolOr(key string, def bool) func(*Runtime) string {
	return func(*Runtime) string {
		v := strings.ToLower(trimmed(key))
		switch v {
		case "1", "true", "yes", "on":
			return "true"
		case "0", "false", "no", "off":
			return "false"
		}
		return strconv.FormatBool(def)
	}
}

func trimmed(key string) string { return strings.TrimSpace(os.Getenv(key)) }

func yesNo(v bool) string { return strconv.FormatBool(v) }

// policy resolves the auth policy, preferring the one boot already built.
func policy(rt *Runtime) *config.AuthPolicy {
	if rt != nil && rt.Policy != nil {
		return rt.Policy
	}
	return config.LoadAuthPolicy(MailDelivers(rt))
}

// MailDelivers reports whether the configured transport puts mail on the wire.
func MailDelivers(rt *Runtime) bool {
	if rt != nil && rt.MailTransportKind != "" {
		return rt.MailDelivers
	}
	return config.MailTransport() != config.MailTransportLog
}
