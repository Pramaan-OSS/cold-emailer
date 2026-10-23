package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pramaan-oss/cold-email/internal/models"
)

// A campaign's sending window is a promise about when mail leaves, and an
// OVERDUE follow-up used to break it.
//
// Every schedule gate in the placer is asked about the step's candidate time,
// which for a follow-up starts at "when its wait elapsed". Once that moment is
// in the past, two gates stop working: nextScheduleSlot deliberately returns an
// instant that was ALREADY inside a window unchanged ("keep the exact
// instant"), and the end-date comparison finds a candidate that predates the
// end date. So the hard floor sat in the past, the not-due check read the step
// as due, and the task sent it immediately — hours after the window closed, or
// after the campaign was over.
//
// The chain reaches those hours routinely: a deferral is capped at
// config.CampaignMaxDeferMinutes so a wake-up near closing time lands past it,
// the reconciler re-seeds a dead chain a minute from now whatever the clock
// says, and a dispatch failure is retried the same way.
//
//	COLD_EMAIL_TEST_DB=postgres://coldemail:coldemail@localhost:15432/coldemail_dev?sslmode=disable \
//	  go test ./internal/scheduler/ -run LiveOverdue -v

// overdueFollowUp gives the fixture's lead a step 1 sent at sentAt and a step 2
// routed out of it with no wait, so step 2 has been due since sentAt.
func overdueFollowUp(t *testing.T, pool *pgxpool.Pool, f *liveFixture, step1, contact uuid.UUID, sentAt time.Time) {
	t.Helper()
	ctx := context.Background()
	step2 := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO sequences (id, campaign_id, organization_id, name, subject,
	        body_plain, body_html, wait_after, position, kind)
	    VALUES ($1, $2, $3, 'Step 2', 'Bump', 'Bump', '<p>Bump</p>', 0, 1, 'email')`,
		step2, f.campaign, f.org); err != nil {
		t.Fatalf("add step 2: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sequences SET conditions = jsonb_build_object('branches',
	        jsonb_build_array(jsonb_build_object('branch_id', 'else', 'target_step_id', $1::text)))
	    WHERE id = $2`, step2.String(), step1); err != nil {
		t.Fatalf("connect steps: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO campaign_contact_progress
	    (campaign_id, contact_id, sequence_id, sent_at) VALUES ($1, $2, $3, $4)`,
		f.campaign, contact, step1, sentAt); err != nil {
		t.Fatalf("stamp step 1: %v", err)
	}
}

// hhmm renders an instant as the campaigns table's time-of-day.
func hhmm(t time.Time) string { return t.UTC().Format("15:04") }

// A time-relative fixture can span midnight. Legacy start/end fields only
// describe same-day windows, so use the supported per-day intervals instead.
func setOverdueTestWindow(t *testing.T, pool *pgxpool.Pool, campaign uuid.UUID, opened, closed time.Time) models.ScheduleWindows {
	t.Helper()
	start := opened.UTC().Hour()*60 + opened.UTC().Minute()
	end := closed.UTC().Hour()*60 + closed.UTC().Minute()
	var windows models.ScheduleWindows
	for day := range windows {
		if start < end {
			windows[day] = []models.TimeInterval{{Start: start, End: end}}
		} else {
			windows[day] = []models.TimeInterval{{Start: start, End: 1440}}
			if end > 0 {
				windows[day] = append(windows[day], models.TimeInterval{Start: 0, End: end})
			}
		}
	}
	if _, err := pool.Exec(context.Background(), `UPDATE campaigns SET schedule_windows = $2 WHERE id = $1`, campaign, windows); err != nil {
		t.Fatalf("set the window: %v", err)
	}
	return windows
}

// An overdue follow-up must wait for the window to reopen, not go out the
// moment the chain happens to wake up outside it.
func TestLiveOverdueFollowUpWaitsForTheWindowToReopen(t *testing.T) {
	for _, hour := range []int{15, 1} {
		now := time.Date(2025, 10, 23, hour, 0, 0, 0, time.UTC)
		t.Run(now.Format("15:04"), func(t *testing.T) {
			handle, pool := liveDB(t)
			f := newLiveFixture(t, pool, "UTC")
			ctx := context.Background()
			step1, contact := liveLead(t, pool, f.campaign)
			opened, closed := now.Add(-3*time.Hour), now.Add(-time.Hour)
			windows := setOverdueTestWindow(t, pool, f.campaign, opened, closed)
			overdueFollowUp(t, pool, f, step1, contact, now.Add(-2*time.Hour))

			at, pair, _, err := liveSchedulerAt(t, handle, pool, now).CalculateNextCampaignTime(ctx, f.campaign)
			if pair != nil && err == nil {
				t.Fatalf("the scheduler authorised a send at %s, after the window closed at %s", hhmm(now), hhmm(closed))
			}
			if !errors.Is(err, ErrCampaignDeferred) {
				t.Fatalf("want a deferral, got %v", err)
			}
			if !at.After(now) || !at.Before(now.AddDate(0, 0, 8)) {
				t.Fatalf("deferred to %s, outside the fixed clock's eight-day schedule horizon", at.UTC().Format(time.RFC3339))
			}
			if !nextScheduleSlot(at, windows, time.UTC).Equal(at) {
				t.Fatalf("deferred to %s, outside the %s-%s window", at.UTC().Format(time.RFC3339), hhmm(opened), hhmm(closed))
			}
		})
	}
}

// The same step, with the window still open, must still go out promptly: the
// fix is a floor on the candidate, not a new wait.
func TestLiveOverdueFollowUpStillSendsInsideTheWindow(t *testing.T) {
	handle, pool := liveDB(t)
	f := newLiveFixture(t, pool, "UTC")
	ctx := context.Background()
	step1, contact := liveLead(t, pool, f.campaign)

	// A window that opened two hours ago and is open for another two.
	now := time.Now().UTC()
	setOverdueTestWindow(t, pool, f.campaign, now.Add(-2*time.Hour), now.Add(2*time.Hour))
	overdueFollowUp(t, pool, f, step1, contact, now.Add(-90*time.Minute))

	_, pair, _, err := liveScheduler(t, handle, pool).CalculateNextCampaignTime(ctx, f.campaign)
	if err != nil || pair == nil {
		t.Fatalf("an overdue follow-up inside the window must send, got pair=%v err=%v", pair, err)
	}
}

// The end date is the other gate a past candidate slipped under: the comparison
// asked whether the moment the step came due was past the end date, not whether
// now is.
func TestLiveOverdueFollowUpRespectsTheEndDate(t *testing.T) {
	handle, pool := liveDB(t)
	f := newLiveFixture(t, pool, "UTC")
	ctx := context.Background()
	step1, contact := liveLead(t, pool, f.campaign)

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `UPDATE campaigns SET end_date = $2 WHERE id = $1`,
		f.campaign, now.Add(-time.Hour)); err != nil {
		t.Fatalf("set the end date: %v", err)
	}
	overdueFollowUp(t, pool, f, step1, contact, now.Add(-2*time.Hour))

	_, pair, _, err := liveScheduler(t, handle, pool).CalculateNextCampaignTime(ctx, f.campaign)
	if pair != nil && err == nil {
		t.Fatal("the scheduler authorised a send for a campaign that ended an hour ago")
	}
	if !errors.Is(err, ErrCampaignEnded) {
		t.Fatalf("want the campaign reported as ended, got %v", err)
	}
}
