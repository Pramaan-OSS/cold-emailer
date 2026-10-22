package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"testing"
)

func manyTools() []ToolDef {
	out := []ToolDef{}
	for i := 0; i < 130; i++ {
		out = append(out, ToolDef{Name: fmt.Sprintf("workspace_action_%03d", i), Risk: RiskRead, InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}})
	}
	return out
}
func TestEveryLargeCatalogueActionRemainsDiscoverable(t *testing.T) {
	defs := manyTools()
	c, e := newToolCatalog(defs, nil)
	if e != nil {
		t.Fatal(e)
	}
	c.bindDiscovery(func() []AgentMessage { return nil })
	for _, target := range defs {
		c.query = target.Name
		selected, e := c.selection(nil)
		if e != nil || len(selected) > 64 {
			t.Fatal(e, len(selected))
		}
		found := false
		for _, x := range selected {
			if x.Name == target.Name {
				found = true
			}
		}
		if !found {
			t.Fatal("action silently dropped", target.Name)
		}
	}
	history := []AgentMessage{{Role: "assistant", ToolCalls: []ToolCall{{Name: defs[0].Name}, {Name: discoveryName}}}}
	c.query = defs[129].Name
	selected, e := c.selection(history)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, x := range selected {
		if x.Name == defs[0].Name {
			found = true
		}
	}
	if !found {
		t.Fatal("historical definition lost")
	}
}

type discoveringFixture struct{ calls int }

func (f *discoveringFixture) Call(_ context.Context, id string, in, out any, _ ...pramaan.Option) error {
	if e := pramaan.ValidateInput(id, in); e != nil {
		return e
	}
	f.calls++
	b, _ := json.Marshal(in)
	var request struct{ Tools []struct{ Name string } }
	_ = json.Unmarshal(b, &request)
	if len(request.Tools) > 64 {
		return fmt.Errorf("platform refuses more than 64 actions")
	}
	if f.calls == 1 {
		return json.Unmarshal([]byte(`{"toolCalls":[{"id":"tc_discover","name":"find_workspace_actions","arguments":{"query":"update_special_record"}}]}`), out)
	}
	found := false
	for _, tool := range request.Tools {
		if tool.Name == "update_special_record" {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("requested definition missing after discovery")
	}
	if f.calls == 2 {
		return json.Unmarshal([]byte(`{"toolCalls":[{"id":"tc_write","name":"update_special_record","arguments":{}}]}`), out)
	}
	return json.Unmarshal([]byte(`{"text":"Saved","toolCalls":[],"finishReason":"stop"}`), out)
}
func TestDiscoveredWriteStillRequiresApproval(t *testing.T) {
	for _, approved := range []bool{false, true} {
		t.Run(fmt.Sprint(approved), func(t *testing.T) {
			defs := manyTools()
			writes := 0
			defs = append(defs, ToolDef{Name: "update_special_record", Risk: RiskWrite, Handler: func(context.Context, json.RawMessage) (string, error) { writes++; return "saved", nil }})
			fake := &discoveringFixture{}
			req := AgentRequest{Messages: []AgentMessage{{Role: "user", Content: "Help with my workspace"}}, Tools: defs}
			if approved {
				req.Approve = func(context.Context, ToolDef, ToolCall) error { return nil }
			}
			r, e := NewClient(fake).RunAgent(context.Background(), req)
			if e != nil {
				t.Fatal(e)
			}
			if approved {
				if writes != 1 || r.StopReason != "stop" || fake.calls != 3 {
					t.Fatal(writes, r.StopReason, fake.calls)
				}
			} else if writes != 0 || r.Pending == nil || r.Pending.Call.Name != "update_special_record" || fake.calls != 2 {
				t.Fatal("discovery bypassed approval", writes, r, fake.calls)
			}
		})
	}
}
