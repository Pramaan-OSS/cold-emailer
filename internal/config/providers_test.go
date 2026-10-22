package config

import "testing"

// Server verification is explicit; the unavailable widget cannot enable it.
func TestCaptchaDefaultsOff(t *testing.T) {
	for value, want := range map[string]string{"": "none", "none": "none", "pramaan": "pramaan", "unknown": "none"} {
		t.Setenv("CAPTCHA_PROVIDER", value)
		if got := CaptchaProvider(); got != want {
			t.Fatalf("%q: got %q, want %q", value, got, want)
		}
	}
}
