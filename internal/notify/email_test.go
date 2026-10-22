package notify

import (
	"context"
	"encoding/json"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"net/mail"
	"testing"
)

type notificationFake struct {
	inputs []map[string]any
	ids    []string
}

func (f *notificationFake) Call(ctx context.Context, id string, in, out any, opts ...pramaan.Option) error {
	b, _ := json.Marshal(in)
	var m map[string]any
	json.Unmarshal(b, &m)
	f.inputs = append(f.inputs, m)
	f.ids = append(f.ids, id)
	if id == "email.send" && len(opts) == 0 {
		panic("missing action reference")
	}
	if out != nil {
		b = []byte(`{"blocked":false}`)
		return json.Unmarshal(b, out)
	}
	return nil
}
func TestPlatformNotificationSenderAndOwnerReplyTo(t *testing.T) {
	f := &notificationFake{}
	s := NewEmailNotificationService(f, "Cold Email", "owner@pramaan.dev")
	if e := s.Send(context.Background(), []string{"recipient@pramaan.dev"}, nil, nil, "Test", "<p>Test</p>"); e != nil {
		t.Fatal(e)
	}
	in := f.inputs[0]
	from, e := mail.ParseAddress(in["from"].(string))
	if e != nil || from.Address != "noreply@"+DefaultSenderDomain || from.Name != "Cold Email" || in["from"] != "Cold Email <noreply@"+DefaultSenderDomain+">" || in["replyTo"] != "owner@pramaan.dev" {
		t.Fatalf("Unexpected envelope: %+v", in)
	}
	if e := s.SendOutreach(context.Background(), []string{"recipient@pramaan.dev"}, "support@pramaan.dev", "Test", "Test"); e != nil {
		t.Fatal(e)
	}
	if f.inputs[len(f.inputs)-1]["replyTo"] != "support@pramaan.dev" || f.ids[1] != "messaging.opt-out.check" {
		t.Fatal("Outreach reply and consent gates changed")
	}
}

func TestPlatformNotificationOmitsUnconfiguredReplyTo(t *testing.T) {
	f := &notificationFake{}
	s := NewEmailNotificationService(f, "Cold Email", "")
	if err := s.Send(context.Background(), []string{"recipient@pramaan.dev"}, nil, nil, "Test", "<p>Test</p>"); err != nil {
		t.Fatal(err)
	}
	if _, present := f.inputs[0]["replyTo"]; present {
		t.Fatal("Unconfigured owner address must not be sent as Reply-To")
	}
	if f.inputs[0]["from"] != "Cold Email <noreply@"+DefaultSenderDomain+">" {
		t.Fatal("Unexpected platform sender")
	}
}

func TestReservedNotificationMakesNoCapabilityCalls(t *testing.T) {
	for _, field := range []string{"to", "cc", "bcc", "replyTo", "outreach"} {
		t.Run(field, func(t *testing.T) {
			f := &notificationFake{}
			owner := ""
			if field == "replyTo" {
				owner = "owner@example.test"
			}
			s := NewEmailNotificationService(f, "Cold Email", owner)
			to := []string{"recipient@pramaan.dev"}
			var cc, bcc []string
			if field == "to" || field == "outreach" {
				to = []string{"test@example.test"}
			}
			if field == "cc" {
				cc = []string{"test@sub.example.com"}
			}
			if field == "bcc" {
				bcc = []string{"test@reserved.invalid"}
			}
			var err error
			if field == "outreach" {
				err = s.SendOutreach(context.Background(), to, "", "Test", "Test")
			} else {
				err = s.Send(context.Background(), to, cc, bcc, "Test", "Test")
			}
			if err == nil || len(f.ids) != 0 {
				t.Fatalf("Expected refusal before any capability call: %v, %v", err, f.ids)
			}
		})
	}
}

func TestTemplateSenderDomain(t *testing.T) {
	if DefaultSenderDomain != "updates.pramaan.one" {
		t.Fatal("Unexpected template sender domain")
	}
	f := &notificationFake{}
	s := NewEmailNotificationService(f, "Cold Email", "")
	if err := s.Send(context.Background(), []string{"recipient@pramaan.dev"}, nil, nil, "Test", "Test"); err != nil {
		t.Fatal(err)
	}
	from, err := mail.ParseAddress(f.inputs[0]["from"].(string))
	if err != nil || from.Address != "noreply@"+DefaultSenderDomain {
		t.Fatal("Sender did not use the template domain")
	}
	if _, exists := f.inputs[0]["replyTo"]; exists {
		t.Fatal("Unconfigured Reply-To must stay absent")
	}
}
