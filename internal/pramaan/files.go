package pramaan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/google/uuid"
)

func (c *Client) Upload(ctx context.Context, body []byte, contentType string) (string, error) {
	r, e := http.NewRequestWithContext(ctx, "POST", c.base+"/v1/files?kind=data", nil)
	if e != nil {
		return "", e
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Set("Authorization", "Bearer "+c.key)
	r.Header.Set("Idempotency-Key", uuid.NewString())
	r.Header.Set("Content-Type", contentType)
	resp, e := c.http.Do(r)
	if e != nil {
		return "", errors.New("The file could not be uploaded.")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New("The file could not be uploaded.")
	}
	var out struct {
		ID string `json:"fileId"`
	}
	e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out)
	if e != nil || out.ID == "" {
		return "", errors.New("The file response is incomplete.")
	}
	return out.ID, nil
}
func (c *Client) ReadFile(ctx context.Context, raw string) ([]byte, error) {
	u, e := url.Parse(raw)
	base, _ := url.Parse(c.base)
	if e != nil || u.Scheme != base.Scheme || u.Host != base.Host || u.User != nil {
		return nil, errors.New("The file address is not on Pramaan.")
	}
	r, e := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if e != nil {
		return nil, e
	}
	r.Header.Set("Authorization", "Bearer "+c.key)
	resp, e := c.http.Do(r)
	if e != nil {
		return nil, errors.New("The file could not be read.")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("The file could not be read.")
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, (25<<20)+1))
	if e != nil || len(b) > 25<<20 {
		return nil, errors.New("The file exceeds the read limit.")
	}
	return b, nil
}
