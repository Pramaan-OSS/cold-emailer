package captcha

import (
	"context"

	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type Config struct {
	Enabled bool
	Client  pramaan.Caller
}
type BotCheck struct{ cfg Config }

func NewBotCheck(cfg Config) *BotCheck { return &BotCheck{cfg: cfg} }
func (b *BotCheck) Verify(ctx context.Context, token, remoteIP string) *errx.Error {
	if !b.cfg.Enabled {
		return nil
	}
	if token == "" {
		return errx.ErrCaptcha
	}
	in := map[string]any{"token": token}
	if remoteIP != "" {
		in["remoteIp"] = remoteIP
	}
	var out struct {
		Passed bool `json:"passed"`
	}
	var err error
	if b.cfg.Client != nil {
		err = b.cfg.Client.Call(ctx, "auth.bot-check.verify", in, &out)
	} else {
		err = pramaan.Call(ctx, "auth.bot-check.verify", in, &out)
	}
	if err != nil || !out.Passed {
		return errx.ErrCaptcha
	}
	return nil
}
