package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/app/token"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

func (s *authService) ManagedBegin(ctx context.Context) (string, *errx.Error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", errx.InternalError()
	}
	state := hex.EncodeToString(b)
	if s.cache == nil || s.cache.SetEx(ctx, "managed_begin:"+state, "1", 10*time.Minute).Err() != nil {
		return "", errx.InternalError()
	}
	return state, nil
}

// ManagedFinish preserves registration rules, account linking, bans and second factors.
func (s *authService) ManagedFinish(ctx context.Context, state string, i pramaan.ManagedIdentity, setupToken, ip, agent string, claim func(context.Context, string, string, string, string) (*models.User, *errx.Error)) (*models.LoginResult, *errx.Error) {
	if s.cache == nil || len(state) != 64 || i.Subject == "" || i.TokenHash == "" {
		return nil, errx.ErrExternalCode
	}
	used, e := s.cache.GetDel(ctx, "managed_begin:"+state).Result()
	if e != nil || used != "1" {
		return nil, errx.ErrExternalCode
	}
	fresh, e := s.cache.SetNX(ctx, "managed_token:"+i.TokenHash, "1", 24*time.Hour).Result()
	if e != nil || !fresh {
		return nil, errx.ErrExternalCode
	}
	address, e := mail.ParseAddress(i.Email)
	if e != nil {
		return nil, errx.ErrExternalEmail
	}
	address.Address = normalizeEmail(address.Address)
	names := strings.Fields(i.Name)
	first, last := "Member", ""
	if len(names) > 0 {
		first = names[0]
		last = strings.Join(names[1:], " ")
	}
	if s.identities == nil {
		return nil, errx.ErrExternalProvider
	}
	if setupToken != "" {
		if claim == nil {
			return nil, errx.ErrSetupToken
		}
		u, x := claim(ctx, setupToken, address.Address, first, last)
		if x != nil {
			return nil, x
		}
		if e = s.identities.Link(ctx, u.ID, models.UserIdentity{Provider: models.IdentityProviderOIDC, Issuer: i.Issuer, Subject: i.Subject, Email: address.Address}); e != nil {
			return nil, errx.InternalError()
		}
		return s.finishLoginAs(ctx, u.ID, ip, agent, token.AuthProviderOIDC)
	}
	empty, e := s.userRepository.IsEmpty(ctx)
	if e != nil {
		return nil, errx.InternalError()
	}
	if empty {
		return nil, errx.ErrSetupToken
	}
	// An unverified address must never attach to an existing account by email alone.
	existing, e := s.identities.FindUserByIdentity(ctx, i.Issuer, i.Subject)
	if e != nil {
		return nil, errx.InternalError()
	}
	if existing == uuid.Nil && !i.EmailVerified {
		u, x := s.userRepository.GetUserByEmail(ctx, address.Address)
		if x != nil && !errors.Is(x, errx.ErrUser) {
			return nil, errx.InternalError()
		}
		if u != nil || s.Policy().RequireEmailVerification {
			return nil, errx.ErrExternalEmail
		}
	}
	res, x := s.resolveFederatedUser(ctx, models.IdentityProviderOIDC, i.Issuer, i.Subject, address, first, last)
	if x != nil {
		return nil, x
	}
	return s.finishFederatedLogin(ctx, res, ip, agent, token.AuthProviderOIDC)
}
