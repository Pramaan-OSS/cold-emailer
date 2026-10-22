package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/pramaan-oss/cold-email/internal/infrastructure/db"
	"github.com/pramaan-oss/cold-email/internal/models"
)

// DomainGrantRepository stores administrator grants over whole domains. Every
// read a caller can reach is scoped by organization.
type DomainGrantRepository interface {
	// Upsert records a verified grant, replacing a previous one for the same tenant.
	Upsert(ctx context.Context, g *models.DomainGrant, createdBy uuid.UUID) error
	List(ctx context.Context, orgID uuid.UUID) ([]models.DomainGrant, error)
	Get(ctx context.Context, orgID, id uuid.UUID) (*models.DomainGrant, error)
	// GetByID skips the organization; the caller checks it against the mailbox.
	GetByID(ctx context.Context, id uuid.UUID) (*models.DomainGrant, error)
	SetStatus(ctx context.Context, id uuid.UUID, status, lastError string) error
	// Delete removes the grant and returns the mailboxes it connected.
	Delete(ctx context.Context, orgID, id uuid.UUID) ([]uuid.UUID, error)
	// ForDomain is the workspace's active grant covering a domain for a provider, nil when none.
	ForDomain(ctx context.Context, orgID uuid.UUID, provider, domain string) (*models.DomainGrant, error)
	ListActive(ctx context.Context) ([]models.DomainGrant, error)
	// InactiveMailboxes are the grant's mailboxes a failure took out of work, never one switched off by hand.
	InactiveMailboxes(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error)
}

type domainGrantRepository struct{ DB *db.DB }

func NewDomainGrantRepository(database *db.DB) DomainGrantRepository {
	return &domainGrantRepository{DB: database}
}

const grantColumns = `g.id, g.organization_id, g.provider, g.tenant, g.admin_email, g.domains, g.status, g.last_error,
	g.verified_at, g.created_at,
	(SELECT count(*) FROM email_accounts ea WHERE ea.domain_grant_id = g.id)`

func scanGrant(row pgx.Row, g *models.DomainGrant) error {
	return row.Scan(&g.ID, &g.OrganizationID, &g.Provider, &g.Tenant, &g.AdminEmail, &g.Domains, &g.Status, &g.LastError,
		&g.VerifiedAt, &g.CreatedAt, &g.Mailboxes)
}

func (r *domainGrantRepository) Upsert(ctx context.Context, g *models.DomainGrant, createdBy uuid.UUID) error {
	lowered := make([]string, 0, len(g.Domains))
	for _, d := range g.Domains {
		lowered = append(lowered, strings.ToLower(strings.TrimSpace(d)))
	}
	query := `
		INSERT INTO mailbox_domain_grants (id, organization_id, provider, tenant, admin_email, domains, status, last_error, verified_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, 'active', '', now(), $7)
		ON CONFLICT (organization_id, provider, tenant) DO UPDATE
		SET admin_email = EXCLUDED.admin_email, domains = EXCLUDED.domains, status = 'active', last_error = '',
		    verified_at = now(), updated_at = now()
		RETURNING id`
	if err := r.DB.QueryRow(ctx, query, g.ID, g.OrganizationID, g.Provider, g.Tenant, g.AdminEmail, lowered, createdBy).Scan(&g.ID); err != nil {
		db.CaptureError(err, query, nil, "queryrow")
		return err
	}
	return nil
}

