package pramaan

import (
	"context"
	"errors"
	"strings"
)

type Apps struct{ Client Caller }

func (a Apps) Search(ctx context.Context, query, app string, cursor ...string) (map[string]any, error) {
	var out map[string]any
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("Enter an action keyword before searching the catalogue.")
	}
	in := map[string]any{"query": strings.TrimSpace(query), "limit": 50}
	if app != "" {
		in["app"] = app
	}
	if len(cursor) > 0 && cursor[0] != "" {
		in["cursor"] = cursor[0]
	}
	e := a.Client.Call(ctx, "apps.catalog.search", in, &out)
	return out, e
}
func (a Apps) Connect(ctx context.Context, user, app, redirect string, key string) (map[string]any, error) {
	var out map[string]any
	e := a.Client.Call(ctx, "apps.connection.start", map[string]any{"endUserRef": user, "app": app, "redirectUrl": redirect}, &out, Idempotency(key))
	return out, e
}

// Inspect searches the bounded catalogue for the selected key; no automation-account APIs.
func (a Apps) Inspect(ctx context.Context, key string) (map[string]any, error) {
	out, e := a.Search(ctx, key, "")
	if e != nil {
		return nil, e
	}
	for _, row := range resultRows(out) {
		if stringField(row, "action") == key {
			if _, ok := row["inputSchema"].(map[string]any); !ok {
				return nil, errors.New("This catalogue result has no input schema. Refine the search in Connections.")
			}
			return row, nil
		}
	}
	return nil, errors.New("The selected action was not found in these catalogue results. Open Connections and load the next catalogue page if needed.")
}

// Execute requires the exact action handle selected from discovery. Never invent per-app IDs.
func (a Apps) Execute(ctx context.Context, user, action string, input map[string]any, key string) (map[string]any, error) {
	var out map[string]any
	if user == "" || action == "" || key == "" {
		return nil, errors.New("Select a connected action first.")
	}
	e := a.Client.Call(ctx, "apps.action.execute", map[string]any{"endUserRef": user, "action": action, "input": input}, &out, Idempotency(key))
	if e == nil && stringField(out, "status") == "failed" {
		return nil, errors.New("The connected action failed. Check the action fields and try again.")
	}
	return out, e
}
func (a Apps) Triggers(ctx context.Context, app, cursor string) (map[string]any, error) {
	var out map[string]any
	in := map[string]any{"app": app, "limit": 50}
	if cursor != "" {
		in["cursor"] = cursor
	}
	e := a.Client.Call(ctx, "apps.triggers.list", in, &out)
	return out, e
}
func (a Apps) Trigger(ctx context.Context, key string) (map[string]any, error) {
	var out map[string]any
	e := a.Client.Call(ctx, "apps.trigger.get", map[string]any{"key": key}, &out)
	return out, e
}
func (a Apps) Deployments(ctx context.Context, user string) (map[string]any, error) {
	var out map[string]any
	e := a.Client.Call(ctx, "apps.trigger-deployments.list", map[string]any{"endUserRef": user, "limit": 50}, &out)
	return out, e
}
func (a Apps) Events(ctx context.Context, user, deployment string) (map[string]any, error) {
	var out map[string]any
	e := a.Client.Call(ctx, "apps.trigger-deployment-events.list", map[string]any{"endUserRef": user, "deploymentId": deployment, "includePayload": true, "limit": 100}, &out)
	return out, e
}
