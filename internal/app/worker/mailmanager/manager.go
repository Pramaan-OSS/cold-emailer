package mailmanager

import (
	"sync"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/app/cipher"
	"github.com/pramaan-oss/cold-email/internal/app/worker/wmail"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/cache"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/storage"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

type MailManager struct {
	sync.RWMutex
	ExecutorID                uuid.UUID
	Emails                    map[uuid.UUID]*wmail.WMail
	OnEvent                   func(eventType models.JobEventType, key string, body any) error
	cache                     *cache.Cache
	storage                   storage.Store
	emailMessageMapRepository repository.EmailMessageMapRepository
	syncContextRepository     repository.SyncContextRepository
	cipherService             cipher.CipherService
}

func NewMailManager(
	onEvent func(eventType models.JobEventType, key string, body any) error,
	cache *cache.Cache,
	storage storage.Store,
	emailMessageMapRepository repository.EmailMessageMapRepository,
	syncContextRepository repository.SyncContextRepository,
	cipherService cipher.CipherService,
) *MailManager {
	return &MailManager{
		Emails:                    make(map[uuid.UUID]*wmail.WMail),
		OnEvent:                   onEvent,
		cache:                     cache,
		storage:                   storage,
		emailMessageMapRepository: emailMessageMapRepository,
		syncContextRepository:     syncContextRepository,
		cipherService:             cipherService,
	}
}

// cfgFor returns the OAuth config the worker uses to refresh a provider's
// delegated token. Cfg is not carried in the AddWorkerEmail payload (avro
// excludes it), so it is rebuilt here from the worker's local oauth config.

// Get returns a WMail by ID, or nil if not present
func (m *MailManager) Get(id uuid.UUID) *wmail.WMail {
	m.RLock()
	defer m.RUnlock()
	return m.Emails[id]
}

// Has returns true if the manager already has this email account
func (m *MailManager) Has(id uuid.UUID) bool {
	m.RLock()
	defer m.RUnlock()
	_, ok := m.Emails[id]
	return ok
}
