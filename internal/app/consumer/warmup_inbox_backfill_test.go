package jobs

import (
	"context"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/events"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

// backfillPublisher records the warmup actions the sweep sends, or refuses
// them all when err is set.
type backfillPublisher struct {
	events.Publisher
	actions []models.WarmupEmailAction
	workers []uuid.UUID
	err     error
}

func (p *backfillPublisher) PublishWarmupAction(_ context.Context, workerID uuid.UUID, a *models.WarmupEmailAction) error {
	if p.err != nil {
		return p.err
	}
	p.actions = append(p.actions, *a)
	p.workers = append(p.workers, workerID)
	return nil
}

// backfillEmailRepo hands back one mailbox with a chosen filing preference,
// or a chosen lookup error.
type backfillEmailRepo struct {
	repository.EmailRepository
	account *models.Email
	err     *errx.Error
}

func (r backfillEmailRepo) GetByID(context.Context, uuid.UUID) (*models.Email, *errx.Error) {
	return r.account, r.err
}

func leakedWarmup() (models.JobEventNewEmail, *warmupCleanupInbox) {
	e := models.JobEventNewEmail{UserID: uuid.New(), Message: &models.EmailMessageStoreData{
		ID: uuid.New(), EmailID: uuid.New(), MessageID: "<leak@test.local>",
		GmailID: "gm-1", UID: 42, Mailbox: 7, FolderPath: "INBOX",
	}}
	return e, &warmupCleanupInbox{events: []models.JobEventNewEmail{e}}
}

// TestCleanupFilesTheLeakOutOfTheCustomersOwnMailbox covers the half of #583
// the sweep never did: deleting the Unibox row hides warmup mail from our
// inbox, but the copy in the customer's Gmail or Outlook is the one they are
// looking at, and nothing else ever goes back for it.

// TestCleanupLeavesWarmupAloneWhenTheOwnerAskedForInbox: placement "inbox" is a
// deliberate choice, so there is no leak to repair and moving the mail would
// override the setting.

// The Unibox row is the only retry record the sweep has. If the filing action
// never reaches the bus, deleting the row hides the leak in our inbox while
// leaving it in the customer's, with nothing left to try again from. A bus
// refusal is exactly the failure that took production down in #583.

// A lookup that fails for a reason other than "no such mailbox" is transient
// and keeps the row; a mailbox that is genuinely gone has nothing to file
// into and the row is cleared as before.
