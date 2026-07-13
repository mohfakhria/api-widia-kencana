package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type DocumentBuilderMetadataRepository interface {
	ListPapers(ctx context.Context) ([]entity.DocumentPaper, error)
	ListElements(ctx context.Context) ([]entity.DocumentElement, error)
	ListProperties(ctx context.Context) ([]entity.DocumentProperty, error)
	ListPropertyOptions(ctx context.Context) ([]entity.DocumentPropertyOption, error)
	ListElementProperties(ctx context.Context) ([]entity.DocumentElementProperty, error)
}
