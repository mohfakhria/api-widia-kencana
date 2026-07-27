package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type DocumentRepository interface {
	List(ctx context.Context, query input.ListDocumentQuery) ([]entity.Document, error)
	GetByToken(ctx context.Context, token string) (*entity.Document, error)
	Create(ctx context.Context, document *entity.Document) (*entity.Document, error)
	Update(ctx context.Context, token string, document *entity.Document) error
	Delete(ctx context.Context, token string) error
}
