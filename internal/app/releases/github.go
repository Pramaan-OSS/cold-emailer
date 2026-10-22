package releases

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

// Release is one GitHub release as the update surfaces read it.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
}

// FetchReleases lists the newest releases of owner/repo. token is optional and
// only raises the unauthenticated rate limit.
func FetchReleases(ctx context.Context, client *http.Client, repo, token string) ([]Release, error) {
	return nil, pramaan.ErrUnavailable
}

// PickChannelHeads returns the most recent published release for each channel:
//   - stable: latest non-prerelease, non-draft
//   - dev:    latest published (including prereleases)
func PickChannelHeads(releases []Release) (stable, dev *Release) {
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].PublishedAt.After(releases[j].PublishedAt)
	})
	for i := range releases {
		r := &releases[i]
		if r.Draft {
			continue
		}
		if dev == nil {
			dev = r
		}
		if !r.Prerelease && stable == nil {
			stable = r
		}
		if dev != nil && stable != nil {
			break
		}
	}
	return
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
