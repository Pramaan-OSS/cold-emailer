package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

type warmupInboxRepo struct {
	repository.UniboxRepository
	entries int
}

func (r *warmupInboxRepo) CreateEntry(context.Context, uuid.UUID, *models.EmailMessageStoreData) error {
	r.entries++
	return nil
}

type warmupInboxEmailRepo struct{ repository.EmailRepository }

func (warmupInboxEmailRepo) GetByID(context.Context, uuid.UUID) (*models.Email, *errx.Error) {
	return nil, nil
}

func TestHandleNewEmailKeepsLocalWarmupSentCopyOutOfInbox(t *testing.T) {
	account, token := uuid.New(), uuid.New()
	inbox := &warmupInboxRepo{}
	s := &JobsService{
		WarmupRepo: &stubWarmupTokenRepo{live: &models.WarmupToken{
			Token: token, SenderAccountID: account, RecipientAccountID: uuid.New(),
		}},
		UniboxRepository: inbox,
		EmailRepository:  warmupInboxEmailRepo{},
	}
	err := s.HandleNewEmail(context.Background(), &models.JobEventNewEmail{
		UserID: uuid.New(),
		Message: &models.EmailMessageStoreData{
			EmailID: account, Folder: models.FolderSent,
			Flags: []string{config.WarmupVerifyHeader + ":" + token.String()},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inbox.entries != 0 {
		t.Fatal("warmup Sent copy was exposed as an ordinary Unibox conversation")
	}
}

func TestHandleNewEmailConsumedAndExpiredWarmupStayHidden(t *testing.T) {
	for _, senderCopy := range []bool{false, true} {
		for _, consumed := range []bool{false, true} {
			account, token := uuid.New(), uuid.New()
			known := &models.WarmupToken{Token: token, SenderAccountID: uuid.New(), RecipientAccountID: account, ExpiresAt: time.Now().Add(-time.Hour)}
			if senderCopy {
				known.SenderAccountID, known.RecipientAccountID = account, uuid.New()
			}
			if consumed {
				now := time.Now()
				known.ConsumedAt = &now
				known.ExpiresAt = now.Add(time.Hour)
			}
			inbox := &warmupInboxRepo{}
			s := &JobsService{WarmupRepo: &stubWarmupTokenRepo{live: known}, UniboxRepository: inbox, EmailRepository: warmupInboxEmailRepo{}}
			err := s.HandleNewEmail(context.Background(), &models.JobEventNewEmail{Message: &models.EmailMessageStoreData{
				EmailID: account, Flags: []string{config.WarmupVerifyHeader + ":" + token.String()},
			}})
			if err != nil || inbox.entries != 0 {
				t.Fatalf("sender=%v consumed=%v: error=%v entries=%d", senderCopy, consumed, err, inbox.entries)
			}
		}
	}
}

type warmupCleanupInbox struct {
	repository.UniboxRepository
	events  []models.JobEventNewEmail
	deleted []uuid.UUID
}

func (r *warmupCleanupInbox) ListWarmupReviewCandidates(_ context.Context, afterID uuid.UUID, limit int) ([]models.JobEventNewEmail, error) {
	var out []models.JobEventNewEmail
	for _, e := range r.events {
		if e.Message.ID.String() > afterID.String() {
			out = append(out, e)
		}
	}
	return out[:min(len(out), limit)], nil
}

func (r *warmupCleanupInbox) Delete(_ context.Context, owner, id uuid.UUID) error {
	for _, e := range r.events {
		if e.Message.ID == id && e.UserID != owner {
			panic("cleanup crossed mailbox owners")
		}
	}
	r.deleted = append(r.deleted, id)
	return nil
}
