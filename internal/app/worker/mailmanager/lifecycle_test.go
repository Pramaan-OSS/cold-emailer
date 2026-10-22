package mailmanager

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type countedClient struct {
	pramaan.Caller
	closes atomic.Int32
}

func (c *countedClient) Close() error { c.closes.Add(1); return nil }

func smtpPayload() *models.AddWorkerEmail {
	return &models.AddWorkerEmail{ID: uuid.New(), UserID: uuid.New(), Type: models.InboxProviderSMTPIMAP, Email: "a@example.test", SmtpImap: &models.AddWorkerEmailSmtpImapData{Credentials: &models.SmtpImap{SMTP: &models.Service{Host: "smtp.example.test", Port: 587, Password: "old"}, IMAP: &models.Service{Host: "imap.example.test", Port: 993, Password: "old"}}, Mailboxes: []models.Mailbox{{Name: "Inbox", UIDValidity: 5, UIDNext: 25}}}, Sync: &models.AddWorkerEmailSyncData{State: &models.SyncState{BackfillStatus: models.SyncBackfillComplete, BackfillSynced: 7}}}
}

func TestMailboxReplacementDrainsClosesAndPreservesCursors(t *testing.T) {
	brokerFixture(t)
	m := NewMailManager(func(models.JobEventType, string, any) error { return nil }, nil, nil, nil, nil, nil)
	d := smtpPayload()
	if err := m.AddWMail(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	old := m.Get(d.ID)
	c := &countedClient{}
	old.Broker = c
	for i := 0; i < 3; i++ {
		if err := m.AddWMail(t.Context(), d); err != nil {
			t.Fatal(err)
		}
	}
	if m.Get(d.ID) != old || c.closes.Load() != 0 {
		t.Fatal("unchanged add restarted client")
	}
	copyData := *d
	data := &copyData
	s := *d.SmtpImap
	creds := *s.Credentials
	smtp := *creds.SMTP
	smtp.Password = "replacement"
	creds.SMTP = &smtp
	s.Credentials = &creds
	data.SmtpImap = &s
	done, ok := old.BeginExecution()
	if !ok {
		t.Fatal("live executor not admitted")
	}
	result := make(chan error, 1)
	go func() { result <- m.AddWMail(context.Background(), data) }()
	select {
	case <-old.Ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("replacement did not cancel old executor")
	}
	if m.Get(d.ID) != old {
		t.Fatal("new executor installed before old operation drained")
	}
	done()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("replacement stuck")
	}
	next := m.Get(d.ID)
	if next == nil || next == old || !next.SameExecution(data) || c.closes.Load() != 1 {
		t.Fatal("replacement credentials/cleanup not installed")
	}
	if data.Sync.State.BackfillSynced != 7 {
		t.Fatal("disabled sync lost saved cursor")
	}
	old.Terminate()
	if m.Get(d.ID) != next {
		t.Fatal("old auth callback retired replacement")
	}
	m.Terminate(d.ID)
	m.Terminate(d.ID)
	if next.Ctx.Err() == nil || c.closes.Load() != 1 {
		t.Fatal("removal did not stop session idempotently")
	}
}

func TestMailboxReplacementLocalOAuthAndPolicy(t *testing.T) {
	brokerFixture(t)
	for _, provider := range []models.InboxProvider{models.InboxProviderGoogle, models.InboxProviderOutlook} {
		t.Run(string(provider), func(t *testing.T) {
			m := NewMailManager(func(models.JobEventType, string, any) error { return nil }, nil, nil, nil, nil, nil)
			d := &models.AddWorkerEmail{ID: uuid.New(), UserID: uuid.New(), Type: provider, Google: &models.AddWorkerEmailGoogleData{Token: &models.MailToken{AccessToken: "old", RefreshToken: "old-refresh"}, LastHistoryID: 19}, Graph: &models.AddWorkerEmailGraphData{Token: &models.MailToken{AccessToken: "old", RefreshToken: "old-refresh"}, DeltaLinks: map[string]string{"Inbox": "live"}}}
			if err := m.AddWMail(t.Context(), d); err != nil {
				t.Fatal(err)
			}
			old := m.Get(d.ID)
			copyData := *d
			copyData.Sync = &models.AddWorkerEmailSyncData{Policy: models.SyncPolicy{DailyMessages: 90}}
			if err := m.AddWMail(t.Context(), &copyData); err != nil {
				t.Fatal(err)
			}
			if m.Get(d.ID) != old {
				t.Fatal("budget change rebuilt provider")
			}
			g := *d.Google
			g.Token = &models.MailToken{AccessToken: "new", RefreshToken: "new-refresh"}
			copyData.Google = &g
			graph := *d.Graph
			graph.Token = g.Token
			copyData.Graph = &graph
			if err := m.AddWMail(t.Context(), &copyData); err != nil {
				t.Fatal(err)
			}
			next := m.Get(d.ID)
			if next == old || old.Ctx.Err() == nil || !next.SameExecution(&copyData) {
				t.Fatal("reauthorization kept local OAuth client")
			}
			m.Terminate(d.ID)
			if err := m.AddWMail(t.Context(), &copyData); err != nil {
				t.Fatal(err)
			}
			if m.Get(d.ID) == nil || m.Get(d.ID) == next {
				t.Fatal("reload after removal failed")
			}
			m.Terminate(d.ID)
		})
	}
}

func TestMailboxReplacementEndpointSkipAndSentSettings(t *testing.T) {
	brokerFixture(t)
	for _, field := range []string{"imap-password", "smtp-host", "smtp-security", "skip-folders", "sent-copy"} {
		t.Run(field, func(t *testing.T) {
			m := NewMailManager(func(models.JobEventType, string, any) error { return nil }, nil, nil, nil, nil, nil)
			d := smtpPayload()
			if err := m.AddWMail(t.Context(), d); err != nil {
				t.Fatal(err)
			}
			old := m.Get(d.ID)
			switch field {
			case "imap-password":
				d.SmtpImap.Credentials.IMAP.Password = "new"
			case "smtp-host":
				d.SmtpImap.Credentials.SMTP.Host = "new.example.test"
			case "smtp-security":
				d.SmtpImap.Credentials.SMTP.Security = models.MailSecurityTLS
			case "skip-folders":
				d.Sync.Policy.SkipFolders = []string{"Custom"}
			case "sent-copy":
				v := false
				d.SaveToSent = &v
			}
			if err := m.AddWMail(t.Context(), d); err != nil {
				t.Fatal(err)
			}
			if m.Get(d.ID) == old {
				t.Fatalf("changed %s ignored", field)
			}
			m.Terminate(d.ID)
		})
	}
}

func brokerFixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[]}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("PRAMAAN_API_KEY", "local-test")
	t.Setenv("PRAMAAN_BASE_URL", server.URL)
}
