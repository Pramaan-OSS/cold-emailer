package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type actorKey struct{}

func WithActor(ctx context.Context, owner string) context.Context {
	return context.WithValue(ctx, actorKey{}, owner)
}

// AppAction discovers actual app actions and checks their published schemas before execution.
// Unscoped legacy calls and ambiguous/missing actions stay off, never falling back to a vendor.
func AppAction(ctx context.Context, app, query string, input any, out any, key string) error {
	owner, _ := ctx.Value(actorKey{}).(string)
	if owner == "" {
		return errors.New("Choose a connected action in Connections first.")
	}
	c, e := FromEnv()
	if e != nil {
		return e
	}
	a := Apps{Client: c}
	found, e := a.Search(ctx, query, app)
	if e != nil {
		return e
	}
	rows := resultRows(found)
	if len(rows) != 1 {
		return errors.New("This action needs coverage verification in Connections.")
	}
	selected := rows[0]
	action := stringField(selected, "action")
	if action == "" {
		return ErrUnavailable
	}
	definition := selected
	schema, ok := definition["inputSchema"].(map[string]any)
	if !ok {
		return ErrUnavailable
	}
	b, e := json.Marshal(input)
	if e != nil {
		return e
	}
	in := map[string]any{}
	if input != nil {
		if e = json.Unmarshal(b, &in); e != nil {
			return errors.New("Complete the action fields in Connections.")
		}
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		for k := range in {
			if _, allowed := props[k]; !allowed {
				delete(in, k)
			}
		}
	}
	if required, ok := schema["required"].([]any); ok {
		for _, r := range required {
			name, _ := r.(string)
			if v, ok := in[name]; !ok || v == nil || v == "" {
				return errors.New("Complete this action's required fields in Connections.")
			}
		}
	}
	if key == "" {
		key = uuid.NewString()
	}
	result, e := a.Execute(ctx, "org_"+strings.TrimPrefix(owner, "org_"), action, in, key)
	if e != nil {
		return e
	}
	if out == nil {
		return nil
	}
	value := any(result)
	if v, ok := result["output"]; ok {
		value = v
	}
	b, e = json.Marshal(value)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, out)
}
