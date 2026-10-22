// Package pramaan owns the template's external capability transport.
package pramaan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.pramaan.dev"

var ErrUnavailable = errors.New("This feature is not available yet.")
var ErrIdempotencyRequired = errors.New("The request was refused: an Idempotency-Key header is required.")

var ErrNotConfigured = errors.New("Add PRAMAAN_API_KEY to .env")
var validID = regexp.MustCompile(`^[a-z][a-z0-9.-]+$`)

type Caller interface {
	Call(context.Context, string, any, any, ...Option) error
}
type Option func(*callOptions)
type callOptions struct{ key, account, plan string }

func Idempotency(key string) Option { return func(o *callOptions) { o.key = key } }
func Account(ref string) Option     { return func(o *callOptions) { o.account = ref } }

type Client struct {
	base, key    string
	http         *http.Client
	PollInterval time.Duration
}

func New(key, base string, transport http.RoundTripper) (*Client, error) {
	if strings.TrimSpace(key) == "" {
		return nil, ErrNotConfigured
	}
	if base == "" {
		base = DefaultBaseURL
	}
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
		return nil, errors.New("The Pramaan address is invalid.")
	}
	return &Client{base: strings.TrimRight(base, "/"), key: key, http: &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, PollInterval: 250 * time.Millisecond}, nil
}
func FromEnv() (*Client, error) {
	return New(os.Getenv("PRAMAAN_API_KEY"), os.Getenv("PRAMAAN_BASE_URL"), nil)
}
func Call(ctx context.Context, id string, in, out any, opts ...Option) error {
	c, e := FromEnv()
	if e != nil {
		return e
	}
	return c.Call(ctx, id, in, out, opts...)
}

