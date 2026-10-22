package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrDomainUnavailable = errors.New("Domain purchase is not available yet; use a domain you own")
var ErrDomainUnknown = errors.New("The purchase result is unknown. Check its status; do not buy this domain again.")

func DomainCapability(id string) bool {
	switch id {
	case "domains.registration.search", "domains.registration.quote", "domains.registration.create", "domains.registration.get", "domains.registration.list", "domains.dns-records.list", "domains.dns-records.apply":
		return true
	}
	return false
}
func DomainRoleAllowed(role string) bool { return role == "owner" || role == "admin" }
func domainQuoteReference(token string, in map[string]any) string {
	return planReference(token, OutcomeRequest{Input: map[string]any{"domain": in["domain"], "years": in["years"], "price": in["price"]}})
}

// DomainCall is separate from generic discovery: purchases require a real member,
// a quote bound to that member and an atomic durable claim before the one send.
func (g *Gateway) DomainCall(ctx context.Context, owner, actor, role, id string, in map[string]any, key string) (map[string]any, error) {
	if !DomainRoleAllowed(role) || owner == "" || actor == "" {
		return nil, &HTTPError{Status: 403, Code: "forbidden", Effect: "none"}
	}
	if g == nil || g.Client == nil || g.Registry == nil {
		return nil, ErrNotConfigured
	}
	if !DomainCapability(id) {
		return nil, ErrUnavailable
	}
	b, _ := json.Marshal(in)
	var request map[string]any
	_ = json.Unmarshal(b, &request)
	if request == nil {
		request = map[string]any{}
	}
	domain := strings.ToLower(strings.TrimSpace(stringField(request, "domain")))
	if domain != "" {
		request["domain"] = domain
	}
	if id == "domains.registration.quote" || id == "domains.registration.create" {
		if domain == "" {
			return nil, &HTTPError{Status: 400, Code: "invalid_request", Effect: "none"}
		}
		if request["years"] == nil {
			request["years"] = float64(1)
		}
	}
	if id == "domains.registration.create" {
		token := stringField(request, "confirmation")
		delete(request, "confirmation")
		if token == "" || !g.owns(ctx, owner, "domain_quote_"+actor, domainQuoteReference(token, request)) {
			return nil, &HTTPError{Status: 403, Code: "forbidden", Effect: "none"}
		}
		timestamp, err := uuid.Parse(token)
		// Time-bearing quote tokens expire before an old quote can be approved.
		if err != nil || timestamp.Version() != 1 {
			return nil, &HTTPError{Status: 409, Code: "conflict", Effect: "none"}
		}
		seconds, nanos := timestamp.Time().UnixTime()
		if time.Since(time.Unix(seconds, nanos)) > 15*time.Minute {
			return nil, &HTTPError{Status: 409, Code: "conflict", Effect: "none"}
		}
		if _, err = uuid.Parse(key); err != nil {
			return nil, &HTTPError{Status: 400, Code: "invalid_request", Effect: "none"}
		}
		claims, ok := g.Registry.(interface {
			Claim(context.Context, string, string, string) (bool, error)
		})
		if !ok {
			return nil, ErrNotConfigured
		}
		used, err := claims.Claim(ctx, owner, "domain_confirmation", token)
		if err != nil {
			return nil, err
		}
		if !used {
			return nil, ErrDomainUnknown
		}
		claimed, err := claims.Claim(ctx, owner, "registration_attempt", domain)
		if err != nil {
			return nil, err
		}
		if !claimed {
			return nil, ErrDomainUnknown
		}
	}
	if id == "domains.registration.get" || strings.HasPrefix(id, "domains.dns-records.") {
		if !g.owns(ctx, owner, "registration", stringField(request, "ref")) {
			return nil, &HTTPError{Status: 403, Code: "forbidden", Effect: "none"}
		}
	}
	var out map[string]any
	if err := g.Client.Call(ctx, id, request, &out, Idempotency(key)); err != nil {
		if errors.Is(err, ErrDomainUnavailable) || errors.Is(err, ErrUnavailable) {
			if id == "domains.registration.create" {
				// Only an explicit pre-effect refusal releases a domain for a
				// fresh quote and a new human confirmation. This quote stays used.
				if e := g.Registry.Remove(ctx, owner, "registration_attempt", domain); e != nil {
					return nil, ErrDomainUnknown
				}
			}
			return nil, ErrDomainUnavailable
		}
		if id == "domains.registration.create" {
			var he *HTTPError
			if errors.As(err, &he) && he.Effect == "none" {
				if e := g.Registry.Remove(ctx, owner, "registration_attempt", domain); e != nil {
					return nil, ErrDomainUnknown
				}
				return nil, he
			}
			return nil, ErrDomainUnknown
		}
		return nil, err
	}
	if id == "domains.registration.quote" {
		if out["domain"] != domain || out["action"] != "register" || out["price"] == nil {
			return nil, ErrDomainUnavailable
		}
		quote := map[string]any{"domain": out["domain"], "years": out["years"], "price": out["price"]}
		token, err := uuid.NewUUID()
		if err != nil {
			return nil, err
		}
		if err = g.Registry.Save(ctx, owner, "domain_quote_"+actor, domainQuoteReference(token.String(), quote)); err != nil {
			return nil, err
		}
		out["confirmation"] = token.String()
	}
	if id == "domains.registration.create" {
		ref := stringField(out, "ref")
		if ref == "" || stringField(out, "domain") != domain {
			return nil, ErrDomainUnknown
		}
		if err := g.Registry.Save(ctx, owner, "registration", ref); err != nil {
			return nil, ErrDomainUnknown
		}
	}
	if id == "domains.registration.list" {
		// The platform project may serve several workspaces: only attempted domains
		// belonging to this workspace can become recoverable refs here.
		rows := []map[string]any{}
		for _, row := range resultRows(out) {
			if g.owns(ctx, owner, "registration_attempt", stringField(row, "domain")) {
				if err := g.Registry.Save(ctx, owner, "registration", stringField(row, "ref")); err != nil {
					return nil, err
				}
				rows = append(rows, row)
			}
		}
		out["results"] = rows
	}
	return out, nil
}
