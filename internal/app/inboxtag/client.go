package inboxtag

import (
	"context"

	"github.com/pramaan-oss/cold-email/internal/pkg/textcheck"
)

// The API client lives in internal/pkg/textcheck and is shared by every feature
// that asks TextCheck a question. This file keeps the names the policy and the
// recorded fixtures were written against.
type (
	Question = textcheck.Question
	Answer   = textcheck.Answer
	Response = textcheck.Response
)

const (
	QuestionNoul   = textcheck.QuestionNoul
	QuestionChoice = textcheck.QuestionChoice
	QuestionScore  = textcheck.QuestionScore
)

// Client asks the shared client the tagging questions.
type Client struct {
	inner textcheck.Asker
}

// NewClient builds a client for one API key.
func NewClient(apiKey string) *Client {
	return &Client{inner: textcheck.NewClient(apiKey)}
}

// NewAsker wraps an already-built client, so one process shares one HTTP
// client and one retry policy across features.
func NewAsker(inner textcheck.Asker) *Client {
	return &Client{inner: inner}
}

// Ask sends one state and every question in one call.
func (c *Client) Ask(ctx context.Context, state any, questions map[string]Question) (*Response, error) {
	return c.inner.Ask(ctx, state, questions)
}
