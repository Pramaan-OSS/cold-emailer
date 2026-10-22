package storage

import (
	"context"
	"errors"
	"os"
)

func NewFromEnv(ctx context.Context, cfg any, defaultBucket string) (Store, error) {
	switch os.Getenv("BLOB_PROVIDER") {
	case "", "filesystem", "fs":
		root := os.Getenv("BLOB_FS_ROOT")
		if root == "" {
			root = ".data/blobs"
		}
		return NewFilesystem(root, os.Getenv("BLOB_PUBLIC_BASE_URL"))
	case "pramaan", "brokered":
		return NewClient(ctx, nil, os.Getenv("PRAMAAN_STORAGE_BUCKET"))
	default:
		return nil, errors.New("Use local files or connected Pramaan storage.")
	}
}
