package generation

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type BatchRequest struct {
	CustomID, Theme, Model string
	MaxMessages            int
}
type BatchResult struct {
	CustomID, Theme string
	Conversation    *Conversation
	Err             string
}
type BatchCounts struct{ Completed, Failed, Total int }
type BatchState struct {
	Status, OutputFileID, ErrorFileID, FailureReason string
	Counts                                           BatchCounts
}

func (c *GenerationClient) CheckModel(ctx context.Context, model string) error {
	if c == nil || c.client == nil {
		return ErrNotConfigured
	}
	return nil
}
func (c *GenerationClient) SubmitBatch(ctx context.Context, requests []BatchRequest, _ string) (string, string, error) {
	client, ok := c.client.(*pramaan.Client)
	if !ok {
		return "", "", ErrNotConfigured
	}
	if len(requests) == 0 || len(requests) > 16 {
		return "", "", errors.New("Choose between 1 and 16 simulated threads.")
	}
	rows := []map[string]any{}
	for _, r := range requests {
		rows = append(rows, map[string]any{"id": r.CustomID, "model": tier(r.Model), "messages": []map[string]any{{"role": "system", "content": warmupSystemPrompt(r.Theme, normalizeMaxMessages(r.MaxMessages))}, {"role": "user", "content": "Generate one simulated thread."}}, "jsonSchema": ConversationSchema, "maxTokens": 2048})
	}
	j, e := client.Start(ctx, "llm.complete-bulk", map[string]any{"rows": rows})
	return j.ID, j.ID, e
}
func (c *GenerationClient) GetBatch(ctx context.Context, id string) (BatchState, error) {
	client, ok := c.client.(*pramaan.Client)
	if !ok {
		return BatchState{}, ErrNotConfigured
	}
	j, e := client.Job(ctx, id)
	s := BatchState{Status: j.Status, OutputFileID: id}
	if j.Status == "done" {
		s.Status = "completed"
	}
	if j.Status == "failed" {
		s.FailureReason = "The generation job could not finish."
	}
	return s, e
}
func (c *GenerationClient) CancelBatch(ctx context.Context, id string) error {
	client, ok := c.client.(*pramaan.Client)
	if !ok {
		return ErrNotConfigured
	}
	return client.CancelJob(ctx, id)
}
func (c *GenerationClient) FetchBatchResults(ctx context.Context, id string) ([]BatchResult, error) {
	client, ok := c.client.(*pramaan.Client)
	if !ok {
		return nil, ErrNotConfigured
	}
	j, e := client.Job(ctx, id)
	if e != nil {
		return nil, e
	}
	if j.Status != "done" {
		return nil, errors.New("The generation job is not finished.")
	}
	var raw struct {
		Results []struct {
			ID           string `json:"id"`
			Found        bool   `json:"found"`
			FinishReason string `json:"finishReason"`
			Text         string `json:"text"`
			Result       struct {
				Text         string `json:"text"`
				FinishReason string `json:"finishReason"`
			} `json:"result"`
		} `json:"results"`
	}
	if json.Unmarshal(j.Result, &raw) != nil {
		return nil, errors.New("The generation results could not be read.")
	}
	out := []BatchResult{}
	for _, row := range raw.Results {
		r := BatchResult{CustomID: row.ID}
		if row.FinishReason == "length" || row.Result.FinishReason == "length" {
			r.Err = "The simulated thread is incomplete. Retry with a larger reply limit."
			out = append(out, r)
			continue
		}
		text := row.Text
		if text == "" {
			text = row.Result.Text
		}
		var conv Conversation
		if json.Unmarshal([]byte(text), &conv) != nil || ValidateDiagnostic(conv) != nil {
			r.Err = "The simulated thread failed its checks."
		} else {
			r.Conversation = &conv
		}
		out = append(out, r)
	}
	return out, nil
}
