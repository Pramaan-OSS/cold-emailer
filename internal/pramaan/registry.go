package pramaan

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Registry interface {
	Owns(context.Context, string, string, string) (bool, error)
	Save(context.Context, string, string, string) error
	List(context.Context, string, string) ([]string, error)
	Remove(context.Context, string, string, string) error
}
type PGRegistry struct{ DB *pgxpool.Pool }

func (r PGRegistry) Owns(ctx context.Context, owner, kind, handle string) (bool, error) {
	var yes bool
	if kind == "contact" {
		e := r.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM contacts WHERE organization_id::text=$1 AND lower(email)=lower(btrim($2)))", owner, handle).Scan(&yes)
		return yes, e
	}
	e := r.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pramaan_resources WHERE owner=$1 AND kind=$2 AND handle=$3)", owner, kind, handle).Scan(&yes)
	return yes, e
}
func (r PGRegistry) Save(ctx context.Context, owner, kind, handle string) error {
	if handle == "" {
		return nil
	}
	if kind != "action" && kind != "definition" && kind != "app_connection" {
		var other bool
		if e := r.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pramaan_resources WHERE owner<>$1 AND kind=$2 AND handle=$3)", owner, kind, handle).Scan(&other); e != nil {
			return e
		}
		if other {
			return errors.New("This resource is not available in this workspace.")
		}
	}
	_, e := r.DB.Exec(ctx, "INSERT INTO pramaan_resources(owner,kind,handle) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", owner, kind, handle)
	if e != nil {
		return e
	}
	own, e := r.Owns(ctx, owner, kind, handle)
	if e != nil {
		return e
	}
	if !own {
		return errors.New("This resource is not available in this workspace.")
	}
	return nil
}
func (r PGRegistry) List(ctx context.Context, owner, kind string) ([]string, error) {
	rows, e := r.DB.Query(ctx, "SELECT handle FROM pramaan_resources WHERE owner=$1 AND kind=$2 ORDER BY created_at", owner, kind)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if e = rows.Scan(&s); e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r PGRegistry) Remove(ctx context.Context, owner, kind, handle string) error {
	_, err := r.DB.Exec(ctx, "DELETE FROM pramaan_resources WHERE owner=$1 AND kind=$2 AND handle=$3", owner, kind, handle)
	return err
}

// Claim records a purchase attempt atomically, including across workspaces.
// A timeout keeps the claim; only an explicit pre-effect refusal can release it.
func (r PGRegistry) Claim(ctx context.Context, owner, kind, handle string) (bool, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", kind+":"+handle); err != nil {
		return false, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pramaan_resources WHERE kind=$1 AND handle=$2)", kind, handle).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	if _, err = tx.Exec(ctx, "INSERT INTO pramaan_resources(owner,kind,handle) VALUES($1,$2,$3)", owner, kind, handle); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
