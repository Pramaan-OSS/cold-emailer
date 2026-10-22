package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnterpriseConnection stores a stable local identity namespace and a platform reference.
// Issuers, client ids and secrets are not stored here.
type EnterpriseConnection struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Ref            string    `json:"-"`
	Name           string    `json:"name"`
	CreatedAt      time.Time `json:"created_at"`
}
type EnterpriseConnections interface {
	Save(context.Context, uuid.UUID, string, string, string) (EnterpriseConnection, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (EnterpriseConnection, error)
	List(context.Context, uuid.UUID) ([]EnterpriseConnection, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}
type enterpriseConnections struct{ db *pgxpool.Pool }

func NewEnterpriseConnections(db *pgxpool.Pool) EnterpriseConnections {
	return &enterpriseConnections{db}
}
func (r *enterpriseConnections) Save(ctx context.Context, org uuid.UUID, ref, name, created string) (EnterpriseConnection, error) {
	var out EnterpriseConnection
	t, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return out, err
	}
	err = r.db.QueryRow(ctx, `INSERT INTO enterprise_connections(id,organization_id,ref,name,created_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(organization_id,name,created_at) DO UPDATE SET name=EXCLUDED.name RETURNING id,organization_id,ref,name,created_at`, uuid.New(), org, ref, name, t).Scan(&out.ID, &out.OrganizationID, &out.Ref, &out.Name, &out.CreatedAt)
	return out, err
}
func (r *enterpriseConnections) Get(ctx context.Context, org, id uuid.UUID) (EnterpriseConnection, error) {
	var out EnterpriseConnection
	err := r.db.QueryRow(ctx, `SELECT id,organization_id,ref,name,created_at FROM enterprise_connections WHERE organization_id=$1 AND id=$2`, org, id).Scan(&out.ID, &out.OrganizationID, &out.Ref, &out.Name, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, errors.New("Organisation connection is unavailable.")
	}
	return out, err
}
func (r *enterpriseConnections) List(ctx context.Context, org uuid.UUID) ([]EnterpriseConnection, error) {
	rows, err := r.db.Query(ctx, `SELECT id,organization_id,ref,name,created_at FROM enterprise_connections WHERE organization_id=$1 ORDER BY created_at,id LIMIT 200`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnterpriseConnection{}
	for rows.Next() {
		var row EnterpriseConnection
		if err = rows.Scan(&row.ID, &row.OrganizationID, &row.Ref, &row.Name, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
func (r *enterpriseConnections) Delete(ctx context.Context, org, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM enterprise_connections WHERE organization_id=$1 AND id=$2`, org, id)
	return err
}
