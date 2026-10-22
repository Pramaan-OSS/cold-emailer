package email

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

// stubReauthRepo serves one mailbox and records the reconnect writes.
type stubReauthRepo struct {
	repository.EmailRepository

	account       *models.Email
	storedRefresh string
	updateErr     *errx.Error

	wroteAccess  string
	wroteRefresh string
	updated      *models.UpdateEmail
}

// GetSyncSkipFolders answers the loader's skip-list read with an empty list.
func (s *stubReauthRepo) GetSyncSkipFolders(context.Context, uuid.UUID) ([]string, *errx.Error) {
	return nil, nil
}

func (s *stubReauthRepo) GetByID(ctx context.Context, emailAccountID uuid.UUID) (*models.Email, *errx.Error) {
	return s.account, nil
}

func (s *stubReauthRepo) Get(ctx context.Context, orgID, emailAccountID string) (*models.Email, *errx.Error) {
	// The real org-scoped Get does not select user_id or organization_id;
	// mimic that so a caller depending on them fails here too (it did once).
	partial := *s.account
	partial.UserID = ""
	partial.OrganizationID = nil
	return &partial, nil
}

func (s *stubReauthRepo) GetOAuthCredentials(ctx context.Context, emailAccountID uuid.UUID) (*repository.OAuthCredentials, *errx.Error) {
	return &repository.OAuthCredentials{RefreshToken: s.storedRefresh}, nil
}

func (s *stubReauthRepo) RefreshBoxToken(ctx context.Context, id uuid.UUID, accessToken, refreshToken string, expiresAt time.Time) error {
	s.wroteAccess = accessToken
	s.wroteRefresh = refreshToken
	return nil
}

func (s *stubReauthRepo) Update(ctx context.Context, userID, emailAccountID string, udata *models.UpdateEmail) (*models.Email, *errx.Error) {
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	s.updated = udata
	return s.account, nil
}

// stubErrorsRepo records which error codes a reconnect resolved.
type stubErrorsRepo struct {
	repository.EmailAccountErrorRepository

	resolved []string
}

func (s *stubErrorsRepo) ResolveByCodes(ctx context.Context, accountID uuid.UUID, codes []string, resolvedBy string) *errx.Error {
	s.resolved = append(s.resolved, codes...)
	return nil
}

func reauthFixture(provider, email string) (*emailService, *stubReauthRepo, *stubErrorsRepo, *models.EmailOnboardingState) {
	org := uuid.New()
	accountID := uuid.New()
	repo := &stubReauthRepo{
		account: &models.Email{
			ID:             accountID,
			UserID:         uuid.NewString(),
			OrganizationID: &org,
			Email:          email,
			Provider:       provider,
			Status:         "inactive",
		},
		storedRefresh: "stored-refresh",
	}
	errs := &stubErrorsRepo{}
	svc := &emailService{emailRepository: repo, accountErrors: errs}
	sess := &models.EmailOnboardingState{
		UserID:         repo.account.UserID,
		OrganizationID: &org,
		Provider:       provider,
		EmailAccountID: &accountID,
	}
	return svc, repo, errs, sess
}

func TestUpdateSMTPIMAPCredentials_RefusesOAuthMailboxes(t *testing.T) {
	svc, repo, _, _ := reauthFixture("gmail", "owner@example.com")

	creds := &models.SmtpImap{
		SMTP: &models.Service{Host: "smtp.example.com", Port: 587},
		IMAP: &models.Service{Host: "imap.example.com", Port: 993},
	}
	_, xerr := svc.UpdateSMTPIMAPCredentials(context.Background(), repo.account.OrganizationID, repo.account.ID, creds)
	if xerr != errx.ErrEmailReauthOAuthOnly {
		t.Fatalf("expected ErrEmailReauthOAuthOnly, got %v", xerr)
	}
}
