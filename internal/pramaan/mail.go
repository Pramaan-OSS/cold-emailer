package pramaan

import (
	"context"
	"errors"
	"strings"
)

type MailService struct{ Client Caller }
type MailAttachment struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
	MimeType string `json:"mimeType,omitempty"`
}
type MailRequest struct {
	From          string           `json:"from"`
	To            []string         `json:"to"`
	Cc            []string         `json:"cc,omitempty"`
	Bcc           []string         `json:"bcc,omitempty"`
	ReplyTo       string           `json:"replyTo,omitempty"`
	Subject       string           `json:"subject"`
	Text          string           `json:"text,omitempty"`
	HTML          string           `json:"html,omitempty"`
	InReplyTo     string           `json:"inReplyTo,omitempty"`
	References    []string         `json:"references,omitempty"`
	AutoSubmitted string           `json:"autoSubmitted,omitempty"`
	Attachments   []MailAttachment `json:"attachments,omitempty"`
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
	if r.InReplyTo != "" {
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
		if errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotConfigured) {
			return out, err
		}
		return out, ErrSendUncertain
	}
	if out.Status != "sent" || out.MessageID == "" {
		return out, ErrSendUncertain
	}
	return out, nil
}
