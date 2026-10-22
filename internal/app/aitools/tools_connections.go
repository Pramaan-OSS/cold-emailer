package aitools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pkg/generation"
)

func (d Deps) registerIntegrationTools(r *Registry) {
	if d.Automations == nil {
		return
	}
	r.Register(Tool{
		Name:        "list_integrations",
		Description: "List the integrations this workspace has connected (with their status and account) and the ones available to connect.",
		InputSchema: objectSchema(map[string]any{}),
		Risk:        generation.RiskRead,
		JWTOnly:     true,
		Handler:     d.listIntegrations,
	})
	r.Register(Tool{
		Name:            "integration_connect_link",
		Description:     "Get a link that opens the connect window for a connected app (app id returned by discovery). Connecting needs the user in a browser: give them the link.",
		InputSchema:     objectSchema(map[string]any{"provider": strProp("Integration provider id.")}, "provider"),
		Risk:            generation.RiskRead,
		JWTOnly:         true,
		RequiredOrgPerm: models.PermManageSettings,
		Handler:         d.integrationConnectLink,
	})
}

// canSeeIntegrations is the Integrations page's own gate: either permission.
func canSeeIntegrations(inv Invocation) bool {
	return inv.OrgPerms.HasPermission(models.PermManageSettings) || inv.OrgPerms.HasPermission(models.PermUseIntegrations)
}

func (d Deps) listIntegrations(ctx context.Context, inv Invocation, _ json.RawMessage) (string, error) {
	if !canSeeIntegrations(inv) {
		return "", ErrToolForbidden
	}
	conns, err := d.Automations.ListConnections(ctx, inv.OrgID)
	if err != nil {
		return "", errors.New("could not list integrations")
	}
	type connected struct {
		Provider string `json:"provider"`
		Label    string `json:"label"`
		Account  string `json:"account,omitempty"`
		Status   string `json:"status"`
	}
	type available struct {
		Provider string `json:"provider"`
		Name     string `json:"name"`
		Category string `json:"category"`
	}
	out := struct {
		Connected []connected `json:"connected"`
		Available []available `json:"available"`
	}{Connected: []connected{}, Available: []available{}}
	for _, c := range conns {
		if c.Status == models.IntegrationStatusDisconnected {
			continue
		}
		out.Connected = append(out.Connected, connected{Provider: string(c.Provider), Label: c.Label, Account: c.ExternalAccountName, Status: string(c.Status)})
	}
	for _, e := range d.Automations.Catalog(ctx) {
		// An app this instance has no credentials for cannot be connected.
		if !e.Configured {
			continue
		}
		out.Available = append(out.Available, available{Provider: string(e.Provider), Name: e.Name, Category: string(e.Category)})
	}
	return jsonResult(out)
}

func (d Deps) integrationConnectLink(ctx context.Context, _ Invocation, args json.RawMessage) (string, error) {
	in, err := decodeArgs[struct {
		Provider string `json:"provider"`
	}](args)
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(in.Provider)
	for _, e := range d.Automations.Catalog(ctx) {
		if string(e.Provider) == p {
			if !e.Configured {
				return "", errors.New(e.Name + " is not set up on this ColdEmail instance yet: an admin has to add its app credentials first")
			}
			return jsonResult(map[string]string{
				"name": e.Name,
				"url":  d.link("/app/integrations"),
			})
		}
	}
	return "", errors.New("unknown integration: use a provider id from list_integrations")
}
