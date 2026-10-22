package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/api/middleware"
)

func TestMailboxOutcomeRetainsLocalRegistrationWarning(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/outcomes/mailbox.connect", nil)
	c.Set(middleware.OrganizationIDKey, uuid.New())
	c.Set(middleware.UserIDKey, uuid.NewString())
	h := Handler{}
	for _, state := range []string{"pending", "connected"} {
		result := map[string]any{"status": state, "inbox": "mailbox@pramaan.dev", "tested": true}
		report := map[string]any{"outcome": "mailbox.connect", "result": result}
		h.registerMailboxOutcome(c, report)
		_, warned := result["localRegistration"]
		if warned != (state == "connected") {
			t.Fatal("local registration must follow confirmed connection, even with a partial read", result)
		}
	}
	result := map[string]any{"status": "connected", "inbox": "mailbox@pramaan.dev"}
	h.registerMailboxOutcome(c, map[string]any{"outcome": "lead.find", "result": result})
	if result["localRegistration"] != nil {
		t.Fatal("unrelated outcome registered a mailbox")
	}
}
