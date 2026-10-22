package eventbus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
)

type setupFixture struct {
	jetstream.JetStream
	calls atomic.Int64
}

func (f *setupFixture) CreateOrUpdateStream(context.Context, jetstream.StreamConfig) (jetstream.Stream, error) {
	if f.calls.Add(1) == 1 {
		return nil, errors.New("synthetic storage refusal")
	}
	return nil, nil
}
func TestStreamSetupRefusalIsRetryableWithoutResettingALockedGuard(t *testing.T) {
	f := &setupFixture{}
	b := &NATSBus{js: f, stream: "fixture", prefix: "fixture"}
	if err := b.ensureStream(t.Context()); err == nil {
		t.Fatal("storage refusal must be returned")
	}
	var workers sync.WaitGroup
	for range 20 {
		workers.Go(func() {
			if err := b.ensureStream(t.Context()); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	if f.calls.Load() != 2 {
		t.Fatalf("expected one refused setup and one shared successful retry, got %d", f.calls.Load())
	}
	if b.streamErr != nil {
		t.Fatal("successful setup retained stale refusal")
	}
}
