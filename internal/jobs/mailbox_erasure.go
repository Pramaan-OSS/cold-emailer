package jobs

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/pramaan-oss/cold-email/internal/pramaan"

	"github.com/rs/zerolog/log"

	"github.com/pramaan-oss/cold-email/internal/infrastructure/storage"
	"github.com/pramaan-oss/cold-email/internal/jobrun"
	"github.com/pramaan-oss/cold-email/internal/observability/errs"
	"github.com/pramaan-oss/cold-email/internal/pkg/encrypt"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

// MailboxErasureJob finishes what deleting a mailbox starts.
//
// The delete removes the rows and returns; two things it cannot do in a
// transaction are left written down in mailbox_erasures, and this works them
// off:
//
//   - hand the OAuth grant back to the provider, so ColdEmail stops appearing in
//     the customer's connected apps and no copy of the token is useful to
//     anyone. Deleting our copy is not revocation.
//   - delete the message bodies the mailbox synced into object storage. The
//     unibox rows cascade with the mailbox; the bytes they point at do not.
//
// It runs on a short interval rather than a nightly sweep because "delete my
// data" is not a request to answer tomorrow, and it is durable rather than a
// goroutine because a restart between the two halves would otherwise leave a
// live Gmail grant and a bucket full of somebody's mail with nothing recording
// that either was owed.
type MailboxErasureJob struct {
	repo  repository.MailboxErasureRepository
	blobs storage.Store
	// enc opens the sealed refresh token copied off the mailbox. Without it
	// tokens cannot be revoked; bodies are still erased, and the erasure stays
	// outstanding so the failure is visible rather than silent.
	enc       *encrypt.Encrypter
	broker    pramaan.Caller
	resources pramaan.Registry
}

func NewMailboxErasureJob(repo repository.MailboxErasureRepository, blobs storage.Store, enc *encrypt.Encrypter) *MailboxErasureJob {
	return &MailboxErasureJob{repo: repo, blobs: blobs, enc: enc}
}

const (
	// erasureBatch is how many mailboxes one pass works. Bounded because each
	// is a provider round trip plus a prefix walk.
	erasureBatch = 50
	// erasureLease is how long a claimed row is held. Comfortably longer than
	// a prefix walk over a large mailbox, short enough that a backend killed
	// mid-pass releases its work within one interval of the next one starting.
	erasureLease = 10 * time.Minute
	// erasureBatchBudget is how long one pass will keep working. Half the
	// lease, so a pass always finishes with the claim it started with.
	erasureBatchBudget = erasureLease / 2
	// erasureRetryBase and erasureRetryMax bound the backoff. A provider
	// outage should not be retried every minute for an hour, and an erasure
	// must never stop being retried: an outstanding one is a live grant on
	// somebody's mailbox.
	erasureRetryBase = 2 * time.Minute
	erasureRetryMax  = 6 * time.Hour
	// erasureStaleAfter is when an outstanding erasure stops being ordinary
	// in-flight work and starts being something an operator should look at.
	erasureStaleAfter = 24 * time.Hour
	// erasureAlertAfterAttempts is when repeated failure is raised to error
	// tracking rather than only logged. Late enough that a provider blip does
	// not page anyone, early enough to beat erasureStaleAfter.
	erasureAlertAfterAttempts = 5
)

