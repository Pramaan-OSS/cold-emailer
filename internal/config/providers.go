package config

import (
	"os"
	"strings"
)

// Owned infrastructure switches and brokered service settings.

// KMSProvider returns the selected KMS provider (local or the owned backend). Default local.
func KMSProvider() string { return providerOr("KMS_PROVIDER", "local") }

// BlobProvider returns the selected blob provider (local filesystem or brokered storage). Default filesystem.
func BlobProvider() string { return providerOr("BLOB_PROVIDER", "filesystem") }

// EventBusProvider returns the selected event bus ("kafka" | "nats").
// Default kafka.
func EventBusProvider() string { return providerOr("EVENTBUS_PROVIDER", "nats") }

// CodecProvider returns the selected codec ("avro" | "json"). Default avro.
func CodecProvider() string { return providerOr("CODEC_PROVIDER", "json") }

// TasksProvider returns the selected task scheduler (local).
func TasksProvider() string { return providerOr("TASKS_PROVIDER", "local") }

// BillingProvider returns the selected billing backend ("none" | "billing").
// Default none — self-host boots without Billing and unlocks all features.
func BillingProvider() string { return providerOr("BILLING_PROVIDER", "none") }

const (
	MailTransportPramaan = "pramaan"
	MailTransportLog     = "log"
)

// MailTransport labels the supported delivery mode. No protocol relay is used.
func MailTransport() string {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("MAIL_TRANSPORT")), "pramaan") {
		return MailTransportPramaan
	}
	return MailTransportLog
}

// SelfHosted reports whether this deployment is a self-host install.
// DEPLOYMENT_MODE is the explicit switch; BILLING_PROVIDER=none is the historic
// signal and stays the fallback so existing installs keep their behavior.
func SelfHosted() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DEPLOYMENT_MODE"))) {
	case "self_hosted", "selfhosted", "self-hosted":
		return true
	case "cloud", "hosted":
		return false
	}
	return BillingProvider() == "none"
}

// CaptchaProvider defaults off; pramaan enables server token verification only.
func CaptchaProvider() string {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("CAPTCHA_PROVIDER")), "pramaan") {
		return "pramaan"
	}
	return "none"
}

func providerOr(env, def string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return def
}
