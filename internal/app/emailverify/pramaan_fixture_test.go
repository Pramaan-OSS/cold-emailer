package emailverify

import (
	"context"
	"encoding/json"

	verify "github.com/pramaan-oss/cold-email/internal/pkg/emailverify"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type verificationFixture struct {
	status string
	fail   func() bool
}

func (f verificationFixture) Call(_ context.Context, id string, in, out any, _ ...pramaan.Option) error {
	if f.fail != nil && f.fail() {
		return verify.ErrProviderCredits
	}
	b, _ := json.Marshal(map[string]any{"email": in.(map[string]any)["email"], "status": f.status})
	return json.Unmarshal(b, out)
}
func fixtureVerifier(status string, fail func() bool) *verify.BrokerVerifier {
	return &verify.BrokerVerifier{Client: verificationFixture{status: status, fail: fail}}
}
