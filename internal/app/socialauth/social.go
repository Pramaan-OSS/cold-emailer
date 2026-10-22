// Package socialauth hands authorization codes to Pramaan; it owns no vendor transport.
package socialauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"

	"github.com/pramaan-oss/cold-email/internal/pkg/idtoken"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type Flow struct {
	Client                                                   pramaan.Caller
	ProviderKey, Label, ClientID, Redirect, AuthorizationURL string
}

func (f *Flow) ProviderName() string { return f.Label }
func (f *Flow) AuthCodeURL(state, nonce, verifier string) string {
	u, e := url.Parse(f.AuthorizationURL)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ""
	}
	q := u.Query()
	q.Set("client_id", f.ClientID)
	q.Set("redirect_uri", f.Redirect)
	q.Set("response_type", "code")
	if q.Get("scope") == "" {
		q.Set("scope", "openid email profile")
	}
	q.Set("state", state)
	q.Set("nonce", nonce)
	if verifier != "" {
		sum := sha256.Sum256([]byte(verifier))
		q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:]))
		q.Set("code_challenge_method", "S256")
	}
	// The app owns the authorization client; Pramaan checks the code and signed identity.
	u.RawQuery = q.Encode()
	return u.String()
}
func (f *Flow) Exchange(ctx context.Context, code, verifier, nonce string) (*idtoken.Claims, error) {
	if f.Client == nil || nonce == "" {
		return nil, pramaan.ErrUnavailable
	}
	var out struct {
		ProviderKey string `json:"providerKey"`
		Subject     string `json:"subjectId"`
		Email       string `json:"email"`
		Verified    bool   `json:"emailVerified"`
		Name        string `json:"displayName"`
	}
	input := map[string]any{"providerKey": f.ProviderKey, "code": code, "redirectUri": f.Redirect, "expectedNonce": nonce}
	if verifier != "" {
		input["pkceVerifier"] = verifier
	}
	if e := f.Client.Call(ctx, "auth.social.exchange", input, &out); e != nil {
		return nil, e
	}
	if out.ProviderKey != f.ProviderKey || out.Subject == "" || out.Email == "" || !out.Verified {
		return nil, errors.New("Sign-in did not confirm your identity and email.")
	}
	return &idtoken.Claims{Subject: out.Subject, Email: out.Email, EmailVerified: true, GivenName: out.Name, Issuer: out.ProviderKey, Nonce: nonce}, nil
}
