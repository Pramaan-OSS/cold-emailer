package pramaan

import (
	"encoding/json"
	"errors"
)

// IncompleteReply preserves charged usage without accepting a partial result.
type IncompleteReply struct {
	Text       string
	Tokens     int
	Structured bool
	Cause      error
}

func (e *IncompleteReply) Unwrap() error { return e.Cause }
func (e *IncompleteReply) Error() string {
	return "The reply is incomplete. Retry with a larger reply limit."
}

var ErrReplyLimit = errors.New("The reply limit is too small. Increase maxTokens in the request settings before trying again.")

func checkGenerationResult(id string, in any, body []byte) error {
	if id != "llm.chat" && id != "llm.complete" {
		return nil
	}
	var reply struct {
		Text         string            `json:"text"`
		FinishReason string            `json:"finishReason"`
		Calls        []json.RawMessage `json:"toolCalls"`
		Usage        struct {
			Input  int `json:"inputTokens"`
			Output int `json:"outputTokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &reply) != nil || reply.FinishReason != "length" {
		return nil
	}
	b, _ := json.Marshal(in)
	var input map[string]any
	_ = json.Unmarshal(b, &input)
	return &IncompleteReply{Text: reply.Text, Tokens: reply.Usage.Input + reply.Usage.Output, Structured: input["jsonSchema"] != nil || len(reply.Calls) > 0}
}
