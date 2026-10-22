package pramaan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCapabilityRefusalWording(t *testing.T) {
	cases := []struct {
		name, id, code, effect, want string
		status                       int
	}{
		{"unavailable read", "domains.registration.search", "upstream_unavailable", "", "This feature is not available yet.", 502},
		{"invalid read", "domains.registration.search", "invalid_request", "", "The request was not accepted. Check the input fields.", 400},
		{"invalid no effect", "domains.registration.search", "invalid_request", "none", "The request was not accepted. Check the input fields.", 400},
		{"known no effect", "domains.registration.search", "internal", "none", "The request could not complete.", 500},
		{"purchase without effect metadata", "domains.registration.create", "internal", "", "The request could not complete. Its result may be unknown; check before trying again.", 500},
		{"unknown purchase", "domains.registration.create", "internal", "unknown", "The request could not complete. Its result may be unknown; check before trying again.", 500},
		{"unknown unavailable purchase", "domains.registration.create", "upstream_unavailable", "unknown", "This feature is not available yet. Its result may be unknown; check before retrying.", 502},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": tc.code, "effect": tc.effect, "message": "private upstream detail"}})
			}))
			defer s.Close()
			c, _ := New("fixture", s.URL, nil)
			e := c.Call(context.Background(), tc.id, map[string]any{"query": "synthetic"}, nil)
			if e == nil || e.Error() != tc.want || strings.Contains(e.Error(), "outcome") || calls != 1 {
				t.Fatalf("error=%v calls=%d; want %q", e, calls, tc.want)
			}
		})
	}
}
