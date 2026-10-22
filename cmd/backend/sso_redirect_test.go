package main

import "testing"

// The provider returns the browser to a route the API serves. Deriving it from
// APP_URL instead is the mistake that produces a correctly configured OAuth
// client and a sign-in button that lands on a page that does not exist.
func TestSSORedirectURLDerivesFromAPIPublicURL(t *testing.T) {
	t.Setenv("API_PUBLIC_URL", "https://api.example.com/")

	tests := map[string]string{
		"google": "https://api.example.com/v1/auth/google/callback",
		"apple":  "https://api.example.com/v1/auth/apple/callback",
		"oidc":   "https://api.example.com/v1/auth/oidc/callback",
	}
	for provider, want := range tests {
		if got := ssoRedirectURL("", provider); got != want {
			t.Errorf("ssoRedirectURL(\"\", %q) = %q, want %q", provider, got, want)
		}
	}
}

func TestSSORedirectURLPrefersTheExplicitOverride(t *testing.T) {
	t.Setenv("API_PUBLIC_URL", "https://api.example.com")

	if got := ssoRedirectURL("  https://proxy.example.com/callback  ", "google"); got != "https://proxy.example.com/callback" {
		t.Errorf("got %q, want the trimmed override", got)
	}
}

// Local URLs resolve without configuration; hosted sign-in still requires HTTPS.
func TestSSORedirectURLDefaultsToLocalAPI(t *testing.T) {
	t.Setenv("API_PUBLIC_URL", "")
	t.Setenv("OIDC_REDIRECT_URL", "")
	if got := ssoRedirectURL("", "enterprise"); got != "http://localhost:8080/v1/auth/enterprise/callback" {
		t.Fatalf("unexpected local callback: %q", got)
	}
	if got := oidcRedirectURL(); got != "http://localhost:8080/v1/auth/oidc/callback" {
		t.Fatalf("unexpected social callback: %q", got)
	}
}

func TestConnectorReturnURLDefaultsAndOverrides(t *testing.T) {
	t.Setenv("APP_URL", "")
	t.Setenv("PRAMAAN_CONNECTOR_RETURN_URL", "")
	if got := connectorReturnURL(); got != "http://localhost:5173/app/settings/connections" {
		t.Fatalf("unexpected local return: %q", got)
	}
	t.Setenv("APP_URL", "https://app.example.test/")
	if got := connectorReturnURL(); got != "https://app.example.test/app/settings/connections" {
		t.Fatalf("unexpected public return: %q", got)
	}
	t.Setenv("PRAMAAN_CONNECTOR_RETURN_URL", "https://app.example.test/return")
	if got := connectorReturnURL(); got != "https://app.example.test/return" {
		t.Fatalf("override ignored: %q", got)
	}
}

func TestOIDCRedirectURLStillReadsItsOwnOverride(t *testing.T) {
	t.Setenv("API_PUBLIC_URL", "https://api.example.com")
	t.Setenv("OIDC_REDIRECT_URL", "https://sso.example.com/finish")

	if got := oidcRedirectURL(); got != "https://sso.example.com/finish" {
		t.Errorf("got %q, want the OIDC_REDIRECT_URL override", got)
	}
}
