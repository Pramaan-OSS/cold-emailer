package textcheck

import (
	"context"
	"encoding/json"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"testing"
)

type judgmentFake struct{}

func (judgmentFake) Call(_ context.Context, id string, in, out any, _ ...pramaan.Option) error {
	request := in.(map[string]any)
	options := request["options"].([]pramaan.JudgmentOption)
	results := []pramaan.JudgmentProbability{}
	for _, o := range options {
		results = append(results, pramaan.JudgmentProbability{OptionID: o.ID, Probability: 1 / float64(len(options))})
	}
	b, _ := json.Marshal(map[string]any{"results": results})
	return json.Unmarshal(b, out)
}
func TestPlatformJudgmentsMapToExistingAnswerTypes(t *testing.T) {
	c := NewClient("")
	c.broker = judgmentFake{}
	out, err := c.Ask(context.Background(), map[string]string{"text": "Synthetic reply"}, map[string]Question{"binary": Noul("Is this a reply?"), "choice": Choice("Category?", map[string]string{"a": "A", "b": "B"}), "score": Score("Score?", []string{"Low", "Medium", "High"})})
	if err != nil || out.Answers["binary"].Noul != .5 || out.Answers["score"].Score != 1 || len(out.Answers["choice"].Probabilities) != 2 {
		t.Fatalf("%+v %v", out, err)
	}
}
