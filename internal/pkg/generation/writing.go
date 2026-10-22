package generation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

var ErrNotConfigured = errors.New("Connect Pramaan to use AI writing.")

const (
	ModelWritingFree = "fast"
	ModelWritingPaid = "best"
)

type WritingResult struct {
	Text       string
	Model      string
	TokensUsed int
}
type WritingGenerator interface {
	GenerateWriting(context.Context, string, string, VoiceContext) (*WritingResult, error)
	ModelForTier(bool) string
	IsLocal() bool
}

func (c *GenerationClient) ModelForTier(paid bool) string {
	if paid {
		return "best"
	}
	return "balanced"
}
func (c *GenerationClient) Name() string  { return "pramaan" }
func (c *GenerationClient) IsLocal() bool { return false }
func tier(s string) string {
	if s == "fast" || s == "best" {
		return s
	}
	return "balanced"
}
func (c *GenerationClient) Complete(ctx context.Context, r CompletionRequest) (*WritingResult, error) {
	if c == nil || c.client == nil {
		return nil, ErrNotConfigured
	}
	in := map[string]any{"model": tier(r.Model), "messages": []map[string]any{{"role": "system", "content": r.System}, {"role": "user", "content": r.Prompt}}}
	in["maxTokens"] = 1024
	if r.MaxTokens > 0 {
		in["maxTokens"] = min(max(r.MaxTokens, 256), 8192)
	}
	if r.Temperature != nil {
		in["temperature"] = *r.Temperature
	}
	var out struct {
		FinishReason string            `json:"finishReason"`
		ToolCalls    []json.RawMessage `json:"toolCalls"`
		Text         string            `json:"text"`
		Usage        struct {
			Input  int `json:"inputTokens"`
			Output int `json:"outputTokens"`
		} `json:"usage"`
	}
	e := pramaan.CheckedCall(ctx, c.client, "llm.chat", in, &out)
	var partial *pramaan.IncompleteReply
	charged := 0
	if errors.As(e, &partial) && r.Structured {
		partial.Structured = true
	}
	if errors.As(e, &partial) && !partial.Structured && strings.TrimSpace(partial.Text) != "" {
		// One bounded continuation, never a repeat of the original request.
		prefix := partial.Text
		charged = partial.Tokens
		messages := in["messages"].([]map[string]any)
		in["messages"] = append(messages, map[string]any{"role": "assistant", "content": prefix}, map[string]any{"role": "user", "content": "Continue exactly where the reply stopped. Do not repeat previous text. Finish the reply within this remaining budget."})
		in["maxTokens"] = min(max(r.MaxTokens, 256), 4096)
		out.Text, out.FinishReason = "", ""
		out.Usage.Input, out.Usage.Output = 0, 0
		out.ToolCalls = nil
		e = pramaan.CheckedCall(ctx, c.client, "llm.chat", in, &out)
		if e != nil {
			return nil, &pramaan.IncompleteReply{Text: prefix + out.Text, Tokens: charged + out.Usage.Input + out.Usage.Output, Cause: e}
		}
		out.Text = prefix + out.Text
	}
	if e != nil {
		return nil, e
	}
	if strings.TrimSpace(out.Text) == "" {
		return nil, errors.New("No writing was returned.")
	}
	return &WritingResult{Text: out.Text, Model: tier(r.Model), TokensUsed: charged + out.Usage.Input + out.Usage.Output}, nil
}
func (c *GenerationClient) GenerateWriting(ctx context.Context, model, prompt string, voice VoiceContext) (*WritingResult, error) {
	return c.Complete(ctx, CompletionRequest{System: BuildVoiceRules(voice), Prompt: prompt, Model: model})
}
