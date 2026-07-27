package output

import (
	"context"
	"io"
	"time"
)

type ObjectStorage interface {
	Bucket() string
	Upload(ctx context.Context, object UploadObject) (*StoredObject, error)
	Delete(ctx context.Context, objectName string) error
	PresignGet(ctx context.Context, objectName string, expiry time.Duration) (string, error)
	PresignPut(ctx context.Context, objectName string, expiry time.Duration, contentType string) (string, error)
	Stat(ctx context.Context, objectName string) (*StoredObject, error)
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
