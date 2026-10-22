package socialauth

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type identityFixture struct {
	reply string
	id    string
	input map[string]any
}

func (f *identityFixture) Call(_ context.Context, id string, in, out any, _ ...pramaan.Option) error {
	f.id = id
	f.input = in.(map[string]any)
	return json.Unmarshal([]byte(f.reply), out)
}
func TestSocialCodeUsesBrokerAndRejectsUnverifiedIdentity(t *testing.T) {
	for _, verified := range []bool{true, false} {
		f := &identityFixture{reply: `{"providerKey":"fixture","subjectId":"subject","email":"owner@example.test","emailVerified":` + map[bool]string{true: "true", false: "false"}[verified] + `}`}
		flow := Flow{Client: f, ProviderKey: "fixture", Redirect: "https://app.example.test/callback"}
		claims, err := flow.Exchange(context.Background(), "single-use-code", "", "nonce")
		if _, present := f.input["state"]; present {
			t.Fatal("enterprise-only state sent without a connection")
		}
		if _, present := f.input["connectionRef"]; present {
			t.Fatal("social flow sent an enterprise connection")
		}
		if f.id != "auth.social.exchange" || f.input["expectedNonce"] != "nonce" {
			t.Fatal("exchange did not use broker")
		}
		if verified && (err != nil || claims.Subject != "subject") {
			t.Fatal(err)
		}
		if !verified && err == nil {
			t.Fatal("unverified email accepted")
		}
	}
}
func TestSocialAuthorizationBindsStateNonceAndProof(t *testing.T) {
	f := Flow{AuthorizationURL: "https://signin.example.test/authorize", ClientID: "owned-client", Redirect: "https://app.example.test/callback"}
	u, err := url.Parse(f.AuthCodeURL("state", "nonce", "verifier"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("state") != "state" || q.Get("nonce") != "nonce" || q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Fatal("request is not bound to its initiating session")
	}
}
