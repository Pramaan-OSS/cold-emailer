// Package enterpriseauth keeps organisation ownership and single-use sign-in state local.
package enterpriseauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/cache"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

var ErrRefused = errors.New("Organisation sign-in was refused. Use your local account or start a fresh sign-in.")
var ErrUnavailable = errors.New("Organisation sign-in is not available yet. Use your local account. SAML is not supported.")

type Members interface {
	GetMember(context.Context, uuid.UUID, uuid.UUID) (*models.OrganizationMember, error)
}
type StateStore interface {
	Put(context.Context, string, []byte, time.Duration) error
	Take(context.Context, string) ([]byte, error)
}
type RedisStates struct{ Cache *cache.Cache }

func (r RedisStates) Put(ctx context.Context, key string, b []byte, ttl time.Duration) error {
	return r.Cache.SetEx(ctx, "enterprise_state:"+key, b, ttl).Err()
}
func (r RedisStates) Take(ctx context.Context, key string) ([]byte, error) {
	return r.Cache.GetDel(ctx, "enterprise_state:"+key).Bytes()
}

type Service struct {
	Platform    pramaan.Enterprise
	Connections repository.EnterpriseConnections
	Members     Members
	Identities  repository.IdentityRepository
	States      StateStore
}
type Redirect struct {
	URL     string `json:"url"`
	Binding string `json:"binding"`
	State   string `json:"-"`
}
type flow struct {
	Org                                       uuid.UUID
	Connection                                uuid.UUID
	LinkUser                                  uuid.UUID
	State, Nonce, Verifier, Binding, Redirect string
}
type Resolution struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	Binding        string
}

