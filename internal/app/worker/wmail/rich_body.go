package wmail

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"strings"

	"github.com/emersion/go-message/mail"
	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/models"
)

// Normalized text remains available when a mailbox cannot supply raw MIME.
func (w *WMail) readRichBody(ctx context.Context, msg *models.EmailMessageData) {
	var out struct {
		Content string `json:"content"`
	}
	if w.Broker.Call(ctx, "email.message.raw.get", map[string]any{"inbox": w.Email, "messageId": msg.MessageID}, &out) != nil || len(out.Content) > 14_000_000 {
		return
	}
	raw, err := base64.StdEncoding.DecodeString(out.Content)
	if err != nil || len(raw) > 10_000_000 {
		return
	}
	defer clear(raw)
	reader, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return
	}
	defer reader.Close()
	if token := reader.Header.Get(config.WarmupVerifyHeader); token != "" {
		msg.Flags = append(msg.Flags, config.WarmupVerifyHeader+": "+token)
	}
	for count := 0; count < 100; count++ {
		part, err := reader.NextPart()
		if err != nil {
			return
		}
		if header, ok := part.Header.(*mail.InlineHeader); ok {
			kind, _, err := header.ContentType()
			if err != nil {
				continue
			}
			body, err := io.ReadAll(io.LimitReader(part.Body, 1_000_001))
			if err != nil || len(body) > 1_000_000 {
				continue
			}
			switch strings.ToLower(kind) {
			case "text/html":
				msg.BodyHTML = string(body)
			case "text/plain":
				msg.BodyPlain = string(body)
			}
		}
	}
}
