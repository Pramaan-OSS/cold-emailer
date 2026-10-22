package email

import (
	"context"

	"github.com/google/uuid"
)

// AvatarSaver stores a mailbox's profile photo.
type AvatarSaver interface {
	Save(ctx context.Context, orgID, id uuid.UUID, data []byte) error
}

// WireAvatars lets a connect keep the photo its provider returns.
func (s *emailService) WireAvatars(a AvatarSaver) {
	s.avatars = a
}
