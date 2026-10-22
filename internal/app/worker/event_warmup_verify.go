package worker

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/pramaan-oss/cold-email/internal/app/worker/wmail"
	"github.com/pramaan-oss/cold-email/internal/models"
)

// verifyWarmupRemoval reports where a warmup message the sync saw removed is.
func (w *WorkerService) verifyWarmupRemoval(ctx context.Context, mail *wmail.WMail, action models.WarmupEmailAction) error {
	outcome, err := locateWarmupMessage(ctx, mail, action.RFCMessageID)
	if err != nil {
		return fmt.Errorf("verify warmup removal: %w", err)
	}
	log.Debug().
		Str("email_id", action.EmailID.String()).
		Str("rfc_message_id", action.RFCMessageID).
		Str("outcome", outcome).
		Msg("Checked where a removed warmup message went")
	return w.Produce(models.JobEventTypeWarmupRemovalChecked, action.EmailID.String(), &models.JobEventWarmupRemovalChecked{
		UserID:       action.UserID,
		EmailID:      action.EmailID,
		RFCMessageID: action.RFCMessageID,
		Outcome:      outcome,
		Recheck:      action.Recheck,
	})
}

// locateWarmupMessage searches the whole mailbox by Message-ID; trashed means
// every copy found is in the trash.
func locateWarmupMessage(ctx context.Context, mail *wmail.WMail, id string) (string, error) {
	found, trashed, e := mail.LocateMessage(ctx, id)
	return removalOutcome(found, trashed, e)
}

func removalOutcome(found, trashed bool, err error) (string, error) {
	switch {
	case err != nil:
		return "", err
	case !found:
		return models.WarmupRemovalGone, nil
	case trashed:
		return models.WarmupRemovalTrashed, nil
	}
	return models.WarmupRemovalPresent, nil
}

// imapMessageLocator is the slice of the IMAP client the search needs.
type imapMessageLocator interface {
	LocateMessageID(ctx context.Context, mailboxes []string, rfcMessageID string) (held []string, unsearched int, err error)
}

// locateImapMessage searches every synced folder. Not found, or found only in
// the trash with a folder left unsearched, cannot be told apart from a move
// into a folder the sync does not list.
