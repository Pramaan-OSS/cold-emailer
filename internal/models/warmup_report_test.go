package models

import (
	"encoding/json"
	"testing"
)

func TestMergeWarmupStatsPreservesActiveDaysAcrossTransport(t *testing.T) {
	raw, err := json.Marshal([]WarmupDailyStats{{Date: "2026-10-01", EmailsSent: 9, TargetVolume: 10, Active: true}})
	if err != nil {
		t.Fatal(err)
	}
	var cloud []WarmupDailyStats
	if err := json.Unmarshal(raw, &cloud); err != nil {
		t.Fatal(err)
	}
	out := MergeWarmupStats([]WarmupDailyStats{
		{Date: "2026-10-02", EmailsReceived: 2},
		{Date: "2026-10-01", EmailsSent: 1, EmailsReplied: 1, TargetVolume: 2, Active: true},
	}, cloud)
	if len(out) != 2 || out[0].Date != "2026-10-01" || out[0].EmailsSent != 10 || out[0].TargetVolume != 12 || !out[0].Active || out[1].Active {
		t.Fatalf("merged=%+v", out)
	}
}
