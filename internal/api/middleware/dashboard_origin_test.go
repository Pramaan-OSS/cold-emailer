package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pramaan-oss/cold-email/internal/config"
)

func TestDashboardOriginMiddleware(t *testing.T) {
	t.Setenv("APP_URL", "https://example.test")
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://example.test")
	r := gin.New()
	r.Use(DashboardOriginMiddleware())
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, config.DashboardBaseURL(c.Request.Context())) })
	for _, tt := range []struct{ origin, want string }{
		{"https://example.test", "https://example.test"},
		{"https://evil.example.com", "https://example.test"},
		{"", "https://example.test"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", tt.origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Body.String() != tt.want {
			t.Errorf("request origin %q returned %q", tt.origin, w.Body.String())
		}
	}
}
