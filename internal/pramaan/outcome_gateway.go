package pramaan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Only a recipe that fits an existing feature is exposed to app sessions.
func AllowedOutcome(id string) bool { return id == "lead.find" || id == "mailbox.connect" }

// Mailbox identity and the return page come from the workspace, never browser routing fields.
func (g *Gateway) mailboxOutcomeInput(ctx context.Context, owner string, in OutcomeRequest) (OutcomeRequest, error) {
	if len(in.Options) != 0 || len(in.Basis) != 0 || len(in.Input) > 1 {
		return in, &HTTPError{Status: 400, Code: "invalid_request", Effect: "none"}
	}
	check := false
	if len(in.Input) != 0 {
		check = in.Input["check"] == true
		if !check {
			return in, &HTTPError{Status: 400, Code: "invalid_request", Effect: "none"}
		}
	}
	input := map[string]any{"endUserRef": "org_" + owner}
	if check {
		refs, err := g.Registry.List(ctx, owner, "mail_connection")
		if err != nil {
			return in, err
		}
		if len(refs) == 0 {
			return in, &HTTPError{Status: 409, Code: "conflict", Effect: "none"}
		}
		input["ref"] = refs[len(refs)-1]
	} else {
		input["returnUrl"] = g.ReturnURL
	}
	return OutcomeRequest{Input: input}, nil
}

func (g *Gateway) saveMailboxOutcome(ctx context.Context, owner string, report map[string]any) error {
	result, ok := report["result"].(map[string]any)
	if !ok {
		return nil
	}
	if ref := stringField(result, "ref"); ref != "" {
		if err := g.Registry.Save(ctx, owner, "mail_connection", ref); err != nil {
			return err
		}
	}
	if stringField(result, "status") == "connected" {
		for kind, field := range map[string]string{"account": "accountRef", "inbox": "inbox"} {
			if err := g.Registry.Save(ctx, owner, kind, stringField(result, field)); err != nil {
				return err
			}
		}
	}
	return nil
}
func (g *Gateway) outcomeClient() (OutcomeCaller, error) {
	if g == nil || g.Client == nil || g.Registry == nil {
		return nil, ErrNotConfigured
	}
	c, ok := g.Client.(OutcomeCaller)
	if !ok {
		return nil, ErrOutcomeUnavailable
	}
	return c, nil
}
func (g *Gateway) OutcomesList(ctx context.Context) (map[string]any, error) {
	c, e := g.outcomeClient()
	if e != nil {
		return nil, e
	}
	out, e := c.OutcomesList(ctx)
	if e != nil {
		return nil, e
	}
	rows := []map[string]any{}
	for _, r := range resultRows(out) {
		if AllowedOutcome(stringField(r, "id")) {
			rows = append(rows, r)
		}
	}
	return map[string]any{"results": rows}, nil
}
func (g *Gateway) OutcomeGet(ctx context.Context, id string) (map[string]any, error) {
	if !AllowedOutcome(id) {
		return nil, ErrUnavailable
	}
	c, e := g.outcomeClient()
	if e != nil {
		return nil, e
	}
	return c.OutcomeGet(ctx, id)
}
func planReference(plan string, in OutcomeRequest) string {
	b, _ := json.Marshal(in)
	sum := sha256.Sum256(b)
	return plan + ":" + hex.EncodeToString(sum[:])
}
func (g *Gateway) OutcomeEstimate(ctx context.Context, owner, actor, id string, in OutcomeRequest) (map[string]any, error) {
	if owner == "" || actor == "" {
		return nil, &HTTPError{Status: 403, Code: "forbidden", Effect: "none"}
	}
	detail, e := g.OutcomeGet(ctx, id)
	if e != nil {
		return nil, e
	}
	if detail["available"] != true {
		return nil, ErrOutcomeUnavailable
	}
	c, _ := g.outcomeClient()
	if id == "mailbox.connect" {
		in, e = g.mailboxOutcomeInput(ctx, owner, in)
		if e != nil {
			return nil, e
		}
	}
	out, e := c.OutcomeEstimate(ctx, id, in)
	if e != nil {
		return nil, e
	}
	if plan := stringField(out, "plan"); plan != "" {
		if e = g.Registry.Save(ctx, owner, "outcome_plan_"+actor, planReference(plan, in)); e != nil {
			return nil, e
		}
	}
	return out, nil
}
func (g *Gateway) OutcomeStart(ctx context.Context, owner, actor, id string, in OutcomeRequest, key, plan string) (int, map[string]any, error) {
	if !AllowedOutcome(id) {
		return 404, nil, ErrUnavailable
	}
	if _, e := g.outcomeClient(); e != nil {
		return 0, nil, e
	}
	if owner == "" || actor == "" {
		return 403, nil, &HTTPError{Status: 403, Code: "forbidden", Effect: "none"}
	}
	if plan == "" {
		return 403, nil, &HTTPError{Status: 403, Code: "forbidden", Effect: "none"}
	}
	if id == "mailbox.connect" {
		var err error
		in, err = g.mailboxOutcomeInput(ctx, owner, in)
		if err != nil {
			return 400, nil, err
		}
	}
	if !g.owns(ctx, owner, "outcome_plan_"+actor, planReference(plan, in)) {
		return 409, nil, &HTTPError{Status: 409, Code: "conflict", Effect: "none"}
	}
	detail, e := g.OutcomeGet(ctx, id)
	if e != nil {
		return 0, nil, e
	}
	if detail["available"] != true {
		return 502, nil, ErrOutcomeUnavailable
	}
	c, _ := g.outcomeClient()
	status, out, e := c.OutcomeStart(ctx, id, in, key, plan)
	if e != nil {
		return status, nil, e
	}
	if status == 202 {
		ref := stringField(out, "job_id")
		if ref == "" {
			return 502, nil, &HTTPError{Status: 502, Code: "internal", Effect: "unknown"}
		}
		if e = g.Registry.Save(ctx, owner, "outcome_job_"+actor, ref); e != nil {
			return 502, nil, &HTTPError{Status: 502, Code: "internal", Effect: "unknown"}
		}
		if id == "mailbox.connect" {
			if e = g.Registry.Save(ctx, owner, "mailbox_job_"+actor, ref); e != nil {
				return 502, nil, e
			}
		}
	} else if id == "mailbox.connect" {
		if e = g.saveMailboxOutcome(ctx, owner, out); e != nil {
			return 502, nil, e
		}
	}
	return status, out, nil
}
func (g *Gateway) OutcomeJob(ctx context.Context, owner, actor, id string) (map[string]any, error) {
	if _, e := g.outcomeClient(); e != nil {
		return nil, e
	}
	if !g.owns(ctx, owner, "outcome_job_"+actor, id) {
		return nil, &HTTPError{Status: 403, Code: "forbidden", Effect: "none"}
	}
	c, e := g.outcomeClient()
	if e != nil {
		return nil, e
	}
	out, e := c.OutcomeJob(ctx, id)
	if e == nil && g.owns(ctx, owner, "mailbox_job_"+actor, id) {
		if report, ok := out["result"].(map[string]any); ok {
			e = g.saveMailboxOutcome(ctx, owner, report)
		}
	}
	return out, e
}

func (g *Gateway) OutcomeCancel(ctx context.Context, owner, actor, id string) (map[string]any, error) {
	c, e := g.outcomeClient()
	if e != nil {
		return nil, e
	}
	if !g.owns(ctx, owner, "outcome_job_"+actor, id) {
		return nil, &HTTPError{Status: 403, Code: "forbidden", Effect: "none"}
	}
	return c.OutcomeCancel(ctx, id)
}
