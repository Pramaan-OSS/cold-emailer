package pramaan

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Enterprise owns the external OIDC connection/start/exchange contracts.
// Credentials are forwarded once; they never become local connection data.
type Enterprise struct {
	Client      Caller
	RedirectURI string
}
type EnterpriseCreate struct {
	Name            string   `json:"name"`
	Issuer          string   `json:"issuer"`
	ClientID        string   `json:"clientId"`
	ClientSecret    string   `json:"clientSecret,omitempty"`
	TokenAuthMethod string   `json:"tokenAuthMethod"`
	EndpointHosts   []string `json:"endpointHosts,omitempty"`
	Approved        bool     `json:"approved"`
}
type EnterpriseConnection struct {
	Ref             string `json:"ref"`
	OrganizationRef string `json:"organizationRef"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	CreatedAt       string `json:"createdAt"`
}
type EnterprisePage struct {
	Results    []EnterpriseConnection `json:"results"`
	NextCursor string                 `json:"nextCursor,omitempty"`
}
type EnterpriseLink struct {
	URL       string `json:"authorizationUrl"`
	ExpiresAt string `json:"expiresAt"`
}
type EnterpriseIdentity struct {
	ProviderKey   string `json:"providerKey"`
	Subject       string `json:"subjectId"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
	Name          string `json:"displayName"`
}

var ErrEnterpriseApproval = errors.New("An organisation owner or admin must approve this connection change.")
var ErrEnterpriseConfiguration = errors.New("Organisation sign-in needs a public HTTPS callback and supported OIDC client settings. SAML is not available.")

