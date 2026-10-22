package aiagent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/app/aitools"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pkg/generation"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

type failureRepo struct {
	repository.AgentRepository
	saved models.AgentSessionContext
	rows  []models.AgentMessageRow
}

func (r *failureRepo) UpdateSessionContext(ctx context.Context, _, _, _ uuid.UUID, v models.AgentSessionContext) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	r.saved = v
	return nil
}
func (r *failureRepo) ClearPendingTool(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *failureRepo) GetToolPolicies(context.Context, uuid.UUID) (map[string]string, error) {
	return nil, nil
}
func (r *failureRepo) AppendMessages(_ context.Context, _, _, _ uuid.UUID, rows []models.AgentMessageRow) error {
	r.rows = append(r.rows, rows...)
	return nil
}

type failureGate struct{}

func (failureGate) IsPaidOrganization(context.Context, uuid.UUID) (bool, *errx.Error) {
	return false, nil
}

type failureProvider struct {
	generation.Provider
	fail bool
}

func (f *failureProvider) ModelForTier(bool) string { return "balanced" }
func (f *failureProvider) Name() string             { return "Pramaan" }
func (f *failureProvider) IsLocal() bool            { return true }
func (f *failureProvider) RunAgent(_ context.Context, r generation.AgentRequest) (*generation.AgentResult, error) {
	if f.fail {
		return &generation.AgentResult{Messages: r.Messages, StopReason: "failed"}, &pramaan.SchemaError{Name: "tool find_contact", Path: "#/properties/email/pattern", Reason: "unsupported keyword pattern"}
	}
	return &generation.AgentResult{Messages: r.Messages, StopReason: "stop"}, nil
}
func TestFailedRunIsSavedAndExplicitRetryCompletes(t *testing.T) {
	repo := &failureRepo{}
	p := &failureProvider{fail: true}
	s := &service{repo: repo, provider: p, feature: failureGate{}, registry: aitools.NewRegistry()}
	sess := &models.AgentSession{ID: uuid.New(), OrgID: uuid.New(), UserID: uuid.New()}
	inv := aitools.Invocation{OrgID: sess.OrgID, UserID: sess.UserID}
	events := []StreamEvent{}
	emit := func(e StreamEvent) { events = append(events, e) }
	if e := s.runLoop(context.Background(), inv, sess, nil, 0, "fixture", emit); e != nil {
		t.Fatal(e)
	}
	if repo.saved.RunStatus != "failed" || !strings.Contains(repo.saved.RunError, "#/properties/email/pattern") {
		t.Fatal(repo.saved)
	}
	for _, e := range events {
		if e.Type == evDone {
			t.Fatal("failure emitted completion")
		}
	}
	p.fail = false
	events = nil
	if e := s.runLoop(context.Background(), inv, sess, nil, 0, "explicit-retry", emit); e != nil {
		t.Fatal(e)
	}
	if repo.saved.RunStatus != "completed" || repo.saved.RunError != "" {
		t.Fatal("retry stayed failed", repo.saved)
	}
}
func TestCancelledRequestStillSavesFailedAction(t *testing.T) {
	repo := &failureRepo{}
	s := &service{repo: repo}
	sess := &models.AgentSession{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.failRun(ctx, sess, &generation.ActionError{Name: "create_task", Risk: generation.RiskWrite, Cause: errors.New("uncertain")}, func(StreamEvent) {}, 0)
	if repo.saved.RunStatus != "failed" || !strings.Contains(repo.saved.RunError, "check before retrying") {
		t.Fatal(repo.saved)
	}
}
func TestHydrationNeverMarksFailedActionDone(t *testing.T) {
	turns := hydrateTranscript([]generation.AgentMessage{{Role: "assistant", ToolCalls: []generation.ToolCall{{ID: "tc_fixture", Name: "create_task"}}}, {Role: "tool", ToolCallID: "tc_fixture", Content: `{"status":"failed","message":"Check before retrying"}`}})
	b := turns[0].Blocks[0]
	if b.Done || !b.Failed {
		t.Fatal(b)
	}
}

func TestCutOffFailureKeepsSafeReason(t *testing.T) {
	repo := &failureRepo{}
	s := &service{repo: repo}
	sess := &models.AgentSession{}
	s.failRun(context.Background(), sess, &pramaan.IncompleteReply{Text: "partial", Tokens: 30}, func(StreamEvent) {}, 0)
	if repo.saved.RunStatus != "failed" || !strings.Contains(repo.saved.RunError, "incomplete") {
		t.Fatal(repo.saved)
	}
}
