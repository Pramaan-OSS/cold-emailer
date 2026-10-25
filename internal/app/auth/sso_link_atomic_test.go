package auth

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/cache"
	"github.com/pramaan-oss/cold-email/internal/models"
	"sync"
	"testing"
	"time"
)

// The final consume must allow exactly one of two verified requests through.
func TestSSOLinkPendingConsumedOnce(t *testing.T) {
	server := miniredis.RunT(t)
	client, err := cache.New("redis://" + server.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	service := &authService{cache: client}
	sid, uid := uuid.New(), uuid.New()
	if err := service.saveSSOLinkPending(ctx, sid, &models.SSOLinkPending{UserID: uid, Nonce: "verified"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan *errx.Error, 2)
	for range 2 {
		wg.Go(func() { results <- service.consumeSSOLinkPending(ctx, sid, "verified", uid) })
	}
	wg.Wait()
	close(results)
	wins, spent := 0, 0
	for err := range results {
		if err == nil {
			wins++
		} else if err == errx.ErrSSOLinkExpired {
			spent++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || spent != 1 {
		t.Fatalf("successful consumes=%d, spent=%d", wins, spent)
	}
}
