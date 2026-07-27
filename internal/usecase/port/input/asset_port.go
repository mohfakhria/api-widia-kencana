package input

import (
	"context"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type AssetUseCase interface {
	RequestUpload(ctx context.Context, cmd RequestAssetUploadCommand) (*AssetUploadRequestResult, error)
	CompleteUpload(ctx context.Context, token string, uploadedBy *int64) (*entity.Asset, error)
	List(ctx context.Context, query ListAssetQuery) ([]entity.Asset, error)
	GetByToken(ctx context.Context, token string, uploadedBy *int64) (*entity.Asset, error)
	PresignGet(ctx context.Context, token string, uploadedBy *int64) (*AssetPresignGetResult, error)
	Delete(ctx context.Context, token string, uploadedBy *int64) error
}

type RequestAssetUploadCommand struct {
	OriginalFilename string
	MimeType         string
	Size             int64
	Scope            string
	UploadedBy       *int64
}

type AssetUploadRequestResult struct {
	Asset     *entity.Asset
	UploadURL string
	ExpiresAt time.Time
}

type ListAssetQuery struct {
	Status     string
	Scope      string
	MimeType   string
	Extension  string
	UploadedBy *int64
}

type AssetPresignGetResult struct {
	Asset     *entity.Asset
	URL       string
	ExpiresIn int64
}
