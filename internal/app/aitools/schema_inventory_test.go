package aitools

import (
	"context"
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"github.com/pramaan-oss/cold-email/internal/repository"
	"testing"
)

type inventoryAdvisor struct{}

func (inventoryAdvisor) List(context.Context, uuid.UUID, repository.AdvisorFindingFilter) ([]models.AdvisorFinding, *errx.Error) {
	return nil, nil
}
func (inventoryAdvisor) Summary(context.Context, uuid.UUID) (*models.AdvisorSummary, *errx.Error) {
	return nil, nil
}
func TestEveryWorkspaceToolSchema(t *testing.T) {
	r := BuildRegistry(Deps{})
	RegisterAdvisorTools(r, inventoryAdvisor{})
	for _, name := range r.order {
		t.Run(name, func(t *testing.T) {
			if e := pramaan.ValidateSchema("tool "+name, r.tools[name].InputSchema); e != nil {
				t.Fatal(e)
			}
		})
	}
	t.Logf("validated all %d workspace tool schemas", len(r.order))
}
func TestInvalidBuiltinSchemaFailsAtStartup(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("invalid builtin registered")
		}
	}()
	NewRegistry().Register(Tool{Name: "invalid_builtin", InputSchema: map[string]any{"type": "object", "oneOf": []any{map[string]any{}}}})
}
