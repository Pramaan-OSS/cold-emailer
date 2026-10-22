package email

import (
	"context"
	"net/mail"
	"strings"

	"github.com/pramaan-oss/cold-email/internal/pramaan"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/pubsub"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/observability/errs"
	"github.com/pramaan-oss/cold-email/internal/pkg/crypt"
	"github.com/pramaan-oss/cold-email/internal/pkg/displayname"
	"github.com/pramaan-oss/cold-email/internal/utils/validate"
)

// WebStatePrefix marks a state the dashboard started, so a sign-in window that lost its opener returns
// to the dashboard rather than the native app scheme. A base64url nonce never contains '.'.
const WebStatePrefix = "w."

// IsWebState reports whether a callback state belongs to a flow the dashboard started.
func IsWebState(state string) bool {
	return strings.HasPrefix(state, WebStatePrefix)
}

// newState mints a state nonce, marked when the dashboard started the flow.
func newState(web bool) (string, error) {
	n, err := crypt.Nonce()
	if err != nil || !web {
		return n, err
	}
	return WebStatePrefix + n, nil
}

// guardInboxLimit refuses a connect that would take the workspace past its
// mailbox allowance (fair use for paid plans, FreeWorkspaceMailboxLimit for
// free ones, unlimited without billing) and returns the resolved allowance so
// the insert can enforce it again under the organization's lock. The
// allowance is counted per org, so no org means it cannot be applied and the
// connect is refused. Without an allowance source wired, the feature gate's
// free-or-paid split stands in and the insert is not re-checked.
func (s *emailService) guardInboxLimit(ctx context.Context, orgID *uuid.UUID) (*models.MailboxAllowance, *errx.Error) {
	if orgID == nil {
		return nil, errx.ErrNoOrganization
	}
	if s.allowance != nil {
		a, xerr := s.allowance.MailboxAllowance(ctx, *orgID)
		if xerr != nil {
			return nil, xerr
		}
		if a.CanAdd(1) {
			return a, nil
		}
		return nil, errx.MailboxAllowanceReached(a.Used, *a.Allowance, a.Paid)
	}
	if s.featureGate == nil {
		return nil, nil
	}
	count, xerr := s.emailRepository.CountForOrganization(ctx, *orgID)
	if xerr != nil {
		return nil, xerr
	}
	allowed, xerr := s.featureGate.CanAddInbox(ctx, *orgID, count)
	if xerr != nil {
		return nil, xerr
	}
	if allowed {
		return nil, nil
	}
	return nil, errx.MailboxAllowanceReached(count, models.FreeWorkspaceMailboxLimit, false)
}

// OnboardSMTPIMAP validates the supplied SMTP/IMAP credentials against a live worker, then
// persists the email account on success. Returns ErrEmailCredentials if the worker reports failure.
func (s *emailService) OnboardSMTPIMAP(ctx context.Context, userID string, orgID *uuid.UUID, data *models.NewSMTPIMAPAccount) (*models.Email, *errx.Error) {
	if data == nil || !pramaan.OwnedInbox(ctx, data.Email) {
		return nil, errx.New(errx.ServiceUnavailable, "Connect a mailbox through Connections first.")
	}
	ctx, cancel := detach(ctx, connectBudget)
	defer cancel()
	if xerr := validateSMTPIMAPInput(data); xerr != nil {
		return nil, xerr
	}

	allowance, xerr := s.guardInboxLimit(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}

	if existing, xerr := s.findExisting(ctx, userID, orgID, data.Email); xerr != nil {
		return nil, xerr
	} else if existing != nil {
		return nil, errx.ErrEmailOnboardAlreadyExists
	}

	data.OrganizationID = orgID
	data.Allowance = allowance

	acc, xerr := s.emailRepository.NewSMTPIMAPAccount(ctx, userID, *data)
	if xerr != nil {
		return nil, xerr
	}
	ctx, cancelAfter := afterSave(ctx)
	defer cancelAfter()

	// Place the mailbox for real. Failure here is non-fatal: the scheduler
	// picks the account up on its next pass.
	if orgID != nil && s.workerAssignment != nil {
		if _, err := s.workerAssignment.AssignWorkerToEmail(ctx, acc.ID, *orgID); err != nil {
			errs.CaptureException(err)
		}
	}

	s.syncWarmupPoolMembership(ctx, acc)
	s.publishAccountEvent(ctx, pubsub.EventAccountConnected, acc)
	s.dispatchAccountConnected(ctx, orgID, acc)
	// Load the mailbox onto its assigned worker so it starts sending/syncing
	// immediately; the reconciler is the fallback if this fails.
	s.loadAccountBestEffort(ctx, acc.ID)
	return acc, nil
}

