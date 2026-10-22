package mailboximport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/observability/errs"
	"github.com/pramaan-oss/cold-email/internal/pkg/mailcause"
	"github.com/pramaan-oss/cold-email/internal/pkg/mailhost"
	"github.com/pramaan-oss/cold-email/internal/pkg/textcheck"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

// maxAttempts is how many times a row is claimed before a crash loop is called a failure.
const maxAttempts = 3

// Kick wakes the runner now instead of at its next tick.
func (s *Service) Kick() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// Start runs the import loop until ctx ends, plus the slower upkeep loops. It
// returns only once the pass in progress has finished or handed back its rows.
func (s *Service) Start(ctx context.Context) {
	defer close(s.stopped)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		s.pass(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.kick:
		}
	}
}

// pass works claimed rows until none are left, then closes finished imports.
// Once ctx ends (a deploy or restart) nothing new is claimed, a row still
// waiting for a slot is handed back at once, and rows already connecting finish
// on a context the shutdown does not cancel, so a deploy loses no work.
func (s *Service) pass(ctx context.Context) {
	work := context.WithoutCancel(ctx)
	lease := time.Duration(config.MailboxImportLeaseSeconds) * time.Second
	hostSlots := map[string]chan struct{}{}
	var hostMu sync.Mutex
	slot := func(host string) chan struct{} {
		hostMu.Lock()
		defer hostMu.Unlock()
		ch, ok := hostSlots[host]
		if !ok {
			ch = make(chan struct{}, config.MailboxImportPerHostConcurrency)
			hostSlots[host] = ch
		}
		return ch
	}

	for ctx.Err() == nil {
		// Claim only what can start now, so no lease runs out while a row waits for a slot.
		rows, err := s.repo.Claim(ctx, config.MailboxImportConcurrency, lease)
		if err != nil || len(rows) == 0 {
			break
		}
		var wg sync.WaitGroup
		sem := make(chan struct{}, config.MailboxImportConcurrency)
		for _, w := range rows {
			wg.Add(1)
			go func(w repository.ImportWorkRow) {
				defer wg.Done()
				defer func() {
					if rec := recover(); rec != nil {
						errs.CaptureException(fmt.Errorf("mailbox import row panic: %v", rec))
						s.finish(work, w, models.ImportRowFailed, causeInternal, causeInternal, causeInfo(causeInternal).Title, nil, true)
					}
				}()
				sem <- struct{}{}
				defer func() { <-sem }()
				hs := slot(w.MailHost)
				hs <- struct{}{}
				defer func() { <-hs }()
				if ctx.Err() != nil {
					_ = s.repo.Release(work, w.ImportID, w.Line, w.Attempts)
					return
				}
				// The lease starts now that the row has a slot; another replica may have taken it meanwhile.
				if ok, err := s.repo.Touch(work, w.ImportID, w.Line, w.Attempts, lease); err != nil || !ok {
					return
				}
				s.process(work, w)
			}(w)
		}
		wg.Wait()
		s.completeFinished(work)
	}
	s.completeFinished(work)
}

