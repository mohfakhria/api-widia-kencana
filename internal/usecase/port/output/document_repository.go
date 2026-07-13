package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type DocumentRepository interface {
	ListPapers(ctx context.Context) ([]entity.DocumentPaper, error)
	ListElements(ctx context.Context) ([]entity.DocumentElement, error)
	ListProperties(ctx context.Context) ([]entity.DocumentProperty, error)
	ListPropertyOptions(ctx context.Context) ([]entity.DocumentPropertyOption, error)
	ListElementProperties(ctx context.Context) ([]entity.DocumentElementProperty, error)
	List(ctx context.Context, query input.ListDocumentQuery) ([]entity.Document, error)
	GetByToken(ctx context.Context, token string) (*entity.Document, error)
	Create(ctx context.Context, document *entity.Document) (*entity.Document, error)
	Update(ctx context.Context, token string, document *entity.Document) error
	Delete(ctx context.Context, token string) error
}
