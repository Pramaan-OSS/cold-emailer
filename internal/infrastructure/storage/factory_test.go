package storage

import (
	"context"
	"testing"
)

func TestFactoryRefusesDirectObjectStoreConfigurations(t *testing.T) {
	for _, provider := range []string{"s3", "object-store", "https://storage.example.test", "unknown"} {
		t.Run(provider, func(t *testing.T) {
			t.Setenv("BLOB_PROVIDER", provider)
			store, err := NewFromEnv(context.Background(), nil, "unused")
			if store != nil || err == nil {
				t.Fatal("direct backend accepted", provider)
			}
		})
	}
}
func TestFactoryOnlySelectsOwnedFilesOrPramaan(t *testing.T) {
	t.Setenv("BLOB_FS_ROOT", t.TempDir())
	t.Setenv("PRAMAAN_API_KEY", "fake-client-only")
	t.Setenv("PRAMAAN_STORAGE_BUCKET", "synthetic-bucket")
	for _, provider := range []string{"", "filesystem", "fs", "pramaan", "brokered"} {
		t.Setenv("BLOB_PROVIDER", provider)
		store, err := NewFromEnv(context.Background(), nil, "")
		if err != nil {
			t.Fatal(provider, err)
		}
		if provider == "pramaan" || provider == "brokered" {
			if store.Name() != "pramaan" {
				t.Fatal("not brokered", store.Name())
			}
		} else {
			if _, ok := store.(*FilesystemStore); !ok {
				t.Fatal("not local", provider)
			}
		}
	}
}
