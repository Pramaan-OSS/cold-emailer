package storage

import (
	"context"
	"net/http"
	"os"
)

type BrokeredStore = Client
type BrokeredOption func(*BrokeredStore)

func WithBrokeredHTTPClient(_ *http.Client) BrokeredOption { return func(*BrokeredStore) {} }
func NewBrokered(_ string, _ string, _ ...BrokeredOption) (*BrokeredStore, error) {
	return NewClient(context.Background(), nil, os.Getenv("PRAMAAN_STORAGE_BUCKET"))
}
