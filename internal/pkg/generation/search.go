package generation

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

var ErrSearchNotConfigured = errors.New("web search is not configured")

// SearchResult is one normalized web-search hit, provider-agnostic.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// SearchClient is the pluggable web-search backend the search_web tool (M2)
// calls. Kept an interface so the hosted product and self-hosters can swap
// backends (Serper, SearXNG, or a custom endpoint) without touching the tool.
type SearchClient interface {
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

type brokerSearch struct{ client pramaan.Caller }

func NewSearchClient(client pramaan.Caller) SearchClient {
	if client == nil {
		return nil
	}
	return brokerSearch{client}
}
func (c brokerSearch) Search(ctx context.Context, q string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	var out struct {
		Results []SearchResult `json:"results"`
	}
	e := c.client.Call(ctx, "web.search", map[string]any{"query": q, "limit": min(limit, 50)}, &out)
	return out.Results, e
}
func FormatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results."
	}
	var b strings.Builder
	for i, r := range results {
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(". ")
		b.WriteString(r.Title)
		b.WriteString("\n   ")
		b.WriteString(r.URL)
		if r.Snippet != "" {
			b.WriteString("\n   ")
			b.WriteString(r.Snippet)
		}
		b.WriteString("\n")
	}
	return b.String()
}