func (r *domainGrantRepository) List(ctx context.Context, orgID uuid.UUID) ([]models.DomainGrant, error) {
	query := `SELECT ` + grantColumns + ` FROM mailbox_domain_grants g WHERE g.organization_id = $1 ORDER BY g.created_at`
	rows, err := r.DB.Query(ctx, query, orgID)
	if err != nil {
		db.CaptureError(err, query, nil, "query")
		return nil, err
	}
	defer rows.Close()
	out := make([]models.DomainGrant, 0)
	for rows.Next() {
		var g models.DomainGrant
		if err := scanGrant(rows, &g); err != nil {
			db.CaptureError(err, query, nil, "scan")
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *domainGrantRepository) one(ctx context.Context, query string, args ...any) (*models.DomainGrant, error) {
	var g models.DomainGrant
	if err := scanGrant(r.DB.QueryRow(ctx, query, args...), &g); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		db.CaptureError(err, query, nil, "queryrow")
		return nil, err
	}
	return &g, nil
}

func (r *domainGrantRepository) Get(ctx context.Context, orgID, id uuid.UUID) (*models.DomainGrant, error) {
	return r.one(ctx, `SELECT `+grantColumns+` FROM mailbox_domain_grants g WHERE g.organization_id = $1 AND g.id = $2`, orgID, id)
}

func (r *domainGrantRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.DomainGrant, error) {
	return r.one(ctx, `SELECT `+grantColumns+` FROM mailbox_domain_grants g WHERE g.id = $1`, id)
}

func (r *domainGrantRepository) ForDomain(ctx context.Context, orgID uuid.UUID, provider, domain string) (*models.DomainGrant, error) {
	return r.one(ctx, `SELECT `+grantColumns+` FROM mailbox_domain_grants g
		WHERE g.organization_id = $1 AND g.provider = $2 AND g.status = 'active' AND lower($3) = ANY(g.domains)
		ORDER BY g.verified_at DESC NULLS LAST LIMIT 1`, orgID, provider, domain)
}

func (r *domainGrantRepository) SetStatus(ctx context.Context, id uuid.UUID, status, lastError string) error {
	query := `UPDATE mailbox_domain_grants SET status = $2, last_error = $3, updated_at = now(),
		verified_at = CASE WHEN $2 = 'active' THEN now() ELSE verified_at END WHERE id = $1`
	if _, err := r.DB.Exec(ctx, query, id, status, lastError); err != nil {
		db.CaptureError(err, query, nil, "exec")
		return err
	}
	return nil
}

func (r *domainGrantRepository) Delete(ctx context.Context, orgID, id uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	query := `SELECT ea.id FROM email_accounts ea JOIN mailbox_domain_grants g ON g.id = ea.domain_grant_id
		WHERE g.organization_id = $1 AND g.id = $2`
	rows, err := r.DB.Query(ctx, query, orgID, id)
	if err != nil {
		db.CaptureError(err, query, nil, "query")
		return nil, err
	}
	for rows.Next() {
		var a uuid.UUID
		if err := rows.Scan(&a); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, a)
	}
	rows.Close()
	if _, err := r.DB.Exec(ctx, `DELETE FROM mailbox_domain_grants WHERE organization_id = $1 AND id = $2`, orgID, id); err != nil {
		db.CaptureError(err, "delete mailbox_domain_grants", nil, "exec")
		return nil, err
	}
	return ids, nil
}

func (r *domainGrantRepository) ListActive(ctx context.Context) ([]models.DomainGrant, error) {
	query := `SELECT ` + grantColumns + ` FROM mailbox_domain_grants g ORDER BY g.updated_at LIMIT 500`
	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		db.CaptureError(err, query, nil, "query")
		return nil, err
	}
	defer rows.Close()
	var out []models.DomainGrant
	for rows.Next() {
		var g models.DomainGrant
		if err := scanGrant(rows, &g); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *domainGrantRepository) InactiveMailboxes(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	// Only mailboxes a failure stopped: revoked (a workspace import, a provider
	// revocation) or inactive with an unresolved error. One switched off by hand has no error.
	query := `SELECT id FROM email_accounts ea WHERE ea.domain_grant_id = $1
		AND (ea.status = 'revoked' OR (ea.status = 'inactive' AND EXISTS (
			SELECT 1 FROM email_account_errors e WHERE e.email_account_id = ea.id AND e.resolved_at IS NULL)))`
	rows, err := r.DB.Query(ctx, query, id)
	if err != nil {
		db.CaptureError(err, query, nil, "query")
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var a uuid.UUID
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
