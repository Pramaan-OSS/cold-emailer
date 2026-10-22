package storage

import (
	"strings"
	"testing"
)

func TestPublicObjectAddressStaysOnApp(t *testing.T) {
	c := &Client{PublicBaseURL: "/public"}
	if got := c.PublicURL("avatars/example.png"); !strings.HasPrefix(got, "/public/") {
		t.Fatal(got)
	}
}
