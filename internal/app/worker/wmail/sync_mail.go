package wmail

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type brokerMessage struct {
	MessageID     string   `json:"messageId"`
	Inbox         string   `json:"inbox"`
	From          string   `json:"from"`
	To            []string `json:"to"`
	Cc            []string `json:"cc"`
	Bcc           []string `json:"bcc"`
	ReplyTo       []string `json:"replyTo"`
	Subject       string   `json:"subject"`
	Preview       string   `json:"preview"`
	Text          string   `json:"text"`
	Timestamp     string   `json:"timestamp"`
	Labels        []string `json:"labels"`
	InReplyTo     string   `json:"inReplyTo"`
	AutoSubmitted string   `json:"autoSubmitted"`
}

func (w *WMail) SyncMail(ctx context.Context) *errx.MailError {
	done, ok := w.BeginExecution()
	if !ok {
		return errx.ErrMailServerUnreachable
	}
	defer done()
	w.syncMu.Lock()
	defer w.syncMu.Unlock()
	ctx, cancel := w.ExecutionContext(ctx)
	defer cancel()
	if w.Broker == nil {
		return errx.ErrMailServerUnreachable
	}
	w.retryDiagnostics(ctx)
	w.beginTick()
	stats := &tickStats{denied: map[SyncLane]Admission{}}
	complete := false
	defer func() { w.endTick(ctx, stats, complete) }()
	in := map[string]any{"inbox": w.Email, "limit": 50}
	if w.cursor != "" {
		in["cursor"] = w.cursor
	}
	var page struct {
		Results    []brokerMessage `json:"results"`
		NextCursor string          `json:"nextCursor"`
	}
	if e := w.Broker.Call(ctx, "email.messages.list", in, &page); e != nil {
		return brokerMailError(e)
	}
	for _, row := range page.Results {
		if row.MessageID == "" {
			continue
		}
		known, e := w.EmailMessageMapRepository.Get(ctx, w.UserID, w.ID, row.MessageID)
		if e != nil {
			return brokerMailError(e)
		}
		if known != nil {
			continue
		}
		if !w.admit(ctx, LaneLive, stats) {
			return nil
		}
		var body brokerMessage
		if e = w.Broker.Call(ctx, "email.message.get", map[string]any{"inbox": w.Email, "messageId": row.MessageID}, &body); e != nil {
			return brokerMailError(e)
		}
		sentAt, _ := time.Parse(time.RFC3339, body.Timestamp)
		folder := models.FolderInbox
		for _, label := range body.Labels {
			if models.FilableFolder(label) {
				folder = label
			}
		}
		id := uuid.New()
		msg := &models.EmailMessageData{ID: id, MessageID: body.MessageID, GmailID: body.MessageID, ThreadID: body.InReplyTo, ParentID: body.InReplyTo, From: []string{body.From}, To: body.To, CC: body.Cc, BCC: body.Bcc, ReplyTo: body.ReplyTo, Subject: body.Subject, Snippet: body.Preview, Date: sentAt, InternalDate: sentAt, BodyPlain: body.Text, Folder: folder, Flags: body.Labels}
		if body.InReplyTo != "" {
			msg.InReplyTo = []string{body.InReplyTo}
		}
		if body.AutoSubmitted != "" {
			msg.Flags = append(msg.Flags, "Auto-Submitted: "+body.AutoSubmitted)
		}
		w.readRichBody(ctx, msg)
		data := &models.EmailMessageStoreData{ID: id, EmailID: w.ID, MessageID: msg.MessageID, GmailID: msg.MessageID, ThreadID: msg.ThreadID, ParentID: msg.ParentID, Folder: folder, ProviderFolder: folder, Flags: msg.Flags, FromAddr: msg.From, ToAddr: msg.To, CC: msg.CC, BCC: msg.BCC, ReplyTo: msg.ReplyTo, InReplyTo: msg.InReplyTo, Subject: msg.Subject, SentDate: sentAt, InternalDate: sentAt, Snippet: GenerateSnippet(msg.BodyPlain, ""), BodyText: SearchText(msg.BodyPlain, ""), CreatedAt: time.Now().UTC()}
		if e = w.storeNew(ctx, msg, data, msg.MessageID); e != nil {
			return brokerMailError(e)
		}
	}
	w.cursor = page.NextCursor
	w.tracker.state.BackfillCursor.PageToken = page.NextCursor
	w.tracker.mark()
	w.tracker.flush(time.Now())
	complete = page.NextCursor == ""
	return nil
}
func brokerMailError(e error) *errx.MailError {
	return errx.MError(errx.MailErrorWarning, errx.MailErrorCodeServerUnreachable, e.Error(), errx.MailErrorResolveMethodNone)
}
func (w *WMail) ChangeLabels(ctx context.Context, id string, add, remove []string, key string) error {
	if w.Broker == nil {
		return pramaan.ErrNotConfigured
	}
	return w.Broker.Call(ctx, "email.message.update", map[string]any{"inbox": w.Email, "messageId": id, "addLabels": add, "removeLabels": remove}, nil, pramaan.Idempotency(key))
}
func (w *WMail) DeleteMessage(ctx context.Context, id, key string) error {
	if w.Broker == nil {
		return pramaan.ErrNotConfigured
	}
	return w.Broker.Call(ctx, "email.message.delete", map[string]any{"inbox": w.Email, "messageId": id}, nil, pramaan.Idempotency(key))
}
func (w *WMail) LocateMessage(ctx context.Context, id string) (bool, bool, error) {
	if w.Broker == nil {
		return false, false, pramaan.ErrNotConfigured
	}
	cursor := ""
	for pages := 0; pages < 100; pages++ {
		var out struct {
			Results    []brokerMessage `json:"results"`
			NextCursor string          `json:"nextCursor"`
		}
		in := map[string]any{"inbox": w.Email, "query": id, "limit": 50}
		if cursor != "" {
			in["cursor"] = cursor
		}
		if err := w.Broker.Call(ctx, "email.messages.search", in, &out); err != nil {
			return false, false, err
		}
		for _, m := range out.Results {
			if strings.Trim(m.MessageID, "<>") == strings.Trim(id, "<>") {
				trash := false
				for _, label := range m.Labels {
					trash = trash || label == "trash" || label == "deleted"
				}
				return true, trash, nil
			}
		}
		if out.NextCursor == "" {
			return false, false, nil
		}
		if out.NextCursor == cursor {
			return false, false, errors.New("The search is incomplete. Check again later.")
		}
		cursor = out.NextCursor
	}
	return false, false, errors.New("The search is incomplete. Check again later.")
}
