package generation

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

func (c *GenerationClient) RunAgent(ctx context.Context, r AgentRequest) (*AgentResult, error) {
	if c == nil || c.client == nil {
		return nil, ErrNotConfigured
	}
	limit := r.MaxIterations
	if limit <= 0 {
		limit = defaultMaxIterations
	}
	maxTokens := r.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultAgentTokens
	}
	res := &AgentResult{Messages: append([]AgentMessage{}, r.Messages...), Model: tier(r.Model)}
	catalog, e := newToolCatalog(r.Tools, res.Messages)
	if e != nil {
		return failedAgent(res, e)
	}
	catalog.bindDiscovery(func() []AgentMessage { return res.Messages })
	for i := 0; i < limit; i++ {
		if r.PreIteration != nil {
			if e := r.PreIteration(ctx, i); e != nil {
				res.StopReason = "stopped"
				return res, nil
			}
		}
		selected, e := catalog.selection(res.Messages)
		if e != nil {
			return failedAgent(res, e)
		}
		tools := []map[string]any{}
		byName := map[string]ToolDef{}
		for _, t := range selected {
			tools = append(tools, map[string]any{"name": t.Name, "description": t.Description, "parameters": t.InputSchema})
			byName[t.Name] = t
		}
		messages := []map[string]any{}
		if r.System != "" {
			messages = append(messages, map[string]any{"role": "system", "content": r.System})
		}
		for _, m := range res.Messages {
			x := map[string]any{"role": m.Role}
			if m.Content != "" || m.Role == "tool" {
				x["content"] = m.Content
			}
			if m.Role == "tool" {
				x["toolCallId"] = m.ToolCallID
			}
			if len(m.ToolCalls) > 0 {
				calls := []map[string]any{}
				for _, tc := range m.ToolCalls {
					var args map[string]any
					if json.Unmarshal(tc.Args, &args) != nil {
						return failedAgent(res, errors.New("The tool arguments could not be read."))
					}
					calls = append(calls, map[string]any{"id": tc.ID, "name": tc.Name, "arguments": args})
				}
				x["toolCalls"] = calls
			}
			messages = append(messages, x)
		}
		var out struct {
			FinishReason string `json:"finishReason"`
			Text         string `json:"text"`
			Calls        []struct {
				ID        string          `json:"id"`
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"toolCalls"`
			Usage struct {
				Input  int `json:"inputTokens"`
				Output int `json:"outputTokens"`
			} `json:"usage"`
		}
		in := map[string]any{"messages": messages, "tools": tools, "model": tier(r.Model), "maxTokens": min(max(maxTokens, 256), 8192)}
		if e := pramaan.CheckedCall(ctx, c.client, "llm.chat", in, &out); e != nil {
			var partial *pramaan.IncompleteReply
			if errors.As(e, &partial) {
				res.TokensUsed += partial.Tokens
				res.Iterations = i + 1
				if !partial.Structured && partial.Text != "" {
					res.Text = partial.Text
					res.Messages = append(res.Messages, AgentMessage{Role: "assistant", Content: partial.Text})
					if r.OnEvent != nil {
						r.OnEvent(AgentEvent{Type: EventText, Text: partial.Text, Iteration: i})
					}
				}
			}
			return failedAgent(res, e)
		}
		res.Iterations = i + 1
		res.TokensUsed += out.Usage.Input + out.Usage.Output
		res.Text = out.Text
		msg := AgentMessage{Role: "assistant", Content: out.Text}
		for _, call := range out.Calls {
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{ID: call.ID, Name: call.Name, Args: call.Arguments})
		}
		res.Messages = append(res.Messages, msg)
		if len(out.Calls) == 0 {
			res.StopReason = "stop"
			if r.OnEvent != nil {
				r.OnEvent(AgentEvent{Type: EventText, Text: out.Text, Iteration: i})
			}
			return res, nil
		}
		for _, call := range msg.ToolCalls {
			tool, ok := byName[call.Name]
			if !ok {
				return failedAgent(res, errors.New("The requested tool is not available."))
			}
			if tool.Risk != RiskRead {
				if r.Approve == nil {
					res.Pending = &PendingToolCall{Call: call, Risk: tool.Risk}
					res.StopReason = "approval_required"
					return res, nil
				}
				if e := r.Approve(ctx, tool, call); e != nil {
					if errors.Is(e, ErrApprovalRequired) {
						res.Pending = &PendingToolCall{Call: call, Risk: tool.Risk}
						res.StopReason = "approval_required"
						return res, nil
					}
					return failedAgent(res, e)
				}
			}
			if r.OnEvent != nil {
				r.OnEvent(AgentEvent{Type: EventToolStart, ToolName: call.Name, ToolArgs: call.Args, Iteration: i})
			}
			text, e := tool.Handler(ctx, call.Args)
			if e != nil {
				err := &ActionError{Name: call.Name, Risk: tool.Risk, Cause: e}
				b, _ := json.Marshal(map[string]string{"status": "failed", "message": FailureReason(err)})
				res.Messages = append(res.Messages, AgentMessage{Role: "tool", ToolCallID: call.ID, Content: string(b)})
				return failedAgent(res, err)
			}
			res.Messages = append(res.Messages, AgentMessage{Role: "tool", ToolCallID: call.ID, Content: text})
			if r.OnEvent != nil {
				r.OnEvent(AgentEvent{Type: EventToolResult, ToolName: call.Name, ToolResult: text, Iteration: i})
			}
		}
	}
	res.StopReason = "max_iterations"
	return res, nil
}
