package minio

import (
	"context"
	"errors"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	miniosdk "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	client *miniosdk.Client
	bucket string
}

func NewStorage(ctx context.Context, cfg config.Config) (output.ObjectStorage, error) {
	if cfg.MinIORootUser == "" || cfg.MinIORootPassword == "" {
		return nil, errors.New("minio root credentials are required")
	}

	client, err := miniosdk.New(cfg.MinIOEndpoint, &miniosdk.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIORootUser, cfg.MinIORootPassword, ""),
		Secure: cfg.MinIOUseSSL,
	})
	if err != nil {
		return nil, err
	}

	storage := &Storage{
		client: client,
		bucket: cfg.MinIOBucket,
	}
	if err := storage.ensureBucket(ctx); err != nil {
		return nil, err
	}

	return storage, nil
}

func (s *Storage) Upload(ctx context.Context, object output.UploadObject) (*output.StoredObject, error) {
	info, err := s.client.PutObject(ctx, s.bucket, object.ObjectName, object.Reader, object.Size, miniosdk.PutObjectOptions{
		ContentType: object.ContentType,
	})
	if err != nil {
		return nil, err
	}

	return &output.StoredObject{
		Bucket:      s.bucket,
		ObjectName:  object.ObjectName,
		ETag:        info.ETag,
		Size:        info.Size,
		ContentType: object.ContentType,
	}, nil
}

func (s *Storage) Delete(ctx context.Context, objectName string) error {
	return s.client.RemoveObject(ctx, s.bucket, objectName, miniosdk.RemoveObjectOptions{})
}

func (s *Storage) PresignGet(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	url, err := s.client.PresignedGetObject(ctx, s.bucket, objectName, expiry, nil)
	if err != nil {
		return "", err
	}

	return url.String(), nil
}

func (s *Storage) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	return s.client.MakeBucket(ctx, s.bucket, miniosdk.MakeBucketOptions{})
}
