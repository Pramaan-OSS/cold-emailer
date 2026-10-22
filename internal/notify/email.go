package notify

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

// DefaultSenderDomain is the single platform sender-domain setting.
const DefaultSenderDomain = "updates.pramaan.one"

type EmailNotificationService interface {
	Send(ctx context.Context, to, cc, bcc []string, subject, message string) error
	// SendOutreach is the same as Send but lets the caller override
	// the Reply-To header. Used by the admin outreach composer so a
	// noreply From: address can still funnel replies into a real
	// support inbox. Empty replyTo falls back to the From address.
	SendOutreach(ctx context.Context, to []string, replyTo, subject, message string) error
}

type emailNotificationService struct {
	Name, Address, ReplyTo string
	Client                 pramaan.Caller
}

func NewEmailNotificationService(client pramaan.Caller, name, address string) EmailNotificationService {
	if strings.TrimSpace(name) == "" || strings.ContainsAny(name, "\"<>\r\n") {
		name = "Cold Email"
	}
	// Notifications use platform sending; the configured owner address receives replies.
	return &emailNotificationService{Name: name, Address: strings.TrimSpace(name) + " <noreply@" + DefaultSenderDomain + ">", ReplyTo: address, Client: client}
}
func (s *emailNotificationService) Send(ctx context.Context, to, cc, bcc []string, subject, message string) error {
	return s.send(ctx, to, cc, bcc, "", subject, message)
}
func (s *emailNotificationService) SendOutreach(ctx context.Context, to []string, replyTo, subject, message string) error {
	effectiveReplyTo := replyTo
	if effectiveReplyTo == "" {
		effectiveReplyTo = s.ReplyTo
	}
	if effectiveReplyTo != "" {
		if err := pramaan.ValidateMailRecipients([]string{effectiveReplyTo}); err != nil {
			return err
		}
	}
	if err := pramaan.ValidateMailRecipients(to); err != nil {
		return err
	}
	for _, address := range to {
		var check struct {
			Blocked *bool `json:"blocked"`
		}
		if err := s.Client.Call(ctx, "messaging.opt-out.check", map[string]any{"address": address, "channel": "email"}, &check); err != nil {
			return err
		}
		if check.Blocked == nil || *check.Blocked {
			return errors.New("Recipient permission does not allow this email.")
		}
	}
	return s.send(ctx, to, nil, nil, replyTo, subject, message)
}
func (s *emailNotificationService) send(ctx context.Context, to, cc, bcc []string, replyTo, subject, message string) error {
	if replyTo == "" {
		replyTo = s.ReplyTo
	}
	if err := pramaan.ValidateMailRecipients(to, cc, bcc); err != nil {
		return err
	}
	if replyTo != "" {
		if err := pramaan.ValidateMailRecipients([]string{replyTo}); err != nil {
			return err
		}
	}
	in := map[string]any{"from": s.Address, "to": to, "subject": subject, "html": message}
	if len(cc) > 0 {
		in["cc"] = cc
	}
	if len(bcc) > 0 {
		in["bcc"] = bcc
	}
	if replyTo != "" {
		in["replyTo"] = replyTo
	}
	return s.Client.Call(ctx, "email.send", in, nil, pramaan.Idempotency(uuid.NewString()))
}