func (s *Service) Ready() bool {
	return s != nil && s.Platform.Ready() && s.Connections != nil && s.Members != nil && s.Identities != nil && s.States != nil
}
func (s *Service) member(ctx context.Context, org, user uuid.UUID, admin bool) error {
	if user == uuid.Nil || org == uuid.Nil || s.Members == nil {
		return ErrRefused
	}
	m, e := s.Members.GetMember(ctx, org, user)
	if e != nil || m == nil || m.AcceptedAt == nil || m.OrganizationID != org || m.UserID != user {
		return ErrRefused
	}
	if admin && m.Role != "owner" && m.Role != "admin" {
		return pramaan.ErrEnterpriseApproval
	}
	return nil
}
func (s *Service) Create(ctx context.Context, org, user uuid.UUID, key string, in pramaan.EnterpriseCreate) (repository.EnterpriseConnection, error) {
	var out repository.EnterpriseConnection
	if !s.Ready() {
		return out, ErrUnavailable
	}
	if e := s.member(ctx, org, user, true); e != nil {
		return out, e
	}
	row, e := s.Platform.Create(ctx, org, "admin", key, in)
	if e != nil {
		return out, ErrUnavailable
	}
	out, e = s.Connections.Save(ctx, org, row.Ref, row.Name, row.CreatedAt)
	if e != nil {
		return out, errors.New("Connection creation is unconfirmed locally. List connections before trying again.")
	}
	return out, nil
}
func (s *Service) List(ctx context.Context, org, user uuid.UUID, cursor string) ([]repository.EnterpriseConnection, string, error) {
	if !s.Ready() {
		return nil, "", ErrUnavailable
	}
	if e := s.member(ctx, org, user, true); e != nil {
		return nil, "", e
	}
	page, e := s.Platform.List(ctx, org, cursor)
	if e != nil {
		return nil, "", ErrUnavailable
	}
	rows := []repository.EnterpriseConnection{}
	for _, remote := range page.Results {
		row, e := s.Connections.Save(ctx, org, remote.Ref, remote.Name, remote.CreatedAt)
		if e != nil {
			return nil, "", ErrUnavailable
		}
		rows = append(rows, row)
	}
	return rows, page.NextCursor, nil
}
func (s *Service) Choices(ctx context.Context, org, user uuid.UUID) ([]repository.EnterpriseConnection, error) {
	if !s.Ready() {
		return nil, ErrUnavailable
	}
	if e := s.member(ctx, org, user, false); e != nil {
		return nil, e
	}
	return s.Connections.List(ctx, org)
}
func (s *Service) Delete(ctx context.Context, org, user, id uuid.UUID, key string, approved bool) error {
	if !s.Ready() {
		return ErrUnavailable
	}
	if e := s.member(ctx, org, user, true); e != nil {
		return e
	}
	row, e := s.Connections.Get(ctx, org, id)
	if e != nil {
		return ErrRefused
	}
	if e = s.Platform.Delete(ctx, row.Ref, "admin", key, approved); e != nil {
		return errors.New("Deletion could not be confirmed. Check connections before another attempt.")
	}
	return s.Connections.Delete(ctx, org, id)
}
func fresh() (string, error) {
	b := make([]byte, 32)
	_, e := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), e
}
func (s *Service) Begin(ctx context.Context, org, id, user uuid.UUID, link bool) (Redirect, error) {
	var out Redirect
	if !s.Ready() {
		return out, ErrUnavailable
	}
	if link {
		if e := s.member(ctx, org, user, false); e != nil {
			return out, e
		}
	}
	row, e := s.Connections.Get(ctx, org, id)
	if e != nil {
		return out, ErrRefused
	}
	f := flow{Org: org, Connection: id, Redirect: s.Platform.RedirectURI}
	if link {
		f.LinkUser = user
	}
	for _, value := range []*string{&f.State, &f.Nonce, &f.Verifier, &f.Binding} {
		*value, e = fresh()
		if e != nil {
			return out, ErrUnavailable
		}
	}
	// Save before the platform call; remove it if no usable start result arrives.
	data, _ := json.Marshal(f)
	if e = s.States.Put(ctx, f.State, data, 10*time.Minute); e != nil {
		return out, ErrUnavailable
	}
	remote, e := s.Platform.Start(ctx, row.Ref, f.State, f.Nonce, f.Verifier)
	if e != nil {
		_, _ = s.States.Take(ctx, f.State)
		return out, ErrUnavailable
	}
	return Redirect{URL: remote.URL, Binding: f.Binding, State: f.State}, nil
}
func (s *Service) Redeem(ctx context.Context, code, state, browserState string) (Resolution, error) {
	var out Resolution
	if !s.Ready() {
		return out, ErrUnavailable
	}
	// Verify the returning browser before consuming state or sending a code.
	if state == "" || code == "" || browserState == "" || subtle.ConstantTimeCompare([]byte(state), []byte(browserState)) != 1 {
		return out, ErrRefused
	}
	data, e := s.States.Take(ctx, state)
	if e != nil || len(data) == 0 {
		return out, ErrRefused
	}
	var f flow
	if json.Unmarshal(data, &f) != nil || f.State != state || f.Redirect != s.Platform.RedirectURI {
		return out, ErrRefused
	}
	row, e := s.Connections.Get(ctx, f.Org, f.Connection)
	if e != nil {
		return out, ErrRefused
	}
	identity, e := s.Platform.Redeem(ctx, row.Ref, code, state, f.Nonce, f.Verifier)
	if e != nil {
		return out, ErrRefused
	}
	issuer := "enterprise:" + row.ID.String()
	user, e := s.Identities.FindUserByIdentity(ctx, issuer, identity.Subject)
	if e != nil {
		return out, ErrRefused
	}
	if f.LinkUser != uuid.Nil {
		if user != uuid.Nil && user != f.LinkUser {
			return out, ErrRefused
		}
		if e = s.member(ctx, f.Org, f.LinkUser, false); e != nil {
			return out, e
		}
		// Explicit account proof came from the local session, never from email.
		if e = s.Identities.Link(ctx, f.LinkUser, models.UserIdentity{Provider: models.IdentityProviderOIDC, Issuer: issuer, Subject: identity.Subject}); e != nil {
			return out, ErrRefused
		}
		user = f.LinkUser
	}
	if user == uuid.Nil {
		return out, errors.New("This organisation identity is not linked. Sign in locally and link it in Security first.")
	}
	if e = s.member(ctx, f.Org, user, false); e != nil {
		return out, e
	}
	if e = s.Identities.TouchLogin(ctx, issuer, identity.Subject); e != nil {
		return out, ErrRefused
	}
	return Resolution{UserID: user, OrganizationID: f.Org, Binding: f.Binding}, nil
}
