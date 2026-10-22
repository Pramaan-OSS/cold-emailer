package worker

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/pramaan-oss/cold-email/internal/app/worker/mailmanager"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/eventbus"
	"github.com/pramaan-oss/cold-email/internal/models"
)

type capturedEvents struct {
	mu     sync.Mutex
	events []models.JobEventType
	bodies []any
}

func (c *capturedEvents) on(t models.JobEventType, _ string, body any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events, c.bodies = append(c.events, t), append(c.bodies, body)
	return nil
}

func newLoadingWorker(c *capturedEvents) *WorkerService {
	return &WorkerService{mailManager: mailmanager.NewMailManager(c.on, nil, nil, nil, nil, nil)}
}

type dispatchResultBus struct {
	eventbus.EventBus
	mu       sync.Mutex
	fail     bool
	failures int
	events   int
}

func (b *dispatchResultBus) Publish(context.Context, string, string, []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail || b.failures > 0 {
		b.fail = false
		b.failures--
		return errors.New("lost broker acknowledgement")
	}
	b.events++
	return nil
}

type filingBus struct {
	eventbus.EventBus
	events []models.JobEvent
	err    error
}

func (b *filingBus) Publish(_ context.Context, _, _ string, payload []byte) error {
	if b.err != nil {
		return b.err
	}
	var event models.JobEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}
	b.events = append(b.events, event)
	return nil
}
