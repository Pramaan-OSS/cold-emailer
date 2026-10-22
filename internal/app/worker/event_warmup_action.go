package worker

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/app/worker/wmail"
	"github.com/pramaan-oss/cold-email/internal/client/smtpimap/imap"
	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/repository"
	"github.com/rs/zerolog/log"
)

// HandleWarmupAction executes recipient-side warmup actions on the mailbox.
//
// The worker just runs whatever actions it receives, immediately. The
// recipient-side "dwell" and the immediate-vs-delayed split are now owned by the
// CONSUMER's durable schedule (internal/app/consumer/warmup_engagement_poller):
// the consumer publishes the immediate leg (folder + spam-rescue) right away and
// the delayed leg (read / important / star) when its fire_at passes, each with
// DelaySeconds=0. That makes the dwell survive a worker restart, which the old
// in-process time.AfterFunc here could not.
func (w *WorkerService) HandleWarmupAction(ctx context.Context, action models.WarmupEmailAction) error {
	filingPending := false
	var filingProof *models.WarmupFilingRecoveryProof
	if gate, ok := w.SyncContextRepository.(repository.WarmupActionAdmission); ok {
		worker, err := uuid.Parse(w.ID)
		if err != nil {
			return err
		}
		if recovery, ok := w.SyncContextRepository.(repository.WarmupActionRecoveryAdmission); ok && action.FilingID != "" && len(action.Actions) == 1 && action.Actions[0] == models.WarmupActionFile {
			decision, xerr := recovery.AdmitWarmupAction(ctx, models.WarmupActionRequest{MailboxID: action.EmailID, WorkerID: worker, Actions: action.Actions, FilingID: action.FilingID})
			action.Actions, filingPending, filingProof, err = decision.Actions, decision.FilingPending, decision.FilingRecovery, xerr
			if err == nil {
				filing, perr := uuid.Parse(action.FilingID)
				if filingProof != nil && (perr != nil || !filingPending || len(action.Actions) != 1 || action.Actions[0] != models.WarmupActionFile || !filingProof.ValidFor(action.EmailID, worker, filing)) {
					return errors.New("invalid warmup filing recovery proof")
				}
				if pending, ok := w.pendingFilingDeferrals.Load(filing); ok {
					request := pending.(models.WarmupFilingDeferralRequest)
					if request.MailboxID == action.EmailID && request.WorkerID == worker && len(action.Actions) == 1 && action.Actions[0] == models.WarmupActionFile && (filingProof == nil || filingProof.ProviderRetryAt == nil || filingProof.ProviderRetryAt.Before(request.ProviderRetryAt)) {
						return w.deferWarmupFiling(ctx, request)
					}
					if filingProof != nil {
						w.pendingFilingDeferrals.CompareAndDelete(filing, request)
					}
				}
				if filingProof != nil && filingProof.ProviderRetryAt != nil && filingProof.ProviderRetryAt.After(time.Now()) {
					return nil
				}
			}
		} else {
			action.Actions, err = gate.PermittedWarmupActions(ctx, action.EmailID, worker, action.Actions)
		}
		if err != nil {
			return err
		}
	} else if w.SyncContextRepository != nil {
		return errors.New("warmup action authority unavailable")
	}
	log.Info().
		Str("email_id", action.EmailID.String()).
		Str("gmail_id", action.GmailID).
		Uint32("uid", action.UID).
		Uint32("mailbox_uid_validity", action.MailboxUIDValidity).
		Strs("actions", action.Actions).
		Msg("Processing warmup email action")

	if len(action.Actions) == 0 {
		if filing, err := uuid.Parse(action.FilingID); err == nil {
			w.pendingFilingDeferrals.Delete(filing)
		}
		return nil
	}

	mail, exists := w.loadedMailbox(ctx, action.EmailID)
	if exists {
		if done, ok := mail.BeginExecution(); ok {
			defer done()
			var cancel func()
			ctx, cancel = mail.ExecutionContext(ctx)
			defer cancel()
		} else {
			exists = false
		}
	}
	var err error
	switch {
	case !exists:
		// Engagement on a mailbox this worker is not holding is dropped; it
		// is best effort and a mailbox mid-move earns its signal elsewhere.
		// A retention delete (its row is already retired) and a removal check
		// are redelivered instead, in case the mailbox is still loading.
		log.Warn().Str("email_id", action.EmailID.String()).Msg("Email account not found for warmup action")
		if hasWarmupAction(action.Actions, models.WarmupActionDelete) ||
			hasWarmupAction(action.Actions, models.WarmupActionVerifyRemoval) || action.FilingID != "" {
			err = errors.New("mailbox not loaded on this worker")
		}
	case hasWarmupAction(action.Actions, models.WarmupActionVerifyRemoval):
		err = w.verifyWarmupRemoval(ctx, mail, action)
	case mail.Broker != nil:
		err = w.runBrokerWarmupActions(ctx, mail, action)
	default:
		if action.FilingID != "" {
			err = errors.New("mailbox has no provider client for warmup filing")
		}
		log.Warn().
			Str("email_id", action.EmailID.String()).
			Msg("No mail client available for warmup actions; skipping")
	}
	if err == nil {
		if action.FilingID != "" {
			id, perr := uuid.Parse(action.FilingID)
			if perr != nil {
				return perr
			}
			return w.Produce(models.JobEventTypeWarmupFiled, action.EmailID.String(),
				&models.JobEventWarmupFiled{EmailID: action.EmailID, FilingID: id})
		}
		return nil
	}
	if filingPending && len(action.Actions) == 1 && action.Actions[0] == models.WarmupActionFile {
		if at := warmupProviderRetryAt(err); filingProof != nil && !at.IsZero() {
			request := models.WarmupFilingDeferralRequest{MailboxID: action.EmailID, WorkerID: filingProof.WorkerID, FilingID: filingProof.FilingID, ProviderRetryAt: at}
			if deferErr := w.deferWarmupFiling(ctx, w.retainFilingDeferral(request)); deferErr != nil {
				return deferErr
			}
		}
		// Only the confirmed durable filing queue can take over this failed command.
		log.Warn().Err(err).Str("email_id", action.EmailID.String()).Str("filing_id", action.FilingID).
			Msg("Warmup filing deferred to durable recovery")
		return nil
	}
	// Engagement is best effort and never returns here. A retention delete
	// (the row is already retired) and a removal check are redelivered, a
	// bounded number of times.
	if d := deliveryOf(ctx); (d.redelivers || action.FilingID != "") && d.attempt < warmupDeleteRedeliveries {
		return err
	}
	log.Error().Err(err).
		Str("email_id", action.EmailID.String()).
		Str("rfc_message_id", action.RFCMessageID).
		Strs("actions", action.Actions).
		Msg("Warmup action gave up")
	return nil
}

