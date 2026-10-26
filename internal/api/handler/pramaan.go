package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/pramaan-oss/cold-email/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/pramaan-oss/cold-email/internal/api/middleware"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

func (h *Handler) PramaanCall(c *gin.Context) {
	org := middleware.GetOrganizationID(c)
	if org == nil {
		c.JSON(403, gin.H{"error": gin.H{"code": "forbidden", "message": "Choose a workspace first."}})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<20)
	var in map[string]any
	if json.NewDecoder(c.Request.Body).Decode(&in) != nil {
		c.JSON(400, gin.H{"error": gin.H{"code": "invalid_request", "message": "The request could not be read."}})
		return
	}
	key := c.GetHeader("Idempotency-Key")
	if key == "" {
		c.JSON(400, gin.H{"error": gin.H{"code": "invalid_request", "message": "The action needs a request reference."}})
		return
	}
	var out any
	var e error
	if pramaan.DomainCapability(c.Param("capability")) || pramaan.AdoptionCapability(c.Param("capability")) {
		member := middleware.GetMember(c)
		if member == nil && h.OrganizationService != nil {
			userID, err := middleware.GetUserUUID(c)
			if err == nil {
				member, _ = h.OrganizationService.GetMembership(c.Request.Context(), *org, userID)
			}
		}
		role := ""
		if member != nil {
			role = member.Role
		}
		if pramaan.AdoptionCapability(c.Param("capability")) {
			out, e = h.PramaanGateway.AdoptionCall(c.Request.Context(), org.String(), role, c.Param("capability"), in, key)
		} else {
			out, e = h.PramaanGateway.DomainCall(c.Request.Context(), org.String(), middleware.GetUserID(c), role, c.Param("capability"), in, key)
		}
	} else {
		out, e = h.PramaanGateway.Call(c.Request.Context(), org.String(), c.Param("capability"), in, key)
	}
	if e != nil {
		status, code, effect := 502, "internal", "unknown"
		reason := ""
		var partial *pramaan.IncompleteReply
		if errors.As(e, &partial) {
			status, code, effect = 422, "upstream_unavailable", "unknown"
			reason = "reply_incomplete"
		}
		if errors.Is(e, pramaan.ErrReplyLimit) {
			status, code, effect = 400, "invalid_request", "none"
			reason = "reply_limit"
		}
		var he *pramaan.HTTPError
		if errors.As(e, &he) {
			status, code, effect = he.Status, he.Code, he.Effect
		}
		if errors.Is(e, pramaan.ErrDomainUnavailable) {
			status, code, effect = 502, "upstream_unavailable", "none"
		}
		if errors.Is(e, pramaan.ErrMailHeadersRefused) {
			status, code, effect = 400, "invalid_request", "none"
			reason = "headers_refused"
		}
		if e == pramaan.ErrUnavailable {
			status, code = 501, "not_found"
		}
		if e == pramaan.ErrNotConfigured {
			status, code = 503, "connection_required"
		}
		c.JSON(status, gin.H{"error": gin.H{"code": code, "message": e.Error(), "effect": effect, "reason": reason}})
		return
	}
	if c.Param("capability") == "email.inbox.create" || c.Param("capability") == "email.account.connect.status" {
		if resource, ok := out.(map[string]any); ok {
			address, _ := resource["address"].(string)
			if address == "" && resource["status"] == "connected" {
				address, _ = resource["inbox"].(string)
			}
			name, _ := in["displayName"].(string)
			h.registerPramaanMailbox(c, resource, address, name)
		}
	}
	c.JSON(200, out)
}

// Register only a broker-owned mailbox returned by the scoped gateway.
func (h *Handler) registerPramaanMailbox(c *gin.Context, resource map[string]any, address, name string) {
	if address == "" {
		return
	}
	org := middleware.GetOrganizationID(c)
	if org == nil {
		return
	}
	if name == "" {
		name = "Pramaan mailbox"
	}
	ctx := pramaan.WithOwnedInbox(c.Request.Context(), address)
	if h.EmailService == nil {
		resource["localRegistration"] = "Mailbox connection is saved. Campaign registration needs another check."
		return
	}
	_, err := h.EmailService.OnboardSMTPIMAP(ctx, middleware.GetUserID(c), org, &models.NewSMTPIMAPAccount{Email: address, Name: name, SMTP: &models.Service{Host: "broker.invalid", Port: 587, Username: address, Password: "broker-owned"}, IMAP: &models.Service{Host: "broker.invalid", Port: 993, Username: address, Password: "broker-owned"}})
	if err != nil {
		resource["localRegistration"] = "Mailbox connection is saved. Campaign registration needs another check."
	}
}
func (h *Handler) registerMailboxOutcome(c *gin.Context, report map[string]any) {
	if report["outcome"] != "mailbox.connect" {
		return
	}
	if result, ok := report["result"].(map[string]any); ok && result["status"] == "connected" {
		address, _ := result["inbox"].(string)
		h.registerPramaanMailbox(c, result, address, "")
	}
}
