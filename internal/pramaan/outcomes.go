package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// OutcomeRequest keeps the estimated input, options and stated basis together.
// Outcome runs intentionally have no account, end-user or price-ceiling options.
type OutcomeRequest struct {
	Input   map[string]any `json:"input"`
	Options map[string]any `json:"options,omitempty"`
	Basis   map[string]any `json:"basis,omitempty"`
}
type OutcomeCaller interface {
	OutcomesList(context.Context) (map[string]any, error)
	OutcomeGet(context.Context, string) (map[string]any, error)
	OutcomeEstimate(context.Context, string, OutcomeRequest) (map[string]any, error)
	OutcomeStart(context.Context, string, OutcomeRequest, string, string) (int, map[string]any, error)
	OutcomeJob(context.Context, string) (map[string]any, error)
	OutcomeCancel(context.Context, string) (map[string]any, error)
}

var ErrOutcomeUnavailable = errors.New("This outcome is unavailable. Use the capability path.")

// HTTPError exposes only a stable code and safe local text, never upstream details.
type HTTPError struct {
	Status       int
	Code, Effect string
}

func (e *HTTPError) Error() string {
	if e.Code == "upstream_unavailable" {
		if e.Effect == "unknown" {
			return "This feature is not available yet. Its result may be unknown; check before retrying."
		}
		return "This feature is not available yet."
	}
	if e.Code == "invalid_request" {
		if e.Effect == "unknown" {
			return "The request was not accepted. Its result may be unknown; check before trying again."
		}
		return "The request was not accepted. Check the input fields."
	}
	switch e.Status {
	case 429:
		return "Pramaan is busy. Try again later."
	case 403:
		return "Approval is required before this request can run."
	case 409:
		return "The plan changed or expired. Review a new estimate before running."
	}
	if e.Effect == "unknown" {
		return "The request could not complete. Its result may be unknown; check before trying again."
	}
	return "The request could not complete."
}
func (e *HTTPError) Is(target error) bool {
	return target == ErrUnavailable && e.Code == "upstream_unavailable" && e.Effect == "none"
}

func (c *Client) outcomeRequest(ctx context.Context, method, path string, in any, key, plan string) (int, map[string]any, error) {
	if c == nil {
		return 0, nil, ErrNotConfigured
	}
	var b []byte
	var err error
	if in != nil {
		b, err = json.Marshal(in)
		if err != nil {
			return 0, nil, errors.New("The request could not be prepared.")
		}
	}
	status, body, err := c.request(ctx, method, path, b, callOptions{key: key, plan: plan})
	if err != nil {
		return status, nil, err
	}
	var out map[string]any
	if json.Unmarshal(body, &out) != nil {
		return status, nil, errors.New("The outcome response could not be read.")
	}
	return status, out, nil
}
func outcomePath(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", ErrUnavailable
	}
	return "/v1/outcomes/" + url.PathEscape(id), nil
}
func (c *Client) OutcomesList(ctx context.Context) (map[string]any, error) {
	_, out, e := c.outcomeRequest(ctx, "GET", "/v1/outcomes", nil, "", "")
	return out, e
}
func (c *Client) OutcomeGet(ctx context.Context, id string) (map[string]any, error) {
	p, e := outcomePath(id)
	if e != nil {
		return nil, e
	}
	_, out, e := c.outcomeRequest(ctx, "GET", p, nil, "", "")
	return out, e
}
func (c *Client) OutcomeEstimate(ctx context.Context, id string, in OutcomeRequest) (map[string]any, error) {
	p, e := outcomePath(id)
	if e != nil {
		return nil, e
	}
	_, out, e := c.outcomeRequest(ctx, "POST", p+"/estimate", in, "", "")
	return out, e
}

// OutcomeStart sends once, preserving reports that say a step acted or may have acted.
func (c *Client) OutcomeStart(ctx context.Context, id string, in OutcomeRequest, key, plan string) (int, map[string]any, error) {
	p, e := outcomePath(id)
	if e != nil {
		return 0, nil, e
	}
	if key == "" {
		key = uuid.NewString()
	}
	return c.outcomeRequest(ctx, "POST", p, in, key, plan)
}
func (c *Client) OutcomeJob(ctx context.Context, id string) (map[string]any, error) {
	_, out, e := c.outcomeRequest(ctx, "GET", "/v1/jobs/"+url.PathEscape(id), nil, "", "")
	return out, e
}

// OutcomeRun polls an accepted run. It never submits the run again after a timeout.
func (c *Client) OutcomeRun(ctx context.Context, id string, in OutcomeRequest, key, plan string) (map[string]any, error) {
	status, out, e := c.OutcomeStart(ctx, id, in, key, plan)
	if e != nil || status != http.StatusAccepted {
		return out, e
	}
	jobID := stringField(out, "job_id")
	if jobID == "" {
		return nil, errors.New("The run may have started. Its job reference is missing; check before retrying.")
	}
	deadline := time.NewTimer(36 * time.Minute)
	defer deadline.Stop()
	for {
		timer := time.NewTimer(c.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-deadline.C:
			timer.Stop()
			return nil, errors.New("The run is still uncertain. Check its status before trying again.")
		case <-timer.C:
		}
		job, err := c.OutcomeJob(ctx, jobID)
		if err != nil {
			var limited *HTTPError
			if errors.As(err, &limited) && limited.Status == 429 {
				continue
			}
			return nil, err
		}
		switch stringField(job, "status") {
		case "done":
			report, ok := job["result"].(map[string]any)
			if !ok {
				return nil, errors.New("The run report is incomplete. Check before retrying.")
			}
			return report, nil
		case "failed", "cancelled":
			if report, ok := job["result"].(map[string]any); ok {
				return report, nil
			}
			code := "internal"
			if details, ok := job["error"].(map[string]any); ok && stringField(details, "code") == "upstream_timeout" {
				code = "upstream_timeout"
			}
			return nil, &HTTPError{Status: 502, Code: code, Effect: "unknown"}
		}
	}
}

// CapabilityEstimate quotes the retained company search and mail-domain setup.
func (c *Client) CapabilityEstimate(ctx context.Context, id string, in map[string]any) (map[string]any, error) {
	if id != "companies.search" && id != "email.domain.create" {
		return nil, ErrUnavailable
	}
	_, out, e := c.outcomeRequest(ctx, "POST", "/v1/capabilities/"+url.PathEscape(id)+"/estimate", in, "", "")
	return out, e
}

func (c *Client) OutcomeCancel(ctx context.Context, id string) (map[string]any, error) {
	_, out, e := c.outcomeRequest(ctx, "POST", "/v1/jobs/"+url.PathEscape(id)+"/cancel", nil, uuid.NewString(), "")
	return out, e
}
