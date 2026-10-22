package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/app/auth"
	"github.com/pramaan-oss/cold-email/internal/errx"
)

type enterpriseSessionFake struct {
	auth.AuthService
	calls int
}

func (f *enterpriseSessionFake) EnterpriseHandoff(context.Context, uuid.UUID, string, string, string) (string, *errx.Error) {
	f.calls++
	return "handoff", nil
}
func TestEnterpriseCallbackRequiresSameBrowserStateBeforeSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	session := &enterpriseSessionFake{}
	h := &Handler{AuthService: session}
	router := gin.New()
	router.GET("/v1/auth/enterprise/callback", h.EnterpriseCallback)
	for _, query := range []string{"code=synthetic", "code=synthetic&state=wrong", "code=synthetic&state=unknown"} {
		req := httptest.NewRequest("GET", "/v1/auth/enterprise/callback?"+query, nil)
		req.AddCookie(&http.Cookie{Name: enterpriseCookie, Value: "expected"})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 302 || !strings.Contains(w.Header().Get("Location"), "sso_error=") {
			t.Fatal(w.Code, w.Header())
		}
	}
	if session.calls != 0 {
		t.Fatal("local session created from unstarted callback")
	}
}
func TestEnterpriseStartCookieIsBrowserPrivateAndCallbackScoped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "https://app.example.test/v1/auth/enterprise/begin", nil)
	enterpriseSetCookie(c, "synthetic-state", 600)
	response := w.Header()
	raw := response.Get("Set-Cookie")
	if !strings.Contains(raw, "HttpOnly") || !strings.Contains(raw, "SameSite=Lax") || !strings.Contains(raw, "Secure") || !strings.Contains(raw, "Path=/v1/auth/enterprise") {
		t.Fatal(raw)
	}
	if response.Get("Cache-Control") != "no-store" {
		t.Fatal(response)
	}
}
func TestEnterpriseCreateRejectsUnsupportedFieldsWithoutEchoingCredentials(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"samlMetadata":"synthetic-secret"}`))
	var in map[string]string
	// An explicit struct, as in the connection handler, rejects unsupported options.
	var typed struct {
		Name string `json:"name"`
	}
	if enterpriseBody(c, &typed) {
		t.Fatal("unknown fields allowed")
	}
	json.Unmarshal(w.Body.Bytes(), &in)
	if strings.Contains(w.Body.String(), "synthetic-secret") {
		t.Fatal("credential echoed")
	}
}
