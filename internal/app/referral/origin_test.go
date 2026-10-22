package referral

import (
	"context"
	"testing"

	"github.com/pramaan-oss/cold-email/internal/config"
)

func TestReferralShareLinkUsesTrustedDashboard(t *testing.T) {
	t.Setenv("APP_URL", "https://example.test")
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://example.test")
	s := &service{shareBase: "https://example.test"}
	for _, tt := range []struct{ origin, want string }{
		{"https://example.test", "https://example.test"},
		{"", "https://example.test"},
		{"https://evil.example.com", "https://example.test"},
	} {
		ctx := config.WithDashboardOrigin(context.Background(), tt.origin)
		if got := s.shareURL(ctx, "CODE1234"); got != tt.want+"/auth/register?ref=CODE1234" {
			t.Fatalf("share link = %q", got)
		}
	}
}
