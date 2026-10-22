package idtoken

import (
	"context"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

// Claims are verified identity facts; local accounts and sessions remain app-owned.
type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	GivenName     string
	FamilyName    string
	// Issuer is carried through so a generic OIDC caller can key the account
	// on (issuer, subject) rather than on the email address alone.
	Issuer string
	// Nonce binds the token to one authorization request. This package does
	// not compare it: only the caller knows which nonce it minted, so the
	// caller must check it and reject a mismatch.
	Nonce string
	// Groups carries the group or role claim when the provider sends one.
	Groups []string
}

// Generic external identity verification is unavailable until a broker contract covers it.
type Verifier struct{}

func NewVerifier(_ string, _ []string, _ []string) *Verifier      { return &Verifier{} }
func (*Verifier) Verify(context.Context, string) (*Claims, error) { return nil, pramaan.ErrUnavailable }
