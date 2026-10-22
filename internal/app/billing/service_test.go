package billing

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type checkoutFake struct {
	ids      []string
	price    map[string]any
	response map[string]any
	fail     error
}

func (f *checkoutFake) Call(_ context.Context, id string, input, output any, opts ...pramaan.Option) error {
	f.ids = append(f.ids, id)
	if len(opts) == 0 {
		panic("action reference required")
	}
	var value any
	switch id {
	case "billing.price.get":
		value = f.price
	case "payments.checkout.create":
		if f.fail != nil {
			return f.fail
		}
		in := input.(map[string]any)
		if in["mode"] != "subscription" {
			panic("wrong checkout mode")
		}
		items := in["lineItems"].([]map[string]any)
		if len(items) != 1 || items[0]["quantity"] != 1 || items[0]["interval"] != "month" {
			panic("wrong checkout items")
		}
		value = f.response
	default:
		panic("unexpected capability: " + id)
	}
	b, e := json.Marshal(value)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, output)
}
func validCheckoutFake() *checkoutFake {
	return &checkoutFake{
		price:    map[string]any{"unitPrice": float64(2000), "currency": "usd", "renewalIntervalUnit": "month", "renewalIntervalCount": float64(1), "category": "subscription", "scheme": "standard"},
		response: map[string]any{"ref": "checkout_reference", "checkoutUrl": "https://checkout.example.test/pay"},
	}
}
func TestCheckoutUsesCapabilityResponse(t *testing.T) {
	f := validCheckoutFake()
	s := &billingService{client: f}
	out, e := s.CreateCheckoutSession(context.Background(), uuid.New(), uuid.New(), "price_reference", "https://app.example.test/return", "https://app.example.test/cancel", "")
	if e != nil || out.ID != "checkout_reference" || out.URL != "https://checkout.example.test/pay" {
		t.Fatalf("unexpected checkout: %+v %v", out, e)
	}
	if len(f.ids) != 2 || f.ids[0] != "billing.price.get" || f.ids[1] != "payments.checkout.create" {
		t.Fatal(f.ids)
	}
}
func TestCheckoutRefusalAndUncertaintyNeverRetry(t *testing.T) {
	for _, scenario := range []string{"unavailable", "unknown", "missing_reference", "invalid_url", "old_response"} {
		t.Run(scenario, func(t *testing.T) {
			f := validCheckoutFake()
			switch scenario {
			case "unavailable":
				f.fail = pramaan.ErrUnavailable
			case "unknown":
				f.fail = &pramaan.HTTPError{Status: 502, Code: "upstream_timeout", Effect: "unknown"}
			case "missing_reference":
				delete(f.response, "ref")
			case "invalid_url":
				f.response["checkoutUrl"] = "javascript:alert(1)"
			case "old_response":
				f.response = map[string]any{"id": "old", "url": "https://checkout.example.test/pay"}
			}
			out, e := (&billingService{client: f}).CreateCheckoutSession(context.Background(), uuid.New(), uuid.New(), "price_reference", "https://app.example.test/return", "https://app.example.test/cancel", "")
			if e == nil || out != nil || len(f.ids) != 2 {
				t.Fatalf("must stop once: %+v %v %v", out, e, f.ids)
			}
		})
	}
}
func TestCheckoutRefusesUnsupportedPriceWithoutCreating(t *testing.T) {
	for _, field := range []string{"renewalIntervalCount", "scheme", "unitPrice", "setupFeeEnabled", "trialIntervalUnit"} {
		t.Run(field, func(t *testing.T) {
			f := validCheckoutFake()
			switch field {
			case "renewalIntervalCount":
				f.price[field] = float64(12)
			case "scheme":
				f.price[field] = "volume"
			case "unitPrice":
				f.price[field] = float64(1.5)
			case "setupFeeEnabled":
				f.price[field] = true
			case "trialIntervalUnit":
				f.price[field] = "day"
			}
			out, e := (&billingService{client: f}).CreateCheckoutSession(context.Background(), uuid.New(), uuid.New(), "price_reference", "https://app.example.test/return", "https://app.example.test/cancel", "")
			if e == nil || out != nil || len(f.ids) != 1 {
				t.Fatalf("price must be refused before creating: %+v %v %v", out, e, f.ids)
			}
		})
	}
}