// Run works one batch. Individual failures are recorded against their row and
// do not stop the others: one unreachable provider must not hold up erasing
// everybody else's data.
func (j *MailboxErasureJob) Run(ctx context.Context) error {
	if j.repo == nil {
		return nil
	}

	due, err := j.repo.Claim(ctx, erasureBatch, erasureLease)
	if err != nil {
		return fmt.Errorf("claim erasures: %w", err)
	}

	// Stop well inside the lease. Each mailbox is a provider round trip plus a
	// prefix walk, so a full batch of slow ones could otherwise still be
	// running when the lease expires and another backend claims the same rows.
	// Whatever is left keeps its claim and is picked up on the next tick.
	deadline := time.Now().Add(erasureBatchBudget)
	for i := range due {
		// A cancelled context is a shutdown, not a failed erasure. Recording
		// failures here would burn retry attempts and widen the backoff for
		// work that was never actually attempted.
		if ctx.Err() != nil {
			return nil
		}
		if time.Now().After(deadline) {
			log.Info().Int("remaining", len(due)-i).Msg("mailbox erasure: batch budget reached, resuming next tick")
			break
		}
		j.erase(ctx, &due[i])
	}

	// Reported as the job's error so it reaches the admin panel's job list and
	// error tracking, after the batch rather than instead of it.
	stale, err := j.repo.PendingOlderThan(ctx, erasureStaleAfter)
	if err != nil {
		return fmt.Errorf("count outstanding erasures: %w", err)
	}
	if stale > 0 {
		return fmt.Errorf("%d mailbox erasures still outstanding after %s", stale, erasureStaleAfter)
	}
	return nil
}

// erase performs whichever halves are still owed for one mailbox, then clears
// the row when both are done.
func (j *MailboxErasureJob) erase(ctx context.Context, e *repository.MailboxErasure) {
	var failures []error

	// Each half is skipped when an earlier pass already did it.
	revokedNow := false
	if e.TokenRevokedAt == nil {
		if err := j.revoke(ctx, e); err != nil {
			failures = append(failures, err)
		} else {
			revokedNow = true
		}
	}

	erasedNow := false
	if e.BlobsErasedAt == nil {
		if err := j.eraseBlobs(ctx, e); err != nil {
			failures = append(failures, err)
		} else {
			erasedNow = true
		}
	}

	if len(failures) == 0 {
		// Both halves are done, so the row goes. Stamping them first would be
		// two writes to a row this is about to delete.
		if err := j.repo.Complete(ctx, e.EmailAccountID); err != nil {
			log.Error().Err(err).Str("email_account_id", e.EmailAccountID.String()).Msg("mailbox erased but the queue row could not be cleared")
			return
		}
		log.Info().
			Str("email_account_id", e.EmailAccountID.String()).
			Str("provider", e.Provider).
			Msg("mailbox erased: grant revoked and stored mail removed")
		return
	}

	// Only now are the stamps worth writing: they exist so the retry does not
	// repeat the half that already worked. A stamp that fails to persist is
	// not itself a failure of the erasure; the worst case is doing that half
	// again, and revoking twice or walking an empty prefix are both harmless.
	if revokedNow {
		if err := j.repo.MarkTokenRevoked(ctx, e.EmailAccountID); err != nil {
			log.Warn().Err(err).Str("email_account_id", e.EmailAccountID.String()).Msg("could not record a completed revocation")
		}
	}
	if erasedNow {
		if err := j.repo.MarkBlobsErased(ctx, e.EmailAccountID); err != nil {
			log.Warn().Err(err).Str("email_account_id", e.EmailAccountID.String()).Msg("could not record a completed blob erasure")
		}
	}

	cause := errors.Join(failures...)
	retryAt := time.Now().Add(retryDelay(e.Attempts))
	log.Warn().Err(cause).
		Str("email_account_id", e.EmailAccountID.String()).
		Int("attempts", e.Attempts+1).
		Time("retry_at", retryAt).
		Msg("mailbox erasure incomplete")
	// An erasure that keeps failing is a customer whose data we said we
	// deleted and did not, so it is raised rather than only logged.
	if e.Attempts+1 >= erasureAlertAfterAttempts {
		errs.CaptureExceptionContext(ctx, cause,
			errs.Tag("email_account_id", e.EmailAccountID.String()),
			errs.Tag("provider", e.Provider))
	}
	if err := j.repo.Fail(ctx, e.EmailAccountID, cause.Error(), retryAt); err != nil {
		log.Error().Err(err).Str("email_account_id", e.EmailAccountID.String()).Msg("could not record a failed mailbox erasure")
	}
}

