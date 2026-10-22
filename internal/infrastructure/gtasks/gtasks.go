package gtasks

import (
	"context"
	"time"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"github.com/pramaan-oss/cold-email/internal/tasks/proto"
)

type Client struct{}

func NewClient(context.Context, string, string, string, string) (*Client, error) {
	return nil, pramaan.ErrUnavailable
}
func (c *Client) CreateTask(context.Context, *proto.ProcessTask, time.Time) (string, error) {
	return "", pramaan.ErrUnavailable
}
func (c *Client) DeleteTask(context.Context, string) error { return pramaan.ErrUnavailable }
