package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	emailverifyapp "github.com/pramaan-oss/cold-email/internal/app/emailverify"
	"github.com/pramaan-oss/cold-email/internal/pramaan"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/app/advanced"
	"github.com/pramaan-oss/cold-email/internal/app/audit"
	"github.com/pramaan-oss/cold-email/internal/app/cipher"
	jobs "github.com/pramaan-oss/cold-email/internal/app/consumer"
	"github.com/pramaan-oss/cold-email/internal/app/contact"
	"github.com/pramaan-oss/cold-email/internal/app/correlate"
	"github.com/pramaan-oss/cold-email/internal/app/credits"
	"github.com/pramaan-oss/cold-email/internal/app/creditwatch"
	"github.com/pramaan-oss/cold-email/internal/app/feature"
	"github.com/pramaan-oss/cold-email/internal/app/inboxagent"
	"github.com/pramaan-oss/cold-email/internal/app/inboxtag"
	"github.com/pramaan-oss/cold-email/internal/app/instancesettings"
	"github.com/pramaan-oss/cold-email/internal/app/integration"
	"github.com/pramaan-oss/cold-email/internal/app/nativeactions"
	"github.com/pramaan-oss/cold-email/internal/app/notification"
	"github.com/pramaan-oss/cold-email/internal/app/opsnotify"
	"github.com/pramaan-oss/cold-email/internal/app/orgrisk"
	"github.com/pramaan-oss/cold-email/internal/app/replyclassify"
	warmupapp "github.com/pramaan-oss/cold-email/internal/app/warmup"
	"github.com/pramaan-oss/cold-email/internal/app/webhook"
	workerapp "github.com/pramaan-oss/cold-email/internal/app/worker"
	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/events"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/cache"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/codec"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/db"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/encryptedkeys"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/eventbus"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/kafka"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/kms"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/pubsub"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/pushnotify"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/storage"
	"github.com/pramaan-oss/cold-email/internal/jobrun"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/notify"
	"github.com/pramaan-oss/cold-email/internal/observability"
	"github.com/pramaan-oss/cold-email/internal/observability/errs"
	"github.com/pramaan-oss/cold-email/internal/pkg/encrypt"
	"github.com/pramaan-oss/cold-email/internal/pkg/generation"
	"github.com/pramaan-oss/cold-email/internal/pkg/geo"
	"github.com/pramaan-oss/cold-email/internal/pkg/nodeagent"
	"github.com/pramaan-oss/cold-email/internal/pkg/textcheck"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

