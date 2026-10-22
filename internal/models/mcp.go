package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MCPServer is an org-connected external MCP server (client direction). Its
// discovered tools are exposed to the AI assistant only while Enabled.
type MCPServer struct {
	ID              uuid.UUID  `json:"id"`
	OrgID           uuid.UUID  `json:"org_id"`
	Name            string     `json:"name"`
	URL             string     `json:"-"`
	AuthType        string     `json:"auth_type"`
	Enabled         bool       `json:"enabled"`
	DiscoveredTools []MCPTool  `json:"discovered_tools"`
	LastError       string     `json:"last_error,omitempty"`
	CreatedBy       *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	// CredentialsEncrypted is never serialized to clients.
	CredentialsEncrypted string `json:"-"`
	ConnectorRef         string `json:"connector_ref"`
	State                string `json:"state"`
	ReadOnly             bool   `json:"read_only"`
	ConnectURL           string `json:"connect_url,omitempty"`
}

// MCPTool is one tool discovered from a connected MCP server.
type MCPTool struct {
	AssistantError   string          `json:"assistant_error,omitempty"`
	ToolRef          string          `json:"tool_ref"`
	SchemaDigest     string          `json:"schema_digest"`
	ApprovalRequired bool            `json:"approval_required"`
	Description      string          `json:"description"`
	InputSchema      json.RawMessage `json:"input_schema,omitempty"`
}

// CreateMCPServer is the connect payload.
type CreateMCPServer struct {
	Name  string `json:"name" binding:"required"`
	URL   string `json:"url" binding:"required"`
	Auth  string `json:"auth_type"` // none | bearer | oauth
	Token string `json:"token"`     // sent once to Pramaan; never persisted locally
}

// UpdateMCPServer patches a server; nil fields unchanged.
type UpdateMCPServer struct {
	Name                 *string  `json:"name"`
	Enabled              *bool    `json:"enabled"`
	Token                *string  `json:"token"`
	ReadOnly             *bool    `json:"read_only"`
	AllowWithoutApproval []string `json:"allow_without_approval"`
	Approved             bool     `json:"approved"`
}
