package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/pramaan-oss/cold-email/internal/api/middleware"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"net/http"
)

func (h *Handler) PramaanCompany(c *gin.Context) {
	org := middleware.GetOrganizationID(c)
	if org == nil {
		c.JSON(403, gin.H{"error": "Choose a workspace first."})
		return
	}
	member := middleware.GetMember(c)
	if member == nil && h.OrganizationService != nil {
		if user, err := middleware.GetUserUUID(c); err == nil {
			member, _ = h.OrganizationService.GetMembership(c.Request.Context(), *org, user)
		}
	}
	role := ""
	if member != nil {
		role = member.Role
	}
	if role != "owner" && role != "admin" {
		c.JSON(403, gin.H{"error": gin.H{"code": "forbidden", "effect": "none", "message": "Only a workspace owner or admin can manage a company connection."}})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var in map[string]any
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "Check the company connection fields."})
		return
	}
	key := c.GetHeader("Idempotency-Key")
	out, err := h.PramaanGateway.CompanyCall(c.Request.Context(), org.String(), role, c.Param("action"), in, key)
	if err != nil {
		outcomeError(c, err)
		return
	}
	if c.Param("action") == "enroll" {
		address, _ := out["mailbox"].(string)
		ref, _ := out["companyRef"].(string)
		// The opaque reference is stored in the existing encrypted mailbox settings,
		// then reconstructed by the worker. It is never a credential or vendor ID.
		h.registerCompanyMailbox(c, out, address, ref)
	}
	c.JSON(200, out)
}
func (h *Handler) registerCompanyMailbox(c *gin.Context, out map[string]any, address, ref string) {
	org := middleware.GetOrganizationID(c)
	if org == nil || h.EmailService == nil {
		out["localRegistration"] = "Connection checked. Local mailbox registration needs another check."
		return
	}
	ctx := c.Request.Context()
	ctx = pramaan.WithOwnedInbox(ctx, address)
	_, err := h.EmailService.OnboardSMTPIMAP(ctx, middleware.GetUserID(c), org, &models.NewSMTPIMAPAccount{Email: address, Name: address, SMTP: &models.Service{Host: "broker.invalid", Port: 587, Username: "company:" + ref, Password: "broker-owned"}, IMAP: &models.Service{Host: "broker.invalid", Port: 993, Username: address, Password: "broker-owned"}})
	if err != nil {
		out["localRegistration"] = "Company mailbox access was checked. Local registration needs another check."
	}
}
