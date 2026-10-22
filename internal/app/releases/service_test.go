package releases

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pramaan-oss/cold-email/internal/models"
)

type memSettings struct{ release *models.FleetReleaseState }

func (m *memSettings) GetRelease(context.Context) (*models.FleetReleaseState, error) {
	return m.release, nil
}

func (m *memSettings) SetRelease(_ context.Context, s *models.FleetReleaseState) error {
	m.release = s
	return nil
}

func (m *memSettings) GetJoinToken(context.Context) (string, *time.Time, error) { return "", nil, nil }
func (m *memSettings) SetJoinToken(context.Context, string, time.Time) error    { return nil }

type githubStub struct{}

func (githubStub) RoundTrip(*http.Request) (*http.Response, error) {
	body := `[{"tag_name":"v2.0.0","published_at":"2026-09-29T12:00:00Z"},{"tag_name":"v1.1.0","published_at":"2026-09-28T12:00:00Z"}]`
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
}