// Drain waits until the runner has stopped (ctx for Start ended and the pass in
// progress finished its rows, or handed back the ones it never started), or
// until ctx ends; false when ctx ended first.
func (s *Service) Drain(ctx context.Context) bool {
	select {
	case <-s.stopped:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *Service) completeFinished(ctx context.Context) {
	done, err := s.repo.CompleteFinished(ctx, config.MailboxImportCredentialDays)
	if err != nil {
		return
	}
	for _, f := range done {
		s.forgetDomainSetup(f.ID)
		s.publish(ctx, f.OrgID, f.ID, models.ImportCompleted, true)
		if f.CreatedBy != nil {
			s.audit(ctx, f.OrgID, *f.CreatedBy, models.AuditActionUpdate, f.ID, map[string]string{"status": models.ImportCompleted})
		}
	}
}

// process connects, updates or parks one row, and records the outcome.
func (s *Service) process(ctx context.Context, w repository.ImportWorkRow) {
	ctx = textcheck.WithUsage(ctx, "mailbox_import_errors", w.OrgID.String())
	rowCtx, cancel := context.WithTimeout(ctx, time.Duration(config.MailboxImportLeaseSeconds-10)*time.Second)
	defer cancel()

	if w.Attempts > maxAttempts {
		s.finish(ctx, w, models.ImportRowFailed, causeInternal, causeInternal, "This row stopped the import worker more than once.", nil, true)
		return
	}
	if w.CreatedBy == nil {
		s.finish(ctx, w, models.ImportRowFailed, causeCreatorRemoved, causeCreatorRemoved, causeInfo(causeCreatorRemoved).Title, nil, true)
		return
	}
	ciph, err := s.cipher.Cipher(rowCtx, w.OrgID)
	if err != nil {
		errs.CaptureException(err)
		s.finish(ctx, w, models.ImportRowFailed, causeInternal, causeInternal, causeInfo(causeInternal).Title, nil, true)
		return
	}
	p, xerr := unseal(rowCtx, ciph, w.Payload)
	if xerr != nil {
		s.finish(ctx, w, models.ImportRowFailed, causeInternal, causeInternal, causeInfo(causeInternal).Title, nil, true)
		return
	}
	p.email, p.importID = w.Email, w.ImportID
	var batch models.MailboxImportSettings
	_ = json.Unmarshal(w.Settings, &batch)
	settings := mergeSettings(batch, p.Settings)
	userID := w.CreatedBy.String()
	orgID := w.OrgID

	existing, xerr := s.mailboxes.FindInOrganization(rowCtx, orgID, w.Email)
	if xerr != nil {
		s.finish(ctx, w, models.ImportRowFailed, causeInternal, causeInternal, causeInfo(causeInternal).Title, nil, true)
		return
	}

	// A restart between creating this mailbox and recording the row leaves it for a
	// later claim: finish the row's work rather than treat the mailbox as pre-existing.
	if existing != nil && w.Attempts > 1 {
		acc, xerr := s.mailboxes.Get(rowCtx, orgID.String(), existing.ID.String())
		ours := w.AccountID != nil && existing.ID == *w.AccountID
		if xerr == nil && acc != nil && (ours || acc.CreatedAt.After(w.ImportCreatedAt)) {
			s.connected(ctx, w, p, acc, settings, userID)
			return
		}
	}

	if existing != nil && w.OnExisting == "skip" {
		s.finish(ctx, w, models.ImportRowSkipped, "already_connected", "", "Already in this workspace.", &existing.ID, false)
		return
	}

	// A vendor row fixed with server settings alone still takes its password from the vendor.

	// A vendor row learns its credentials only now, and is judged like a file row.

	// A row waiting on sign-in connects through a grant that now covers its domain.
	if p.Signin && existing == nil && p.GrantID == nil {
		if id := s.grantForHost(rowCtx, orgID, p.MailHost, w.Email); id != nil {
			p.GrantID, p.Signin, p.AuthMethod = id, false, models.MailAuthDelegated
		}
	}

	if p.GrantID != nil {
		// Connect relinks an existing grant mailbox and moves a per-mailbox
		// sign-in onto the grant; anything else keeps its credentials.
		if existing != nil && existing.AuthMethod != models.MailAuthDelegated && existing.AuthMethod != models.MailAuthOAuth {
			note := s.applySettings(rowCtx, orgID, userID, existing.ID, p, settings, true)
			s.finish(ctx, w, models.ImportRowUpdated, "", "", joinNote("Already connected; its settings were updated.", note), &existing.ID, false)
			return
		}
		if s.delegator == nil {
			s.finish(ctx, w, models.ImportRowFailed, causeInternal, causeInternal, causeInfo(causeInternal).Title, nil, true)
			return
		}
		acc, xerr := s.delegator.Connect(rowCtx, orgID, userID, *p.GrantID, w.Email, p.Name)
		if xerr != nil {
			if errors.Is(xerr, errx.ErrEmailOnboardAlreadyExists) {
				s.finish(ctx, w, models.ImportRowSkipped, "already_connected", "", "Already in this workspace.", nil, false)
				return
			}
			s.fail(ctx, w, xerr)
			return
		}
		s.connected(ctx, w, p, acc, settings, userID)
		return
	}

	if p.Signin {
		if existing == nil {
			cause := causeMicrosoftSignin
			if mailhost.Host(p.MailHost).Google() {
				cause = causeGoogleSignin
			}
			s.finish(ctx, w, models.ImportRowNeedsSignin, cause, cause, "Waiting for someone to sign in as this mailbox.", nil, true)
			return
		}
		note := s.applySettings(rowCtx, orgID, userID, existing.ID, p, settings, false)
		s.finish(ctx, w, models.ImportRowUpdated, "", "", joinNote("Already connected; its settings were updated.", note), &existing.ID, false)
		return
	}

	if existing != nil {
		if models.InboxProvider(existing.Provider) != models.InboxProviderSMTPIMAP {
			note := s.applySettings(rowCtx, orgID, userID, existing.ID, p, settings, true)
			s.finish(ctx, w, models.ImportRowUpdated, "", "",
				joinNote("Signs in with Google or Microsoft, so only its settings were updated.", note), &existing.ID, false)
			return
		}
		creds := &models.SmtpImap{SMTP: p.SMTP, IMAP: p.IMAP}
		if xerr := retryUnanswered(rowCtx, func() *errx.Error {
			_, xerr := s.emails.UpdateSMTPIMAPCredentials(rowCtx, &orgID, existing.ID, creds)
			return xerr
		}); xerr != nil {
			s.fail(ctx, w, xerr)
			return
		}
		s.link(ctx, p, existing.ID)
		note := s.applySettings(rowCtx, orgID, userID, existing.ID, p, settings, true)
		s.finish(ctx, w, models.ImportRowUpdated, "", "", joinNote("Password and settings updated.", note), &existing.ID, false)
		return
	}

	var acc *models.Email
	xerr = retryUnanswered(rowCtx, func() *errx.Error {
		var xerr *errx.Error
		acc, xerr = s.emails.OnboardSMTPIMAP(rowCtx, userID, &orgID, &models.NewSMTPIMAPAccount{
			Email: w.Email, Name: p.Name, SMTP: p.SMTP, IMAP: p.IMAP, MailHost: p.MailHost, AuthMethod: p.AuthMethod,
		})
		return xerr
	})
	if xerr != nil {
		if errors.Is(xerr, errx.ErrEmailOnboardAlreadyExists) {
			// A teammate connected it between the check and the insert.
			s.finish(ctx, w, models.ImportRowSkipped, "already_connected", "", "Already in this workspace.", nil, false)
			return
		}
		s.fail(ctx, w, xerr)
		return
	}
	s.connected(ctx, w, p, acc, settings, userID)
}

// unansweredBackoff is the first wait before a connect no worker answered is tried again; it triples.
var unansweredBackoff = 5 * time.Second

// unansweredReserve is what one more connect needs of the row's lease: two silent workers, then saving the mailbox.
var unansweredReserve = 48 * time.Second

// retryUnanswered repeats a connect whose check no worker answered, while the row's lease leaves room.
func retryUnanswered(ctx context.Context, connect func() *errx.Error) *errx.Error {
	for wait := unansweredBackoff; ; wait *= 3 {
		xerr := connect()
		if xerr == nil || !unanswered(xerr) {
			return xerr
		}
		if d, ok := ctx.Deadline(); ok && time.Until(d) < wait+unansweredReserve {
			return xerr
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return xerr
		case <-t.C:
		}
	}
}

// unanswered is a connect that never reached the mail server because no worker took or answered the check.
func unanswered(xerr *errx.Error) bool {
	return errors.Is(xerr, errx.ErrEmailOnboardNoWorker) || (xerr.Cause == "" && xerr.Identifier == errx.ErrEmailValidation.Identifier)
}

// connected records a new mailbox: audit, vendor link, settings, row outcome.
func (s *Service) connected(ctx context.Context, w repository.ImportWorkRow, p payload, acc *models.Email, settings models.MailboxImportSettings, userID string) {
	// Recorded first, so a restart before the row is finished resumes here instead of skipping the mailbox.
	if w.AccountID == nil || *w.AccountID != acc.ID {
		_ = s.repo.SetRowAccount(ctx, w.ImportID, w.Line, w.Attempts, acc.ID)
	}
	if s.auditor != nil {
		s.auditor.LogAction(ctx, w.OrgID, *w.CreatedBy, models.AuditActionConnect, models.AuditEntityEmailAccount, &acc.ID, "", "", nil,
			map[string]string{"provider": acc.Provider, "email": acc.Email, "import_id": w.ImportID.String()})
	}
	s.link(ctx, p, acc.ID)
	note := s.applySettings(ctx, w.OrgID, userID, acc.ID, p, settings, false)
	s.finish(ctx, w, models.ImportRowConnected, "", "", note, &acc.ID, false)
}

// domainExtras applies the review step's tracking host and root redirect for the row's domain.
func (s *Service) domainExtras(ctx context.Context, w repository.ImportWorkRow, p payload, accountID uuid.UUID) string {
	var notes []string
	domain := domainOf(w.Email)
	if p.TrackingDomain != "" {
		// Once per import and domain, the vendor holding the domain writes the CNAME itself.
		if s.domains != nil {
			if _, done := s.redirected.LoadOrStore(w.ImportID.String()+"\x00tracking\x00"+domain, true); !done {
				s.domains.AutoTracking(ctx, w.OrgID, domain, p.TrackingDomain)
			}
		}
		if st, xerr := s.emails.UpdateTrackingDomain(ctx, w.OrgID.String(), accountID.String(), p.TrackingDomain); xerr != nil {
			notes = append(notes, "Tracking domain not set: "+xerr.Message)
		} else if !st.TrackingDomainVerified {
			notes = append(notes, "Tracking domain "+p.TrackingDomain+" is waiting for its DNS record.")
		}
	}
	if p.RedirectURL != "" && s.domains != nil && w.CreatedBy != nil {
		if _, done := s.redirected.LoadOrStore(w.ImportID.String()+"\x00"+domain, true); !done {
			if xerr := s.domains.AutoRedirect(ctx, w.OrgID, *w.CreatedBy, domain, p.RedirectURL); xerr != nil {
				// A later row or a retry tries again.
				s.redirected.Delete(w.ImportID.String() + "\x00" + domain)
				notes = append(notes, "Redirect not set up: "+xerr.Message)
			}
		}
	}
	return strings.Join(notes, " ")
}

// link remembers which vendor mailbox a mailbox came from.
func (s *Service) link(ctx context.Context, p payload, accountID uuid.UUID) {

}

// authorizeVendorDomain asks the row's vendor to authorize ColdEmail on its domain; signinCause names the provider.

// grantForHost is the workspace's grant covering an address on a Google or Microsoft host, nil when none.
func (s *Service) grantForHost(ctx context.Context, orgID uuid.UUID, mailHost, email string) *uuid.UUID {
	if s.delegator == nil {
		return nil
	}
	provider := ""
	switch h := mailhost.Host(mailHost); {
	case h.Google():
		provider = models.GrantProviderGoogle
	case h.Microsoft():
		provider = models.GrantProviderMicrosoft
	default:
		return nil
	}
	g, err := s.delegator.GrantFor(ctx, orgID, provider, domainOf(email))
	if err != nil || g == nil {
		return nil
	}
	return &g.ID
}

// resumeGrantedSignins queues again the rows waiting on sign-in whose domain an
// administrator's grant now covers, so one admin approval finishes all of them.
func (s *Service) resumeGrantedSignins(ctx context.Context) {
	if s.delegator == nil {
		return
	}
	rows, err := s.repo.CoveredSigninRows(ctx, 1000)
	if err != nil || len(rows) == 0 {
		return
	}
	type key struct {
		org              uuid.UUID
		provider, domain string
	}
	usable := map[key]bool{}
	resumed := false
	for _, w := range rows {
		provider := models.GrantProviderGoogle
		if w.Code == causeMicrosoftSignin {
			provider = models.GrantProviderMicrosoft
		}
		k := key{w.OrgID, provider, domainOf(w.Email)}
		ok, seen := usable[k]
		if !seen {
			// GrantFor also answers nil when this instance no longer has the provider configured,
			// which a stored grant alone cannot tell; resuming then would only park the row again.
			g, err := s.delegator.GrantFor(ctx, w.OrgID, provider, k.domain)
			ok = err == nil && g != nil
			usable[k] = ok
		}
		if !ok {
			continue
		}
		if err := s.repo.ResumeParked(ctx, w.ImportID, w.Line, w.Code); err == nil {
			resumed = true
		}
	}
	if resumed {
		s.Kick()
	}
}

// authorizingMessage tells the person watching a parked row who is doing what, and how long it can take.

// resumeVendorAuthorizations requeues rows parked on a vendor authorization once
// it has an answer, asking the vendor once per domain.

// resolveVendorRow fetches a vendor row's credentials and builds it the way a
// file row is built, so detection, grants and validation all apply.

// fail records a connect that did not go through, under the most specific cause known.
func (s *Service) fail(ctx context.Context, w repository.ImportWorkRow, xerr *errx.Error) {
	code := xerr.ResponseCode()
	cause := xerr.Cause
	switch {
	case code == "mailbox_allowance_reached":
		cause = causeAllowanceReached
	case errors.Is(xerr, errx.ErrEmailOnboardNoWorker):
		cause = causeNoWorker
	case cause == "" && importCauses[code].Key != "":
		cause = code
	case cause == "" && xerr.Code == errx.Internal:
		cause = causeInternal
	case cause == "" && code == errx.ErrEmailValidation.Identifier:
		// No verdict at all: the worker never answered, which says nothing about the mail server.
		cause = causeNoWorker
	case cause == "" && xerr.Code == errx.BadRequest:
		cause = causeInvalidValue
	case cause == "":
		cause = mailcause.ServerDeclined
	}
	if mailcause.Generic(cause) && xerr.Detail != "" {
		cause = s.refineCause(ctx, cause, w.MailHost, xerr.Detail)
	}
	msg := xerr.Message
	if xerr.Code == errx.Internal {
		msg = causeInfo(causeInternal).Title
	}
	s.finish(ctx, w, models.ImportRowFailed, code, cause, msg, nil, true)
}

// refineCause asks Jev which known cause an unrecognized server reply means.
// It sees the scrubbed reply and the host family only.
func (s *Service) refineCause(ctx context.Context, fallback, host, detail string) string {
	if s.asker == nil {
		return fallback
	}
	key := host + "\x00" + detail
	if v, ok := s.causes.Load(key); ok {
		return v.(string)
	}
	criteria := map[string]string{}
	for _, k := range mailcause.Keys() {
		c, _ := mailcause.Lookup(k)
		criteria[k] = c.Title
	}
	askCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := s.asker.Ask(textcheck.WithUsage(askCtx, "mailbox_import_errors", ""), map[string]string{"server_reply": detail, "mail_host": host}, map[string]textcheck.Question{
		"cause": textcheck.Choice("A mail server refused an app's sign-in with state.server_reply. Which of these is the reason?", criteria),
	})
	out := fallback
	if err == nil && resp != nil {
		if ans, ok := resp.Answers["cause"]; ok && ans.Confidence >= jevMinConfidence {
			if _, known := mailcause.Lookup(ans.Choice); known {
				out = ans.Choice
			}
		}
	}
	s.causes.Store(key, out)
	return out
}

func (s *Service) finish(ctx context.Context, w repository.ImportWorkRow, status, code, cause, message string, accountID *uuid.UUID, keepPayload bool) {
	if err := s.repo.FinishRow(ctx, w.ImportID, w.Line, w.Attempts, status, code, cause, message, accountID, keepPayload); err != nil {
		log.Error().Err(err).Str("import_id", w.ImportID.String()).Int("line", w.Line).Msg("mailbox import: recording a row failed")
		return
	}
	s.publish(ctx, w.OrgID, w.ImportID, models.ImportRunning, false)
}

// applySettings writes the import's settings onto a mailbox. It never fails
// the row: the mailbox is connected either way, and what did not apply is
// said in the row's message.
func (s *Service) applySettings(ctx context.Context, orgID uuid.UUID, userID string, accountID uuid.UUID, p payload, st models.MailboxImportSettings, rename bool) string {
	note := s.settingsOnly(ctx, orgID, userID, accountID, p, st, rename)
	if p.TrackingDomain == "" && p.RedirectURL == "" {
		return note
	}
	return joinNote(note, s.domainExtras(ctx, repository.ImportWorkRow{OrgID: orgID, Email: p.email, CreatedBy: parseUUIDOrNil(userID), ImportID: p.importID}, p, accountID))
}

func (s *Service) settingsOnly(ctx context.Context, orgID uuid.UUID, userID string, accountID uuid.UUID, p payload, st models.MailboxImportSettings, rename bool) string {
	upd := models.UpdateEmail{
		CampaignLimit: st.DailyLimit, MinWaitTime: st.MinWait, ReplyTo: st.ReplyTo, Timezone: st.Timezone,
		WarmupBase: st.WarmupStart, WarmupMax: st.WarmupMax, WarmupIncrease: st.WarmupIncrease, WarmupReplyRate: st.WarmupReplyRate,
	}
	if rename && p.NameGiven && p.Name != "" {
		upd.Name = &p.Name
	}
	if st.Signature != nil {
		plain := *st.Signature
		htmlSig := strings.ReplaceAll(html.EscapeString(plain), "\n", "<br>")
		upd.SignaturePlain, upd.SignatureHTML = &plain, &htmlSig
	}
	var notes []string
	tagIDs, tagNote := s.resolveTags(ctx, orgID, *parseUUIDOrNil(userID), st.TagIDs, p.TagNames)
	if tagNote != "" {
		notes = append(notes, tagNote)
	}
	if len(tagIDs) > 0 {
		current, xerr := s.mailboxes.Get(ctx, orgID.String(), accountID.String())
		if xerr == nil && current != nil {
			upd.Tags = union(current.Tags, tagIDs)
		}
	}
	if !emptyUpdate(upd) {
		if _, xerr := s.emails.Update(ctx, orgID.String(), userID, accountID.String(), &upd); xerr != nil {
			notes = append(notes, "Settings not applied: "+xerr.Message)
		}
	}
	if st.Warmup != nil {
		action := "disable"
		if *st.Warmup {
			action = "start"
		}
		if _, xerr := s.emails.SetWarmupLifecycle(ctx, orgID.String(), accountID.String(), action); xerr != nil {
			if *st.Warmup {
				notes = append(notes, "Warmup not started: "+xerr.Message)
			}
		} else if *st.Warmup && s.warmup != nil {
			_ = s.warmup.EnsureWarmupScheduled(ctx, accountID)
		}
	}
	return strings.Join(notes, " ")
}

func emptyUpdate(u models.UpdateEmail) bool {
	return u.Name == nil && u.SignaturePlain == nil && u.CampaignLimit == nil && u.MinWaitTime == nil &&
		u.ReplyTo == nil && u.Timezone == nil && u.WarmupBase == nil && u.WarmupMax == nil &&
		u.WarmupIncrease == nil && u.WarmupReplyRate == nil && u.Tags == nil
}

// resolveTags turns tag names from the file into the workspace's tag ids,
// creating the ones that do not exist yet.
func (s *Service) resolveTags(ctx context.Context, orgID, userID uuid.UUID, ids []string, names []string) ([]string, string) {
	out := append([]string(nil), ids...)
	if len(names) == 0 || s.tags == nil {
		return out, ""
	}
	existing, xerr := s.tags.List(ctx, orgID)
	if xerr != nil {
		return out, "Tags not applied."
	}
	byTitle := map[string]string{}
	for _, t := range existing {
		byTitle[strings.ToLower(strings.TrimSpace(t.Title))] = t.ID.String()
	}
	for _, n := range names {
		key := strings.ToLower(n)
		if id, ok := byTitle[key]; ok {
			out = append(out, id)
			continue
		}
		g, xerr := s.tags.Create(ctx, orgID, userID, &models.GroupCreate{Title: n})
		if xerr != nil {
			return out, "Some tags could not be created."
		}
		byTitle[key] = g.ID.String()
		out = append(out, g.ID.String())
	}
	return out, ""
}

func union(a, b []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(a)+len(b))
	for _, v := range append(append([]string(nil), a...), b...) {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func parseUUIDOrNil(s string) *uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return &uuid.Nil
	}
	return &id
}

