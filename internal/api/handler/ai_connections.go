// Private remote connectors: local ownership/settings and Pramaan-only transport.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pramaan-oss/cold-email/internal/api/middleware"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
)

func (h *Handler) mcpOrgUser(c *gin.Context) (uuid.UUID, uuid.UUID, *errx.Error) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil || h.MCPService == nil {
		return uuid.Nil, uuid.Nil, errx.New(errx.BadRequest, "no organization selected")
	}
	userID, err := middleware.GetUserUUID(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, errx.New(errx.Unauthorized, "invalid user")
	}
	return *orgID, userID, nil
}

// ListMCPServers — GET /ai/connections
func (h *Handler) ListMCPServers(c *gin.Context) {
	orgID, userID, xerr := h.mcpOrgUser(c)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	list, lerr := h.MCPService.List(c.Request.Context(), orgID, userID)
	if lerr != nil {
		errx.JSON(c, lerr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// CreateMCPServer — POST /ai/connections
func (h *Handler) CreateMCPServer(c *gin.Context) {
	orgID, userID, xerr := h.mcpOrgUser(c)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	var req models.CreateMCPServer
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 120<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	sv, cerr := h.MCPService.Create(c.Request.Context(), orgID, userID, &req, c.GetHeader("Idempotency-Key"))
	if cerr != nil {
		errx.JSON(c, cerr)
		return
	}
	h.auditOrg(c, models.AuditActionCreate, models.AuditEntityMCPServer, &sv.ID, nil, map[string]string{"name": sv.Name})
	c.JSON(http.StatusOK, sv)
}

// UpdateMCPServer — PATCH /ai/connections/:id (rename, enable/disable, re-token)
func (h *Handler) UpdateMCPServer(c *gin.Context) {
	orgID, userID, xerr := h.mcpOrgUser(c)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid connection id"))
		return
	}
	var req models.UpdateMCPServer
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 120<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	sv, uerr := h.MCPService.Update(c.Request.Context(), orgID, userID, id, &req)
	if uerr != nil {
		errx.JSON(c, uerr)
		return
	}
	h.auditOrg(c, models.AuditActionUpdate, models.AuditEntityMCPServer, &sv.ID, nil, map[string]string{"name": sv.Name})
	c.JSON(http.StatusOK, sv)
}

// DeleteMCPServer — DELETE /ai/connections/:id
func (h *Handler) DeleteMCPServer(c *gin.Context) {
	orgID, userID, xerr := h.mcpOrgUser(c)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid connection id"))
		return
	}
	if derr := h.MCPService.Delete(c.Request.Context(), orgID, userID, id); derr != nil {
		errx.JSON(c, derr)
		return
	}
	h.auditOrg(c, models.AuditActionDelete, models.AuditEntityMCPServer, &id, nil, nil)
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// RefreshMCPServer — POST /ai/connections/:id/refresh (re-discover tools)
func (h *Handler) RefreshMCPServer(c *gin.Context) {
	orgID, userID, xerr := h.mcpOrgUser(c)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid connection id"))
		return
	}
	sv, rerr := h.MCPService.Refresh(c.Request.Context(), orgID, userID, id)
	if rerr != nil {
		errx.JSON(c, rerr)
		return
	}
	c.JSON(http.StatusOK, sv)
}

func (h *Handler) CallMCPTool(c *gin.Context) {
	org, user, x := h.mcpOrgUser(c)
	if x != nil {
		errx.JSON(c, x)
		return
	}
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "Choose a connection."))
		return
	}
	var req struct {
		ToolRef  string         `json:"tool_ref"`
		Input    map[string]any `json:"input"`
		Approved bool           `json:"approved"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 120<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || req.Input == nil {
		errx.JSON(c, errx.New(errx.BadRequest, "Use an object with the reviewed tool fields."))
		return
	}
	out, x := h.MCPService.Call(c.Request.Context(), org, user, id, req.ToolRef, req.Input, req.Approved, c.GetHeader("Idempotency-Key"))
	if x != nil {
		errx.JSON(c, x)
		return
	}
	c.JSON(200, out)
}
