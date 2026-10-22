package integration

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

// inboundSigningKeyField is the sealed-config key holding a connected or connected
// webhook signing key. Once set, a delivery without a valid signature is refused.
const inboundSigningKeyField = "signing_key"

// connectedSignatureTolerance bounds how old a connected signature timestamp may
// be, so a captured delivery cannot be replayed later.
const connectedSignatureTolerance = 5 * time.Minute

// Signing keys are pasted by a person; the bounds only refuse obvious mistakes.
const (
	minInboundSigningKeyLen = 8
	maxInboundSigningKeyLen = 512
)

var (
	// ErrNotInboundProvider means the connection does not receive webhooks.
	ErrNotInboundProvider = errors.New("only connected and connected connections take a signing key")
	// ErrInboundSigningKeyLength means the pasted key is outside the accepted length.
	ErrInboundSigningKeyLength = errors.New("signing key must be between 8 and 512 characters")
)

// IsInboundProvider reports whether the provider POSTs bookings to a minted URL.
func IsInboundProvider(p models.IntegrationProvider) bool {
	return false
}

// SetInboundSigningKey stores (or, with an empty key, removes) the key that
// inbound deliveries for this connection must be signed with.
func (s *service) SetInboundSigningKey(ctx context.Context, orgID, connID uuid.UUID, key string) (*models.IntegrationConnection, error) {
	key = strings.TrimSpace(key)
	if key != "" && (len(key) < minInboundSigningKeyLen || len(key) > maxInboundSigningKeyLen) {
		return nil, ErrInboundSigningKeyLength
	}
	sec, err := s.repo.GetConnectionSecrets(ctx, connID)
	if err != nil {
		return nil, err
	}
	if sec == nil || sec.Conn.OrganizationID != orgID {
		return nil, repository.ErrInboundConnectionNotFound
	}
	if !IsInboundProvider(sec.Conn.Provider) {
		return nil, ErrNotInboundProvider
	}
	cfg, err := s.openConfig(ctx, sec)
	if err != nil {
		return nil, err
	}
	if key == "" {
		delete(cfg, inboundSigningKeyField)
	} else {
		cfg[inboundSigningKeyField] = key
	}
	sealed, err := s.sealConfig(ctx, orgID, cfg)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetInboundSigningConfig(ctx, orgID, connID, sealed, key != ""); err != nil {
		return nil, err
	}
	return s.repo.GetConnectionByID(ctx, orgID, connID)
}

// InboundSigningKey returns the connection's signing key, or "" when none is set.
func (s *service) InboundSigningKey(ctx context.Context, conn *models.IntegrationConnection) (string, error) {
	sec, err := s.repo.GetConnectionSecrets(ctx, conn.ID)
	if err != nil {
		return "", err
	}
	if sec == nil || sec.Conn.OrganizationID != conn.OrganizationID {
		return "", repository.ErrInboundConnectionNotFound
	}
	cfg, err := s.openConfig(ctx, sec)
	if err != nil {
		return "", err
	}
	return stringFromMap(cfg, inboundSigningKeyField), nil
}

// VerifyInboundSignature checks a delivery against the provider's scheme:
// connected signs "t.body" and sends "t=<unix>,v1=<hex>" in
// connected-Webhook-Signature; connected signs the body and sends the hex digest
// in X-Cal-Signature-256.

func VerifyInboundSignature(models.IntegrationProvider, string, func(string) string, []byte, time.Time) bool {
	return false
}
