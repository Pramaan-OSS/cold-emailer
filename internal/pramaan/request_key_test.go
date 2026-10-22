package pramaan

import (
	"github.com/google/uuid"
	"testing"
)

func TestRequestKeysPreserveExistingAttempts(t *testing.T) {
	for _, input := range []string{"attempt-one", "a campaign write"} {
		got, err := requestKey(input)
		if err != nil || got != uuid.NewSHA1(uuid.NameSpaceURL, []byte(input)).String() {
			t.Fatalf("request reference changed: %q %v", got, err)
		}
	}
	fixed := "fb2d00af-1f6e-48cc-9a38-4df705002f69"
	if got, err := requestKey(fixed); err != nil || got != fixed {
		t.Fatal("existing attempt replaced")
	}
	first, err := requestKey("")
	if err != nil {
		t.Fatal(err)
	}
	second, err := requestKey("")
	if err != nil || first == second {
		t.Fatal("new attempts need distinct references")
	}
	if _, err := uuid.Parse(first); err != nil {
		t.Fatal(err)
	}
}
