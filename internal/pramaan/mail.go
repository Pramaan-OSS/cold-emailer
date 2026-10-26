package pramaan

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

type MailService struct{ Client Caller }
type MailAttachment struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
	MimeType string `json:"mimeType,omitempty"`
}
type MailRequest struct {
	From          string            `json:"from"`
	To            []string          `json:"to"`
	Cc            []string          `json:"cc,omitempty"`
	Bcc           []string          `json:"bcc,omitempty"`
	ReplyTo       string            `json:"replyTo,omitempty"`
	Subject       string            `json:"subject"`
	Text          string            `json:"text,omitempty"`
	HTML          string            `json:"html,omitempty"`
	InReplyTo     string            `json:"inReplyTo,omitempty"`
	References    []string          `json:"references,omitempty"`
	AutoSubmitted string            `json:"autoSubmitted,omitempty"`
	Attachments   []MailAttachment  `json:"attachments,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
}

var ErrSendUncertain = errors.New("Send outcome is unknown. Check the message before trying again.")

type MailResult struct {
	Status       string `json:"status"`
	Ref          string `json:"ref"`
	MessageID    string `json:"messageId"`
	ThreadID     string `json:"threadId"`
	Acceptance   string `json:"acceptance"`
	RFCMessageID string `json:"rfcMessageId"`
}

// Send fails closed if opt-out verification fails, and uses the task's stable action key.
func (s MailService) Send(ctx context.Context, r MailRequest, key string) (MailResult, error) {
	var out MailResult
	if s.Client == nil {
		return out, ErrNotConfigured
	}
	if key == "" || r.From == "" || len(r.To) == 0 || strings.TrimSpace(r.Text+r.HTML) == "" {
		return out, errors.New("Complete the sender, recipient and message first.")
	}
	for _, address := range append(append(append([]string{}, r.To...), r.Cc...), r.Bcc...) {
		var check struct {
			Blocked *bool `json:"blocked"`
		}
		if e := s.Client.Call(ctx, "messaging.opt-out.check", map[string]any{"address": address, "channel": "email"}, &check); e != nil {
			return out, e
		}
		if check.Blocked == nil {
			return out, errors.New("Recipient permission could not be checked.")
		}
		if *check.Blocked {
			return out, errors.New("This recipient has opted out of email.")
		}
	}
	if err := ValidateMailHeaders(r.Headers); err != nil {
		return out, err
	}
	if r.InReplyTo != "" {
		if len(r.Headers) > 0 {
			return out, errors.New("Custom headers are unavailable on replies. Send a new message or remove the header option explicitly.")
		}
		in := map[string]any{"inbox": r.From, "messageId": r.InReplyTo, "to": r.To, "subject": r.Subject, "text": r.Text, "html": r.HTML}
		if len(r.Cc) > 0 {
			in["cc"] = r.Cc
		}
		if len(r.Bcc) > 0 {
			in["bcc"] = r.Bcc
		}
		if r.ReplyTo != "" {
			in["replyTo"] = r.ReplyTo
		}
		if r.AutoSubmitted != "" {
			in["autoSubmitted"] = r.AutoSubmitted
		}
		if len(r.Attachments) > 0 {
			in["attachments"] = r.Attachments
		}
		err := s.Client.Call(ctx, "email.message.reply", in, &out, Idempotency(key))
		return checkedSend(out, err)
	}
	err := s.Client.Call(ctx, "email.message.send", r, &out, Idempotency(key))
	return checkedSend(out, err)
}

func checkedSend(out MailResult, err error) (MailResult, error) {
	if err != nil {
		if errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotConfigured) || errors.Is(err, ErrMailHeadersRefused) {
			return out, err
		}
		var refusal *HTTPError
		if errors.As(err, &refusal) && refusal.Effect == "none" {
			return out, err
		}
		return out, ErrSendUncertain
	}
	if out.Status != "sent" || out.MessageID == "" {
		return out, ErrSendUncertain
	}
	return out, nil
}

// ValidateMailHeaders refuses unsupported fields before dispatch. The selected path
// may further refuse List-* headers; callers never strip them or resend silently.
func ValidateMailHeaders(headers map[string]string) error {
	diagnostic := 0
	seen := map[string]bool{}
	unsubscribe, post := "", ""
	for name, value := range headers {
		lower := strings.ToLower(name)
		if seen[lower] {
			return errors.New("Duplicate message header.")
		}
		seen[lower] = true
		for _, c := range name {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return errors.New("Invalid message header name.")
			}
		}
		if value == "" {
			return errors.New("Message header value is empty.")
		}
		for _, c := range value {
			if c < 32 || c > 126 {
				return errors.New("Message headers must contain printable ASCII only.")
			}
		}
		switch lower {
		case "list-unsubscribe":
			unsubscribe = value
		case "list-unsubscribe-post":
			post = value
		default:
			if !strings.HasPrefix(lower, "x-") || len(lower) <= 2 {
				return errors.New("This message header is unsupported.")
			}
			diagnostic++
		}
	}
	if diagnostic > 1 {
		return errors.New("Use only one diagnostic message header.")
	}
	if post != "" {
		u, e := url.Parse(strings.TrimSuffix(strings.TrimPrefix(unsubscribe, "<"), ">"))
		if post != "List-Unsubscribe=One-Click" || e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return errors.New("One-click unsubscribe needs an HTTPS unsubscribe link.")
		}
	}
	return nil
}
