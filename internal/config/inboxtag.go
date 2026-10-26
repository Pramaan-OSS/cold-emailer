package config

import (
	"os"
	"strconv"
	"strings"
)

// Calibrated judgments share the platform key; no separate vendor key is accepted.
func TextCheckAvailability() string { return strings.TrimSpace(os.Getenv("PRAMAAN_API_KEY")) }

// InboxTaggingEnabled reports whether the feature should run. Both halves are
// required, and the key check is here rather than at the call site so there is
// one answer to "is this on" for the API, the consumer and the dashboard.
func InboxTaggingEnabled() bool {
	if TextCheckAvailability() == "" {
		return false
	}
	v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("INBOX_TAGGING_ENABLED")))
	return err == nil && v
}
