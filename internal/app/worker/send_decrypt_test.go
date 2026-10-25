package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/storage"
	"io"

	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/pramaan"

	"github.com/pramaan-oss/cold-email/internal/app/cipher"
	"github.com/pramaan-oss/cold-email/internal/app/worker/wmail"

	"github.com/pramaan-oss/cold-email/internal/infrastructure/codec"
	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pkg/emsg"
)

type failingCipherService struct{}

func (failingCipherService) Cipher(context.Context, uuid.UUID) (*cipher.Cipher, error) {
	return nil, errors.New("key store unavailable")
}

func TestSendThatCannotBeDecryptedFailsWithoutSending(t *testing.T) {
	keys := cipher.NewStatic(bytes.Repeat([]byte{5}, 32))
	orgID := uuid.New()
	c, err := keys.Cipher(t.Context(), orgID)
	if err != nil {
		t.Fatal(err)
	}
	seal := func(s string) string {
		out, err := c.Encrypt(t.Context(), s)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	goodSubject, goodBody := seal("Hello Ann"), seal("Body for Ann")
	const garbage = "c2VjcmV0LWJ1dC1ub3QtY2lwaGVydGV4dA=="

	cases := map[string]struct {
		keys          cipher.CipherService
		subject, body string
		html          string
		reason        string
	}{
		"subject undecryptable": {keys, garbage, goodBody, "", sendReasonUndecryptable},
		"plain undecryptable":   {keys, goodSubject, garbage, "", sendReasonUndecryptable},
		"html undecryptable":    {keys, goodSubject, goodBody, garbage, sendReasonUndecryptable},
		"short ciphertext":      {keys, goodSubject, "AAAA", "", sendReasonUndecryptable},
		"key unavailable":       {failingCipherService{}, goodSubject, goodBody, "", sendReasonKeyUnavailable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := &decryptSendGuard{}
			ctx := t.Context()
			bus := &filingBus{}
			w := newLoadingWorker(&capturedEvents{})
			w.ID, w.Bus, w.Codec, w.CipherService = uuid.NewString(), bus, codec.NewJSON(), tc.keys
			id := uuid.New()
			w.mailManager.Emails[id] = &wmail.WMail{OrgID: &orgID, Broker: client}
			blob := &emsg.EmailBlob{PlainText: []byte(tc.body), HTMLBody: []byte(tc.html)}
			data, err := blob.EncodeBinary()
			if err != nil {
				t.Fatal(err)
			}
			w.Storage = decryptBodyStore{data: data}
			command := models.SendEmail{TaskID: uuid.New(), EmailID: id, OrgID: orgID, To: []string{"recipient@example.test"}, Subject: tc.subject, BodyS3Key: "fixture"}
			if err := w.HandleSendEmail(ctx, command); err != nil {
				t.Fatalf("HandleSendEmail: %v", err)
			}
			if client.calls != 0 {
				t.Fatal("a message that could not be opened reached the provider")
			}
			if len(bus.events) != 1 || bus.events[0].Type != models.JobEventTypeEmailFailed {
				t.Fatalf("events = %+v, want one EMAIL_FAILED", bus.events)
			}
			raw, err := json.Marshal(bus.events[0].Body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), tc.reason) || strings.Contains(string(raw), garbage) || strings.Contains(string(raw), "key store") {
				t.Fatalf("failure = %s, want only %q", raw, tc.reason)
			}
		})
	}
}

type decryptSendGuard struct{ calls int }

func (f *decryptSendGuard) Call(context.Context, string, any, any, ...pramaan.Option) error {
	f.calls++
	return errors.New("unexpected capability call")
}

type decryptBodyStore struct {
	storage.Store
	data []byte
}

func (s decryptBodyStore) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}