// dispatchAccountConnected fires an email_account.connected webhook event
// to any subscribed endpoints. Failures here are best-effort and never
// block the onboarding flow.
func (s *emailService) dispatchAccountConnected(ctx context.Context, orgID *uuid.UUID, acc *models.Email) {
	if s.webhookService == nil || orgID == nil || acc == nil {
		return
	}
	payload := map[string]any{
		"email_account_id": acc.ID,
		"email":            acc.Email,
		"provider":         acc.Provider,
		"name":             acc.Name,
		"created_at":       acc.CreatedAt,
	}
	if _, err := s.webhookService.Dispatch(ctx, *orgID, models.WebhookEventEmailAccountConnected, payload); err != nil {
		errs.CaptureException(err)
	}
}

// oauthConfigured reports whether an OAuth client is actually usable, i.e. both
// halves of the credential are present.

func validateSMTPIMAPInput(data *models.NewSMTPIMAPAccount) *errx.Error {
	if data == nil || data.SMTP == nil || data.IMAP == nil {
		return errx.ErrEmailCredentialsRequired
	}
	data.Email = strings.TrimSpace(data.Email)
	if _, err := mail.ParseAddress(data.Email); err != nil {
		return errx.ErrEmail
	}
	if !validNameLen(&data.Name) {
		return errx.ErrEmailName
	}
	// Replaced rather than refused, so a row of an import file still connects.
	if validate.MailboxNameShown(data.Name) != nil {
		data.Name = displayname.FromEmail(data.Email)
		if len([]rune(data.Name)) < 2 {
			data.Name = "Mailbox"
		}
	}
	if strings.TrimSpace(data.SMTP.Host) == "" {
		return errx.ErrEmailSMTPHost
	}
	if !validPort(data.SMTP.Port) {
		return errx.ErrEmailSMTPPort
	}
	if strings.TrimSpace(data.IMAP.Host) == "" {
		return errx.ErrEmailIMAPHost
	}
	if !validPort(data.IMAP.Port) {
		return errx.ErrEmailIMAPPort
	}
	normalizeMailPasswords(data.SMTP, data.IMAP)
	return validateMailSecurity(data.SMTP, data.IMAP)
}

// normalizeMailPasswords drops every space from a Google app password (Google
// prints them in groups of four) and trims only the edges elsewhere, where an
// inner space can be part of the password.
func normalizeMailPasswords(services ...*models.Service) {
	for _, svc := range services {
		if svc == nil {
			continue
		}
		if models.GoogleMailHost(svc.Host) {
			svc.Password = strings.Join(strings.Fields(svc.Password), "")
			continue
		}
		svc.Password = strings.TrimSpace(svc.Password)
	}
}

// validPort accepts any routable TCP port. Mail submission is conventionally
// 465/587 and IMAP 993/143, but plenty of providers and self-hosted servers
// use 2525, 25, or something else entirely, and the security mode (not the
// port) is what decides how we connect.
func validPort(port int) bool {
	return port > 0 && port <= 65535
}

// validateMailSecurity rejects an unknown security mode. Empty is allowed and
// means "infer from the port", which is how existing clients behave.
//
// "none" carries two extra conditions, because it is the one mode that puts a
// password on an unencrypted socket. It is legal only against a loopback host,
// where the socket never reaches a wire, and only on a self-hosted instance,
// where the worker runs on the operator's own machine. On the hosted product
// the worker is never the customer's machine, so a loopback address there is
// the WORKER's loopback: the mode could not reach the relay it was meant for
// and would only be a way to speak plaintext to whatever answers on that port.
func validateMailSecurity(smtp, imap *models.Service) *errx.Error {
	if smtp.Security != "" && !models.ValidMailSecurity(smtp.Security) {
		return errx.ErrEmailSMTPSecurity
	}
	if imap.Security != "" && !models.ValidMailSecurity(imap.Security) {
		return errx.ErrEmailIMAPSecurity
	}
	if err := validateCleartextHost(smtp.Security, smtp.Host, errx.ErrEmailSMTPSecurityNotLocal, errx.ErrEmailSMTPSecurityHosted); err != nil {
		return err
	}
	return validateCleartextHost(imap.Security, imap.Host, errx.ErrEmailIMAPSecurityNotLocal, errx.ErrEmailIMAPSecurityHosted)
}

// validateCleartextHost is the "none" gate for one leg.
func validateCleartextHost(security, host string, notLocal, hosted *errx.Error) *errx.Error {
	if security != models.MailSecurityNone {
		return nil
	}
	if !config.SelfHosted() {
		return hosted
	}
	if !models.LoopbackMailHost(host) {
		return notLocal
	}
	return nil
}

func validNameLen(name *string) bool {
	*name = strings.TrimSpace(*name)
	if *name == "" {
		return false
	}
	r := []rune(*name)
	return len(r) >= 2 && len(r) <= 100
}

func deriveNameFromEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return email
	}
	local := email[:at]
	if local == "" {
		return email
	}
	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	return strings.Title(local)
}

// inboxOwner is the per-provider user info shape we normalize on.
type inboxOwner struct {
	Email string
	Name  string
}
