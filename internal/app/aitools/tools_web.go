package aitools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/pramaan-oss/cold-email/internal/pramaan"

	"github.com/microcosm-cc/bluemonday"

	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pkg/generation"
)

const (
	fetchMaxBytes   = 2 << 20 // 2 MB cap on a fetched page
	fetchTimeout    = 10 * time.Second
	fetchCacheTTL   = 15 * time.Minute
	fetchCachePrefx = "aitools:fetch:"
)

// fetchTextPolicy strips all tags, leaving text content only.
var fetchTextPolicy = bluemonday.StrictPolicy()

var wsCollapse = regexp.MustCompile(`[ \t]*\n\s*\n\s*`)

func (d Deps) registerWebTools(r *Registry) {
	r.Register(Tool{
		Name:        "search_web",
		Description: "Search the public web and return the top results (title, url, snippet). Use to find current information about a company or person.",
		InputSchema: objectSchema(map[string]any{
			"query": strProp("The search query."),
			"limit": intProp("Max results (1-10, default 5)."),
		}, "query"),
		Risk:            generation.RiskRead,
		RequiredOrgPerm: models.PermUseAI,
		RequiredAPIPerm: models.APIPermAIAgent,
		Handler:         d.searchWeb,
	})

	r.Register(Tool{
		Name:        "fetch_url",
		Description: "Fetch a public web page and return its text content (HTML stripped). Only https public URLs are allowed; private and internal addresses are blocked.",
		InputSchema: objectSchema(map[string]any{
			"url": strProp("The https URL to fetch."),
		}, "url"),
		Risk:            generation.RiskRead,
		RequiredOrgPerm: models.PermUseAI,
		RequiredAPIPerm: models.APIPermAIAgent,
		Handler:         d.fetchURL,
	})
}

func (d Deps) searchWeb(ctx context.Context, _ Invocation, args json.RawMessage) (string, error) {
	in, err := decodeArgs[struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}](args)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(in.Query) == "" {
		return "", ErrInvalidArgs
	}
	if d.Search == nil {
		return "", generation.ErrSearchNotConfigured
	}
	results, err := d.Search.Search(ctx, in.Query, in.Limit)
	if err != nil {
		return "", err
	}
	return generation.FormatSearchResults(results), nil
}

func (d Deps) fetchURL(ctx context.Context, _ Invocation, args json.RawMessage) (string, error) {
	in, err := decodeArgs[struct {
		URL string `json:"url"`
	}](args)
	if err != nil {
		return "", err
	}
	var out struct {
		URL   string `json:"url"`
		Title string `json:"title"`
		Text  string `json:"text"`
	}
	if err := pramaan.Call(ctx, "web.scrape.structured", map[string]any{"url": in.URL}, &out); err != nil {
		return "", err
	}
	return jsonResult(map[string]any{"url": out.URL, "title": out.Title, "text": truncateRunes(out.Text, 12000)})
}

// htmlToText strips tags and collapses whitespace so a page fits in the model
// context.
func htmlToText(html string) string {
	stripped := fetchTextPolicy.Sanitize(html)
	stripped = wsCollapse.ReplaceAllString(stripped, "\n\n")
	return strings.TrimSpace(stripped)
}

func hashURL(u string) string {
	sum := sha256.Sum256([]byte(u))
	return hex.EncodeToString(sum[:])
}

// truncateRunes caps s to maxRunes runes (not bytes), appending an ellipsis
// when it trims, so a multi-byte UTF-8 sequence is never split mid-rune.
func truncateRunes(s string, maxRunes int) string {
	if len(s) <= maxRunes {
		return s // fast path: byte length <= rune cap means rune count is too
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}
