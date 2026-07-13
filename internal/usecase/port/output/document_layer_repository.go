package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type DocumentLayerRepository interface {
	Create(ctx context.Context, layer *entity.DocumentLayer) (*entity.DocumentLayer, error)
	Update(ctx context.Context, token string, layer *entity.DocumentLayer) error
	Sort(ctx context.Context, documentToken string, parentToken string, region string, layers []entity.DocumentLayer) error
	Delete(ctx context.Context, token string) error
}
