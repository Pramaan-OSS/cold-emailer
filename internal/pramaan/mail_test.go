package pramaan

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeCaller struct {
	ids     []string
	inputs  []map[string]any
	blocked bool
	fail    bool
}

func (f *fakeCaller) Call(ctx context.Context, id string, in, out any, options ...Option) error {
	f.ids = append(f.ids, id)
	b, _ := json.Marshal(in)
	var m map[string]any
	json.Unmarshal(b, &m)
	f.inputs = append(f.inputs, m)
	if f.fail {
		return errors.New("failed")
	}
	if out != nil {
		r := map[string]any{"blocked": f.blocked}
		if id != "messaging.opt-out.check" {
			r = map[string]any{"ref": "sent-1", "messageId": "<sent@example.test>", "status": "sent"}
		}
		b, _ = json.Marshal(r)
		return json.Unmarshal(b, out)
	}
	return nil
}
func TestReplyUsesBrokerAndAutomaticMarker(t *testing.T) {
	f := &fakeCaller{}
	s := MailService{Client: f}
	r, e := s.Send(context.Background(), MailRequest{From: "a@example.test", To: []string{"b@example.test"}, Subject: "Re: test", Text: "Simulated diagnostic.", InReplyTo: "<parent@example.test>", AutoSubmitted: "auto-replied"}, "task-1")
	if e != nil {
		t.Fatal(e)
	}
	if r.Ref != "sent-1" || f.ids[1] != "email.message.reply" || f.inputs[1]["autoSubmitted"] != "auto-replied" || f.inputs[1]["messageId"] != "<parent@example.test>" {
		t.Fatalf("%+v %+v", r, f)
	}
}
func TestOptOutAndCheckFailurePreventSend(t *testing.T) {
	for _, f := range []*fakeCaller{{blocked: true}, {fail: true}} {
		_, e := (MailService{Client: f}).Send(context.Background(), MailRequest{From: "a@example.test", To: []string{"b@example.test"}, Text: "test"}, "task")
		if e == nil || len(f.ids) != 1 {
			t.Fatalf("sent without opt-out proof: %v", f.ids)
		}
	}
}

type mailContractCaller struct {
	calls      int
	receipt    bool
	permission bool
	key        string
}

func (f *mailContractCaller) Call(_ context.Context, id string, _ any, out any, opts ...Option) error {
	f.calls++
	o := callOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	f.key = o.key
	payload := `{}`
	if id == "messaging.opt-out.check" && f.permission {
		payload = `{"blocked":false}`
	}
	if id == "email.message.send" && f.receipt {
		payload = `{"status":"sent","messageId":"<fake@example.test>"}`
	}
	return json.Unmarshal([]byte(payload), out)
}
func TestSendRequiresPermissionAndAcceptanceReceipt(t *testing.T) {
	request := MailRequest{From: "sender@example.test", To: []string{"recipient@example.test"}, Text: "Synthetic message"}
	f := &mailContractCaller{}
	if _, err := (MailService{Client: f}).Send(context.Background(), request, "stable-task"); err == nil || f.calls != 1 {
		t.Fatal("missing permission allowed a send")
	}
	f = &mailContractCaller{permission: true}
	if _, err := (MailService{Client: f}).Send(context.Background(), request, "stable-task"); !errors.Is(err, ErrSendUncertain) {
		t.Fatal("missing receipt was treated as acceptance")
	}
	f = &mailContractCaller{permission: true, receipt: true}
	if _, err := (MailService{Client: f}).Send(context.Background(), request, "stable-task"); err != nil || f.key != "stable-task" {
		t.Fatal("receipt or stable action key was lost")
	}
}
