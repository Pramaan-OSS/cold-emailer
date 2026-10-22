package notify

import (
	"context"

	"github.com/pramaan-oss/cold-email/internal/pramaan"

	"github.com/pramaan-oss/cold-email/internal/config"
)

type Transport struct {
	EmailNotificationService
	Kind, Description string
	Delivers          bool
}

func NewTransport(ctx context.Context, cfg *config.Config, name, address string, client pramaan.Caller) (*Transport, error) {
	if client == nil {
		return &Transport{EmailNotificationService: NewLogEmailNotificationService(name, address), Kind: "log", Description: "Email delivery is not connected.", Delivers: false}, nil
	}
	s := NewEmailNotificationService(client, name, address)
	return &Transport{EmailNotificationService: s, Kind: "pramaan", Description: "Pramaan email delivery", Delivers: true}, nil
}
func (t *Transport) Preflight(context.Context) error { return nil }
