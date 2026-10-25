package models

import (
	"testing"
	"time"
)

func TestWarmupReceivingRequiresWarmupOn(t *testing.T) {
	started, paused := time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour)
	diagnostic, off, legacy := TestParticipationDiagnostic, TestParticipationOff, TestParticipationLegacy
	for _, tc := range []struct {
		name            string
		warmup, pausedA *time.Time
		mode            *TestParticipationMode
		send, receive   bool
		want            bool
	}{
		{"legacy, warmup on", &started, nil, nil, false, false, true},
		{"explicit legacy, warmup on", &started, nil, &legacy, false, false, true},
		{"legacy, never started", nil, nil, nil, false, false, false},
		{"legacy, paused", &started, &paused, nil, false, false, false},
		{"paused as the pause action leaves it", &started, &paused, &diagnostic, false, true, false},
		{"explicit receive-only, warmup on", &started, nil, &diagnostic, false, true, true},
		{"send and receive, warmup on", &started, nil, &diagnostic, true, true, true},
		{"send only, warmup on", &started, nil, &diagnostic, true, false, false},
		{"off with old flags, warmup on", &started, nil, &off, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := Email{Warmup: tc.warmup, WarmupPausedAt: tc.pausedA, TestMode: tc.mode, TestSendEnabled: tc.send, TestReceiveEnabled: tc.receive}
			if got := e.WarmupReceivingAllowed(); got != tc.want {
				t.Fatalf("WarmupReceivingAllowed()=%v want %v", got, tc.want)
			}
		})
	}
}