// Call never retries an action. The same caller-supplied key makes deliberate retries safe.
func (c *Client) Call(ctx context.Context, id string, in, out any, opts ...Option) error {
	if c == nil {
		return ErrNotConfigured
	}
	if !validID.MatchString(id) {
		return errors.New("The requested feature is invalid.")
	}
	o := callOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	var keyErr error
	o.key, keyErr = requestKey(o.key)
	if keyErr != nil {
		return keyErr
	}
	prepared, e := PrepareInput(id, in)
	if e != nil {
		return e
	}
	b, e := json.Marshal(prepared)
	if e != nil {
		return errors.New("The request could not be prepared.")
	}
	status, body, e := c.request(ctx, "POST", "/v1/capabilities/"+url.PathEscape(id), b, o)
	if e != nil {
		return e
	}
	if status == http.StatusAccepted {
		var job struct {
			ID     string          `json:"job_id"`
			Status string          `json:"status"`
			Result json.RawMessage `json:"result"`
			Error  struct {
				Code   string `json:"code"`
				Effect string `json:"effect"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &job) != nil || job.ID == "" {
			return errors.New("The job response is incomplete.")
		}
		deadline := time.NewTimer(5 * time.Minute)
		defer deadline.Stop()
		for job.Status == "running" || job.Status == "queued" || job.Status == "" {
			timer := time.NewTimer(c.PollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-deadline.C:
				timer.Stop()
				return errors.New("The job is still running. Check again later.")
			case <-timer.C:
			}
			_, body, e = c.request(ctx, "GET", "/v1/jobs/"+url.PathEscape(job.ID), nil, callOptions{})
			if e != nil {
				return e
			}
			if json.Unmarshal(body, &job) != nil {
				return errors.New("The job response is incomplete.")
			}
		}
		if job.Status != "done" {
			if job.Error.Code == "upstream_unavailable" {
				effect := "unknown"
				if job.Error.Effect == "none" {
					effect = "none"
				}
				return &HTTPError{Status: 502, Code: job.Error.Code, Effect: effect}
			}
			return errors.New("The job could not finish.")
		}
		body = job.Result
	}
	resultError := checkGenerationResult(id, prepared, body)
	if out == nil {
		return resultError
	}
	if e = json.Unmarshal(body, out); e != nil {
		return errors.New("The response could not be read.")
	}
	return resultError
}
func (c *Client) request(ctx context.Context, method, path string, b []byte, o callOptions) (int, []byte, error) {
	r, e := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(b))
	if e != nil {
		return 0, nil, errors.New("The request could not be prepared.")
	}
	r.Header.Set("Authorization", "Bearer "+c.key)
	r.Header.Set("Accept", "application/json")
	if b != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if o.key != "" {
		r.Header.Set("Idempotency-Key", o.key)
	}
	if o.plan != "" {
		r.Header.Set("Pramaan-Plan", o.plan)
	}
	if o.account != "" {
		r.Header.Set("Pramaan-Account", o.account)
	}
	resp, e := c.http.Do(r)
	if e != nil {
		if ctx.Err() != nil {
			return 0, nil, ctx.Err()
		}
		return 0, nil, errors.New("Pramaan could not be reached. Check again later.")
	}
	defer resp.Body.Close()
	data, e := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if e != nil {
		return 0, nil, errors.New("The response could not be read.")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 400 {
			var refusal struct {
				Error struct{ Code, Message string }
			}
			if json.Unmarshal(data, &refusal) == nil && refusal.Error.Code == "invalid_request" && strings.Contains(strings.ToLower(refusal.Error.Message), "idempotency-key") && strings.Contains(strings.ToLower(refusal.Error.Message), "required") {
				return resp.StatusCode, nil, ErrIdempotencyRequired
			}
		}
		if resp.StatusCode == 400 && (strings.Contains(path, "/capabilities/llm.chat") || strings.Contains(path, "/capabilities/llm.complete")) {
			var wire struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.Unmarshal(data, &wire)
			if wire.Error.Code == "invalid_request" && strings.Contains(strings.ToLower(wire.Error.Message), "maxtokens") {
				return resp.StatusCode, nil, ErrReplyLimit
			}
		}

		if strings.HasPrefix(path, "/v1/outcomes") || strings.HasPrefix(path, "/v1/jobs/") {
			var wire struct {
				Error struct {
					Code   string `json:"code"`
					Effect string `json:"effect"`
				} `json:"error"`
			}
			_ = json.Unmarshal(data, &wire)
			if resp.StatusCode == 502 && wire.Error.Code == "upstream_unavailable" {
				return resp.StatusCode, nil, ErrOutcomeUnavailable
			}
			code := "internal"
			switch resp.StatusCode {
			case 400:
				code = "invalid_request"
			case 401:
				code = "unauthorized"
			case 403:
				code = "forbidden"
			case 409:
				code = "conflict"
			case 404:
				code = "not_found"
			case 429:
				code = "rate_limited"
			}
			effect := "unknown"
			if wire.Error.Effect == "none" {
				effect = "none"
			}
			return resp.StatusCode, nil, &HTTPError{Status: resp.StatusCode, Code: code, Effect: effect}
		}

		if strings.Contains(path, "/capabilities/domains.registration.") || strings.Contains(path, "/capabilities/domains.dns-records.") {
			var wire struct {
				Error struct {
					Code   string `json:"code"`
					Effect string `json:"effect"`
				} `json:"error"`
			}
			_ = json.Unmarshal(data, &wire)
			if (wire.Error.Code == "upstream_unavailable" || wire.Error.Code == "not_found") && wire.Error.Effect == "none" {
				return resp.StatusCode, nil, ErrDomainUnavailable
			}
			if resp.StatusCode == 501 {
				return resp.StatusCode, nil, ErrDomainUnavailable
			}
			effect := wire.Error.Effect
			if effect == "" {
				read := strings.HasSuffix(path, ".search") || strings.HasSuffix(path, ".quote") || strings.HasSuffix(path, ".get") || strings.HasSuffix(path, ".list")
				if read || wire.Error.Code == "invalid_request" || resp.StatusCode == 400 {
					effect = "none"
				} else {
					effect = "unknown"
				}
			}
			return resp.StatusCode, nil, &HTTPError{Status: resp.StatusCode, Code: wire.Error.Code, Effect: effect}
		}
		if strings.Contains(path, "/capabilities/connector.") {
			var wire struct {
				Error struct {
					Code   string `json:"code"`
					Effect string `json:"effect"`
				} `json:"error"`
			}
			_ = json.Unmarshal(data, &wire)
			effect := "unknown"
			if wire.Error.Effect == "none" {
				effect = "none"
			}
			return resp.StatusCode, nil, &HTTPError{Status: resp.StatusCode, Code: wire.Error.Code, Effect: effect}
		}
		var wire struct {
			Error struct {
				Code   string `json:"code"`
				Effect string `json:"effect"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &wire)
		if wire.Error.Code == "upstream_unavailable" {
			effect := "unknown"
			if wire.Error.Effect == "none" {
				effect = "none"
			}
			return resp.StatusCode, nil, &HTTPError{Status: resp.StatusCode, Code: "upstream_unavailable", Effect: effect}
		}
		if wire.Error.Code == "invalid_request" || wire.Error.Code == "unauthorized" || wire.Error.Code == "forbidden" || wire.Error.Code == "conflict" || wire.Error.Code == "rate_limited" {
			effect := wire.Error.Effect
			if effect == "" {
				effect = "none"
			}
			return resp.StatusCode, nil, &HTTPError{Status: resp.StatusCode, Code: wire.Error.Code, Effect: effect}
		}
		// Never surface remote URLs, identifiers, credentials, prices or provider messages.
		if resp.StatusCode == 404 || resp.StatusCode == 501 {
			return 0, nil, ErrUnavailable
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return 0, nil, errors.New("Pramaan could not authorize this request.")
		}
		return 0, nil, errors.New("Pramaan could not complete this request.")
	}
	return resp.StatusCode, data, nil
}

func (c *Client) Close() error {
	if c != nil {
		c.http.CloseIdleConnections()
	}
	return nil
}

// Configured reports whether the environment supplies a valid client configuration.
func Configured() bool { _, err := FromEnv(); return err == nil }
