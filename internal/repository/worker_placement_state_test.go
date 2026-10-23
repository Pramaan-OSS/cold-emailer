package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestResidencyDistinguishesNewFromUnknownAssignments(t *testing.T) {
	now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Hour), now.Add(time.Second)
	for _, tc := range []struct {
		name string
		at   *time.Time
		want time.Duration
	}{
		{"unknown", nil, 0},
		{"established", &past, time.Hour},
		{"just assigned", &now, time.Nanosecond},
		{"database clock ahead", &future, time.Nanosecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := MailboxPlacementState{AssignedAt: tc.at}
			if got := state.Residency(now); got != tc.want {
				t.Fatalf("Residency() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPlacementImbalancedUsesExistingFleetHeadroom(t *testing.T) {
	tests := []struct {
		name  string
		state MailboxPlacementState
		want  bool
	}{
		{
			name: "one worker cannot rebalance",
			state: MailboxPlacementState{
				LiveWorkerCount: 1, WorkerOrgMailboxes: 88, OrgTotalMailboxes: 88,
			},
		},
		{
			name: "workspace concentrated on one of two workers",
			state: MailboxPlacementState{
				LiveWorkerCount: 2, WorkerOrgMailboxes: 88, OrgTotalMailboxes: 88,
			},
			want: true,
		},
		{
			name: "provider concentration above fair share",
			state: MailboxPlacementState{
				LiveWorkerCount: 2, WorkerProviderMailboxes: 70, ProviderTotalMailboxes: 88,
			},
			want: true,
		},
		{
			name: "balanced",
			state: MailboxPlacementState{
				LiveWorkerCount: 2, WorkerOrgMailboxes: 44, OrgTotalMailboxes: 88,
				WorkerProviderMailboxes: 44, ProviderTotalMailboxes: 88,
			},
		},
		{
			name: "reserved worker keeps its organization together",
			state: func() MailboxPlacementState {
				workerID := uuid.New()
				return MailboxPlacementState{
					WorkerID: &workerID, ReservedWorkerID: &workerID,
					LiveWorkerCount: 2, WorkerOrgMailboxes: 88, OrgTotalMailboxes: 88,
				}
			}(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.state.PlacementImbalanced(); got != tc.want {
				t.Fatalf("PlacementImbalanced() = %v, want %v", got, tc.want)
			}
		})
	}
}
