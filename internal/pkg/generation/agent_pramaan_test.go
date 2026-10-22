package generation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type agentFixture struct{ calls int }

func (f *agentFixture) Call(_ context.Context, id string, in, out any, _ ...pramaan.Option) error {
	if id != "llm.chat" {
		return errors.New("unexpected capability")
	}
	if e := pramaan.ValidateInput(id, in); e != nil {
		return e
	}
	f.calls++
	response := `{"text":"Ready to update","toolCalls":[{"id":"call-1","name":"update_contact","arguments":{"name":"Local fixture"}}]}`
	if f.calls > 1 {
		response = `{"text":"Saved","toolCalls":[]}`
	}
	return json.Unmarshal([]byte(response), out)
}
func TestAgentPausesBeforeChangingData(t *testing.T) {
	f := &agentFixture{}
	writes := 0
	c := GenerationClient{client: f}
	tool := ToolDef{Name: "update_contact", Risk: RiskWrite, Handler: func(context.Context, json.RawMessage) (string, error) { writes++; return "saved", nil }}
	r, err := c.RunAgent(context.Background(), AgentRequest{Messages: []AgentMessage{{Role: "user", Content: "Synthetic request"}}, Tools: []ToolDef{tool}})
	if err != nil || r.Pending == nil || r.StopReason != "approval_required" || writes != 0 {
		t.Fatalf("write escaped approval: %+v %v writes=%d", r, err, writes)
	}
}
func TestAgentApprovedWriteRunsOnce(t *testing.T) {
	f := &agentFixture{}
	writes := 0
	approvals := 0
	c := GenerationClient{client: f}
	tool := ToolDef{Name: "update_contact", Risk: RiskWrite, Handler: func(context.Context, json.RawMessage) (string, error) { writes++; return "saved", nil }}
	r, err := c.RunAgent(context.Background(), AgentRequest{Messages: []AgentMessage{{Role: "user", Content: "Synthetic request"}}, Tools: []ToolDef{tool}, Approve: func(context.Context, ToolDef, ToolCall) error { approvals++; return nil }})
	if err != nil || r.Text != "Saved" || writes != 1 || approvals != 1 {
		t.Fatalf("%+v %v writes=%d approvals=%d", r, err, writes, approvals)
	}
}
func TestAgentRefusalDoesNotWrite(t *testing.T) {
	f := &agentFixture{}
	writes := 0
	c := GenerationClient{client: f}
	tool := ToolDef{Name: "update_contact", Risk: RiskWrite, Handler: func(context.Context, json.RawMessage) (string, error) { writes++; return "", nil }}
	_, err := c.RunAgent(context.Background(), AgentRequest{Messages: []AgentMessage{{Role: "user", Content: "Synthetic request"}}, Tools: []ToolDef{tool}, Approve: func(context.Context, ToolDef, ToolCall) error { return errors.New("declined") }})
	if err == nil || writes != 0 {
		t.Fatal("declined action changed data")
	}
}

func TestInvalidToolFailsBeforeFakeCall(t *testing.T) {
	f := &agentFixture{}
	c := NewClient(f)
	_, e := c.RunAgent(context.Background(), AgentRequest{Messages: []AgentMessage{{Role: "user", Content: "Synthetic request"}}, Tools: []ToolDef{{Name: "update_contact", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"email": map[string]any{"pattern": ".+"}}}}}})
	if e == nil || f.calls != 0 {
		t.Fatal("invalid tool reached fake", e, f.calls)
	}
	var se *pramaan.SchemaError
	if !errors.As(e, &se) || se.Name != "tool update_contact" || se.Path != "#/properties/email/pattern" {
		t.Fatal(e)
	}
}
func TestFailedApprovedActionStopsAndRemainsReviewable(t *testing.T) {
	f := &agentFixture{}
	c := NewClient(f)
	writes, successes := 0, 0
	r, e := c.RunAgent(context.Background(), AgentRequest{Messages: []AgentMessage{{Role: "user", Content: "Synthetic request"}}, Tools: []ToolDef{{Name: "update_contact", Risk: RiskWrite, Handler: func(context.Context, json.RawMessage) (string, error) {
		writes++
		return "", errors.New("uncertain completion")
	}}}, Approve: func(context.Context, ToolDef, ToolCall) error { return nil }, OnEvent: func(ev AgentEvent) {
		if ev.Type == EventToolResult {
			successes++
		}
	}})
	if e == nil || r.StopReason != "failed" || f.calls != 1 || writes != 1 || successes != 0 {
		t.Fatalf("false success or retry: %+v %v", r, e)
	}
	if r.Messages[len(r.Messages)-1].Role != "tool" || !strings.Contains(r.Messages[len(r.Messages)-1].Content, "check before retrying") {
		t.Fatal("failed action history lost", r.Messages)
	}
}

type failingModel struct {
	calls int
	fail  bool
}

func (f *failingModel) Call(_ context.Context, id string, in, out any, _ ...pramaan.Option) error {
	f.calls++
	if e := pramaan.ValidateInput(id, in); e != nil {
		return e
	}
	if f.fail {
		return errors.New("generation unavailable")
	}
	return json.Unmarshal([]byte(`{"text":"Ready"}`), out)
}
func TestModelFailureCanBeRetriedExplicitly(t *testing.T) {
	f := &failingModel{fail: true}
	c := NewClient(f)
	req := AgentRequest{Messages: []AgentMessage{{Role: "user", Content: "Draft a message"}}}
	r, e := c.RunAgent(context.Background(), req)
	if e == nil || r.StopReason != "failed" || f.calls != 1 {
		t.Fatal(r, e, f.calls)
	}
	f.fail = false
	r, e = c.RunAgent(context.Background(), req)
	if e != nil || r.StopReason != "stop" || f.calls != 2 {
		t.Fatal(r, e, f.calls)
	}
}
