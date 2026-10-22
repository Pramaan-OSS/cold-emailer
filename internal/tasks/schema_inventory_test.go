package tasks

import (
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pkg/generation"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"testing"
)

func TestEveryCampaignAgentSchema(t *testing.T) {
	s := &tasksService{}
	c := &models.Campaign{}
	contact := &models.Contact{}
	cfg := &models.ActionConfig{}
	defs := []generation.ToolDef{s.campaignTaskTool(c, contact, uuid.Nil, cfg), s.campaignUnsubscribeTool(c, contact, uuid.Nil, cfg)}
	for _, kind := range []string{"add_tag", "remove_tag", "label_email"} {
		defs = append(defs, s.campaignTagTool(c, contact, uuid.Nil, cfg, uuid.Nil, kind, nil, nil))
	}
	for _, kind := range []string{"create_deal", "move_deal_stage"} {
		defs = append(defs, s.campaignDealTool(c, contact, uuid.Nil, cfg, kind))
	}
	for _, def := range defs {
		if e := pramaan.ValidateSchema("campaign "+def.Name, def.InputSchema); e != nil {
			t.Fatal(e)
		}
	}
}
