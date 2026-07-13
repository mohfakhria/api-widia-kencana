package input

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type DocumentUseCase interface {
	GetMetadata(ctx context.Context) (*entity.DocumentMetadata, error)
	List(ctx context.Context) ([]entity.Document, error)
	GetByToken(ctx context.Context, token string) (*entity.Document, error)
	Create(ctx context.Context, cmd CreateDocumentCommand) (*entity.Document, error)
	Update(ctx context.Context, token string, cmd UpdateDocumentCommand) error
	Delete(ctx context.Context, token string) error
}

type CreateDocumentCommand struct {
	DocumentPaperToken string
	ParentToken        string
	Name               string
	DocumentType       string
	Position           int
	Status             string
}

type UpdateDocumentCommand = CreateDocumentCommand
