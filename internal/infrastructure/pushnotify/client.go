// Package pushnotify sends device notifications through the single Pramaan client.
package pushnotify

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

var ErrUnregistered = errors.New("device token is unregistered")

type Notification struct {
	Title string
	Body  string
	// Badge, when non-nil, sets the app icon badge count.
	Badge *int
	// ThreadID groups alerts in the notification center (we use the category).
	ThreadID string
	// CollapseID makes a later push replace an earlier one with the same id.
	CollapseID string
	// Custom keys delivered alongside aps (link, category, notification id).
	Custom map[string]any
}

type Client struct{ caller pramaan.Caller }

func New(caller pramaan.Caller) *Client {
	if caller == nil {
		return nil
	}
	return &Client{caller: caller}
}
func (c *Client) Push(ctx context.Context, deviceToken, environment string, n Notification) error {
	data := map[string]string{}
	for key, value := range n.Custom {
		if text, ok := value.(string); ok {
			data[key] = text
		}
	}
	return c.caller.Call(ctx, "notify.push.send", map[string]any{"deviceToken": deviceToken, "title": n.Title, "body": n.Body, "data": data}, nil, pramaan.Idempotency(uuid.NewString()))
}
