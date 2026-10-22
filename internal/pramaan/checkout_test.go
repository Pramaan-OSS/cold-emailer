package pramaan

import (
	"context"
	"errors"
	"testing"
)

func TestCheckoutConfirmationRequiresCapturedPaymentAndOwnership(t *testing.T) {
	for _, scenario := range []string{"paid", "completed", "unknown", "authorised", "missing_ref", "other_workspace"} {
		t.Run(scenario, func(t *testing.T) {
			f := &responseCaller{response: map[string]any{"status": "paid", "payments": []any{map[string]any{"status": "captured", "ref": "payment_reference"}}}}
			if scenario == "completed" || scenario == "unknown" {
				f.response = map[string]any{"status": scenario, "payments": []any{}}
			}
			if scenario == "authorised" {
				f.response = map[string]any{"status": "paid", "payments": []any{map[string]any{"status": "authorised", "ref": "payment_reference"}}}
			}
			if scenario == "missing_ref" {
				f.response = map[string]any{"status": "paid", "payments": []any{map[string]any{"status": "captured"}}}
			}
			g := &Gateway{Client: f, Registry: memoryRegistry{"one": {"checkout:checkout_reference": true}}}
			owner := "one"
			if scenario == "other_workspace" {
				owner = "two"
			}
			out, e := g.CheckoutStatus(context.Background(), owner, "checkout_reference")
			if scenario == "other_workspace" {
				if e == nil || f.calls != 0 {
					t.Fatal("unowned checkout read")
				}
				return
			}
			if e != nil || out["confirmed"] != (scenario == "paid") || f.calls != 1 || f.input["ref"] != "checkout_reference" {
				t.Fatalf("wrong confirmation: %v %v", out, e)
			}
		})
	}
}

func TestCheckoutStatusRefusalNeverConfirmsOrRetries(t *testing.T) {
	for _, failure := range []error{ErrUnavailable, &HTTPError{Status: 502, Code: "upstream_timeout", Effect: "unknown"}} {
		f := &managedFake{fail: failure}
		g := &Gateway{Client: f, Registry: memoryRegistry{"one": {"checkout:checkout_reference": true}}}
		out, e := g.CheckoutStatus(context.Background(), "one", "checkout_reference")
		if out != nil || !errors.Is(e, failure) || len(f.ids) != 1 || f.ids[0] != "payments.checkout.get" {
			t.Fatalf("unexpected status recovery: %v %v %v", out, e, f.ids)
		}
	}
}
