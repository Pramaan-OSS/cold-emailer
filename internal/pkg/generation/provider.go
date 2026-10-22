package generation

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

// Shared agent-loop contract. Generation goes through Pramaan capabilities.

// RiskClass classifies what a tool can do, which drives the approval policy in
// the agent loop (read auto-runs; write pauses for approval; send always
// requires an explicit per-action human action).
type RiskClass string

const (
	RiskRead  RiskClass = "read"
	RiskWrite RiskClass = "write"
	RiskSend  RiskClass = "send"
)

// ToolDef is the LLM-facing definition of a tool the agent loop can call. It is
// the provider-level projection of a richer aitools.Tool (M2): only what the
// model needs (name, description, JSON-schema input) plus the Go handler the
// loop executes. Permission gating and approval happen in the caller/registry,
// not inside the provider.
type ToolDef struct {
	Name        string
	Description string
	// InputSchema is a JSON Schema object describing the tool's arguments.
	InputSchema map[string]any
	Risk        RiskClass
	// Handler executes the tool for the given JSON args and returns a result
	// string (usually JSON) fed back to the model as the tool result. An error
	// stops the run as failed; actions with uncertain results are not retried.
	Handler func(ctx context.Context, args json.RawMessage) (string, error)
}

// ToolCall is one tool invocation the model requested, carried in an assistant
// message so the transcript is provider-agnostic and resumable (M3 persists it
// as agent_messages jsonb).
type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// AgentMessage is one entry in the running transcript. Role is one of
// "user", "assistant", "tool". An assistant turn may carry ToolCalls; a tool
// turn carries ToolCallID + Content (the result).
type AgentMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// AgentEventType enumerates the streamed step kinds surfaced via OnEvent (the
// M3 SSE loop maps these to text deltas / tool_start / tool_result events).
type AgentEventType string

const (
	EventText AgentEventType = "text"
	// EventTextDelta is a partial text chunk streamed while the model is
	// still generating; a closing EventText with the full block always
	// follows, so consumers may treat deltas as purely cosmetic.
	EventTextDelta  AgentEventType = "text_delta"
	EventToolStart  AgentEventType = "tool_start"
	EventToolResult AgentEventType = "tool_result"
	EventIteration  AgentEventType = "iteration"
)

// AgentEvent is a single streamed step from RunAgent.
type AgentEvent struct {
	Type       AgentEventType
	Text       string
	ToolName   string
	ToolArgs   json.RawMessage
	ToolResult string
	Iteration  int
}

// AgentRequest is one RunAgent invocation.
type AgentRequest struct {
	// System is the system prompt.
	System string
	// Messages is the running transcript (may include prior tool turns for a
	// resumed run). At minimum one user message.
	Messages []AgentMessage
	// Tools available this run. May be empty for a plain completion.
	Tools []ToolDef
	// Model is the neutral generation tier; empty means the default for
	// the tier (see ModelForTier).
	Model string
	// MaxIterations bounds tool-use rounds (each model call is one iteration).
	// Zero uses the provider default.
	MaxIterations int
	// MaxTokens caps a single completion. Zero uses the provider default.
	MaxTokens int
	// OnEvent, if set, receives streamed step events. Optional.
	OnEvent func(AgentEvent)
	// Approve, if set, is called before executing a non-read tool. Returning
	// ErrApprovalRequired stops the loop and returns a result whose Pending
	// holds the tool call awaiting approval, so the caller can persist state
	// and resume. Returning any other error aborts the run.
	Approve func(ctx context.Context, tool ToolDef, call ToolCall) error
	// PreIteration, if set, is called at the start of each loop iteration
	// (before the model call). Returning an error stops the loop cleanly with
	// StopReason "stopped" and returns the transcript so far. Used to charge
	// per-iteration credits and enforce a per-run budget; the caller records
	// its own reason (e.g. out-of-credits) before returning.
	PreIteration func(ctx context.Context, iteration int) error
}

// PendingToolCall is the tool awaiting approval when a run stops with
// StopReason "approval_required".
type PendingToolCall struct {
	Call ToolCall
	Risk RiskClass
}

// AgentResult is the final output of a RunAgent run.
type AgentResult struct {
	// Text is the model's final assistant text (empty when the run stopped for
	// approval before producing final text).
	Text string
	// Messages is the full transcript including this run's new turns, so a
	// caller can persist it and resume.
	Messages []AgentMessage
	Model    string
	// TokensUsed is the summed input+output tokens across all iterations.
	TokensUsed int
	Iterations int
	// StopReason is "stop" (final text), "max_iterations", or
	// "approval_required".
	StopReason string
	// Pending is set only when StopReason == "approval_required".
	Pending *PendingToolCall
}

// CompletionRequest is a single, tool-less generation with an explicit system
// prompt. Used by reply drafts (M4) and the automation ai_step (generate mode).
type CompletionRequest struct {
	// Structured disables continuation for machine-parsed or single-label replies.
	Structured bool
	System     string
	Prompt     string
	Model      string
	MaxTokens  int
	// Temperature optionally pins sampling. nil leaves it to the provider default
	// (creative writing); a pointer to 0 forces deterministic output (the reply
	// classifier's Layer 3 needs a stable single-label verdict).
	Temperature *float64
}

// Deterministic returns a *float64 pointing at 0, for
// CompletionRequest.Temperature when a caller needs a stable, non-sampled result
// (e.g. single-label classification). Each call returns a fresh pointer.
func Deterministic() *float64 {
	z := 0.0
	return &z
}

// Provider is a pluggable LLM backend that can run a tool-use agent loop.
type Provider interface {
	// RunAgent executes the tool-use loop and returns the final result.
	RunAgent(ctx context.Context, req AgentRequest) (*AgentResult, error)
	// Complete runs a single completion (no tools) with an explicit system
	// prompt, returning the text + token usage.
	Complete(ctx context.Context, req CompletionRequest) (*WritingResult, error)
	// ModelForTier returns the neutral generation tier for the workspace.
	ModelForTier(paid bool) string
	// Name identifies the capability adapter for local accounting.
	Name() string
	// IsLocal reports whether generation runs locally. The Pramaan adapter is remote.
	IsLocal() bool
}

// ErrApprovalRequired is returned by an AgentRequest.Approve hook to pause the
// loop for human approval of a write/send tool. RunAgent turns it into a
// result with StopReason "approval_required".
var ErrApprovalRequired = errors.New("tool approval required")

// ErrProviderNotConfigured means Pramaan is not configured.
var ErrProviderNotConfigured = errors.New("Connect Pramaan to use AI writing.")

// Default agent-loop bounds, applied when a request leaves them zero.
const (
	defaultMaxIterations = 12
	defaultAgentTokens   = 2048
)

type ProviderConfig struct {
	Search SearchClient
	Client pramaan.Caller
}

func NewProvider(cfg ProviderConfig) (Provider, error) {
	if cfg.Client == nil {
		return nil, ErrProviderNotConfigured
	}
	return &GenerationClient{client: cfg.Client}, nil
}
