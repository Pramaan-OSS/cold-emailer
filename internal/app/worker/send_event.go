package worker

import (
	"context"

	"github.com/pramaan-oss/cold-email/internal/infrastructure/kafka"
	"github.com/pramaan-oss/cold-email/internal/models"
)

func (s *WorkerService) Produce(jobEventType models.JobEventType, key string, body any) error {
	ctx := context.Background()
	payload, err := s.Codec.Serialize(ctx, kafka.TopicWorkerEvents, &models.JobEvent{
		Type: jobEventType,
		Body: body,
	})
	if err != nil {
		return err
	}
	return s.Bus.Publish(ctx, kafka.TopicWorkerEvents, key, payload)
}
