package opsnotify

import (
	"html"
	"strings"
)

func emailSubject(e Event) string {
	prefix := "[ColdEmail]"
	if e.Severity == SeverityUrgent {
		prefix = "[ColdEmail] Urgent:"
	}
	return prefix + " " + e.Title
}

// emailBodyHTML renders the alert. The transport derives the plain-text part
// from this, so it stays simple and table-free.
func emailBodyHTML(e Event) string {
	var sb strings.Builder
	sb.WriteString(`<div style="font-family:-apple-system,Segoe UI,Helvetica,Arial,sans-serif;font-size:14px;color:#0f172a;line-height:1.6">`)
	sb.WriteString(`<p style="margin:0 0 8px;font-size:16px;font-weight:600">` + esc(e.Title) + `</p>`)
	if e.Summary != "" {
		sb.WriteString(`<p style="margin:0 0 12px;color:#475569">` + esc(e.Summary) + `</p>`)
	}
	if len(e.Fields) > 0 {
		sb.WriteString(`<ul style="margin:0 0 12px;padding-left:18px;color:#334155">`)
		for _, f := range e.Fields {
			sb.WriteString(`<li><strong>` + esc(f.Label) + `:</strong> ` + esc(emptyDash(f.Value)) + `</li>`)
		}
		sb.WriteString(`</ul>`)
	}
	if e.Link != "" {
		sb.WriteString(`<p style="margin:0 0 12px"><a href="` + esc(e.Link) + `">Open in the admin panel</a></p>`)
	}
	sb.WriteString(`<p style="margin:16px 0 0;color:#94a3b8;font-size:12px">You are receiving this because this address is an operator notification channel on your ColdEmail instance.</p>`)
	sb.WriteString(`</div>`)
	return sb.String()
}

func esc(v string) string { return html.EscapeString(v) }

func emptyDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "—"
	}
	return v
}

func truncate(v string, max int) string {
	if len(v) <= max {
		return v
	}
	if max <= 1 {
		return v[:max]
	}
	return v[:max-1] + "…"
}
