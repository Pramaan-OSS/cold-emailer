package wmail

import (
	"context"
	"encoding/base64"
	"errors"
	"html"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

// parentReference resolves what a reply should be threaded onto, from whichever
// of the two independent handles the request carries.
//
// They are genuinely independent. ThreadID is the provider's own conversation
// id and is what makes the message appear in the thread in the SENDER's
// mailbox; it is meaningless to anyone else. MessageID becomes the RFC
// In-Reply-To/References headers, which is the only thing the RECIPIENT's mail
// client can thread on. A dashboard reply starts with only a ThreadID and a
// warmup reply with only a Message-ID, so requiring both threads neither.
func parentReference(req *SendRequest) *models.EmailMessageData {
	if req == nil {
		return nil
	}
	var messageID, threadID string
	if req.Parent != nil {
		messageID = req.Parent.MessageID
		threadID = req.Parent.ThreadID
	}
	if messageID == "" && req.InReplyTo != "" {
		messageID = strings.Trim(req.InReplyTo, "<>")
	}
	if messageID == "" && threadID == "" {
		return nil
	}
	return &models.EmailMessageData{MessageID: messageID, ThreadID: threadID}
}

// Attachment is a fully-resolved attachment ready to be MIME-encoded: the
// worker has already fetched Data from object storage.
type Attachment struct {
	Filename string
	MimeType string
	Data     []byte
}

// SendRequest contains all parameters needed to send an email
type SendRequest struct {
	TaskID      uuid.UUID
	To          []string
	Cc          []string
	Bcc         []string
	MessageID   string
	Subject     string
	BodyPlain   string
	BodyHTML    string
	InReplyTo   string
	References  string
	Parent      *models.EmailParent
	IsWarmup    bool
	WarmupToken string
	// UnsubscribeURL, when set (campaign sends with the unsubscribe header
	// enabled), produces RFC 8058 one-click unsubscribe headers.
	UnsubscribeURL string
	// Attachments, when present, are encoded as multipart/mixed parts after the
	// multipart/alternative text body. Warmup sends never carry attachments.
	Attachments []Attachment
	// FromName is the display name the control plane holds for the mailbox at
	// publish time. Empty falls back to the name cached from ADD_EMAIL.
	FromName string
	// FromEmail is the verified provider alias the mailbox was told to send
	// as. Gmail only, and empty for almost every send, which means the
	// mailbox's own address. It is never set on a warmup send: warmup pairs
	// on the mailbox address, so an alias there would break verification.
	FromEmail string
	// ReplyTo is the address the Reply-To header names, so a reply lands in
	// another mailbox. Empty for no header; never set on a warmup send.
	ReplyTo string
}

type SendResult struct {
	Success       bool
	MessageID     string
	ProviderMsgID string
	// ThreadID is the provider-side conversation this message landed in.
	// Gmail is the only provider that has one and the only one that needs it
	// back: appending a later message to the same thread requires handing the
	// id to the API, so the control plane records it against the task and
	// gives it to the next step (issue #472). Empty everywhere else.
	ThreadID string
	SentAt   time.Time
	Error    *errx.MailError
}

func (w *WMail) Send(ctx context.Context, r *SendRequest) *SendResult {
	result := &SendResult{SentAt: time.Now().UTC()}
	done, ok := w.BeginExecution()
	if !ok {
		result.Error = errx.ErrMailServerUnreachable
		return result
	}
	defer done()
	ctx, cancel := w.ExecutionContext(ctx)
	defer cancel()
	from := w.Email
	if r.FromEmail != "" {
		from = r.FromEmail
	}
	in := pramaan.MailRequest{From: from, To: r.To, Cc: r.Cc, Bcc: r.Bcc, ReplyTo: r.ReplyTo, Subject: r.Subject, Text: r.BodyPlain, HTML: r.BodyHTML, InReplyTo: r.InReplyTo}
	if r.References != "" {
		in.References = strings.Fields(r.References)
	}
	if r.UnsubscribeURL != "" {
		in.Headers = map[string]string{"List-Unsubscribe": "<" + r.UnsubscribeURL + ">", "List-Unsubscribe-Post": "List-Unsubscribe=One-Click"}
		if !strings.Contains(in.Text, r.UnsubscribeURL) {
			in.Text += "\n\nTo stop these emails: " + r.UnsubscribeURL
		}
		if in.HTML != "" && !strings.Contains(in.HTML, r.UnsubscribeURL) {
			in.HTML += "<p><a href=\"" + html.EscapeString(r.UnsubscribeURL) + "\">Stop these emails</a></p>"
		}
	}
	if parent := parentReference(r); parent != nil && in.InReplyTo == "" {
		in.InReplyTo = parent.MessageID
	}
	if r.IsWarmup {
		if r.WarmupToken != "" {
			if in.Headers == nil {
				in.Headers = map[string]string{}
			}
			in.Headers[config.WarmupVerifyHeader] = r.WarmupToken
		}
		in.HTML = ""
		in.AutoSubmitted = "auto-generated"
		if in.InReplyTo != "" {
			in.AutoSubmitted = "auto-replied"
		}
	}
	for _, a := range r.Attachments {
		in.Attachments = append(in.Attachments, pramaan.MailAttachment{Filename: a.Filename, Content: base64.StdEncoding.EncodeToString(a.Data), MimeType: a.MimeType})
	}
	sent, e := (pramaan.MailService{Client: w.Broker}).Send(ctx, in, "mail-task-"+r.TaskID.String())
	if e != nil {
		result.Error = errx.MError(errx.MailErrorWarning, errx.MailErrorCodeSendRejected, e.Error(), errx.MailErrorResolveMethodNone)
		if errors.Is(e, pramaan.ErrSendUncertain) {
			result.Error = errx.WithSendFailure(result.Error, errx.SendFailure{Disposition: errx.SendAmbiguous, Scope: "mailbox", Protocol: "pramaan", ObservedAt: time.Now()})
		}
		return result
	}
	result.Success = true
	result.MessageID = sent.MessageID
	result.ProviderMsgID = sent.Ref
	result.ThreadID = sent.ThreadID
	return result
}

func DetermineErrorEventType(err *errx.MailError) models.JobEventType {
	if err == nil {
		return models.JobEventTypeEmailFailed
	}
	if err.Failure != nil {
		switch err.Failure.Disposition {
		case errx.SendRetry, errx.SendAmbiguous:
			return models.JobEventTypeEmailServerError
		case errx.SendThrottle:
			return models.JobEventTypeEmailRateLimited
		}
	}

	switch err.Code {
	case errx.MailErrorCodeGoogleAuth, errx.MailErrorCodeAuthenticationFailed, errx.MailErrorCodeInvalidCredentials:
		return models.JobEventTypeEmailAuthError

	case errx.MailErrorCodeAccountSuspended, errx.MailErrorCodeAuthorizationFailed:
		return models.JobEventTypeEmailDisabled

	case errx.MailErrorCodeRateLimitExceeded:
		return models.JobEventTypeEmailRateLimited

	case errx.MailErrorCodeServerUnreachable, errx.MailErrorCodeConnectionLost, errx.MailErrorCodeNotFound:
		return models.JobEventTypeEmailServerError

	default:
		return models.JobEventTypeEmailFailed
	}
}

func MailErrorToSendError(err *errx.MailError) *models.EmailSendError {
	if err == nil {
		return nil
	}

	userInfo := err.GetUserErrorInfo()
	failure := err.Failure
	if failure == nil && err.RetryAfter > 0 && (err.Code == errx.MailErrorCodeSendingTooFast || err.Code == errx.MailErrorCodeQuotaExceeded) {
		now := time.Now().UTC()
		at := now.Add(err.RetryAfter)
		failure = &errx.SendFailure{Disposition: errx.SendThrottle, Scope: "mailbox", ObservedAt: now, RetryAt: &at}
	}

	return &models.EmailSendError{
		Failure:        failure,
		Code:           string(err.Code),
		Type:           string(err.Type),
		Message:        err.Message,
		ResolveMethod:  string(err.ResolveMethod),
		UserVisible:    err.IsUserVisible(),
		UserTitle:      userInfo.Title,
		UserMessage:    userInfo.Message,
		ActionRequired: userInfo.ActionRequired,
		Recipient:      err.Recipient,
	}
}

func threadParentID(msg *models.EmailMessageData) string {
	if msg == nil {
		return ""
	}
	for i := len(msg.InReplyTo) - 1; i >= 0; i-- {
		if id := strings.TrimSpace(msg.InReplyTo[i]); id != "" {
			return id
		}
	}
	return ""
}
