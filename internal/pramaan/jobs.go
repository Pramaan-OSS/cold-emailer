package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/google/uuid"
)

type Job struct {
	ID     string          `json:"job_id"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result"`
}

func (c *Client) Start(ctx context.Context, id string, in any) (Job, error) {
	var job Job
	if !validID.MatchString(id) {
		return job, ErrUnavailable
	}
	prepared, e := PrepareInput(id, in)
	if e != nil {
		return job, e
	}
	b, e := json.Marshal(prepared)
	if e != nil {
		return job, e
	}
	status, body, e := c.request(ctx, "POST", "/v1/capabilities/"+url.PathEscape(id), b, callOptions{key: uuid.NewString()})
	if e != nil {
		return job, e
	}
	if status != 202 {
		return job, errors.New("The background job did not start.")
	}
	e = json.Unmarshal(body, &job)
	return job, e
}
func (c *Client) Job(ctx context.Context, id string) (Job, error) {
	var out Job
	_, b, e := c.request(ctx, "GET", "/v1/jobs/"+url.PathEscape(id), nil, callOptions{})
	if e != nil {
		return out, e
	}
	e = json.Unmarshal(b, &out)
	return out, e
}
func (c *Client) CancelJob(ctx context.Context, id string) error {
	_, _, e := c.request(ctx, "POST", "/v1/jobs/"+url.PathEscape(id)+"/cancel", nil, callOptions{key: uuid.NewString()})
	return e
}
