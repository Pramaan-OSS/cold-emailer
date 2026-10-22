package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/pramaan-oss/cold-email/internal/models"
)

// folderHolder answers which folders hold the message, and which it cannot open.
type folderHolder struct {
	holds    map[string]bool
	fails    string
	searched []string
}

func (h *folderHolder) LocateMessageID(_ context.Context, mailboxes []string, _ string) ([]string, int, error) {
	h.searched = mailboxes
	var held []string
	unsearched := 0
	for _, m := range mailboxes {
		if m == h.fails {
			unsearched++
			continue
		}
		if h.holds[m] {
			held = append(held, m)
		}
	}
	return held, unsearched, nil
}

func TestRemovalOutcome(t *testing.T) {
	cases := []struct {
		found, trashed bool
		want           string
	}{
		{true, false, models.WarmupRemovalPresent},
		{true, true, models.WarmupRemovalTrashed},
		{false, false, models.WarmupRemovalGone},
	}
	for _, tc := range cases {
		if got, err := removalOutcome(tc.found, tc.trashed, nil); err != nil || got != tc.want {
			t.Fatalf("removalOutcome(%v, %v) = %q, %v; want %q", tc.found, tc.trashed, got, err, tc.want)
		}
	}
	if got, err := removalOutcome(false, false, errors.New("down")); err == nil || got != "" {
		t.Fatalf("a failed search answered %q, %v", got, err)
	}
}
