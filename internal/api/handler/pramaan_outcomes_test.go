package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/api/middleware"
)

func TestOutcomeProxyRejectsRoutingOverridesAndInvalidBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, header := range []string{"Pramaan-Account", "Pramaan-End-User", "Pramaan-End-User-Group", "Pramaan-Max-Price-Micro", ""} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"input":{},"account":"forbidden"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/outcomes/lead.find", strings.NewReader(body))
		if header != "" {
			c.Request.Header.Set(header, "override")
		}
		c.Set(middleware.OrganizationIDKey, uuid.New())
		c.Set(middleware.UserIDKey, uuid.NewString())
		c.Params = gin.Params{{Key: "outcome", Value: "lead.find"}}
		// A nil gateway proves refusal happened before any broker transport.
		(&Handler{}).PramaanOutcome(c)
		if w.Code != 400 {
			t.Fatal(header, w.Code, w.Body.String())
		}
	}
}