// revoke opens the sealed token and hands the grant back.
func (j *MailboxErasureJob) revoke(ctx context.Context, e *repository.MailboxErasure) error {
	owner := e.PramaanOwner
	if owner == "" {
		if e.OrganizationID != nil {
			owner = e.OrganizationID.String()
		} else {
			owner = e.UserID.String()
		}
	}
	if j.broker != nil && j.resources != nil {
		native, err := j.resources.Owns(ctx, owner, "inbox_native", e.Email)
		if err != nil {
			return err
		}
		if native {
			if err := j.broker.Call(ctx, "email.inbox.delete", map[string]any{"address": e.Email}, nil, pramaan.Idempotency("erase-"+e.EmailAccountID.String())); err != nil {
				return fmt.Errorf("revoke mailbox access: %w", err)
			}
			if err := j.resources.Remove(ctx, owner, "inbox", e.Email); err != nil {
				return err
			}
			return j.resources.Remove(ctx, owner, "inbox_native", e.Email)
		}
		owned, err := j.resources.Owns(ctx, owner, "inbox", e.Email)
		if err != nil {
			return err
		}
		if owned {
			var accounts struct {
				Results []struct {
					Ref   string `json:"ref"`
					Inbox string `json:"inbox"`
				} `json:"results"`
			}
			if err := j.broker.Call(ctx, "email.accounts.list", map[string]any{"endUserRef": "org_" + owner, "limit": 50}, &accounts); err != nil {
				return err
			}
			for _, a := range accounts.Results {
				if a.Inbox == e.Email {
					if err := j.broker.Call(ctx, "email.account.disconnect", map[string]any{"ref": a.Ref, "endUserRef": "org_" + owner, "confirm": true}, nil, pramaan.Idempotency("erase-"+e.EmailAccountID.String())); err != nil {
						return err
					}
					if err := j.resources.Remove(ctx, owner, "account", a.Ref); err != nil {
						return err
					}
					return j.resources.Remove(ctx, owner, "inbox", e.Email)
				}
			}
			return fmt.Errorf("revoke mailbox access: the connected mailbox was not found")
		}
	}
	if e.RefreshToken != "" {
		return fmt.Errorf("revoke mailbox access: imported direct grants are unavailable; connect the mailbox through Pramaan first")
	}
	return nil
}

func (j *MailboxErasureJob) WithPramaan(client pramaan.Caller, resources pramaan.Registry) *MailboxErasureJob {
	j.broker = client
	j.resources = resources
	return j
}

func (j *MailboxErasureJob) eraseBlobs(ctx context.Context, e *repository.MailboxErasure) error {
	if e.BlobPrefix == "" {
		return nil
	}
	if j.blobs == nil {
		return errors.New("no blob store wired: stored message bodies cannot be erased")
	}

	n, err := j.blobs.DeletePrefix(ctx, e.BlobPrefix)
	if err != nil {
		return fmt.Errorf("erase stored mail under %q: %w", e.BlobPrefix, err)
	}
	log.Info().
		Str("email_account_id", e.EmailAccountID.String()).
		Int("objects", n).
		Msg("stored mail erased")
	return nil
}

// retryDelay doubles from erasureRetryBase up to erasureRetryMax.
func retryDelay(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	if attempts > 12 {
		return erasureRetryMax
	}
	d := time.Duration(float64(erasureRetryBase) * math.Pow(2, float64(attempts)))
	if d > erasureRetryMax {
		return erasureRetryMax
	}
	return d
}

// Start runs the job on boot and then on the interval until ctx ends.
func (j *MailboxErasureJob) Start(ctx context.Context, interval time.Duration) {
	jobrun.Loop(ctx, "mailbox_erasure", interval, true, j.Run)
}
