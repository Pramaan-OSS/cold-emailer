package api

import (
	"github.com/gin-gonic/gin"
	"github.com/pramaan-oss/cold-email/internal/api/handler"
	"github.com/pramaan-oss/cold-email/internal/api/middleware"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOutcomeApprovalHeaderAllowedFromAppOrigin(t *testing.T) {
	r := Run(&handler.Handler{}, &middleware.Handler{}, &middleware.OidcHandler{}, "", gin.TestMode, []string{"http://localhost:54525"})
	req := httptest.NewRequest(http.MethodOptions, "/v1/auth/pramaan/v1/outcomes/mailbox.connect", nil)
	req.Header.Set("Origin", "http://localhost:54525")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type,idempotency-key,pramaan-plan")
	out := httptest.NewRecorder()
	r.ServeHTTP(out, req)
	if out.Code != 204 || !strings.Contains(strings.ToLower(out.Header().Get("Access-Control-Allow-Headers")), "pramaan-plan") {
		t.Fatalf("approval preflight refused: status=%d headers=%v", out.Code, out.Header())
	}
}
