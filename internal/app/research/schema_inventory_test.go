package research

import (
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"testing"
)

func TestResearchSaveSchema(t *testing.T) {
	if e := pramaan.ValidateSchema("save_research", saveResearchSchema()); e != nil {
		t.Fatal(e)
	}
}
