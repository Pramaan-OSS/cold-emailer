package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type warmupFixture struct {
	pool      *pgxpool.Pool
	user      uuid.UUID
	org       uuid.UUID
	recipient uuid.UUID
	sender    uuid.UUID
	senderTo  string
	task      uuid.UUID
	warmup    WarmupRepository
}

func newWarmupFixture(t *testing.T) *warmupFixture {
	t.Helper()
	_, pool := liveContactDB(t)
	ctx := context.Background()
	f := &warmupFixture{
		pool: pool, user: uuid.New(), org: uuid.New(),
		recipient: uuid.New(), sender: uuid.New(), task: uuid.New(),
		warmup: NewWarmupRepository(pool),
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql[:min(60, len(sql))], err)
		}
	}
	exec(`INSERT INTO users (id, email, first_name, last_name, password_hash) VALUES ($1, $2, 'Pool', 'Link', 'x')`,
		f.user, "pl-"+f.user.String()[:8]+"@test.local")
	exec(`INSERT INTO organizations (id, name, slug, owner_user_id) VALUES ($1, 'Pool Link', $2, $3)`,
		f.org, "pl-"+f.org.String()[:8], f.user)
	for _, id := range []uuid.UUID{f.recipient, f.sender} {
		addr := "pl-" + id.String()[:8] + "@test.local"
		exec(`INSERT INTO email_accounts (id, user_id, organization_id, email, name, signature_plain,
		          signature_html, provider, status, campaign_limit, min_wait_time, timezone,test_mode)
		      VALUES ($1, $2, $3, $4, 'PL', '', '', 'smtp_imap', 'active', 50, 600, 'UTC',NULL)`,
			id, f.user, f.org, addr)
		if id == f.sender {
			f.senderTo = addr
		}
	}
	exec(`INSERT INTO tasks (id, task_type, email_account_id, status, message_id)
	      VALUES ($1, 'warmup', $2, 'completed', '')`, f.task, f.sender)
	t.Cleanup(func() {
		c := context.Background()
		for _, sql := range []string{
			`DELETE FROM warmup_received WHERE email_account_id IN (SELECT id FROM email_accounts WHERE organization_id = $1)`,
			`DELETE FROM warmup_tokens WHERE recipient_account_id IN (SELECT id FROM email_accounts WHERE organization_id = $1)`,
			`DELETE FROM tasks WHERE email_account_id IN (SELECT id FROM email_accounts WHERE organization_id = $1)`,
			`DELETE FROM email_accounts WHERE organization_id = $1`,
			`DELETE FROM organizations WHERE id = $1`,
			`DELETE FROM users WHERE id = $1`,
		} {
			if _, err := pool.Exec(c, sql, f.org); err != nil {
				t.Errorf("cleanup %q: %v", sql[:min(50, len(sql))], err)
			}
		}
	})
	return f
}

// token writes one warmup token addressed to the recipient.
func (f *warmupFixture) token(t *testing.T, messageID, subject string, consumed bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var consumedAt any
	if consumed {
		consumedAt = time.Now()
	}
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO warmup_tokens (token, task_id, sender_account_id, recipient_account_id, sent_message_id, subject, consumed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, f.task, f.sender, f.recipient, messageID, subject, consumedAt)
	if err != nil {
		t.Fatalf("insert token: %v", err)
	}
	return id
}
