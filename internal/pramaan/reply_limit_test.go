package pramaan

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlainCapabilityCutOffAndLimitRefusal(t *testing.T) {
	for _, id := range []string{"llm.chat", "llm.complete"} {
		for _, mode := range []string{"length", "limit"} {
			t.Run(id+mode, func(t *testing.T) {
				posts := 0
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					posts++
					if mode == "limit" {
						w.WriteHeader(400)
						_, _ = w.Write([]byte(`{"error":{"code":"invalid_request","message":"maxTokens exhausted before text","effect":"none"}}`))
						return
					}
					_, _ = w.Write([]byte(`{"text":"partial","finishReason":"length","usage":{"inputTokens":2,"outputTokens":3}}`))
				}))
				defer s.Close()
				c, _ := New("fixture", s.URL, nil)
				var out map[string]any
				e := c.Call(context.Background(), id, map[string]any{"messages": []any{map[string]any{"role": "user", "content": "Synthetic"}}, "maxTokens": 256}, &out)
				if e == nil || posts != 1 {
					t.Fatal(e, posts)
				}
				if mode == "limit" && !errors.Is(e, ErrReplyLimit) {
					t.Fatal(e)
				}
				var partial *IncompleteReply
				if mode == "length" && (!errors.As(e, &partial) || partial.Tokens != 5 || out["finishReason"] != "length") {
					t.Fatal(e, out)
				}
			})
		}
	}
}
