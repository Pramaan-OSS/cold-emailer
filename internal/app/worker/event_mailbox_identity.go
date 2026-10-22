package worker

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/observability/errs"
	"github.com/rs/zerolog/log"
)

// mailboxIdentityChannel names the reply channel for one request, matching the
// credential-validation round trip.
func mailboxIdentityChannel(processID string) string {
	return "mailbox_identity:" + processID
}

// identityDeadline bounds the provider call. The backend waits a little longer
// than this, so a slow provider produces our own answer rather than the
// caller's timeout.
const identityDeadline = 8 * time.Second

// HandleMailboxIdentity reads a mailbox's send-as identities from its provider
// and answers on the process channel.
//
// This is here rather than in the backend because it is an account operation:
// the worker is what holds the mailbox's credential and what the provider has
// learned to expect a connection from. Reading a customer's settings from the
// control plane would show Google a second client address for the same
// mailbox, which is what earns a sign-in challenge.
func (w *WorkerService) HandleMailboxIdentity(ctx context.Context, data models.EventWorkerMailboxIdentity) error {
	ctx, cancel := context.WithTimeout(ctx, identityDeadline)
	defer cancel()

	reply := func(res models.MailboxIdentityResult) error {
		payload, err := json.Marshal(res)
		if err != nil {
			errs.CaptureException(err)
			return nil
		}
		if err := w.Cache.Publish(ctx, mailboxIdentityChannel(data.ProcessID.String()), payload).Err(); err != nil {
			errs.CaptureException(err)
		}
		return nil
	}

	mail, exists := w.loadedMailbox(ctx, data.EmailID)
	if !exists {
		// Nothing to answer with: this worker is not running the mailbox. The
		// backend turns the silence-shaped answer into a message that tells
		// the customer to try again.
		log.Warn().Str("email_id", data.EmailID.String()).Msg("Mailbox not loaded; cannot read its sending identity")
		return reply(models.MailboxIdentityResult{Error: "mailbox is not loaded on this worker"})
	}

	return reply(models.MailboxIdentityResult{OK: true, Identities: []models.SendAsIdentity{{Email: mail.Email, Name: strings.TrimSpace(mail.FirstName + " " + mail.LastName), IsDefault: true, IsPrimary: true}}})
}
