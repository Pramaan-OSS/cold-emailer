package pramaan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEnterpriseTransportCredentialsAndStateContract(t *testing.T) {
	org := uuid.New()
	seen := []string{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		seen = append(seen, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer synthetic" {
			t.Error("key not held by client")
		}
		switch r.URL.Path {
		case "/v1/capabilities/auth.enterprise-connection.create":
			if in["organizationRef"] != EnterpriseOrganization(org) || in["clientSecret"] != "synthetic-secret" || len(in["redirectUris"].([]any)) != 1 || r.Header.Get("Idempotency-Key") == "" {
				t.Error(in)
			}
			json.NewEncoder(w).Encode(EnterpriseConnection{Ref: "ref-1", OrganizationRef: EnterpriseOrganization(org), Status: "ready"})
		case "/v1/capabilities/auth.enterprise.start":
			if in["state"] == nil || in["nonce"] == nil || len(in["codeChallenge"].(string)) != 43 {
				t.Error(in)
			}
			json.NewEncoder(w).Encode(EnterpriseLink{URL: "https://api.pramaan.dev/connect/link/test", ExpiresAt: time.Now().Add(time.Minute).Format(time.RFC3339)})
		case "/v1/capabilities/auth.social.exchange":
			if in["state"] == nil || in["pkceVerifier"] == nil || in["expectedNonce"] == nil || in["connectionRef"] != "ref-1" || in["providerKey"] != "enterprise" {
				t.Error(in)
			}
			json.NewEncoder(w).Encode(EnterpriseIdentity{ProviderKey: "enterprise", Subject: "subject-1"})
		}
	}))
	defer s.Close()
	c, _ := New("synthetic", s.URL, nil)
	e := Enterprise{Client: c, RedirectURI: "https://app.example.test/callback"}
	ctx := context.Background()
	_, err := e.Create(ctx, org, "admin", uuid.NewString(), EnterpriseCreate{Name: "Team", Issuer: "https://issuer.example.test", ClientID: "client", ClientSecret: "synthetic-secret", TokenAuthMethod: "client_secret_basic", Approved: true})
	if err != nil {
		t.Fatal(err)
	}
	v := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if _, err = e.Start(ctx, "ref-1", v, v, v); err != nil {
		t.Fatal(err)
	}
	if _, err = e.Redeem(ctx, "ref-1", "code", v, v, v); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 3 {
		t.Fatal(seen)
	}
}
func TestEnterpriseRequiresSupportedClientAndStateBeforeSending(t *testing.T) {
	c, _ := New("synthetic", "http://localhost:1", nil)
	e := Enterprise{Client: c, RedirectURI: "https://app.example.test/callback"}
	ctx := context.Background()
	if _, err := e.Redeem(ctx, "ref", "code", "", "nonce", "verifier"); err == nil {
		t.Fatal("missing state")
	}
	for _, method := range []string{"saml", "none", "client_secret_basic"} {
		_, err := e.Create(ctx, uuid.New(), "member", uuid.NewString(), EnterpriseCreate{Name: "Team", Issuer: "https://issuer.example.test", ClientID: "client", TokenAuthMethod: method, Approved: true})
		if err != ErrEnterpriseApproval {
			t.Fatal(err)
		}
	}
	e.RedirectURI = "http://localhost/callback"
	if e.Ready() {
		t.Fatal("insecure callback")
	}
}

func TestEnterpriseCapabilitiesCannotBypassTheirOwnedSignInRoutes(t *testing.T) {
	caller := &responseCaller{}
	gateway := Gateway{Client: caller, Registry: memoryRegistry{}}
	for _, id := range []string{"auth.enterprise-connection.create", "auth.enterprise-connection.delete", "auth.enterprise-connections.list", "auth.enterprise.start", "auth.social.exchange"} {
		if _, err := gateway.Call(context.Background(), "workspace", id, map[string]any{"organizationRef": "other", "connectionRef": "unowned"}, "attempt"); err != ErrUnavailable {
			t.Fatal(id, err)
		}
	}
	if caller.calls != 0 {
		t.Fatal("generic gateway bypassed sign-in ownership")
	}
}