// warmupDeleteRedeliveries bounds how many times a failed retention delete or
// removal check is redelivered before it is given up.
const warmupDeleteRedeliveries = 5

func warmupProviderRetryAt(err error) time.Time {
	var mailErr *errx.MailError
	if !errors.As(err, &mailErr) || mailErr.Failure == nil || mailErr.Failure.RetryAt == nil {
		return time.Time{}
	}
	return mailErr.Failure.RetryAt.UTC()
}

func (w *WorkerService) retainFilingDeferral(request models.WarmupFilingDeferralRequest) models.WarmupFilingDeferralRequest {
	for {
		old, loaded := w.pendingFilingDeferrals.LoadOrStore(request.FilingID, request)
		if !loaded {
			return request
		}
		prior := old.(models.WarmupFilingDeferralRequest)
		if prior.MailboxID == request.MailboxID && prior.WorkerID == request.WorkerID && prior.ProviderRetryAt.After(request.ProviderRetryAt) {
			return prior
		}
		if w.pendingFilingDeferrals.CompareAndSwap(request.FilingID, prior, request) {
			return request
		}
	}
}

func (w *WorkerService) deferWarmupFiling(ctx context.Context, request models.WarmupFilingDeferralRequest) error {
	authority, ok := w.SyncContextRepository.(repository.WarmupFilingDeferralAuthority)
	if !ok {
		return errors.New("warmup filing deferral authority unavailable")
	}
	out, err := authority.DeferWarmupFiling(ctx, request)
	if err != nil {
		return err
	}
	if !out.Persisted || !out.FilingRecovery.ValidFor(request.MailboxID, request.WorkerID, request.FilingID) || out.FilingRecovery.ProviderRetryAt == nil || out.FilingRecovery.ProviderRetryAt.Before(request.ProviderRetryAt) {
		return errors.New("unconfirmed warmup filing deferral")
	}
	w.pendingFilingDeferrals.CompareAndDelete(request.FilingID, request)
	return nil
}

