package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pramaan-oss/cold-email/internal/app/warmupcontent"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
)

// AdminCancelWarmupBatch cancels an in-flight batch generation job (Pramaan + the
// local job row).
func (h *Handler) AdminCancelWarmupBatch(c *gin.Context) {
	if h.WarmupContentService == nil {
		errx.JSON(c, errx.New(errx.BadRequest, "warmup generation is not configured"))
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid id"))
		return
	}
	if err := h.WarmupContentService.CancelBatch(c.Request.Context(), id); err != nil {
		if errors.Is(err, warmupcontent.ErrNotConfigured) {
			errx.JSON(c, errx.New(errx.BadRequest, "Connect Pramaan to generate warmup messages."))
			return
		}
		var xe *errx.Error
		if errors.As(err, &xe) {
			errx.JSON(c, xe)
		} else {
			errx.JSON(c, errx.New(errx.Internal, err.Error()))
		}
		return
	}
	h.audit(c, models.AuditActionUpdate, warmupContentEntity, &id, map[string]string{"action": "cancel_batch"})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
