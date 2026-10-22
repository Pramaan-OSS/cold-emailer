package generation

import (
	"errors"
	"fmt"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

// ActionError stops a run after an action failure. A write may already have
// happened: preserve the transcript and require checking before a manual retry.
type ActionError struct {
	Name  string
	Risk  RiskClass
	Cause error
}

func (e *ActionError) Error() string { return fmt.Sprintf("Action %s failed: %v", e.Name, e.Cause) }
func (e *ActionError) Unwrap() error { return e.Cause }
func FailureReason(err error) string {
	var partial *pramaan.IncompleteReply
	if errors.As(err, &partial) {
		if errors.Is(err, pramaan.ErrReplyLimit) {
			return "The reply is incomplete. " + pramaan.ErrReplyLimit.Error()
		}
		return partial.Error()
	}
	if errors.Is(err, pramaan.ErrReplyLimit) {
		return pramaan.ErrReplyLimit.Error()
	}
	var schema *pramaan.SchemaError
	if errors.As(err, &schema) {
		return schema.Error() + ". Correct the schema and try again."
	}
	if errors.Is(err, pramaan.ErrUnavailable) {
		return "This feature is not available yet. Try again later."
	}
	var action *ActionError
	if errors.As(err, &action) {
		if action.Risk != RiskRead {
			return "Action " + action.Name + " failed. Its result may be unknown; check before retrying. It was not retried."
		}
		return "Action " + action.Name + " failed. Try again when the capability is available."
	}
	return "The Pramaan request failed. Check the connection and try again."
}
func failedAgent(res *AgentResult, err error) (*AgentResult, error) {
	res.StopReason = "failed"
	// Close outstanding calls as failed/not-run, so a new message can continue
	// this history without replaying an interrupted action.
	answered := map[string]bool{}
	for _, m := range res.Messages {
		if m.Role == "tool" {
			answered[m.ToolCallID] = true
		}
	}
	for _, m := range append([]AgentMessage(nil), res.Messages...) {
		for _, call := range m.ToolCalls {
			if !answered[call.ID] {
				res.Messages = append(res.Messages, AgentMessage{Role: "tool", ToolCallID: call.ID, Content: `{"status":"failed","message":"Not completed. Check before retrying; this action was not retried."}`})
				answered[call.ID] = true
			}
		}
	}
	return res, err
}

// ChargedFailure distinguishes a cut-off paid reply from a refused request.
func ChargedFailure(err error) bool {
	var partial *pramaan.IncompleteReply
	return errors.As(err, &partial)
}
