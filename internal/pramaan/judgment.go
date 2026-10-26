package pramaan

import (
	"context"
	"errors"
	"math"
)

type JudgmentOption struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}
type JudgmentProbability struct {
	OptionID    string  `json:"optionId"`
	Probability float64 `json:"probability"`
}

func Judgment(ctx context.Context, c Caller, question, facts string, options []JudgmentOption) ([]JudgmentProbability, error) {
	if c == nil {
		return nil, ErrNotConfigured
	}
	if len(question) == 0 || len(question) > 2000 || len(facts) > 12000 || len(options) < 2 || len(options) > 20 {
		return nil, errors.New("The judgment question or context exceeds its limits.")
	}
	var out struct {
		Results []JudgmentProbability `json:"results"`
	}
	if err := c.Call(ctx, "ai.judgment.calibrated", map[string]any{"question": question, "context": facts, "options": options}, &out); err != nil {
		return nil, err
	}
	if len(out.Results) != len(options) {
		return nil, errors.New("The judgment returned incomplete probabilities.")
	}
	sum := 0.0
	for i, v := range out.Results {
		if v.OptionID != options[i].ID || math.IsNaN(v.Probability) || math.IsInf(v.Probability, 0) || v.Probability < 0 || v.Probability > 1 {
			return nil, errors.New("The judgment returned invalid probabilities.")
		}
		sum += v.Probability
	}
	if math.Abs(sum-1) > 0.0001 {
		return nil, errors.New("The judgment probabilities do not sum to one.")
	}
	return out.Results, nil
}
