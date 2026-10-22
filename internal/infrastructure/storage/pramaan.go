package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

// Client proxies storage bytes through Pramaan, never through a vendor URL.
type Client struct {
	Broker                *pramaan.Client
	Bucket, PublicBaseURL string
}

func NewClient(ctx context.Context, _ any, bucket string) (*Client, error) {
	c, e := pramaan.FromEnv()
	if e != nil {
		return nil, e
	}
	if bucket == "" {
		bucket = os.Getenv("PRAMAAN_STORAGE_BUCKET")
	}
	if bucket == "" {
		return nil, errors.New("Choose a connected storage bucket.")
	}
	return &Client{Broker: c, Bucket: bucket, PublicBaseURL: "/public"}, nil
}
func (c *Client) Name() string { return "pramaan" }
func (c *Client) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	var out struct {
		Content string `json:"content"`
		File    struct {
			URL string `json:"url"`
		} `json:"file"`
	}
	if e := c.Broker.Call(ctx, "storage.object.get", map[string]any{"bucket": c.Bucket, "path": key}, &out); e != nil {
		return nil, e
	}
	var b []byte
	var e error
	if out.Content != "" {
		b, e = base64.StdEncoding.DecodeString(out.Content)
	} else if out.File.URL != "" {
		b, e = c.Broker.ReadFile(ctx, out.File.URL)
	} else {
		e = errors.New("The stored file is unavailable.")
	}
	if e != nil {
		return nil, e
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (c *Client) Put(ctx context.Context, key string, body io.Reader, kind string) error {
	b, e := io.ReadAll(io.LimitReader(body, (25<<20)+1))
	if e != nil {
		return e
	}
	if len(b) > 25<<20 {
		return errors.New("The file exceeds the upload limit.")
	}
	in := map[string]any{"bucket": c.Bucket, "path": key, "overwrite": true}
	if kind != "" {
		in["contentType"] = kind
	}
	if len(b) <= 1<<20 {
		in["content"] = base64.StdEncoding.EncodeToString(b)
	} else {
		id, e := c.Broker.Upload(ctx, b, kind)
		if e != nil {
			return e
		}
		in["fileId"] = id
	}
	return c.Broker.Call(ctx, "storage.object.put", in, nil, pramaan.Idempotency(uuid.NewString()))
}
func (c *Client) PutPublic(ctx context.Context, key string, body io.Reader, kind string) (string, error) {
	e := c.Put(ctx, key, body, kind)
	return c.PublicURL(key), e
}
func (c *Client) PublicURL(key string) string {
	base := strings.TrimRight(c.PublicBaseURL, "/")
	if base == "" {
		base = "/public"
	}
	return base + "/" + url.PathEscape(key)
}
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.Broker.Call(ctx, "storage.object.delete", map[string]any{"bucket": c.Bucket, "path": key}, nil, pramaan.Idempotency(uuid.NewString()))
}
func (c *Client) Has(ctx context.Context, key string) (bool, error) {
	var out map[string]any
	e := c.Broker.Call(ctx, "storage.object.head", map[string]any{"bucket": c.Bucket, "path": key}, &out)
	if errors.Is(e, pramaan.ErrUnavailable) {
		return false, nil
	}
	return e == nil, e
}
func (c *Client) Exists(ctx context.Context, _ string, key string) (bool, error) {
	return c.Has(ctx, key)
}
func (c *Client) DeletePrefix(ctx context.Context, prefix string) (int, error) {
	if c.Broker == nil {
		return 0, ErrUnsupported
	}
	if e := CheckPrefix(prefix); e != nil {
		return 0, e
	}
	count := 0
	cursor := ""
	for {
		in := map[string]any{"bucket": c.Bucket, "prefix": prefix, "limit": 1000}
		if cursor != "" {
			in["cursor"] = cursor
		}
		var out struct {
			Results []struct {
				Path     string `json:"path"`
				IsFolder bool   `json:"isFolder"`
			} `json:"results"`
			NextCursor string `json:"nextCursor"`
		}
		if e := c.Broker.Call(ctx, "storage.objects.list", in, &out); e != nil {
			return count, e
		}
		for _, r := range out.Results {
			if r.IsFolder || !strings.HasPrefix(r.Path, prefix) {
				continue
			}
			if e := c.Delete(ctx, r.Path); e != nil {
				return count, e
			}
			count++
		}
		if out.NextCursor == "" {
			return count, nil
		}
		if out.NextCursor == cursor {
			return count, errors.New("The file listing did not advance.")
		}
		cursor = out.NextCursor
	}
}
func (c *Client) PresignedGetURL(context.Context, string, time.Duration) (string, error) {
	return "", ErrUnsupported
}
func (c *Client) PresignedURL(context.Context, PresignOp, string, string, time.Duration) (string, error) {
	return "", ErrUnsupported
}
