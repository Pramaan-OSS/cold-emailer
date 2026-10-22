package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pramaan-oss/cold-email/internal/api/middleware"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

func outcomeError(c *gin.Context, e error) {
	status, code, effect := 502, "internal", "unknown"
	var he *pramaan.HTTPError
	if errors.As(e, &he) {
		status, code, effect = he.Status, he.Code, he.Effect
	} else if errors.Is(e, pramaan.ErrOutcomeUnavailable) {
		status, code, effect = 502, "upstream_unavailable", "none"
	} else if errors.Is(e, pramaan.ErrUnavailable) {
		status, code, effect = 404, "not_found", "none"
	} else if errors.Is(e, pramaan.ErrNotConfigured) {
		status, code, effect = 503, "connection_required", "none"
	}
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": e.Error(), "effect": effect}})
}
func (h *Handler) PramaanOutcome(c *gin.Context) {
	org := middleware.GetOrganizationID(c)
	if org == nil {
		outcomeError(c, &pramaan.HTTPError{Status: 403, Code: "forbidden", Effect: "none"})
		return
	}
	actor := middleware.GetUserID(c)
	if actor == "" {
		outcomeError(c, &pramaan.HTTPError{Status: 403, Code: "forbidden", Effect: "none"})
		return
	}
	ctx := c.Request.Context()
	id := c.Param("outcome")
	for _, name := range []string{"Pramaan-Account", "Pramaan-End-User", "Pramaan-End-User-Group", "Pramaan-Max-Price-Micro"} {
		if c.GetHeader(name) != "" {
			outcomeError(c, &pramaan.HTTPError{Status: 400, Code: "invalid_request", Effect: "none"})
			return
		}
	}
	var out map[string]any
	var e error
	status := 200
	if c.Request.Method == "GET" {
		if id == "" {
			out, e = h.PramaanGateway.OutcomesList(ctx)
		} else {
			out, e = h.PramaanGateway.OutcomeGet(ctx, id)
		}
	} else {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		var in pramaan.OutcomeRequest
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		if decoder.Decode(&in) != nil || in.Input == nil {
			outcomeError(c, &pramaan.HTTPError{Status: 400, Code: "invalid_request", Effect: "none"})
			return
		}
		if op := c.Param("operation"); op != "" && op != "estimate" {
			outcomeError(c, &pramaan.HTTPError{Status: 404, Code: "not_found", Effect: "none"})
			return
		}
		if c.Param("operation") == "estimate" {
			out, e = h.PramaanGateway.OutcomeEstimate(ctx, org.String(), actor, id, in)
		} else {
			key := c.GetHeader("Idempotency-Key")
			if key == "" {
				outcomeError(c, &pramaan.HTTPError{Status: 400, Code: "invalid_request", Effect: "none"})
				return
			}
			status, out, e = h.PramaanGateway.OutcomeStart(ctx, org.String(), actor, id, in, key, c.GetHeader("Pramaan-Plan"))
		}
	}
	if e != nil {
		outcomeError(c, e)
		return
	}
	if c.Request.Method == "POST" && c.Param("operation") == "" && status != 202 {
		h.registerMailboxOutcome(c, out)
	}
	c.JSON(status, out)
}
func (h *Handler) PramaanOutcomeJob(c *gin.Context) {
	org := middleware.GetOrganizationID(c)
	if org == nil {
		outcomeError(c, &pramaan.HTTPError{Status: 403, Code: "forbidden", Effect: "none"})
		return
	}
	var out map[string]any
	var e error
	if c.Request.Method == "POST" {
		out, e = h.PramaanGateway.OutcomeCancel(c.Request.Context(), org.String(), middleware.GetUserID(c), c.Param("job"))
	} else {
		out, e = h.PramaanGateway.OutcomeJob(c.Request.Context(), org.String(), middleware.GetUserID(c), c.Param("job"))
	}
	if e != nil {
		outcomeError(c, e)
		return
	}
	if report, ok := out["result"].(map[string]any); ok {
		h.registerMailboxOutcome(c, report)
	}
	c.JSON(200, out)
}

func (h *Handler) PramaanLeadSearchEstimate(c *gin.Context) {
	if h.PramaanGateway == nil {
		outcomeError(c, pramaan.ErrNotConfigured)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	var in map[string]any
	if json.NewDecoder(c.Request.Body).Decode(&in) != nil {
		outcomeError(c, &pramaan.HTTPError{Status: 400, Code: "invalid_request", Effect: "none"})
		return
	}
	client, ok := h.PramaanGateway.Client.(interface {
		CapabilityEstimate(context.Context, string, map[string]any) (map[string]any, error)
	})
	if !ok {
		outcomeError(c, pramaan.ErrUnavailable)
		return
	}
	id := c.Param("capability")
	if id == "" {
		id = "companies.search"
	}
	if id != "companies.search" && id != "email.domain.create" {
		outcomeError(c, pramaan.ErrUnavailable)
		return
	}
	out, e := client.CapabilityEstimate(c.Request.Context(), id, in)
	if e != nil {
		outcomeError(c, e)
		return
	}
	c.JSON(200, out)
}
