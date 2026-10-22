package observability

import (
	"context"
	"os"
	"strings"

	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/observability/errs"
	"github.com/pramaan-oss/cold-email/internal/version"
)

// Init configures local error logging; no external credentials are read.
func Init(ctx context.Context, cfg *config.Config, service string) error {
	return errs.Init(errs.Config{Environment: cfg.Env, Release: Release(), Service: service})
}
func InitEnv(service string) error {
	return errs.Init(errs.Config{Environment: strings.TrimSpace(os.Getenv("APP_ENV")), Release: Release(), Service: service})
}

// Release is the value every service tags its events with, so a stack trace
// names the build it came from. It is the stamped release when the build
// stamped one and the short commit otherwise; an unstamped local build reports
// "dev", which is what version.String falls back to.
func Release() string {
	if v := version.String(); v != "" && v != "dev" {
		return v
	}
	if c := version.ShortCommit(); c != "" {
		return c
	}
	return "dev"
}
