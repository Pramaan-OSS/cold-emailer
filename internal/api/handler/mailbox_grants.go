package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/api/middleware"
	"github.com/pramaan-oss/cold-email/internal/errx"
)

func importCaller(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.ErrNoOrganization)
		return uuid.Nil, uuid.Nil, false
	}
	userID, err := middleware.GetUserUUID(c)
	if err != nil {
		errx.Handle(c, errx.ErrUser)
		return uuid.Nil, uuid.Nil, false
	}
	return *orgID, userID, true
}
