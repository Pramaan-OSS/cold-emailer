package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type fakeCheck struct {
	calls  int
	passed bool
	err    error
	id     string
	input  any
}

func (f *fakeCheck) Call(_ context.Context, id string, input, out any, _ ...pramaan.Option) error {
	f.calls++
	f.id = id
	f.input = input
	if f.err != nil {
		return f.err
	}
	body, _ := json.Marshal(map[string]bool{"passed": f.passed})
	return json.Unmarshal(body, out)
}
func TestServerBotCheck(t *testing.T) {
	for _, tc := range []struct {
		name            string
		enabled, passed bool
		token           string
		failure         error
		wantErr         bool
		calls           int
	}{
		{name: "off by default"},
		{name: "requires a token", enabled: true, wantErr: true},
		{name: "accepts verified token", enabled: true, passed: true, token: "fixture-token", calls: 1},
		{name: "rejects refused token", enabled: true, token: "fixture-token", wantErr: true, calls: 1},
		{name: "rejects unavailable verification", enabled: true, token: "fixture-token", failure: errors.New("fixture unavailable"), wantErr: true, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeCheck{passed: tc.passed, err: tc.failure}
			b := NewBotCheck(Config{Enabled: tc.enabled, Client: f})
			err := b.Verify(context.Background(), tc.token, "127.0.0.1")
			if (err != nil) != tc.wantErr || f.calls != tc.calls {
				t.Fatalf("error=%v calls=%d", err, f.calls)
			}
			if f.calls > 0 && (f.id != "auth.bot-check.verify" || f.input.(map[string]any)["token"] != "fixture-token") {
				t.Fatal("wrong broker contract")
			}
		})
	}
}
