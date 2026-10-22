package config

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestDashboardEmailLinks(t *testing.T) {
	t.Setenv("APP_URL", "https://app.example.test")
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://app.example.test,https://tac-security-assessment.example.test")
	for _, tt := range []struct{ origin, want string }{
		{"https://tac-security-assessment.example.test", "https://tac-security-assessment.example.test"},
		{"https://app.example.test", "https://app.example.test"},
		{"", "https://app.example.test"},
		{"https://evil.example.com", "https://app.example.test"},
	} {
		ctx := WithDashboardOrigin(context.Background(), tt.origin)
		if got := DashboardBaseURL(ctx); got != tt.want {
			t.Errorf("dashboard base for %q = %q", tt.origin, got)
		}
		for _, got := range []string{GetPasswordResetURL("token&next=evil", DashboardOriginFromContext(ctx)), GetInviteURL("token&next=evil", DashboardOriginFromContext(ctx))} {
			if !strings.HasPrefix(got, tt.want+"/") || !strings.HasSuffix(got, "token%26next%3Devil") {
				t.Errorf("email URL for %q = %q", tt.origin, got)
			}
		}
	}
}

func TestPrimaryDashboardOriginAndRevalidation(t *testing.T) {
	t.Setenv("APP_URL", "https://app.example.test/")
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://tac-security-assessment.example.test")
	if got := PrimaryDashboardOrigin(); got != "https://app.example.test" {
		t.Fatalf("primary origin = %q", got)
	}
	ctx := WithDashboardOrigin(context.Background(), "https://tac-security-assessment.example.test")
	t.Setenv("CORS_ALLOW_ORIGINS", "*")
	if DashboardOriginFromContext(ctx) != "" || DashboardBaseURL(ctx) != "https://app.example.test" {
		t.Fatal("removed origin must not remain trusted")
	}
}

func TestSharedBackendDashboardOrigins(t *testing.T) {
	t.Setenv("APP_URL", "https://app.example.test")
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("WEBSOCKET_URL", "wss://rt.example.com/socket/websocket")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://app.example.test, https://admin.example.com,https://tac-security-assessment.example.test")
	cfg, err := (&Config{Env: "prod"}).LoadApiConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://app.example.test", "https://admin.example.com", "https://tac-security-assessment.example.test"}
	if !slices.Equal(cfg.AllowedOrigins, want) {
		t.Fatalf("API origins = %v, want %v", cfg.AllowedOrigins, want)
	}
	for _, origin := range want {
		if got := DashboardOrigin(origin); got != origin {
			t.Errorf("OAuth return origin %q = %q", origin, got)
		}
	}
	t.Setenv("CORS_ALLOW_ORIGINS", "https://app.example.test,https://admin.example.com")
	if got := DashboardOrigin("https://tac-security-assessment.example.test"); got != "" {
		t.Fatal("assessment origin must be explicitly configured, not hard-coded")
	}
}

func TestDashboardOrigin(t *testing.T) {
	t.Setenv("APP_URL", "https://app.example.com")
	t.Setenv("APP_ORIGIN", "https://primary.example.com")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://app.example.com, https://assessment.example.com,http://localhost:5173")
	for _, value := range []string{"https://app.example.com", "https://primary.example.com", "https://assessment.example.com", "http://localhost:5173"} {
		if got := DashboardOrigin(value); got != value {
			t.Errorf("DashboardOrigin(%q) = %q", value, got)
		}
	}
	for _, value := range []string{"", "null", "*", "https://evil.example.com", "https://assessment.example.com.evil.test", "http://assessment.example.com", "https://assessment.example.com:444", "https://assessment.example.com/", "https://assessment.example.com/redirect", "https://assessment.example.com?x=1", "https://assessment.example.com?", "https://assessment.example.com#fragment", "https://user@assessment.example.com", "javascript:alert(1)"} {
		if got := DashboardOrigin(value); got != "" {
			t.Errorf("DashboardOrigin(%q) = %q, want empty", value, got)
		}
	}
	t.Setenv("CORS_ALLOW_ORIGINS", "*")
	if got := DashboardOrigin("https://evil.example.com"); got != "" {
		t.Fatal("wildcard must not allow an OAuth return origin")
	}
}

func TestLocalAPIStartsWithoutLiveUpdateConfiguration(t *testing.T) {
	t.Setenv("WEBSOCKET_URL", "")
	t.Setenv("API_HOST", "")
	cfg, err := (&Config{Env: "dev"}).LoadApiConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Hostname != "0.0.0.0:8080" || cfg.WebsocketURI != "" {
		t.Fatalf("unexpected local endpoints: %+v", cfg)
	}
}
