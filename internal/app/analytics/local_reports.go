package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
)

func (s *analyticsService) GetWarmupStatsForAccounts(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID, from, to time.Time) ([]models.WarmupDailyStats, *errx.Error) {
	return s.analyticsRepo.GetWarmupStatsForAccounts(ctx, orgID, ids, from, to)
}

func (s *analyticsService) GetWarmupPlacementDataForAccounts(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID, from, to time.Time) (*models.WarmupPlacementData, *errx.Error) {
	if s.placementRepo == nil {
		return nil, errx.InternalError()
	}
	data, err := s.placementRepo.ForAccounts(ctx, orgID, ids, from, to)
	if err != nil {
		return nil, errx.InternalError()
	}
	return data, nil
}
