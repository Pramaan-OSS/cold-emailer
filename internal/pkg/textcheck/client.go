package textcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

// Calibrated judgments are unavailable until the broker has that contract.
const defaultEndpoint = ""
const Model = ""

// QuestionType names the three primitives used here. The API also has a
// bounding-box type, which has nothing to do with email.
const (
	QuestionNoul   = "noul"
	QuestionChoice = "choice"
	QuestionScore  = "score"
)

// Question is one typed question. Criteria is a map for choice (option ->
// description), a []string for score (ordered levels, at most ten), and either
// absent or a {"true","false"} map for noul.
type Question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

// Noul, Choice and Score build one question each. Every policy in the tree
// spells its questions through these so a typo in a type name is a compile
// error rather than a 422 at runtime.
func Noul(instructions string) Question {
	return Question{Type: QuestionNoul, Instructions: instructions}
}

func Choice(instructions string, options map[string]string) Question {
	return Question{Type: QuestionChoice, Instructions: instructions, Criteria: options}
}

func Score(instructions string, levels []string) Question {
	return Question{Type: QuestionScore, Instructions: instructions, Criteria: levels}
}

// Normalized is a score answer as a 0..1 position on its rubric: the only
// arithmetic ever done with a score, done here because the retired scorer is bad at it.
func (a Answer) Normalized(levels int) float64 {
	if levels < 2 {
		return 0
	}
	v := a.Score / float64(levels-1)
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// Asker is the narrow interface every feature depends on, so tests supply a
// recorded response instead of a network.
type Asker interface {
	Ask(ctx context.Context, state any, questions map[string]Question) (*Response, error)
}

type request struct {
	State     any                 `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]Question `json:"questions"`
}

// Answer is the union of the three answer shapes. Which fields are populated
// is decided by Type.
//
// Noul carries no Confidence: the noul value IS the probability, so a separate
// confidence would be the same number twice.
type Answer struct {
	Type string `json:"type"`

	Noul float64 `json:"noul"`

	Choice string `json:"choice"`

	Score  float64           `json:"score"`
	Legend map[string]string `json:"legend"`

	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

// Response is one call's worth of answers. Output tokens are free; input is
// billed, and is recorded so a workspace can see what the feature costs.
type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Client calls the TextCheck API. Zero value is not usable; use NewClient.
type Client struct {
	endpoint string
	apiKey   string
	http     *http.Client
	// maxAttempts bounds the retry loop. Retries cover 429 and 529 only.
	maxAttempts int
	// sleep is the delay function, swapped in tests so a backoff assertion
	// does not cost wall-clock seconds.
	sleep   func(context.Context, time.Duration) error
	billing *billingGate
}

type Option func(*Client)

func NewClient(apiKey string, options ...Option) *Client {
	c := &Client{
		endpoint:    defaultEndpoint,
		apiKey:      apiKey,
		http:        &http.Client{Timeout: 30 * time.Second},
		maxAttempts: 4,
		sleep:       sleepCtx,
		billing:     newBillingGate(apiKey),
	}
	for _, option := range options {
		option(c)
	}
	return c
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// APIError is a non-2xx response, with the message dug out of whichever shape
// `detail` arrived in.
type APIError struct {
	Status     int
	Message    string
	RetryAfter time.Duration
	// Retryable marks 429 and 529, the two the caller may try again.
	Retryable bool
}

func (e *APIError) Error() string {
	return fmt.Sprintf("textcheck: %d: %s", e.Status, e.Message)
}

// parseDetail pulls a human message out of an error body.
//
// `detail` has three different shapes, and each was confirmed against the live
// API rather than taken from the docs:
//
//	object — {"detail":{"error_type":"api_usage_error","message":"Unknown model: …"}}
//	         (400 unknown model, 401 bad key)
//	string — {"detail":"Too many score levels. Must have at most 10 levels."}
//	         (400 schema violation, e.g. an eleven-level score)
//	array  — {"detail":[{"type":"union_tag_invalid","loc":[…],"msg":"…"}]}
//	         (422 request validation)
//
// One parser handles all three; a parser written for any one of them turns the
// other two into an empty error message at exactly the moment you need it.
func parseDetail(body []byte) string {
	var envelope struct {
		Detail json.RawMessage `json:"detail"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Detail) == 0 {
		if s := string(bytes.TrimSpace(body)); s != "" {
			return truncate(s, 300)
		}
		return "no detail in response"
	}

	// String.
	var asString string
	if err := json.Unmarshal(envelope.Detail, &asString); err == nil {
		return truncate(asString, 300)
	}

	// Object.
	var asObject struct {
		ErrorType string `json:"error_type"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(envelope.Detail, &asObject); err == nil && asObject.Message != "" {
		if asObject.ErrorType != "" {
			return truncate(asObject.ErrorType+": "+asObject.Message, 300)
		}
		return truncate(asObject.Message, 300)
	}

	// Array of validation items.
	var asArray []struct {
		Msg string   `json:"msg"`
		Loc []any    `json:"loc"`
		Typ string   `json:"type"`
		Ctx any      `json:"ctx"`
		In  any      `json:"input"`
		_   struct{} // keep the decoder tolerant of new fields
	}
	if err := json.Unmarshal(envelope.Detail, &asArray); err == nil && len(asArray) > 0 {
		msg := asArray[0].Msg
		if msg == "" {
			msg = asArray[0].Typ
		}
		if len(asArray) > 1 {
			msg = fmt.Sprintf("%s (and %d more validation errors)", msg, len(asArray)-1)
		}
		return truncate(msg, 300)
	}

	return truncate(string(envelope.Detail), 300)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Ask sends one state and every question in one call against the pinned
// model, and retries only the two statuses that mean "try again later".
func (c *Client) Ask(ctx context.Context, state any, questions map[string]Question) (*Response, error) {
	return nil, pramaan.ErrUnavailable
}

func (c *Client) once(ctx context.Context, payload []byte) (*Response, error) {
	return nil, pramaan.ErrUnavailable
}

// parseRetryAfter reads the header in both forms the RFC allows: delta-seconds,
// and an HTTP date.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// backoff honours the server's Retry-After when it sent one, and otherwise
// doubles with jitter. Jitter matters here: an inbox flood retries in lockstep
// without it, and arrives back at the same overloaded service together.
func backoff(attempt int, err error) time.Duration {
	var apiErr *APIError
	if asAPIError(err, &apiErr) && apiErr.RetryAfter > 0 {
		return min(apiErr.RetryAfter, 60*time.Second)
	}
	base := time.Duration(math.Pow(2, float64(attempt))) * 250 * time.Millisecond
	jitter := time.Duration(rand.Int63n(int64(base/2 + 1)))
	return min(base+jitter, 30*time.Second)
}

func asAPIError(err error, target **APIError) bool {
	if e, ok := err.(*APIError); ok {
		*target = e
		return true
	}
	return false
}
