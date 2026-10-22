package textcheck

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const goodAnswer = `{"model":"jev-1.13.0","answers":{"kind":{"type":"choice","choice":"human_reply","confidence":0.9}},"usage":{"input_tokens":1300,"output_tokens":2}}`

func TestBillingRecoveryHasOneConcurrentProbe(t *testing.T) {
	g := newBillingGate("key")
	now := time.Now()
	g.now = func() time.Time { return now }
	g.finish(context.Background(), billingPermit{}, &APIError{Status: 402})
	now = now.Add(billingCooldown)
	var admitted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := g.acquire(context.Background())
			if err == nil && p.owner != "" {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	if admitted.Load() != 1 {
		t.Fatalf("%d recovery probes admitted", admitted.Load())
	}
}

func TestBillingLeaseAndStaleSuccess(t *testing.T) {
	g := newBillingGate("key")
	now := time.Now()
	g.now = func() time.Time { return now }
	g.finish(context.Background(), billingPermit{}, &APIError{Status: 402})
	now = now.Add(billingCooldown)
	old, err := g.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(billingProbeLease)
	current, err := g.acquire(context.Background())
	if err != nil {
		t.Fatal("abandoned probe did not expire", err)
	}
	g.finish(context.Background(), old, nil)
	if _, err := g.acquire(context.Background()); !errors.Is(err, ErrBillingCooldown) {
		t.Fatal("stale probe cleared a newer lease")
	}
	g.finish(context.Background(), current, errors.New("network unavailable"))
	if _, err := g.acquire(context.Background()); !errors.Is(err, ErrBillingCooldown) {
		t.Fatal("failed recovery probe released the billing gate")
	}
}

func TestNonBillingFailureDoesNotPause(t *testing.T) {
	g := newBillingGate("key")
	for _, status := range []int{400, 401, 422, 429, 500, 529} {
		g.finish(context.Background(), billingPermit{}, &APIError{Status: status})
		if _, err := g.acquire(context.Background()); err != nil {
			t.Fatalf("%d paused billing: %v", status, err)
		}
	}
}
