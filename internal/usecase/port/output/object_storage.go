package output

import (
	"context"
	"io"
	"time"
)

type ObjectStorage interface {
	Upload(ctx context.Context, object UploadObject) (*StoredObject, error)
	Delete(ctx context.Context, objectName string) error
	PresignGet(ctx context.Context, objectName string, expiry time.Duration) (string, error)
}

type UploadObject struct {
	ObjectName  string
	Reader      io.Reader
	Size        int64
	ContentType string
}

type StoredObject struct {
	Bucket      string
	ObjectName  string
	ETag        string
	Size        int64
	ContentType string
}