func EnterpriseOrganization(org uuid.UUID) string { return "cold-email-org_" + org.String() }
func enterpriseHTTPS(raw string, issuer bool) bool {
	u, e := url.Parse(raw)
	if e != nil || len(raw) > 512 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || net.ParseIP(u.Hostname()) != nil || u.Hostname() == "localhost" || !strings.Contains(u.Hostname(), ".") {
		return false
	}
	return !issuer || u.RawQuery == ""
}
func (e Enterprise) Ready() bool {
	if client, ok := e.Client.(*Client); ok && client == nil {
		return false
	}
	return e.Client != nil && enterpriseHTTPS(e.RedirectURI, false)
}
func (e Enterprise) Create(ctx context.Context, org uuid.UUID, role, key string, in EnterpriseCreate) (EnterpriseConnection, error) {
	var out EnterpriseConnection
	if role != "owner" && role != "admin" || !in.Approved {
		return out, ErrEnterpriseApproval
	}
	if !e.Ready() || !enterpriseHTTPS(in.Issuer, true) || len(in.Name) < 1 || len(in.Name) > 64 || len(in.ClientID) < 1 || len(in.ClientID) > 512 || len(in.ClientSecret) > 2048 || len(in.EndpointHosts) > 5 {
		return out, ErrEnterpriseConfiguration
	}
	if _, err := uuid.Parse(key); err != nil {
		return out, errors.New("The connection needs its own request reference.")
	}
	if in.TokenAuthMethod != "none" && in.TokenAuthMethod != "client_secret_basic" && in.TokenAuthMethod != "client_secret_post" || (in.TokenAuthMethod == "none") != (in.ClientSecret == "") {
		return out, ErrEnterpriseConfiguration
	}
	for _, h := range in.EndpointHosts {
		if h != strings.ToLower(h) || strings.ContainsAny(h, "/:@?#* ") || strings.HasSuffix(h, ".") || net.ParseIP(h) != nil || !strings.Contains(h, ".") {
			return out, ErrEnterpriseConfiguration
		}
	}
	body := map[string]any{"organizationRef": EnterpriseOrganization(org), "name": in.Name, "issuer": in.Issuer, "clientId": in.ClientID, "tokenAuthMethod": in.TokenAuthMethod, "redirectUris": []string{e.RedirectURI}}
	if in.ClientSecret != "" {
		body["clientSecret"] = in.ClientSecret
	}
	if len(in.EndpointHosts) > 0 {
		body["endpointHosts"] = in.EndpointHosts
	}
	err := e.Client.Call(ctx, "auth.enterprise-connection.create", body, &out, Idempotency(key))
	if err == nil && (out.Ref == "" || out.OrganizationRef != EnterpriseOrganization(org) || out.Status != "ready") {
		err = errors.New("The connection result is uncertain. List connections before trying again.")
	}
	return out, err
}
func (e Enterprise) List(ctx context.Context, org uuid.UUID, cursor string) (EnterprisePage, error) {
	var out EnterprisePage
	if e.Client == nil {
		return out, ErrNotConfigured
	}
	if len(cursor) > 64 {
		return out, ErrEnterpriseConfiguration
	}
	body := map[string]any{"organizationRef": EnterpriseOrganization(org), "limit": 50}
	if cursor != "" {
		body["cursor"] = cursor
	}
	err := e.Client.Call(ctx, "auth.enterprise-connections.list", body, &out)
	for _, row := range out.Results {
		if row.OrganizationRef != EnterpriseOrganization(org) || row.Ref == "" || row.Status != "ready" {
			return EnterprisePage{}, errors.New("The connection list could not be verified.")
		}
	}
	return out, err
}
func (e Enterprise) Delete(ctx context.Context, ref, role, key string, approved bool) error {
	if role != "owner" && role != "admin" || !approved {
		return ErrEnterpriseApproval
	}
	if e.Client == nil {
		return ErrNotConfigured
	}
	if _, err := uuid.Parse(key); err != nil {
		return errors.New("The change needs its own request reference.")
	}
	var out struct {
		Deleted bool `json:"deleted"`
	}
	err := e.Client.Call(ctx, "auth.enterprise-connection.delete", map[string]any{"ref": ref, "confirm": true}, &out, Idempotency(key))
	if err == nil && !out.Deleted {
		return errors.New("Deletion is unconfirmed. Check the connection before trying again.")
	}
	return err
}
func (e Enterprise) Start(ctx context.Context, ref, state, nonce, verifier string) (EnterpriseLink, error) {
	var out EnterpriseLink
	if !e.Ready() {
		return out, ErrEnterpriseConfiguration
	}
	if len(state) < 22 || len(nonce) < 22 || len(verifier) < 43 || len(verifier) > 128 {
		return out, ErrEnterpriseConfiguration
	}
	sum := sha256.Sum256([]byte(verifier))
	err := e.Client.Call(ctx, "auth.enterprise.start", map[string]any{"connectionRef": ref, "redirectUri": e.RedirectURI, "state": state, "nonce": nonce, "codeChallenge": base64.RawURLEncoding.EncodeToString(sum[:])}, &out, Idempotency(uuid.NewString()))
	if err != nil {
		return out, err
	}
	u, parseErr := url.Parse(out.URL)
	expires, dateErr := time.Parse(time.RFC3339, out.ExpiresAt)
	platform, _ := url.Parse(DefaultBaseURL)
	host := platform.Host
	if client, ok := e.Client.(*Client); ok {
		base, _ := url.Parse(client.base)
		if base.Scheme == "https" {
			host = base.Host
		}
	}
	if parseErr != nil || u.Scheme != "https" || u.Host != host || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/connect/link/") || dateErr != nil || !expires.After(time.Now()) {
		return EnterpriseLink{}, errors.New("Organisation sign-in could not start. Start a fresh sign-in.")
	}
	return out, nil
}
func (e Enterprise) Redeem(ctx context.Context, ref, code, state, nonce, verifier string) (EnterpriseIdentity, error) {
	var out EnterpriseIdentity
	if !e.Ready() || ref == "" || code == "" || state == "" || nonce == "" || verifier == "" {
		return out, errors.New("This sign-in was not started here. Start again.")
	}
	err := e.Client.Call(ctx, "auth.social.exchange", map[string]any{"providerKey": "enterprise", "connectionRef": ref, "code": code, "redirectUri": e.RedirectURI, "state": state, "expectedNonce": nonce, "pkceVerifier": verifier}, &out, Idempotency(uuid.NewString()))
	if err == nil && (out.ProviderKey != "enterprise" || out.Subject == "" || len(out.Subject) > 512) {
		err = errors.New("Organisation identity could not be verified. Start again.")
	}
	return out, err
}
