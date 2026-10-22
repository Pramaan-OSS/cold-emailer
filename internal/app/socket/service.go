package socket

import (
	"context"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/app/token"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/cache"
)

type SocketService interface {
	GenerateWebsocketToken(ctx context.Context, userID, sessionID uuid.UUID) (string, *errx.Error)
}

type socketService struct {
	cache        *cache.Cache
	tokenService token.TokenService
}

func NewService(cache *cache.Cache, tokenService token.TokenService) SocketService {
	return &socketService{
		cache:        cache,
		tokenService: tokenService,
	}
}
