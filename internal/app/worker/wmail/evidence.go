package wmail

import (
	"errors"

	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/pkg/nodeevidence"
	"github.com/pramaan-oss/cold-email/internal/repository"
	"github.com/rs/zerolog/log"
)

func evidenceHTTPStatus(err error) int {
	var status *repository.ControlPlaneHTTPError
	if errors.As(err, &status) {
		return status.Status
	}
	return 0
}

func (w *WMail) controlPlaneError(err error, stats *tickStats) *errx.MailError {
	nodeevidence.Emit(nodeevidence.ControlPlaneHeld, w.ID, evidenceHTTPStatus(err), 1)
	log.Warn().Err(err).Str("email_id", w.ID.String()).Msg("sync: control-plane call failed; pass ended, cursors held")
	stats.aborted = true
	return nil
}
