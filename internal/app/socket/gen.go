package socket

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/observability/errs"
	"github.com/pramaan-oss/cold-email/internal/pkg/crypt"
)

func (s *socketService) GenerateWebsocketToken(ctx context.Context, userID, sessionID uuid.UUID) (string, *errx.Error) {
	issuedAt := time.Now()
	expiresAt := issuedAt.Add(SocketTTL)
	id := uuid.New()
	nonce, err := crypt.Nonce()
	if err != nil {
		errs.CaptureException(err)
		return "", errx.InternalError()
	}

	wsToken, err := s.tokenService.GenerateWebsocketSessionToken(userID, id, sessionID, nonce, issuedAt, expiresAt)
	if err != nil {
		errs.CaptureException(err)
		return "", errx.InternalError()
	}

	if err := s.saveToken(ctx, id, nonce, expiresAt); err != nil {
		return "", err
	}

	return wsToken, nil
}
