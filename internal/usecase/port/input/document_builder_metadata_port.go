package input

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type DocumentBuilderMetadataUseCase interface {
	Get(ctx context.Context) (*entity.DocumentBuilderMetadata, error)
}
