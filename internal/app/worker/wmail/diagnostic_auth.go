package wmail

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pkg/diagnosticauth"
	"github.com/pramaan-oss/cold-email/internal/repository"
)

type diagnosticRetry struct {
	token                         uuid.UUID
	messageID, providerID, folder string
	validity, uid                 uint32
	next, expires                 time.Time
	attempts                      int
}

func (w *WMail) verifyDiagnostic(ctx context.Context, msg *models.EmailMessageData, data *models.EmailMessageStoreData) {
	if w.ExecutorID == uuid.Nil {
		return
	}
	_, ok := w.SyncContext.(repository.DiagnosticAuthAuthority)
	if !ok {
		return
	}
	var token uuid.UUID
	for _, flag := range msg.Flags {
		if name, value, ok := strings.Cut(flag, ":"); ok && strings.EqualFold(strings.TrimSpace(name), config.WarmupVerifyHeader) {
			token, _ = uuid.Parse(strings.TrimSpace(value))
			break
		}
	}
	if token == uuid.Nil {
		return
	}
	retry := diagnosticRetry{token: token, messageID: msg.MessageID, providerID: msg.GmailID, folder: data.FolderPath, validity: data.Mailbox, uid: msg.UID, attempts: 1, next: time.Now().Add(time.Minute), expires: time.Now().Add(15 * time.Minute)}
	// IMAP sync owns the selected-folder mutex until the pass finishes.
	if w.EmailType == models.InboxProviderSMTPIMAP {
		retry.attempts = 0
		retry.next = time.Now()
	}
	if w.runDiagnostic(ctx, retry) {
		if w.diagnosticRetries == nil {
			w.diagnosticRetries = make(map[uuid.UUID]diagnosticRetry)
		}
		if len(w.diagnosticRetries) < 64 {
			w.diagnosticRetries[token] = retry
		}
	}
}

func (w *WMail) retryDiagnostics(ctx context.Context) {
	for token, retry := range w.diagnosticRetries {
		if ctx.Err() != nil {
			return
		}
		if time.Now().After(retry.expires) || retry.attempts >= 3 {
			delete(w.diagnosticRetries, token)
			continue
		}
		if time.Now().Before(retry.next) {
			continue
		}
		retry.attempts++
		retry.next = time.Now().Add(2 * time.Minute)
		if !w.runDiagnostic(ctx, retry) {
			delete(w.diagnosticRetries, token)
		} else {
			w.diagnosticRetries[token] = retry
		}
	}
}

func (w *WMail) runDiagnostic(ctx context.Context, retry diagnosticRetry) bool {
	if w.Broker == nil {
		return false
	}
	authority, ok := w.SyncContext.(repository.DiagnosticAuthAuthority)
	if !ok {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request := models.DiagnosticAuthRequest{Token: retry.token, MailboxID: w.ID, WorkerID: w.ExecutorID, MessageID: retry.messageID}
	grant, err := authority.DiagnosticAuth(ctx, request)
	if err != nil || grant == nil {
		return true
	}
	var response struct {
		Content string `json:"content"`
	}
	err = w.Broker.Call(ctx, "email.message.raw.get", map[string]any{"inbox": w.Email, "messageId": retry.messageID}, &response)
	if len(response.Content) > 2*diagnosticauth.MaxMIMEBytes {
		return true
	}
	raw, decodeErr := base64.StdEncoding.DecodeString(response.Content)
	if decodeErr != nil {
		err = decodeErr
	}

	if err != nil || len(raw) == 0 {
		clear(raw)
		return true
	}
	result := diagnosticauth.Verify(ctx, raw, *grant, nil)
	clear(raw)
	request.Nonce = grant.Nonce
	request.Result = &result
	_, err = authority.DiagnosticAuth(ctx, request)
	return err != nil || result.DKIM == "unknown"
}
