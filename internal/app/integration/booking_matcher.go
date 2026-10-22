package integration

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// ContactByEmailFunc resolves a ColdEmail contact id from an email, scoped to the
// org. Provided as a function type so this package does not import the full
// contact service (which would create an import cycle).
type ContactByEmailFunc func(ctx context.Context, orgID uuid.UUID, email string) (*uuid.UUID, error)

// ContactVerifyFunc confirms a contact id actually belongs to the org. Used to
// safely honour a contact-id hint embedded in the booking link (the webhook is
// authenticated by the org's inbound secret, but we still never trust an
// attacker-supplied id without verifying org ownership).
type ContactVerifyFunc func(ctx context.Context, orgID, contactID uuid.UUID) (bool, error)

// BookingMatcher joins an inbound booking payload to a ColdEmail contact. It
// prefers a verified id hint (deterministic, survives a different reply-to
// email) and falls back to an org-scoped email lookup.
type BookingMatcher struct {
	lookup ContactByEmailFunc
	verify ContactVerifyFunc
}

func NewBookingMatcher(lookup ContactByEmailFunc, verify ContactVerifyFunc) *BookingMatcher {
	return &BookingMatcher{lookup: lookup, verify: verify}
}

// Resolve returns the best contact id for a booking: a verified hint if present,
// otherwise the email match. Returns nil when neither resolves.
func (m *BookingMatcher) Resolve(ctx context.Context, orgID uuid.UUID, email, idHint string) *uuid.UUID {
	if id := m.matchHint(ctx, orgID, idHint); id != nil {
		return id
	}
	return m.matchEmail(ctx, orgID, email)
}

func (m *BookingMatcher) matchHint(ctx context.Context, orgID uuid.UUID, idHint string) *uuid.UUID {
	idHint = strings.TrimSpace(idHint)
	if idHint == "" || m.verify == nil {
		return nil
	}
	id, err := uuid.Parse(idHint)
	if err != nil {
		return nil
	}
	ok, verr := m.verify(ctx, orgID, id)
	if verr != nil || !ok {
		return nil
	}
	return &id
}

func (m *BookingMatcher) matchEmail(ctx context.Context, orgID uuid.UUID, email string) *uuid.UUID {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || m.lookup == nil {
		return nil
	}
	id, err := m.lookup(ctx, orgID, email)
	if err != nil {
		return nil
	}
	return id
}
