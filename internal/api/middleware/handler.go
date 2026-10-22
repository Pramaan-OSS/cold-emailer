package middleware

import (
	"github.com/pramaan-oss/cold-email/internal/app/apikey"
	"github.com/pramaan-oss/cold-email/internal/app/idempotency"
	"github.com/pramaan-oss/cold-email/internal/app/oauth"
	"github.com/pramaan-oss/cold-email/internal/app/organization"
	"github.com/pramaan-oss/cold-email/internal/app/ratelimit"
	"github.com/pramaan-oss/cold-email/internal/app/token"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/cache"
)

type Handler struct {
	TokenService        token.TokenService
	APIKeyService       apikey.APIKeyService
	IdempotencyService  idempotency.Service
	RateLimitService    ratelimit.RateLimitService
	OrganizationService organization.OrganizationService
	// OAuthService validates OAuth 2.1 bearer access tokens (nil-safe: when
	// unset, only JWT + API-key auth are accepted).
	OAuthService *oauth.Service
	// Cache backs the pre-login per-IP limiter, which cannot use
	// RateLimitService because that one is keyed on a user id that does not
	// exist yet.
	Cache *cache.Cache
}
