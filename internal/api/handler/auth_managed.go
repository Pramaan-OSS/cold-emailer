package handler

import (
	"crypto/subtle"
	"github.com/gin-gonic/gin"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"net/http"
)

type ManagedSignInConfig struct {
	Available bool   `json:"available"`
	AuthURL   string `json:"authUrl,omitempty"`
}

func (h *Handler) managedReady() bool { return h.ManagedClient != nil && h.ManagedState.AuthReady }
func (h *Handler) managedOrigin(c *gin.Context) bool {
	return h.managedReady() && c.GetHeader("Origin") == h.ManagedState.AuthOrigin
}
func (h *Handler) ManagedBegin(c *gin.Context) {
	if !h.managedOrigin(c) {
		errx.Handle(c, errx.ErrExternalProvider)
		return
	}
	state, x := h.AuthService.ManagedBegin(c.Request.Context())
	if x != nil {
		errx.Handle(c, x)
		return
	}
	c.JSON(http.StatusOK, gin.H{"state": state})
}
func (h *Handler) ManagedSession(c *gin.Context) {
	if !h.managedOrigin(c) {
		errx.Handle(c, errx.ErrExternalProvider)
		return
	}
	var req struct {
		Token      string `json:"token"`
		State      string `json:"state"`
		Binding    string `json:"binding"`
		SetupToken string `json:"setupToken"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.State) != 64 || subtle.ConstantTimeCompare([]byte(req.State), []byte(req.Binding)) != 1 {
		errx.Handle(c, errx.ErrExternalCode)
		return
	}
	identity, e := pramaan.VerifyManagedToken(c.Request.Context(), h.ManagedClient, h.ManagedState, req.Token)
	if e != nil {
		errx.Handle(c, errx.ErrExternalCode)
		return
	}
	var claim = h.BootstrapService
	if claim == nil {
		errx.Handle(c, errx.ErrExternalProvider)
		return
	}
	result, x := h.AuthService.ManagedFinish(c.Request.Context(), req.State, identity, req.SetupToken, c.ClientIP(), c.Request.UserAgent(), claim.ClaimManaged)
	if x != nil {
		errx.Handle(c, x)
		return
	}
	c.JSON(http.StatusOK, result)
}
