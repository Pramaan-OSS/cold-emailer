package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/api/middleware"
	"github.com/pramaan-oss/cold-email/internal/models"
)

func TestDomainProxyRefusesManagerAndAbsentMembership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{"manager", "viewer", ""} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/capabilities/domains.registration.create", strings.NewReader(`{"domain":"synthetic.example","confirmation":"not-a-quote"}`))
		c.Request.Header.Set("Idempotency-Key", uuid.NewString())
		c.Set(middleware.OrganizationIDKey, uuid.New())
		c.Set(middleware.UserIDKey, uuid.NewString())
		if role != "" {
			c.Set("member", &models.OrganizationMember{Role: role})
		}
		c.Params = gin.Params{{Key: "capability", Value: "domains.registration.create"}}
		(&Handler{}).PramaanCall(c)
		if w.Code != 403 {
			t.Fatal(role, w.Code, w.Body.String())
		}
	}
}
