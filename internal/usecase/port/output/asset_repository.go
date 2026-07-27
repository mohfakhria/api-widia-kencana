package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type AssetRepository interface {
	CreatePending(ctx context.Context, asset *entity.Asset) (*entity.Asset, error)
	GetByToken(ctx context.Context, token string) (*entity.Asset, error)
	List(ctx context.Context, query input.ListAssetQuery) ([]entity.Asset, error)
	MarkUploaded(ctx context.Context, token string, stored *StoredObject) (*entity.Asset, error)
	MarkDeleted(ctx context.Context, token string) error
	MarkFailed(ctx context.Context, token string, code string, message string) error
}
