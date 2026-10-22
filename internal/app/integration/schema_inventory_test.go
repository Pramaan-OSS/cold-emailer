package integration

import (
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pkg/generation"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"testing"
)

func TestEveryAutomationAgentSchema(t *testing.T) {
	s := &service{}
	a := models.Automation{}
	n := models.AutomationNode{}
	data := map[string]any{}
	defs := []generation.ToolDef{s.aiTaskTool(a, n, data, false), s.aiSetVarsTool(a, n, data, false), s.aiUnsubscribeTool(a, n, data, false)}
	for _, action := range []models.IntegrationAction{models.IntegrationActionAddTag, models.IntegrationActionRemoveTag, models.IntegrationActionLabelEmail} {
		defs = append(defs, s.aiTagTool(a, n, data, false, uuid.Nil, action, nil, nil, true))
	}
	for _, action := range []models.IntegrationAction{models.IntegrationActionCreateDeal, models.IntegrationActionMoveDealStage} {
		defs = append(defs, s.aiDealTool(a, n, data, false, action))
	}
	for _, def := range defs {
		if e := pramaan.ValidateSchema("automation "+def.Name, def.InputSchema); e != nil {
			t.Fatal(e)
		}
	}
}