func lookupMailboxByUIDValidity(boxes []*models.Mailbox, uidValidity uint32) *models.Mailbox {
	for _, b := range boxes {
		if b != nil && b.UIDValidity == uidValidity {
			return b
		}
	}
	return nil
}

func lookupInbox(boxes []*models.Mailbox) *models.Mailbox {
	for _, b := range boxes {
		if b != nil && imap.IsInboxMailbox(b.Name, b.Attrs) {
			return b
		}
	}
	return nil
}

func lookupSent(boxes []*models.Mailbox) *models.Mailbox {
	for _, b := range boxes {
		if b != nil && imap.IsSentMailbox(b.Name, b.Attrs) {
			return b
		}
	}
	return nil
}

func lookupArchive(boxes []*models.Mailbox) *models.Mailbox {
	for _, b := range boxes {
		if b != nil && imap.IsArchiveMailbox(b.Name, b.Attrs) {
			return b
		}
	}
	return nil
}

// warmupFiling resolves where the filing action should put the message, from
// what the control plane sent with the action. An event published before these
// fields existed carries neither, and the default folder is exactly what every
// mailbox did then, so an in-flight event behaves the same either way.
func warmupFiling(action models.WarmupEmailAction) (placement, folder string) {
	placement = action.Placement
	if !models.ValidWarmupPlacement(placement) {
		placement = models.WarmupPlacementFolder
	}
	folder = strings.TrimSpace(action.TargetFolder)
	if folder == "" {
		folder = config.WarmupFolderDefault
	}
	return placement, folder
}

func (w *WorkerService) runBrokerWarmupActions(ctx context.Context, mail *wmail.WMail, action models.WarmupEmailAction) error {
	id := action.RFCMessageID
	if id == "" {
		id = action.GmailID
	}
	if id == "" {
		return errors.New("Select a message before changing it.")
	}
	for _, act := range action.Actions {
		if act == models.WarmupActionDelete {
			if e := mail.DeleteMessage(ctx, id, "warmup-delete-"+action.FilingID+"-"+id); e != nil {
				return e
			}
			continue
		}
		add, remove := []string{}, []string{}
		switch act {
		case models.WarmupActionFile:
			_, folder := warmupFiling(action)
			add = append(add, folder)
			remove = append(remove, "inbox")
		case models.WarmupActionMarkRead:
			remove = append(remove, "unread")
		case models.WarmupActionRescueFromSpam:
			add = append(add, "inbox")
			remove = append(remove, "spam")
		case models.WarmupActionMarkImportant:
			add = append(add, "important")
		case models.WarmupActionStar:
			add = append(add, "starred")
		default:
			return errors.New("This mailbox action is not available yet.")
		}
		if e := mail.ChangeLabels(ctx, id, add, remove, "warmup-action-"+action.FilingID+"-"+id+"-"+act); e != nil {
			return e
		}
	}
	return nil
}

func hasWarmupAction(actions []string, want string) bool {
	for _, a := range actions {
		if a == want {
			return true
		}
	}
	return false
}

func lookupWarmupSourceFolder(boxes []*models.Mailbox, action models.WarmupEmailAction) *models.Mailbox {
	if action.MailboxFolder == "" {
		return lookupMailboxByUIDValidity(boxes, action.MailboxUIDValidity)
	}
	for _, b := range boxes {
		if b != nil && b.Name == action.MailboxFolder && b.UIDValidity == action.MailboxUIDValidity {
			return b
		}
	}
	return nil
}
