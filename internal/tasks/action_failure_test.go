package tasks

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/repository"
	"strings"
	"testing"
)

type failedActionTaskRepo struct {
	repository.TaskRepository
	reason   string
	failures int
}

func (r *failedActionTaskRepo) RecordTaskFailure(_ context.Context, _ uuid.UUID, _, reason string) error {
	r.reason = reason
	r.failures++
	return nil
}

type failedActionCampaignRepo struct {
	repository.CampaignRepository
	status string
}

func (r *failedActionCampaignRepo) UpdateStatusWithLock(_ context.Context, _ uuid.UUID, status string) error {
	r.status = status
	return nil
}
func TestFailedCampaignActionPausesForReview(t *testing.T) {
	task := &failedActionTaskRepo{}
	campaign := &failedActionCampaignRepo{}
	s := &tasksService{taskRepo: task, campaignRepo: campaign}
	s.recordActionFailure(context.Background(), uuid.New(), uuid.New(), errors.New("capability unavailable"))
	if campaign.status != "paused" || task.failures != 1 || !strings.Contains(task.reason, "before resuming") {
		t.Fatal(campaign.status, task.reason)
	}
}
