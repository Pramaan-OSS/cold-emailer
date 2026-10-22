package models

// ProviderCapability is the static, code-defined descriptor the dashboard renders
// the onboarding + settings UI from, and that AutomationConfig.Validate checks
// against. Adding support for a new provider action is "add a descriptor entry +
// a handler", with no bespoke per-provider React. The descriptor only lists what
// ColdEmail can actually execute today, so the UI never offers a dead action.
type ProviderCapability struct {
	Provider   IntegrationProvider `json:"provider"`
	Directions []SyncDirection     `json:"directions"`
	Objects    []CapabilityObject  `json:"objects,omitempty"`
	Actions    []CapabilityAction  `json:"actions,omitempty"`
	Pickers    []CapabilityPicker  `json:"pickers,omitempty"`

	// SupportsBookingLink is true for scheduling providers (connected, connected):
	// the connection stores a scheduling_url that contextual "Book a call"
	// buttons open, prefilled with the contact's email/name.
	SupportsBookingLink bool `json:"supports_booking_link"`
}

// FieldDef is one selectable field in the field-map editor (either a ColdEmail
// source field or a provider destination field).
type FieldDef struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// CapabilityObject describes one provider object ColdEmail can write (contact,
// person, lead, deal). ColdEmailFields are the source fields a user can map FROM;
// ExternalFields are the provider destination fields they can map TO. When
// DynamicFields is true the destination list can be augmented by live discovery
// (Phase Later); today the static ExternalFields cover the common case.
type CapabilityObject struct {
	Name            string     `json:"name"`
	Label           string     `json:"label"`
	DedupeKeys      []string   `json:"dedupe_keys"`
	Required        []string   `json:"required"`
	ColdEmailFields []FieldDef `json:"coldemail_fields"`
	ExternalFields  []FieldDef `json:"external_fields"`
	DynamicFields   bool       `json:"dynamic_fields"`
}

// CapabilityAction is a descriptor-level action that maps 1:1 to an
// IntegrationAction handler.
type CapabilityAction struct {
	ID            IntegrationAction `json:"id"`
	Label         string            `json:"label"`
	Description   string            `json:"description"`
	Object        string            `json:"object,omitempty"`
	NeedsPipeline bool              `json:"needs_pipeline,omitempty"`
	NeedsChannel  bool              `json:"needs_channel,omitempty"`
	NeedsURL      bool              `json:"needs_url,omitempty"`
}

// CapabilityPicker is a value selector the UI renders (channel, pipeline, stage,
// owner). Endpoint is the discovery route key (empty = manual entry for now).
type CapabilityPicker struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Endpoint  string `json:"endpoint,omitempty"`
	DependsOn string `json:"depends_on,omitempty"`
}

// Action returns the descriptor for an action id, or nil.
func (p *ProviderCapability) Action(id IntegrationAction) *CapabilityAction {
	for i := range p.Actions {
		if p.Actions[i].ID == id {
			return &p.Actions[i]
		}
	}
	return nil
}

// Object returns the descriptor for an object name, or nil.
func (p *ProviderCapability) Object(name string) *CapabilityObject {
	for i := range p.Objects {
		if p.Objects[i].Name == name {
			return &p.Objects[i]
		}
	}
	return nil
}

// coldemailContactFields are the ColdEmail contact source fields offered in the
// field-map editor across every CRM provider.
func coldemailContactFields() []FieldDef {
	return []FieldDef{
		{Key: "email", Label: "Email"},
		{Key: "first_name", Label: "First name"},
		{Key: "last_name", Label: "Last name"},
		{Key: "name", Label: "Full name"},
		{Key: "company", Label: "Company"},
		{Key: "phone", Label: "Phone"},
	}
}

// ProviderSupportsAction reports whether action runs on a connection of provider.
func ProviderSupportsAction(provider IntegrationProvider, action IntegrationAction) bool {
	return action == IntegrationActionConnected || action == IntegrationActionGenericWebhookPing
}

// CapabilityFor returns the descriptor for a provider, or nil if the provider
// has no configurable capability surface.
func CapabilityFor(provider IntegrationProvider) *ProviderCapability {
	caps := Capabilities()
	if c, ok := caps[provider]; ok {
		return &c
	}
	return nil
}

// Capabilities contains no vendor action descriptors. Schemas come from keyword searches through apps.catalog.search.
func Capabilities() map[IntegrationProvider]ProviderCapability {
	return map[IntegrationProvider]ProviderCapability{}
}