func joinNote(a, b string) string {
	if b == "" {
		return a
	}
	return a + " " + b
}

// ResolveSignin closes the rows waiting on a sign-in as this address and
// applies their settings to the mailbox that sign-in connected.
func (s *Service) ResolveSignin(ctx context.Context, orgID uuid.UUID, email string, accountID uuid.UUID) error {
	rows, err := s.repo.ResolveSignin(ctx, orgID, email, accountID)
	if err != nil {
		return err
	}
	for _, w := range rows {
		if w.Payload != "" && w.CreatedBy != nil {
			if ciph, err := s.cipher.Cipher(ctx, orgID); err == nil {
				if p, xerr := unseal(ctx, ciph, w.Payload); xerr == nil {
					p.email, p.importID = w.Email, w.ImportID
					var batch models.MailboxImportSettings
					_ = json.Unmarshal(w.Settings, &batch)
					if note := s.applySettings(ctx, orgID, w.CreatedBy.String(), accountID, p, mergeSettings(batch, p.Settings), false); note != "" {
						_ = s.repo.FinishRow(ctx, w.ImportID, w.Line, 0, models.ImportRowConnected, "", "", note, &accountID, false)
					}
				}
			}
		}
		s.publish(ctx, orgID, w.ImportID, models.ImportRunning, true)
	}
	return nil
}

