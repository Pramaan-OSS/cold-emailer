package generation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type replyFixture struct {
	replies []string
	calls   int
	inputs  []any
	err     error
}

func (f *replyFixture) Call(_ context.Context, id string, in, out any, _ ...pramaan.Option) error {
	if e := pramaan.ValidateInput(id, in); e != nil {
		return e
	}
	f.inputs = append(f.inputs, in)
	f.calls++
	if f.err != nil {
		return f.err
	}
	return json.Unmarshal([]byte(f.replies[min(f.calls-1, len(f.replies)-1)]), out)
}

const cutOff = `{"text":"Partial ","finishReason":"length","usage":{"inputTokens":10,"outputTokens":20}}`

func TestPlainReplyContinuesOnceAndCountsBothPaidReplies(t *testing.T) {
	f := &replyFixture{replies: []string{cutOff, `{"text":"finished.","finishReason":"stop","usage":{"inputTokens":5,"outputTokens":6}}`}}
	r, e := NewClient(f).Complete(context.Background(), CompletionRequest{Prompt: "Synthetic", MaxTokens: 64})
	if e != nil || r.Text != "Partial finished." || r.TokensUsed != 41 || f.calls != 2 {
		t.Fatal(r, e, f.calls)
	}
	b, _ := json.Marshal(f.inputs[1])
	var in map[string]any
	_ = json.Unmarshal(b, &in)
	if in["maxTokens"].(float64) < 256 || len(in["messages"].([]any)) != 4 {
		t.Fatal(string(b))
	}
}
func TestSecondCutOffFailsWithoutThirdCall(t *testing.T) {
	f := &replyFixture{replies: []string{cutOff}}
	r, e := NewClient(f).Complete(context.Background(), CompletionRequest{Prompt: "Synthetic"})
	if r != nil || !ChargedFailure(e) || f.calls != 2 {
		t.Fatal(r, e, f.calls)
	}
}
func TestStructuredReplyNeverParsesCutOffJSON(t *testing.T) {
	f := &replyFixture{replies: []string{`{"text":"{\"subject\":","finishReason":"length"}`}}
	r, e := NewClient(f).GenerateConversation(context.Background(), "synthetic", "balanced", 1)
	var partial *pramaan.IncompleteReply
	if r != nil || !errors.As(e, &partial) || !partial.Structured || f.calls != 1 {
		t.Fatal(r, e, f.calls)
	}
}
func TestStreamedCutOffShowsPartialButNeverCompletes(t *testing.T) {
	f := &replyFixture{replies: []string{cutOff}}
	events := []AgentEvent{}
	r, e := NewClient(f).RunAgent(context.Background(), AgentRequest{Messages: []AgentMessage{{Role: "user", Content: "Synthetic"}}, OnEvent: func(e AgentEvent) { events = append(events, e) }})
	if e == nil || r.StopReason != "failed" || r.TokensUsed != 30 || r.Text != "Partial " || len(events) != 1 || events[0].Type != EventText || f.calls != 1 {
		t.Fatal(r, e, events)
	}
}
func TestCutOffActionCallsNeverExecute(t *testing.T) {
	f := &replyFixture{replies: []string{`{"text":"partial","finishReason":"length","toolCalls":[{"id":"tc_fixture","name":"read_data","arguments":{}}]}`}}
	actions := 0
	r, e := NewClient(f).RunAgent(context.Background(), AgentRequest{Messages: []AgentMessage{{Role: "user", Content: "Synthetic"}}, Tools: []ToolDef{{Name: "read_data", Risk: RiskRead, Handler: func(context.Context, json.RawMessage) (string, error) { actions++; return "ok", nil }}}})
	if e == nil || r.StopReason != "failed" || actions != 0 || r.Text != "" || f.calls != 1 {
		t.Fatal(r, e, actions)
	}
}
func TestReplyLimitConfigurationErrorDoesNotRetry(t *testing.T) {
	f := &replyFixture{err: pramaan.ErrReplyLimit}
	r, e := NewClient(f).Complete(context.Background(), CompletionRequest{Prompt: "Synthetic"})
	if r != nil || !errors.Is(e, pramaan.ErrReplyLimit) || ChargedFailure(e) || f.calls != 1 {
		t.Fatal(r, e, f.calls)
	}
}

func TestStreamedLimitRefusalDoesNotContinue(t *testing.T) {
	f := &replyFixture{err: pramaan.ErrReplyLimit}
	r, e := NewClient(f).RunAgent(context.Background(), AgentRequest{Messages: []AgentMessage{{Role: "user", Content: "Synthetic"}}})
	if !errors.Is(e, pramaan.ErrReplyLimit) || r.StopReason != "failed" || f.calls != 1 || r.TokensUsed != 0 {
		t.Fatal(r, e, f.calls)
	}
}
func TestStructuredLimitRefusalDoesNotRetry(t *testing.T) {
	f := &replyFixture{err: pramaan.ErrReplyLimit}
	r, e := NewClient(f).GenerateConversation(context.Background(), "synthetic", "balanced", 1)
	if r != nil || !errors.Is(e, pramaan.ErrReplyLimit) || f.calls != 1 {
		t.Fatal(r, e, f.calls)
	}
}
func TestBatchCutOffIsNeverParsed(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"job_id":"fixture","status":"done","result":{"results":[{"id":"row-1","found":true,"text":"{\"subject\":","finishReason":"length"}]}}`))
	}))
	defer s.Close()
	client, _ := pramaan.New("fixture", s.URL, nil)
	rows, e := NewClient(client).FetchBatchResults(context.Background(), "fixture")
	if e != nil || len(rows) != 1 || rows[0].Conversation != nil || rows[0].Err == "" {
		t.Fatal(rows, e)
	}
}

func TestMachineParsedPlainReplyDoesNotContinue(t *testing.T) {
	f := &replyFixture{replies: []string{cutOff}}
	r, e := NewClient(f).Complete(context.Background(), CompletionRequest{Structured: true, Prompt: "Return a decision"})
	var partial *pramaan.IncompleteReply
	if r != nil || !errors.As(e, &partial) || !partial.Structured || f.calls != 1 {
		t.Fatal(r, e, f.calls)
	}
}
