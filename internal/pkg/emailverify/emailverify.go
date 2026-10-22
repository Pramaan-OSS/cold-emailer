package emailverify

import (
	"context"
	"strings"
	"time"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type Status string

const (
	// StatusValid means syntax + MX + an accepted RCPT on a non-catch-all domain.
	StatusValid Status = "valid"
	// StatusRisky means deliverable-looking but unsafe to trust — primarily a
	// catch-all domain where acceptance proves nothing.
	StatusRisky Status = "risky"
	// StatusInvalid means a hard failure: bad syntax, no MX, or a 550-class RCPT
	// rejection. These are the addresses the pre-send gate should drop.
	StatusInvalid Status = "invalid"
	// StatusUnknown means we couldn't determine deliverability — timeout, 4xx
	// greylisting, blocked outbound :25, or any transient/ambiguous condition.
	// Unknown is never dropped; it is retried or sent cautiously.
	StatusUnknown Status = "unknown"
)

// SubStatus refines a Status with the reason class a paid provider or the
// in-house checks can name. Empty when nothing more specific is known.
type SubStatus string

const (
	SubStatusNone        SubStatus = ""
	SubStatusCatchAll    SubStatus = "catch_all"
	SubStatusDisposable  SubStatus = "disposable"
	SubStatusRole        SubStatus = "role"
	SubStatusSpamTrap    SubStatus = "spamtrap"
	SubStatusMailboxFull SubStatus = "mailbox_full"
	SubStatusNoMX        SubStatus = "no_mx"
	SubStatusSyntax      SubStatus = "syntax"
	// SubStatusUndisclosed marks a provider (Microsoft, Yahoo) that answers
	// every RCPT the same way, so an SMTP probe cannot judge the mailbox.
	SubStatusUndisclosed SubStatus = "undisclosed"
)

// Provider names for Result.Provider and contacts.verification_provider.
const (
	ProviderBuiltin         = "builtin"
	ProviderMillionVerifier = "millionverifier"
	ProviderCleanMyList     = "cleanmylist"
)

// Result is the outcome of verifying one address. It round-trips into the
// contacts table (verification_status / verification_reason / is_catch_all /
// verification_checked_at / verification_sub_status / verification_provider).
type Result struct {
	Email      string    `json:"email"`
	Status     Status    `json:"status"`
	SubStatus  SubStatus `json:"sub_status,omitempty"`
	Reason     string    `json:"reason"`
	IsCatchAll bool      `json:"is_catch_all"`
	HasMX      bool      `json:"has_mx"`
	// Provider is who produced the verdict: ProviderBuiltin, a paid backend,
	// or the vocabulary an imported result was recognised as.
	Provider string `json:"provider,omitempty"`
	// Confidence is the scored certainty of Status once evidence is applied.
	Confidence int       `json:"confidence,omitempty"`
	CheckedAt  time.Time `json:"checked_at"`
}

// Verifier is the single contract the rest of the platform depends on. The
// in-house SMTPVerifier implements it; a paid provider can implement the same
// interface and be swapped in at wiring time with no other code changes.
type Verifier interface {
	Verify(ctx context.Context, email string) Result
}

type Config struct {
	// HeloHost is the hostname announced in EHLO/HELO, e.g. "example.test".
	HeloHost string
	// MailFrom is the envelope sender for the probe, e.g. "verify@example.test".
	// An empty MAIL FROM ("<>") is technically valid but more often filtered, so
	// a real address on the verifying host is preferred.
	MailFrom string
	// DialTimeout bounds the TCP connect to a single MX host. Default 5s.
	DialTimeout time.Duration
	// CommandTimeout bounds each SMTP command exchange. Default 5s.
	CommandTimeout time.Duration
	// MXTimeout bounds the MX DNS lookup. Default 5s.
	MXTimeout time.Duration
}

type BrokerVerifier struct{ Client pramaan.Caller }
type SMTPVerifier = BrokerVerifier

func New(Config) *BrokerVerifier {
	c, err := pramaan.FromEnv()
	if err != nil {
		return &BrokerVerifier{}
	}
	return &BrokerVerifier{Client: c}
}
func (b *BrokerVerifier) ProbeReady() bool { return b.Client != nil }
func (b *BrokerVerifier) Verify(ctx context.Context, email string) Result {
	out, _ := b.Check(ctx, email)
	return out
}
func (b *BrokerVerifier) Check(ctx context.Context, email string) (Result, error) {
	r := Result{Email: strings.ToLower(strings.TrimSpace(email)), CheckedAt: time.Now().UTC(), Status: StatusUnknown, Provider: "pramaan"}
	if b.Client == nil {
		r.Reason = "Connect Pramaan to verify addresses."
		return r, pramaan.ErrNotConfigured
	}
	var out struct {
		Status      string `json:"status"`
		Risk        string `json:"risk"`
		Deliverable bool   `json:"deliverable"`
	}
	if e := b.Client.Call(ctx, "people.email.verify", map[string]any{"email": r.Email}, &out); e != nil {
		r.Reason = e.Error()
		return r, e
	}
	switch out.Status {
	case "valid":
		r.Status = StatusValid
	case "invalid":
		r.Status = StatusInvalid
	case "accept_all":
		r.Status = StatusRisky
		r.IsCatchAll = true
		r.SubStatus = SubStatusCatchAll
	default:
		r.Status = StatusUnknown
	}
	if out.Risk != "" {
		r.Status = StatusRisky
		if out.Risk == "spam_trap" {
			r.Status = StatusInvalid
			r.SubStatus = SubStatusSpamTrap
		}
		if out.Risk == "disposable" {
			r.Status = StatusInvalid
			r.SubStatus = SubStatusDisposable
		}
	}
	r.Reason = "Address verification complete."
	return r, nil
}
func (b *BrokerVerifier) Account(context.Context) (*int, error) { return nil, nil }
func (b *BrokerVerifier) ObservesBalance() bool                 { return false }
