package wmail

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/app/cipher"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/cache"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/storage"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

type WMail struct {
	diagnosticRetries   map[uuid.UUID]diagnosticRetry
	laneCache           laneCache
	unmapPending        map[string]uuid.UUID
	Broker              pramaan.Caller
	cursor              string
	lifecycleMu         sync.Mutex
	stopped             bool
	stopDone            chan struct{}
	active              sync.WaitGroup
	syncMu              sync.Mutex
	executionKeys       map[[32]byte]struct{}
	initialExecutionKey [32]byte
	executionData       *models.AddWorkerEmail

	ExecutorID uuid.UUID
	UserID     uuid.UUID
	ID         uuid.UUID
	// OrgID scopes the organization-wide sync budget; nil for a legacy
	// personal mailbox.
	OrgID *uuid.UUID

	Email          string
	FirstName      string
	LastName       string
	SignaturePlain string
	SignatureHTML  string

	// SaveToSent files a copy of every outbound message in the mailbox's Sent
	// folder. SMTP/IMAP only: Gmail and Graph file their own copy, so acting on
	// it there would duplicate every sent message.
	SaveToSent bool

	EmailType models.InboxProvider

	Cache                     *cache.Cache
	Storage                   storage.Store
	EmailMessageMapRepository repository.EmailMessageMapRepository
	SyncContext               repository.SyncContextRepository
	CipherService             cipher.CipherService

	// Sync fair use: the budget engine and the relayed state (governor.go,
	// sync_state.go). Built in NewWMail from the ADD_EMAIL payload.
	gov     syncBudget
	tracker *syncTracker
	// transportFailures counts consecutive passes that could not reach the
	// mail server, which paces the retry and keeps one outage to one warning.
	transportFailures int

	Ctx           context.Context
	Cancel        context.CancelFunc
	TerminateFunc func()

	onEvent func(jobType models.JobEventType, body any) error
}

func NewWMail(
	data *models.AddWorkerEmail,
	OnEvent func(eventType models.JobEventType, key string, body any) error,
	terminate func(),
	cache *cache.Cache, storage storage.Store,
	emailMessageMapRepository repository.EmailMessageMapRepository,
	syncContext repository.SyncContextRepository,
	cipherService cipher.CipherService,
) (*WMail, *errx.MailError) {
	// Use background context so the WMail outlives the AddEmail request handler.
	mailCtx, cancel := context.WithCancel(context.Background())

	mail := &WMail{
		ID:        data.ID,
		UserID:    data.UserID,
		OrgID:     data.OrganizationID,
		Email:     data.Email,
		FirstName: data.FirstName,
		LastName:  data.LastName,
		EmailType: data.Type,
		// Unset in the payload means yes; see AddWorkerEmail.SavesSentCopy.
		SaveToSent: data.Type == models.InboxProviderSMTPIMAP && data.SavesSentCopy(),
		onEvent: func(jobType models.JobEventType, body any) error {
			return OnEvent(jobType, data.ID.String(), body)
		},

		Ctx:           mailCtx,
		Cancel:        cancel,
		TerminateFunc: terminate,

		Cache:                     cache,
		Storage:                   storage,
		EmailMessageMapRepository: emailMessageMapRepository,
		SyncContext:               syncContext,
		CipherService:             cipherService,
	}
	mail.rememberExecution(data)
	ready := false
	defer func() {
		if !ready {
			mail.Discard()
		}
	}()

	// A publisher older than the sync policy sends no Sync block; compiled
	// defaults then apply and the mailbox starts its backfill from scratch.
	var seed *models.SyncState
	var policy models.SyncPolicy
	if data.Sync != nil {
		policy = data.Sync.Policy
		seed = data.Sync.State
	}
	// A nil *cache.Cache must not become a non-nil interface holding nil.
	var gcache redisCmdable
	if cache != nil {
		gcache = cache
	}
	mail.gov = newGovernor(data.ID, data.OrganizationID, gcache, policy)
	if seed != nil {
		mail.cursor = seed.BackfillCursor.PageToken
	}
	mail.tracker = newSyncTracker(seed, func(st models.SyncState) error {
		return mail.onEvent(models.JobEventTypeSyncState, &models.JobEventSyncState{
			UserID:  mail.UserID,
			EmailID: mail.ID,
			State:   st,
		})
	})

	brokerClient, brokerErr := pramaan.FromEnv()
	if brokerErr != nil {
		return nil, errx.MError(errx.MailErrorCritical, errx.MailErrorCodeAuthenticationFailed, "Connect Pramaan before loading this mailbox.", errx.MailErrorResolveMethodReload)
	}

	mail.Broker = brokerClient
	ready = true
	return mail, nil
}

func (w *WMail) ApplySyncPolicy(data *models.AddWorkerEmailSyncData) {
	if data == nil || w.gov == nil {
		return
	}
	w.gov.SetPolicy(data.Policy)
}

// Discard releases a WMail that was built but never put to work.
func (w *WMail) Discard() {
	w.Stop()
}