func main() {
	var capabilityClient pramaan.Caller
	if c, err := pramaan.FromEnv(); err == nil {
		capabilityClient = c
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load config with env-first approach
	cfg, err := config.NewConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Error reporting
	if err := observability.Init(ctx, cfg, "consumer"); err != nil {
		log.Fatal(err)
	}

	// AWS SDK config, loaded only when an AWS-backed provider is selected
	// (KMS_PROVIDER=aws or BLOB_PROVIDER=s3). A fully-local self-host needs no
	// AWS_REGION or credentials.
	var awscfg any

	// PostgreSQL
	primaryDBEndpoint, err := cfg.LoadPrimaryDBEndpoint(ctx)
	if err != nil {
		log.Fatal(err)
	}
	primaryDB, err := db.New(ctx, primaryDBEndpoint)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Waiting for backend database migrations...")
	if err := db.WaitForSchema(ctx, primaryDB); err != nil {
		log.Fatal(err)
	}
	log.Println("Database schema ready")

	// Redis
	primaryRedis, err := cfg.LoadPrimaryRedisEndpoint(ctx)
	if err != nil {
		log.Fatal(err)
	}
	redisCache, err := cache.New(primaryRedis)
	if err != nil {
		log.Fatal(err)
	}

	// KMS → CipherService
	var masterKey string = "alias/master-key"
	if cfg.Env != "prod" {
		masterKey += "-dev"
	}

	kmsClient, err := kms.FromEnv(ctx, awscfg, masterKey)
	if err != nil {
		log.Fatal(err)
	}

	encryptedKeys, err := encryptedkeys.FromEnv(
		encryptedkeys.Deps{DB: primaryDB},
		"postgres",
	)
	if err != nil {
		log.Fatal(err)
	}
	cipherService := cipher.NewService(kmsClient, redisCache, encryptedKeys)

	// Blob storage (S3 by default, filesystem when BLOB_PROVIDER=filesystem).
	s3Client, err := storage.NewFromEnv(ctx, awscfg, "main")
	if err != nil {
		log.Fatal(err)
	}

	// Event bus + codec. Kafka bootstrap/SASL is only loaded for
	// EVENTBUS_PROVIDER=kafka; codec.FromEnv owns serialization (Avro pulls
	// SCHEMA_REGISTRY_URL from env, JSON needs nothing). The bus drives both the
	// inbound worker-events subscription and outbound publishing, so a NATS +
	// JSON self-host needs no Kafka or Schema Registry config.
	var kafkaBootstrapServers string
	var kafkaSaslConfig *kafka.SASLConfig
	if config.EventBusProvider() == "kafka" {
		kafkaBootstrapServers, err = cfg.LoadKafkaBootstrapServers(ctx)
		if err != nil {
			log.Fatal(err)
		}
		kafkaSaslConfig, err = cfg.LoadKafkaConfigSasl(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}

	consumerCodec, err := codec.FromEnv()
	if err != nil {
		log.Fatal(err)
	}

	consumerBus, err := eventbus.FromEnv(kafkaBootstrapServers, kafkaSaslConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer consumerBus.Close()

	// Realtime event transport, chosen by PUBSUB_ENABLED — the SAME flag the
	// backend and the Elixir realtime service read, so the three services can
	// never split-brain. PUBSUB_ENABLED=true => Google Pub/Sub (prod); anything
	// else => Redis bridge (local dev / non-GCP). Exactly one transport is active.
	var streamingPublisher *pubsub.StreamingPublisher
	if os.Getenv("PUBSUB_ENABLED") == "true" {
		gcpProjectID := os.Getenv("GCP_PROJECT_ID")
		if gcpProjectID == "" {
			log.Fatal("PUBSUB_ENABLED=true requires GCP_PROJECT_ID")
		}
		pubsubClient, err := pubsub.NewClient(ctx, gcpProjectID)
		if err != nil {
			errs.CaptureFatal(err)
			log.Fatal(err)
		}
		defer pubsubClient.Close()
		// Idempotently ensure the realtime topics + subscriptions exist (safe to
		// run from both backend and consumer; AlreadyExists is treated as success).
		if err := pubsubClient.EnsureRealtimeTopology(ctx); err != nil {
			errs.CaptureFatal(err)
			log.Fatal("Failed to provision Pub/Sub topics/subscriptions: ", err)
		}
		streamingPublisher = pubsub.NewStreamingPublisher(pubsubClient, repository.NewEventAudienceRepository(primaryDB.Pool))
	}
	if streamingPublisher == nil {
		streamingPublisher = pubsub.NewStreamingPublisher(pubsub.NewRedisBus(redisCache.Client, ""), repository.NewEventAudienceRepository(primaryDB.Pool))
		log.Println("Realtime events bridged over Redis (Pub/Sub disabled)")
	}

	// Repositories
	credEncrypter, err := encrypt.FromEnv()
	if err != nil {
		errs.CaptureFatal(err)
		log.Fatal("Invalid CREDENTIALS_ENCRYPTION_KEY: ", err)
	}
	emailRepo := repository.NewEmailRepostory(primaryDB, credEncrypter)
	uniboxRepo := repository.NewUniboxRepository(primaryDB)
	mailboxRepo := repository.NewMailboxRepository(primaryDB)
	emailHistoryIDRepo := repository.NewEmailHistoryIDRepository(primaryDB)
	emailGraphDeltaRepo := repository.NewEmailGraphDeltaRepository(primaryDB)
	emailAccountErrorRepo := repository.NewEmailAccountErrorRepository(primaryDB)
	warmupRepo := repository.NewWarmupRepository(primaryDB.Pool)
	warmupService := warmupapp.NewService(warmupRepo)
	// Push warmup-health transitions live to the dashboard. The health sweep
	// runs in this process, so the realtime publisher is wired here.
	if streamingPublisher != nil {
		warmupService.WireRealtime(streamingPublisher, emailRepo)
	}
	workerRepo := repository.NewWorkerRepository(primaryDB.Pool)
	subscriptionRepoConsumer := repository.NewSubscriptionRepository(primaryDB.Pool)
	planRepoConsumer := repository.NewPlanRepository(primaryDB.Pool)
	workerAssignmentSvc := workerapp.NewAssignmentService(workerRepo, subscriptionRepoConsumer, planRepoConsumer)
	campaignRepo := repository.NewCampaignRepostory(primaryDB)
	taskRepo := repository.NewTaskRepository(primaryDB.Pool)
	contactRepo := repository.NewContactRepostory(primaryDB)
	campaignProgressRepo := repository.NewCampaignProgressRepository(primaryDB.Pool)
	crmRepo := repository.NewCRMRepository(primaryDB.Pool)
	orgRepoConsumer := repository.NewOrganizationRepository(primaryDB.Pool)
	advancedRepo := repository.NewAdvancedOutreachRepository(primaryDB.Pool)

	// Reply → integration fan-out. The consumer is where inbound replies are
	// detected, so this is where "prospect replied" turns into a Chat ping /
	// CRM upsert. webhookService.Dispatch enqueues customer webhook deliveries
	// (drained by the backend's DeliveryWorker) AND, via the wired sink, runs
	// integration actions in-process (cipher + Postgres are available here; the
	// consumer is control-plane, not a worker). Suppression already lives in the
	// advanced repo, so no separate suppression repo is wired here.
	webhookRepoC := repository.NewWebhookRepositorySealed(primaryDB.Pool, credEncrypter)
	webhookService := webhook.NewService(webhookRepoC)
	// The consumer dispatches lower-volume reply/warmup events (not per-contact
	// campaign fan-out), so a generous static cap is enough here; the plan-based
	// resolver lives in the backend where campaign "notify" actions run.
	webhookService.WireThrottle(redisCache, webhook.StaticLimit(config.WebhookDispatchBasePerMinute))
	integrationRepoC := repository.NewIntegrationRepository(primaryDB.Pool)
	integrationServiceC := integration.NewService(integrationRepoC, cipherService, capabilityClient)
	webhookService.WireDispatchSink(integrationServiceC.DispatchAny)
	// AI automation nodes + reply-classifier Layer 3 run in THIS process (reply /
	// warmup / bounce events dispatch here). Build the credit ledger + provider so
	// the ai_step / ai_switch nodes can charge + call, and so the classifier's
	// Text and tool calls use the Pramaan llm.chat capability.
	creditRepoC := repository.NewCreditRepository(primaryDB)
	aiSettingsRepoC := repository.NewAISettingsRepository(primaryDB)
	creditServiceC := credits.NewService(creditRepoC, aiSettingsRepoC, redisCache)
	// Consumer-side debits (automation AI nodes) also feed the low-balance
	// alert. Auto top-up stays backend-only (no Billing service here).
	creditServiceC.SetMonitor(creditwatch.New(aiSettingsRepoC, creditRepoC, redisCache, streamingPublisher, nil).OnBalanceChanged)
	var aiProviderC generation.Provider
	// Pluggable web search (Serper/SearXNG) backs the AI switch's optional
	// company lookup on reply-triggered automations that run in the consumer.
	aiSearchC := generation.NewSearchClient(capabilityClient)
	if provider, err := generation.NewProvider(generation.ProviderConfig{Search: aiSearchC, Client: capabilityClient}); err == nil {
		aiProviderC = provider
	}

	integrationServiceC.SetAI(aiProviderC, creditServiceC)
	integrationServiceC.SetAISearch(aiSearchC)
	// One TextCheck client for every typed judgment in this process. Nil when
	// no key is configured, and every feature that reads it stays off.
	var textCheckClient *textcheck.Client
	if key := config.TextCheckAvailability(); key != "" {
		textCheckClient = textcheck.NewClient(key, textcheck.WithBillingRedis(redisCache.Client))
	}
	if aiProviderC != nil {
		replyclassify.SetModelClassifier(func(ctx context.Context, system, user string) (string, error) {
			res, err := aiProviderC.Complete(ctx, generation.CompletionRequest{Structured: true, System: system, Prompt: user, MaxTokens: 256, Temperature: generation.Deterministic()})
			if err != nil {
				return "", err
			}
			return res.Text, nil
		})
	}
	// Warmup health transitions happen in THIS process (the health sweep + all
	// event-driven re-evaluations run in the consumer). Without wiring the
	// webhook dispatcher here, dispatchHealthEvent saw s.webhooks == nil and
	// every warmup.health_changed / quarantined / blocked event silently fired
	// no webhook. Dispatch only enqueues delivery rows in Postgres (drained by
	// the backend's DeliveryWorker), so no worker/PG boundary is crossed.
	warmupService.WireWebhooks(webhookService, emailRepo)

	advancedService := advanced.NewService(
		advancedRepo,
		campaignRepo,
		emailRepo,
		taskRepo,
		contactRepo,
		campaignProgressRepo,
		crmRepo,
		repository.NewGroupRepostory(primaryDB, models.Categories),
		uniboxRepo,
		nil, // tasksClient: the consumer does not schedule Cloud Tasks
		warmupService,
	)
	// Instant reply actions can pin a contact into or out of a segment.
	if aware, ok := advancedService.(advanced.SegmentAware); ok {
		aware.WireSegments(repository.NewSegmentRepository(primaryDB))
	}
	advancedService.WireDispatcher(webhookService)

	// Replies, bounces, opens and clicks teach verification what real mail
	// showed about each address.
	verificationEvidence := emailverifyapp.NewEvidence(repository.NewVerificationEvidenceRepository(primaryDB))
	if aware, ok := advancedService.(advanced.EvidenceAware); ok {
		aware.WireEvidence(verificationEvidence)
	}
	// Reply/open/click instant action chains run in THIS process (inbox ingest +
	// tracking consumer), so a "run_automation" node on an instant branch must be
	// able to launch the flow here too. Without this it would be stamped sent and
	// never fire. Mirrors the scheduler's automationRunner wiring.
	advancedService.WireAutomationRunner(integrationServiceC)
	// Native (ColdEmail-internal) automation actions run wherever the event is
	// dispatched. Reply/bounce/warmup events dispatch in THIS process, so without
	// wiring native actions here a reply-triggered automation's add_tag /
	// create_deal / label_email node would fail with "native actions are not
	// available". Mirrors the backend wiring.
	// The lead-intake actions (create or update contact, add to campaign) write
	// through a contact service so a reply-triggered flow in this process gets
	// the same plan check, campaign wake and contact.created as the backend.
	contactServiceC := contact.NewService(contactRepo, subscriptionRepoConsumer, planRepoConsumer, streamingPublisher)
	if aware, ok := contactServiceC.(contact.WebhookAware); ok {
		aware.WireWebhooks(webhookService)
	}
	integrationServiceC.SetNativeActions(nativeactions.Adapter{
		Adv:        advancedService,
		Contacts:   contactRepo,
		Orgs:       orgRepoConsumer,
		ContactSvc: contactServiceC,
	})
	// In-app notifications: the reply/bounce/complaint gate fires in THIS
	// process (inbox ingest + deliverability ingest run in the consumer), so the
	// notifier must be wired here. Missing this = notifications silently never
	// created.
	notificationService := notification.NewService(repository.NewNotificationRepository(primaryDB.Pool), streamingPublisher)
	// Email + Chat delivery for notifications. Email is best-effort: the
	// SES/SMTP service only constructs when email config is present (prod, or
	// a dev env that sets it), so a bare dev consumer simply skips the email
	// channel. Chat reuses the integration service (token decryption).
	var notifEmail notification.EmailSender
	if emailCfg, ecErr := cfg.LoadEmailConfig(ctx); ecErr == nil {
		if transport, tErr := notify.NewTransport(ctx, cfg, emailCfg.EmailName, emailCfg.EmailAddress, capabilityClient); tErr == nil {
			notifEmail = transport
			log.Printf("Notification mail transport: %s", transport.Description)
		} else {
			log.Printf("Warning: notification email disabled: %v", tErr)
		}
	} else {
		log.Printf("Warning: notification email disabled, EMAIL_NAME/EMAIL_ADDRESS not set: %v", ecErr)
	}
	notificationService.WireDelivery(notifEmail, nil, repository.NewUserRepostory(primaryDB, kmsClient), orgRepoConsumer)

	// Operator alerts. The dead-worker detector runs in this process, and a
	// stranded fleet is the operator's problem, not a tenant's. Reads the same
	// channel list the admin panel writes; a mail transport is optional (the
	// chat and webhook transports do not need one).
	var opsMailer opsnotify.Mailer
	if notifEmail != nil {
		if m, ok := notifEmail.(opsnotify.Mailer); ok {
			opsMailer = m
		}
	}
	opsNotifierC := opsnotify.NewService(
		instancesettings.NewService(instancesettings.NewStore(primaryDB.Pool)),
		opsMailer,
		config.AppBaseURL(),
	)
	// Mobile push (APNs) fires from THIS process too: reply/bounce/complaint
	// notifications are created here. Redis backs the immediate-then-digest
	// window shared with the backend. The sender stays a nil interface (not a
	// typed-nil *pushnotify.Client) when unconfigured.
	var pushSender notification.PushSender
	if client := pushnotify.New(capabilityClient); client != nil {
		pushSender = client
	}
	notificationService.WirePush(pushSender, repository.NewDeviceTokenRepository(primaryDB.Pool), redisCache.Client)
	advancedService.WireNotifier(notificationService)
	// Reply pulses fire in THIS process too (inbox ingest classifies replies).
	advancedService.WireRealtime(streamingPublisher)
	// A lifted reply opt-out is recorded like a member lifting one by hand.
	advancedService.WireAudit(audit.NewService(repository.NewAuditRepository(primaryDB.Pool), streamingPublisher))
	// Inbox agent (M10): inbound human replies are ingested + classified in THIS
	// process, so the agent that drafts a suggested reply must be wired here. It
	// is paid + opt-in (checked inside) and self-detaches, so a slow model never
	// blocks reply ingest. Nil provider leaves it inert.
	inboxAgentServiceC := inboxagent.NewService(
		aiProviderC,
		creditServiceC,
		feature.NewService(subscriptionRepoConsumer, planRepoConsumer),
		orgRepoConsumer,
		uniboxRepo,
		nil, // skills preamble optional; not constructed in the consumer
		contactRepo,
		repository.NewAIDraftRepository(primaryDB.Pool),
		streamingPublisher,
	)
	advancedService.WireInboxAgent(inboxAgentServiceC)
	if textCheckClient != nil {
		// A bounce whose reason does not name the recipient is classified, so a
		// reputation or policy block does not suppress a good address.
		advancedService.WireBounceJudge(textCheckClient)
	}

	eventsPublisher := events.NewPublisher(consumerBus, s3Client, consumerCodec, cipherService)

	// Every consumer loop records to scheduled_job_runs, so the admin panel
	// lists it next to the backend's and can ask it to run now.
	jobrun.Configure(repository.NewJobRunRepository(primaryDB), "consumer")

	// JobsService
	// Follow-up labels use stored mailbox facts and run without TextCheck.
	// Message classification still requires both the key and opt-in switch.
	tagCategories := repository.NewTagCategoryStore(primaryDB.Pool)
	inboxTagRepo := repository.NewInboxTagRepository(primaryDB.Pool)
	var tagAsker inboxtag.Asker
	classify := config.InboxTaggingEnabled() && textCheckClient != nil
	if classify {
		tagAsker = inboxtag.NewAsker(textCheckClient)
		log.Printf("automatic inbox tagging enabled (model %s)", inboxtag.Model)
	} else {
		log.Printf("automatic inbox classification off; timestamp-based follow-up labels still run locally")
	}
	inboxTagger := inboxtag.NewService(
		tagAsker,
		inboxTagRepo,
		tagCategories,
		tagCategories,
		classify,
	)
	inboxTagger.WireSettings(advancedRepo)
	if textCheckClient != nil {
		// The reply classifier's model layer and the inbox agent's gate both
		// read the verdict the tagger stored moments earlier, so a reply is
		// paid for once. Without tagging the classifier asks one question.
		replyclassify.SetTypedClassifier(inboxtag.ReplyClassifier(textCheckClient, inboxTagRepo))
		inboxAgentServiceC.WireDraftGate(inboxtag.NewDraftGate(inboxTagRepo))
	}
	advancedService.WireInboxTags(inboxTagRepo)

	jobsService := &jobs.JobsService{
		Bus:                         consumerBus,
		Codec:                       consumerCodec,
		UniboxRepository:            uniboxRepo,
		MailboxRepository:           mailboxRepo,
		EmailRepository:             emailRepo,
		EmailHistoryIDRepository:    emailHistoryIDRepo,
		EmailGraphDeltaRepository:   emailGraphDeltaRepo,
		EmailSyncStateRepository:    repository.NewEmailSyncStateRepository(primaryDB),
		ArrivalOutbox:               repository.NewDurableEmailMessageMapRepository(primaryDB, cipherService),
		EmailAccountErrorRepository: emailAccountErrorRepo,
		WarmupRepo:                  warmupRepo,
		WarmupContentRepo:           repository.NewWarmupContentRepository(primaryDB.Pool),
		WarmupEngagementRepo:        repository.NewWarmupEngagementRepository(primaryDB.Pool),
		WarmupRecoveryRepo:          repository.NewWarmupRecoveryRepository(primaryDB.Pool),
		WarmupPlacementRepo:         repository.NewWarmupPlacementRepository(primaryDB),
		PlacementRepo:               repository.NewPlacementRepository(primaryDB),
		WarmupService:               warmupService,
		WorkerRepo:                  workerRepo,
		FleetNodeRepo:               repository.NewFleetNodeRepository(primaryDB),
		LifecycleRepo:               repository.NewSendLifecycleRepository(primaryDB),
		Publisher:                   eventsPublisher,
		StreamingPublisher:          streamingPublisher,
		AdvancedService:             advancedService,
		InboxTagger:                 inboxTagger,
		Cache:                       redisCache,
		Retention:                   instancesettings.NewService(instancesettings.NewStore(primaryDB.Pool)),
		AdminRepo:                   repository.NewAdminRepository(primaryDB.Pool),
		AssignmentService:           workerAssignmentSvc,
		Notifier:                    notificationService,
		OpsNotifier:                 opsNotifierC,
		TaskRepo:                    taskRepo,
		CampaignRepo:                campaignRepo,
		CampaignProgressRepo:        campaignProgressRepo,
		CampaignLogRepo:             repository.NewCampaignLogRepository(primaryDB),
		ContactRepo:                 contactRepo,
		Evidence:                    verificationEvidence,
	}

	jobsService.InitEvents()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down consumer...")
		cancel()
	}()

	// Start DLQ auto-retry loop in background (every 60 seconds)
	go jobsService.StartDLQRetryLoop(ctx, 60*time.Second)

	// Start warmup health evaluation sweep (every hour)
	go jobsService.StartWarmupHealthSweep(ctx, 1*time.Hour)

	// Persist each mailbox's sending-domain SPF/DKIM/DMARC state (observe-only,
	// not yet a send gate): sweep hourly, rechecking each domain at most daily.
	go jobsService.StartAuthCheckSweep(ctx, 1*time.Hour, 24*time.Hour)

	// Drains the durable delayed-engagement schedule (read/important/star) so the
	// recipient-side dwell survives worker restarts. Short interval keeps the
	// effective dwell close to the requested value.
	go jobsService.StartWarmupEngagementPoller(ctx, 30*time.Second)
	go jobsService.StartWarmupInboxCleanup(ctx)
	go jobsService.StartWarmupFilingRecovery(ctx)
	// Deletes warmup mail past its retention window from the mailbox itself
	// and prunes the per-message warmup records after theirs.
	go jobsService.StartWarmupMailRetention(ctx)
	// Searches the mailbox for deletion strikes recorded before removals were
	// checked, withdrawing any whose message is still there.
	go jobsService.StartWarmupTamperingRecheck(ctx)
	// Attributes each warmup email moved to spam once the activity around it settles.
	go jobsService.StartWarmupSpamMoveAttribution(ctx)
	go jobsService.StartWarmupPlacementSweep(ctx)
	go jobsService.StartPendingWarmupVerification(ctx)
	// Re-offers inbound mail that reply processing never claimed, so a
	// reply refused by a since-fixed check is still attributed to its lead.
	go jobsService.StartIncomingReplyRepair(ctx)
	go jobsService.StartReplyOptOutRecheck(ctx)

	// a cloud quarantine pauses their campaigns here too. No-op when unlinked.

	// Start dead worker detection (every 5 minutes)
	go jobsService.StartDeadWorkerDetection(ctx, 5*time.Minute)

	// Resolve campaign sends that were reserved and dispatched but whose worker
	// result never came back, so a lead is never held in flight forever.
	go jobsService.StartStuckSendReclaimer(ctx, 5*time.Minute)

	// Mirror Redis heartbeats into workers.last_seen_at every 60s
	// so the admin dashboard can render liveness without touching Redis.
	go jobsService.StartWorkerHeartbeatSync(ctx, 60*time.Second)

	// Re-evaluate per-mailbox risk bands hourly. The band feeds warmup partner
	// selection and pacing; it does not move mailboxes between workers, because
	// the worker is not the sending identity.
	go jobsService.StartRiskRebalancer(ctx, 1*time.Hour)
	// Same cadence, different question: the band describes reputation, the
	// lifecycle picks whether the mailbox is in cold rotation at all.
	go jobsService.StartLifecycleRebalancer(ctx, 1*time.Hour)
	// The abuse sweep: cross-account shape, plus what each organization's mail
	// did to recipients. Nightly, because both form over days.
	orgRiskService := orgrisk.NewService(repository.NewOrgRiskRepository(primaryDB))
	go correlate.NewService(
		repository.NewCorrelationRepository(primaryDB),
		repository.NewOrgConductRepository(primaryDB),
		orgRiskService,
	).Start(ctx, 24*time.Hour)
	// Evidence a one-shot detector filed cannot retract itself, so it carries
	// an expiry; only a re-derive moves the stored score, which is this.
	go orgrisk.StartExpirySweep(ctx, orgRiskService, orgrisk.ExpirySweepInterval)

	// Tracking consumer (opens/clicks): a second subscription on the shared bus
	// for the tracking topic. It records open/click engagement and fires INSTANT
	// open/click action chains (advancedService), the open/click analog of the
	// reply path. Decodes with the same codec the Rust tracking service writes
	// (Avro on Kafka, JSON on NATS).
	// GeoIP is optional here as on the backend: it only turns an open or
	// click's source network into a country and city on the logs.
	// Nothing waits for the database: opens and clicks are recorded either way
	// and only their country and city label depends on it, so a mirror that is
	// slow or gone must not delay this service coming up.
	geoPath, _ := cfg.LoadGeoDBPath(ctx)
	geoloc, _ := geo.New("")
	geo.Start(ctx, geoloc, geoPath, cfg.LoadGeoDBURL(ctx))
	if trackingCfg, terr := cfg.LoadTrackingConsumerConfig(ctx); terr != nil {
		log.Println("tracking consumer config unavailable; opens/clicks not consumed:", terr)
	} else if trackingConsumer, terr := jobs.NewTrackingConsumer(
		consumerBus,
		consumerCodec,
		trackingCfg.Topic,
		trackingCfg.GroupID,
		taskRepo,
		campaignProgressRepo,
		campaignRepo,
		contactRepo,
		streamingPublisher,
		repository.NewTrackingDedupeRepository(primaryDB.Pool),
		repository.NewTrackedLinkRepository(primaryDB.Pool),
		repository.NewLinkClickRepository(primaryDB.Pool),
		advancedService,
		verificationEvidence,
		repository.NewEmailOpenRepository(primaryDB.Pool),
		geoloc,
	); terr != nil {
		log.Println("tracking consumer unavailable; opens/clicks not consumed:", terr)
	} else {
		// The engagement prune reads its window from the instance settings on
		// every pass, and the machine-window rule reads its windows per event,
		// so editing either in the admin panel needs no restart. Both go
		// through this process's own read cache, so an edit lands within its
		// TTL rather than instantly.
		trackingSettings := instancesettings.NewService(instancesettings.NewStore(primaryDB.Pool))
		trackingConsumer.WireRetention(trackingSettings)
		trackingConsumer.WireTrackingPolicy(trackingSettings)
		trackingConsumer.WireDirectMail(emailRepo)
		defer trackingConsumer.Close()
		go func() {
			if err := trackingConsumer.Start(ctx); err != nil {
				log.Println("tracking consumer stopped:", err)
			}
		}()
		log.Println("Tracking consumer started, listening on", trackingCfg.Topic)
	}

	// The consumer is a fleet node like any other: it enrols, heartbeats,
	// reports what it is running and what it is using, and picks up the
	// version the control plane wants. Before this it was anonymous, so a
	// dead one stayed invisible until work started backing up.
	agentDone := make(chan struct{})
	go func() {
		defer close(agentDone)
		newConsumerAgent().Run(ctx)
	}()

	log.Println("Consumer started, listening on", kafka.TopicWorkerEvents)
	jobsService.Start(ctx)

	// Give the farewell beat a moment to land, bounded so a wedged backend
	// cannot stop the consumer exiting.
	select {
	case <-agentDone:
	case <-time.After(8 * time.Second):
		log.Println("timed out waiting for the shutdown heartbeat")
	}
	log.Println("Consumer stopped")
}

// newConsumerAgent builds the fleet agent for this consumer.
//
// Identity resolution mirrors the worker's: an explicit COLD_EMAIL_NODE_ID wins,
// otherwise it is derived from the hostname so a container recreate keeps the
// same identity instead of leaving a dead row behind on every restart.
func newConsumerAgent() *nodeagent.Agent {
	id := resolveConsumerID()
	return nodeagent.New(nodeagent.Config{
		NodeID:            id,
		Role:              models.NodeRoleConsumer,
		Name:              os.Getenv("COLD_EMAIL_NODE_NAME"),
		Region:            os.Getenv("COLD_EMAIL_NODE_REGION"),
		Version:           os.Getenv("COLD_EMAIL_VERSION"),
		BaseURL:           consumerBackendURL(),
		Token:             config.NodeAPIToken(),
		TargetVersionPath: os.Getenv("COLD_EMAIL_TARGET_VERSION_PATH"),
	})
}

func resolveConsumerID() uuid.UUID {
	if raw := os.Getenv("COLD_EMAIL_NODE_ID"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			return id
		}
		log.Printf("COLD_EMAIL_NODE_ID is not a valid uuid; deriving one from the hostname instead")
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		// Last resort. A random id means this process shows up as a new node
		// on every restart, which is visible in the dashboard rather than
		// silent, so it is a better failure than refusing to start.
		return uuid.New()
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("coldemail-consumer:"+host))
}

func consumerBackendURL() string {
	if v := os.Getenv("COLD_EMAIL_BACKEND_URL"); v != "" {
		return v
	}
	return os.Getenv("ENCRYPTED_KEYS_BACKEND_URL")
}
