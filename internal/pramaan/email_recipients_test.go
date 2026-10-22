package pramaan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReservedRecipientsNeverReachTransport(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(`{}`)) }))
	defer s.Close()
	c, _ := New("fake-key", s.URL, nil)
	for _, domain := range []string{"example.test", "example.com", "example.net", "example.org", "sub.example.com", "a.invalid", "a.localhost", "localhost", "EXAMPLE.COM", "example.com.", "something.example", "device.local"} {
		for _, id := range []string{"email.send", "email.message.send", "email.message.reply", "email.send-bulk"} {
			for _, field := range []string{"to", "cc", "bcc", "replyTo"} {
				t.Run(id+"/"+field+"/"+domain, func(t *testing.T) {
					in := map[string]any{field: "Recipient <test@" + domain + ">"}
					if id == "email.send-bulk" {
						in = map[string]any{"rows": []any{in}}
					}
					err := c.Call(context.Background(), id, in, nil, Idempotency("guard-attempt"))
					if err == nil {
						t.Fatalf("Expected reserved refusal, got %v", err)
					}
				})
			}
		}
	}
	if calls != 0 {
		t.Fatalf("Reserved input caused %d external calls", calls)
	}
}
func TestRecipientDomainMatchingIsExact(t *testing.T) {
	for _, address := range []string{"person@pramaan.dev", "person@example.com.pramaan.dev"} {
		if err := ValidateMailRecipients([]string{address}); err != nil {
			t.Fatal(err)
		}
	}
}
