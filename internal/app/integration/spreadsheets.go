package integration

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

// SheetsClient runs the exact handles selected from connected-action discovery.
type SheetsClient struct {
	client                 pramaan.Caller
	actor                  string
	metadata, rows, append string
}

// SheetRange describes a contiguous block of cells using A1 notation.
type SheetRange struct {
	SheetID string `json:"sheet_id"`
	A1Range string `json:"a1_range"`
}

// SheetMeta is what GetSpreadsheet returns. Only the bits the dashboard
// uses are modelled.
type SheetMeta struct {
	SheetID string `json:"sheet_id"`
	Title   string `json:"title"`
	Tabs    []struct {
		Title string `json:"title"`
		Index int    `json:"index"`
	} `json:"tabs"`
}

func (c *SheetsClient) GetSpreadsheet(ctx context.Context, id string) (*SheetMeta, error) {
	out := &SheetMeta{}
	err := c.run(ctx, c.metadata, map[string]any{"spreadsheet_id": id}, out)
	return out, err
}
func (c *SheetsClient) ReadValues(ctx context.Context, id, rangeName string) ([][]string, error) {
	var out [][]string
	err := c.run(ctx, c.rows, map[string]any{"spreadsheet_id": id, "range": rangeName}, &out)
	return out, err
}
func (c *SheetsClient) AppendValues(ctx context.Context, id, rangeName string, rows [][]string) error {
	return c.run(ctx, c.append, map[string]any{"spreadsheet_id": id, "range": rangeName, "rows": rows}, nil)
}
func (c *SheetsClient) run(ctx context.Context, action string, input map[string]any, dst any) error {
	if c.client == nil || action == "" {
		return errors.New("Choose the spreadsheet action and its schema in Connections first.")
	}
	apps := pramaan.Apps{Client: c.client}
	if _, err := apps.Inspect(ctx, action); err != nil {
		return err
	}
	out, err := apps.Execute(ctx, c.actor, action, input, uuid.NewString())
	if err != nil {
		return err
	}
	if dst == nil {
		return nil
	}
	value := out["output"]
	if value == nil {
		value = out["results"]
	}
	if value == nil {
		value = out["data"]
	}
	if object, ok := value.(map[string]any); ok {
		if rows, exists := object["results"]; exists {
			value = rows
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}
