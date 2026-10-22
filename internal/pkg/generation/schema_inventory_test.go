package generation

import (
	"testing"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

func TestEveryGeneratedResponseSchema(t *testing.T) {
	catalog, err := newToolCatalog(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = pramaan.ValidateSchema("workspace action discovery", catalog.discovery.InputSchema); err != nil {
		t.Fatal(err)
	}

	if e := pramaan.ValidateSchema("warmup conversation (single and batch)", ConversationSchema); e != nil {
		t.Fatal(e)
	}
}
