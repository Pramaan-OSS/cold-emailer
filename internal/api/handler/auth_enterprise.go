package handler

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/api/middleware"
	"github.com/pramaan-oss/cold-email/internal/app/enterpriseauth"
	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

const enterpriseCookie = "enterprise_sign_in_state"

func enterpriseBody(c *gin.Context, out any) bool {
	d := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		c.JSON(400, gin.H{"error": gin.H{"code": "invalid_request", "message": "Use supported OIDC settings. SAML and custom scopes are unavailable."}})
		return false
	}
	return true
}
func enterpriseError(c *gin.Context, e error) {
	status := 502
	if errors.Is(e, pramaan.ErrEnterpriseApproval) || errors.Is(e, enterpriseauth.ErrRefused) {
		status = 403
	}
	c.JSON(status, gin.H{"error": gin.H{"code": "enterprise_sign_in_unavailable", "message": e.Error()}})
}
func (h *Handler) enterpriseActor(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	org := middleware.GetOrganizationID(c)
	user, e := middleware.GetUserUUID(c)
	if org == nil || e != nil || h.EnterpriseService == nil {
		c.JSON(403, gin.H{"error": gin.H{"message": "Choose your workspace and sign in locally first."}})
		return uuid.Nil, uuid.Nil, false
	}
	return *org, user, true
}
func (h *Handler) EnterpriseCreate(c *gin.Context) {
	org, user, ok := h.enterpriseActor(c)
	if !ok {
		return
	}
	var in pramaan.EnterpriseCreate
	if !enterpriseBody(c, &in) {
		return
	}
	out, e := h.EnterpriseService.Create(c.Request.Context(), org, user, c.GetHeader("Idempotency-Key"), in)
	if e != nil {
		enterpriseError(c, e)
		return
	}
	c.JSON(200, out)
}
func (h *Handler) EnterpriseList(c *gin.Context) {
	org, user, ok := h.enterpriseActor(c)
	if !ok {
		return
	}
	rows, cursor, e := h.EnterpriseService.List(c.Request.Context(), org, user, c.Query("cursor"))
	if e != nil {
		enterpriseError(c, e)
		return
	}
	c.JSON(200, gin.H{"results": rows, "nextCursor": cursor, "redirectUri": h.EnterpriseService.Platform.RedirectURI})
}
func (h *Handler) EnterpriseChoices(c *gin.Context) {
	org, user, ok := h.enterpriseActor(c)
	if !ok {
		return
	}
	rows, e := h.EnterpriseService.Choices(c.Request.Context(), org, user)
	if e != nil {
		enterpriseError(c, e)
		return
	}
	c.JSON(200, gin.H{"results": rows, "redirectUri": h.EnterpriseService.Platform.RedirectURI})
}
func (h *Handler) EnterpriseDelete(c *gin.Context) {
	org, user, ok := h.enterpriseActor(c)
	if !ok {
		return
	}
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		c.Status(400)
		return
	}
	var in struct {
		Approved bool `json:"approved"`
	}
	if !enterpriseBody(c, &in) {
		return
	}
	if e = h.EnterpriseService.Delete(c.Request.Context(), org, user, id, c.GetHeader("Idempotency-Key"), in.Approved); e != nil {
		enterpriseError(c, e)
		return
	}
	c.JSON(200, gin.H{"deleted": true})
}
func enterpriseSetCookie(c *gin.Context, state string, age int) {
	c.SetSameSite(http.SameSiteLaxMode)
	secure := c.Request.TLS != nil || strings.HasPrefix(os.Getenv("API_PUBLIC_URL"), "https://")
	c.SetCookie(enterpriseCookie, state, age, "/v1/auth/enterprise", "", secure, true)
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
}
func (h *Handler) EnterpriseBegin(c *gin.Context)     { h.enterpriseBegin(c, false) }
func (h *Handler) EnterpriseLinkBegin(c *gin.Context) { h.enterpriseBegin(c, true) }
func (h *Handler) enterpriseBegin(c *gin.Context, link bool) {
	var in struct {
		OrganizationID uuid.UUID `json:"organization_id"`
		ConnectionID   uuid.UUID `json:"connection_id"`
		Approved       bool      `json:"approved"`
	}
	if !enterpriseBody(c, &in) {
		return
	}
	user := uuid.Nil
	if h.EnterpriseService == nil {
		c.JSON(503, gin.H{"error": gin.H{"message": "Organisation sign-in is not available yet. Use your local account."}})
		return
	}
	if link {
		org, actor, ok := h.enterpriseActor(c)
		if !ok {
			return
		}
		in.OrganizationID = org
		user = actor
		if !in.Approved {
			enterpriseError(c, pramaan.ErrEnterpriseApproval)
			return
		}
	}
	out, e := h.EnterpriseService.Begin(c.Request.Context(), in.OrganizationID, in.ConnectionID, user, link)
	if e != nil {
		enterpriseError(c, e)
		return
	}
	enterpriseSetCookie(c, out.State, 600)
	c.JSON(200, out)
}
func (h *Handler) EnterpriseCallback(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	base := config.AppBaseURL()
	refused := func() {
		c.Redirect(http.StatusFound, base+"/auth/login?sso_error="+url.QueryEscape("Organisation sign-in was refused. Start again or use your local account."))
	}
	state := c.Query("state")
	browserState, _ := c.Cookie(enterpriseCookie)
	if state == "" || browserState == "" || subtle.ConstantTimeCompare([]byte(state), []byte(browserState)) != 1 || h.EnterpriseService == nil || h.AuthService == nil {
		refused()
		return
	}
	enterpriseSetCookie(c, "", -1)
	if c.Query("error") != "" {
		refused()
		return
	}
	resolved, e := h.EnterpriseService.Redeem(c.Request.Context(), c.Query("code"), state, browserState)
	if e != nil {
		refused()
		return
	}
	handoff, xerr := h.AuthService.EnterpriseHandoff(c.Request.Context(), resolved.UserID, resolved.Binding, c.ClientIP(), c.Request.UserAgent())
	if xerr != nil {
		refused()
		return
	}
	c.Redirect(http.StatusFound, base+"/auth/sso?code="+url.QueryEscape(handoff))
}
