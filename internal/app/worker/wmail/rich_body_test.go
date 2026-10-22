package wmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/pramaan-oss/cold-email/internal/models"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type rawFixtureCaller struct {
	content string
	t       *testing.T
}

func (f rawFixtureCaller) Call(_ context.Context, id string, in, out any, _ ...pramaan.Option) error {
	if id != "email.message.raw.get" {
		f.t.Fatal("unexpected capability")
	}
	b, _ := json.Marshal(map[string]any{"content": base64.StdEncoding.EncodeToString([]byte(f.content))})
	return json.Unmarshal(b, out)
}
func TestBrokerRawMessagePreservesRichBody(t *testing.T) {
	raw := "From: sender@example.test\r\nTo: receiver@example.test\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=example\r\n\r\n--example\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nHello\r\n--example\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<p>Hello</p>\r\n--example--\r\n"
	w := WMail{Email: "receiver@example.test", Broker: rawFixtureCaller{raw, t}}
	m := models.EmailMessageData{MessageID: "<fake@example.test>", BodyPlain: "Normalized"}
	w.readRichBody(context.Background(), &m)
	if m.BodyHTML != "<p>Hello</p>" || m.BodyPlain != "Hello" {
		t.Fatalf("body lost: %q %q", m.BodyPlain, m.BodyHTML)
	}
}
