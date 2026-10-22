package generation

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

const discoveryName = "find_workspace_actions"

// A large workspace keeps every permitted action locally. Each conversation
// turn receives a bounded selection and a local discovery action, rather than
// discarding capabilities or sending an oversized declaration to the platform.
type toolCatalog struct {
	all       []ToolDef
	byName    map[string]ToolDef
	query     string
	bounded   bool
	discovery ToolDef
}

func newToolCatalog(defs []ToolDef, messages []AgentMessage) (*toolCatalog, error) {
	c := &toolCatalog{byName: map[string]ToolDef{}}
	size := 0
	for _, t := range defs {
		if t.Name == discoveryName {
			return nil, errors.New("A workspace action uses the reserved discovery name.")
		}
		if _, ok := c.byName[t.Name]; ok {
			return nil, errors.New("Workspace action names must be unique.")
		}
		if t.InputSchema == nil {
			t.InputSchema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		schema, e := pramaan.NormalizeSchema("tool "+t.Name, t.InputSchema)
		if e != nil {
			return nil, e
		}
		t.InputSchema = schema
		b, _ := json.Marshal(schema)
		size += len(b) + 1
		c.all = append(c.all, t)
		c.byName[t.Name] = t
	}
	c.bounded = len(defs) > 64 || size > 60000
	for _, m := range messages {
		if m.Role == "user" {
			c.query = m.Content
		}
		for _, call := range m.ToolCalls {
			if call.Name == discoveryName {
				c.bounded = true
			}
		}
	}
	c.discovery = ToolDef{Name: discoveryName, Description: "Find available workspace actions by task or exact action name. Their input definitions become available on the next turn. This only searches the local permitted catalogue; it does not change records.", Risk: RiskRead, InputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "minLength": 1, "maxLength": 200}}, "required": []string{"query"}, "additionalProperties": false}}
	c.byName[discoveryName] = c.discovery
	return c, nil
}

func (c *toolCatalog) selection(messages []AgentMessage) ([]ToolDef, error) {
	if !c.bounded {
		return c.all, nil
	}
	out := []ToolDef{c.discovery}
	seen := map[string]bool{discoveryName: true}
	size := 0
	add := func(t ToolDef) bool {
		if seen[t.Name] {
			return true
		}
		b, _ := json.Marshal(t.InputSchema)
		if len(out) >= 64 || size+len(b) > 60000 {
			return false
		}
		size += len(b)
		seen[t.Name] = true
		out = append(out, t)
		return true
	}
	// Historical calls must still have their exact definitions in every request,
	// including after approval, resume, or another discovery query.
	for _, m := range messages {
		for _, call := range m.ToolCalls {
			t, ok := c.byName[call.Name]
			if !ok || !add(t) {
				return nil, errors.New("The conversation's action history exceeds the available definitions. Start a new conversation before continuing.")
			}
		}
	}
	words := strings.FieldsFunc(strings.ToLower(c.query), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	stop := map[string]bool{"the": true, "and": true, "for": true, "from": true, "your": true, "with": true, "this": true, "that": true, "not": true, "use": true, "any": true, "about": true}
	ranked := append([]ToolDef{}, c.all...)
	score := func(t ToolDef) int {
		if strings.EqualFold(strings.TrimSpace(c.query), t.Name) {
			return 10000
		}
		name, description := strings.ToLower(t.Name), strings.ToLower(t.Description)
		n := 0
		for _, w := range words {
			if len(w) < 3 || stop[w] {
				continue
			}
			if strings.Contains(name, w) {
				n += 10
			} else if strings.Contains(description, w) {
				n++
			}
		}
		return n
	}
	sort.SliceStable(ranked, func(i, j int) bool { return score(ranked[i]) > score(ranked[j]) })
	added := 0
	for _, t := range ranked {
		if seen[t.Name] {
			continue
		}
		if add(t) {
			added++
		}
		if added >= 24 {
			break
		}
	}
	return out, nil
}

func (c *toolCatalog) bindDiscovery(messages func() []AgentMessage) {
	c.discovery.Handler = func(_ context.Context, raw json.RawMessage) (string, error) {
		var in struct {
			Query string `json:"query"`
		}
		if json.Unmarshal(raw, &in) != nil || len(strings.TrimSpace(in.Query)) == 0 || len(in.Query) > 200 {
			return "", errors.New("Describe the workspace action to find.")
		}
		c.query = in.Query
		selected, e := c.selection(messages())
		if e != nil {
			return "", e
		}
		rows := []map[string]string{}
		for _, t := range selected {
			if t.Name != discoveryName {
				rows = append(rows, map[string]string{"name": t.Name, "description": t.Description})
			}
		}
		b, _ := json.Marshal(map[string]any{"actions": rows, "note": "These actions and their exact input definitions are available on the next turn. Changing or sending still requires approval."})
		return string(b), nil
	}
	c.byName[discoveryName] = c.discovery
}
