package handler

import (
	"context"
	"testing"

	"github.com/pramaan-oss/cold-email/internal/config"
)

func TestBillingReturnURLUsesInitiatingDashboard(t *testing.T) {
	t.Setenv("APP_URL", "https://example.test")
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://example.test")
	const assessment = "https://example.test"
	const fallback = "/app/settings/billing"
	for _, tt := range []struct{ origin, raw, want string }{
		{assessment, assessment + "/app/settings/billing/ai-credits?topup=success", assessment + "/app/settings/billing/ai-credits?topup=success"},
		{assessment, "/app/settings/billing?checkout=done", assessment + "/app/settings/billing?checkout=done"},
		{assessment, "", assessment + fallback},
		{assessment, "https://example.test" + fallback, assessment + fallback},
		{"", "https://example.test" + fallback, "https://example.test" + fallback},
		{"", assessment + fallback, "https://example.test" + fallback},
		{"https://evil.example.com", "/app/settings/billing", "https://example.test" + fallback},
	} {
		ctx := config.WithDashboardOrigin(context.Background(), tt.origin)
		if got := billingReturnURL(ctx, tt.raw, fallback); got != tt.want {
			t.Errorf("origin %q URL %q = %q, want %q", tt.origin, tt.raw, got, tt.want)
		}
	}
	ctx := config.WithDashboardOrigin(context.Background(), assessment)
	for _, raw := range []string{
		"https://evil.example.com/", "//evil.example.com", "/\\evil.example.com", "https://example.test.evil.example.com/",
		"https://evil@example.test/", "http://example.test/", "https://example.test:444/", "javascript:alert(1)", "/path\nother", "/%zz",
	} {
		if got := billingReturnURL(ctx, raw, fallback); got != assessment+fallback {
			t.Errorf("unsafe URL %q returned %q", raw, got)
		}
	}
}
