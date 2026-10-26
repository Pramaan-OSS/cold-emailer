package wmail

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/config"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
	"strings"
	"testing"
)

type headerFixture struct{ last map[string]any }

func (f *headerFixture) Call(_ context.Context, id string, in, out any, _ ...pramaan.Option) error {
	data := map[string]any{"blocked": false}
	if id == "email.message.send" {
		b, _ := json.Marshal(in)
		_ = json.Unmarshal(b, &f.last)
		data = map[string]any{"status": "sent", "ref": "synthetic", "messageId": "<synthetic@example.test>"}
	}
	b, _ := json.Marshal(data)
	return json.Unmarshal(b, out)
}
func TestWorkerPreservesBodyOptOutAndNonceHeaders(t *testing.T) {
	for _, warmup := range []bool{false, true} {
		f := &headerFixture{}
		w := WMail{Email: "sender@example.test", Broker: f}
		r := &SendRequest{TaskID: uuid.New(), To: []string{"recipient@example.test"}, BodyPlain: "Synthetic", IsWarmup: warmup, WarmupToken: "nonce"}
		if !warmup {
			r.UnsubscribeURL = "https://app.example.test/stop/signed"
		}
		result := w.Send(context.Background(), r)
		if !result.Success {
			t.Fatal(result.Error)
		}
		headers := f.last["headers"].(map[string]any)
		if warmup {
			if headers[config.WarmupVerifyHeader] != "nonce" {
				t.Fatal(headers)
			}
		} else if headers["List-Unsubscribe-Post"] != "List-Unsubscribe=One-Click" || !strings.Contains(f.last["text"].(string), r.UnsubscribeURL) {
			t.Fatal("lost header or body opt-out")
		}
	}
}
