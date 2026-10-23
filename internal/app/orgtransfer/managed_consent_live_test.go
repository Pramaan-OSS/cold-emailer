package orgtransfer

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/models"
)

func TestLiveImportDoesNotRestoreRemovedManagedConsentTables(t *testing.T) {
	svc, pool := liveImportService(t)
	dest := newTenantFixture(t, pool)
	ctx := context.Background()
	for _, table := range []string{"cloud_managed_consents", "pool_link_managed_operations"} {
		for _, state := range []string{"revoked", "pending"} {
			id := uuid.New()
			row := map[string]any{"id": id, "organization_id": uuid.New(), "instance_id": uuid.New(), "remote_id": uuid.New(),
				"planned_account_id": uuid.New(), "session_hash": "opaque-source-handle-hash", "kind": "oauth", "provider": "gmail", "consent_state": state, "expires_at": time.Now().Add(time.Minute)}
			if table == "cloud_managed_consents" {
				row["user_id"] = dest.owner
				row["cloud_account_id"] = uuid.New()
			}
			archive := tenantArchive(t, map[string][]map[string]any{table: {row}})
			if _, err := svc.ImportFrom(ctx, dest.org, archive, ImportOptions{Conflict: models.OrgImportConflictOverwrite, ActorUserID: dest.owner}, nil); err != nil {
				t.Fatal(err)
			}
			// These upstream tables were removed from the template. Importing an
			// old archive must not restore consent handles or recreate the tables.
			var absent bool
			if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NULL`, "public."+table).Scan(&absent); err != nil {
				t.Fatal(err)
			}
			if !absent {
				t.Fatalf("import restored removed table %s", table)
			}
		}
	}
}
