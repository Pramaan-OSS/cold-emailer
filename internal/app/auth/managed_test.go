package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/cache"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type managedUsers struct {
	emailUserRepo
	empty  bool
	banned bool
}

func (r *managedUsers) IsEmpty(context.Context) (bool, error) { return r.empty, nil }
func (r *managedUsers) GetBanState(context.Context, uuid.UUID) (uint32, error) {
	if r.banned {
		return uint32(models.BanScopeLogin), nil
	}
	return 0, nil
}

type managedSecondFactor struct{}

func (managedSecondFactor) IsEnabled(context.Context, uuid.UUID) (bool, error) { return true, nil }
func (managedSecondFactor) CreatePendingChallenge(context.Context, uuid.UUID, string, *models.UserIdentity) (string, int, *errx.Error) {
	return "second-factor", 120, nil
}
func managedFixture(t *testing.T) (*authService, pramaan.ManagedIdentity, *managedUsers) {
	t.Helper()
	server := miniredis.RunT(t)
	c, e := cache.New("redis://" + server.Addr())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = c.Close() })
	u := &managedUsers{emailUserRepo: emailUserRepo{user: &models.User{ID: uuid.New(), Email: "member@example.test"}}}
	i := pramaan.ManagedIdentity{Issuer: "https://signin.example.test", Subject: "member-1", Email: u.user.Email, TokenHash: "verified-token-hash", Name: "Test Member"}
	s := &authService{cache: c, userRepository: u, identities: &memoryIdentities{known: map[string]uuid.UUID{i.Issuer + "|" + i.Subject: u.user.ID}}, policy: &config.AuthPolicy{}, twofa: managedSecondFactor{}}
	return s, i, u
}
func TestManagedFinishRequiresStartedStateAndRefusesReplay(t *testing.T) {
	ctx := context.Background()
	s, i, _ := managedFixture(t)
	if _, x := s.ManagedFinish(ctx, strings.Repeat("a", 64), i, "", "", "", nil); x == nil {
		t.Fatal("unstarted sign-in accepted")
	}
	state, x := s.ManagedBegin(ctx)
	if x != nil {
		t.Fatal(x)
	}
	result, x := s.ManagedFinish(ctx, state, i, "", "", "", nil)
	if x != nil || !result.TwoFARequired {
		t.Fatal("must preserve second factor", x)
	}
	if _, x = s.ManagedFinish(ctx, state, i, "", "", "", nil); x == nil {
		t.Fatal("state replay accepted")
	}
	state, _ = s.ManagedBegin(ctx)
	if _, x = s.ManagedFinish(ctx, state, i, "", "", "", nil); x == nil {
		t.Fatal("token replay accepted")
	}
}
func TestManagedFinishPreservesBanAndFirstOwnerClaim(t *testing.T) {
	for _, mode := range []string{"ban", "empty", "unverified-address"} {
		t.Run(mode, func(t *testing.T) {
			s, i, u := managedFixture(t)
			switch mode {
			case "ban":
				u.banned = true
			case "empty":
				u.empty = true
			case "unverified-address":
				s.identities = &memoryIdentities{known: map[string]uuid.UUID{}}
			}
			state, _ := s.ManagedBegin(context.Background())
			if _, x := s.ManagedFinish(context.Background(), state, i, "", "", "", nil); x == nil {
				t.Fatal("security gate bypassed")
			}
		})
	}
}
func TestManagedOwnerClaimRequiresPrivateTokenBeforeLink(t *testing.T) {
	s, i, u := managedFixture(t)
	u.empty = true
	ids := s.identities.(*memoryIdentities)
	ids.known = map[string]uuid.UUID{}
	claim := func(_ context.Context, key, email, first, last string) (*models.User, *errx.Error) {
		if key != "private-claim" {
			return nil, errx.ErrSetupToken
		}
		if email != i.Email || first != "Test" || last != "Member" {
			t.Fatal("profile did not come from verified claims")
		}
		return u.user, nil
	}
	state, _ := s.ManagedBegin(context.Background())
	r, x := s.ManagedFinish(context.Background(), state, i, "private-claim", "", "", claim)
	if x != nil || !r.TwoFARequired || len(ids.linked) != 1 {
		t.Fatal("owner claim failed", x)
	}
}