// reconcileSignins catches a sign-in that connected a mailbox by any other path.
func (s *Service) reconcileSignins(ctx context.Context) {
	pending, err := s.repo.PendingSignins(ctx, 200)
	if err != nil {
		return
	}
	for _, p := range pending {
		if err := s.ResolveSignin(ctx, p.OrgID, p.Email, p.AccountID); err != nil {
			log.Warn().Err(err).Msg("mailbox import: reconciling sign-in rows failed")
		}
	}
}

// classifyHosts fills in where older mailboxes are hosted, one domain at a time.
func (s *Service) classifyHosts(ctx context.Context) {
	domains, xerr := s.mailboxes.ListUnclassifiedDomains(ctx, 50)
	if xerr != nil || len(domains) == 0 {
		return
	}
	for domain, det := range s.detector.DetectMany(ctx, domains) {
		host := det.Host
		if host == mailhost.Unknown {
			host = mailhost.Other
		}
		auth := mailhost.AuthMethodFor(host, det.PasswordAuth)
		if xerr := s.mailboxes.SetDomainMailHost(ctx, domain, string(host), auth); xerr != nil {
			return
		}
	}
}

// publish tells the workspace an import moved, at most once a second per import unless final;
// an update inside the second is sent at its end with the latest status, and a final one cancels it.
func (s *Service) publish(ctx context.Context, orgID, importID uuid.UUID, status string, force bool) {
	if s.publisher == nil {
		return
	}
	now := time.Now()
	if force {
		if v, ok := s.trailing.LoadAndDelete(importID); ok {
			v.(*deferredPublish).timer.Stop()
		}
	} else if last, ok := s.progress.Load(importID); ok {
		if wait := time.Second - now.Sub(last.(time.Time)); wait > 0 {
			d := &deferredPublish{}
			d.status.Store(status)
			// The timer exists before d is shared, so a final publish can always stop it; an
			// unregistered d fires into a CompareAndDelete that fails and does nothing.
			d.timer = time.AfterFunc(wait, func() {
				if s.trailing.CompareAndDelete(importID, d) {
					s.publish(context.WithoutCancel(ctx), orgID, importID, d.status.Load().(string), true)
				}
			})
			if v, pending := s.trailing.LoadOrStore(importID, d); pending {
				d.timer.Stop()
				v.(*deferredPublish).status.Store(status)
			}
			return
		}
	}
	s.progress.Store(importID, now)
	s.publisher.PublishMailboxImportProgress(ctx, orgID, importID, status)
}

// deferredPublish is a progress event waiting for the end of its one-second window.
type deferredPublish struct {
	status atomic.Value
	timer  *time.Timer
}

// fillVendorPasswords reads a vendor row's passwords from the vendor into the
// legs that have none, keeping the servers the row was fixed with.

// forgetDomainSetup drops a finished import's once-per-domain marks, so the map
// does not grow for the life of the process and a reopened import sets up again.
func (s *Service) forgetDomainSetup(importID uuid.UUID) {
	prefix := importID.String() + "\x00"
	s.redirected.Range(func(k, _ any) bool {
		if key, _ := k.(string); strings.HasPrefix(key, prefix) {
			s.redirected.Delete(k)
		}
		return true
	})
}
