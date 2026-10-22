package middleware

import (
	"github.com/MicahParks/keyfunc/v3"
	"github.com/gin-gonic/gin"
)

type OidcHandler struct {
	ServiceAccount string
	KeySet         keyfunc.Keyfunc
	AppEnv         string
	// Audience is the webhook URL Cloud Tasks names in the token it mints.
	// Required: an empty one refuses every request.
	Audience string
}

func (h *OidcHandler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.AbortWithStatusJSON(503, gin.H{"error": "Hosted task callbacks are unavailable; the local dispatcher executes jobs in process."})
	}
}
