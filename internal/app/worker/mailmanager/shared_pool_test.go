package mailmanager

import (
	"context"
	"strings"
	"testing"

	"github.com/pramaan-oss/cold-email/internal/models"
)

func TestRetiredSharedPoolJobCannotLoadAMailbox(t *testing.T) {
	m := NewMailManager(nil, nil, nil, nil, nil, nil)
	err := m.AddWMail(context.Background(), &models.AddWorkerEmail{Brokered: true})
	if err == nil || !strings.Contains(err.Error(), "unavailable") || len(m.Emails) != 0 {
		t.Fatalf("retired job loaded or lacked a clear error: %v", err)
	}
}
