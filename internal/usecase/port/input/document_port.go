package input

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type DocumentUseCase interface {
	ListPapers(ctx context.Context) ([]entity.DocumentPaper, error)
	ListElements(ctx context.Context) ([]entity.DocumentElement, error)
	ListProperties(ctx context.Context) ([]entity.DocumentProperty, error)
	ListPropertyOptions(ctx context.Context) ([]entity.DocumentPropertyOption, error)
	ListElementProperties(ctx context.Context) ([]entity.DocumentElementProperty, error)
	List(ctx context.Context, query ListDocumentQuery) ([]entity.Document, error)
	GetByToken(ctx context.Context, token string) (*entity.Document, error)
	Create(ctx context.Context, cmd CreateDocumentCommand) (*entity.Document, error)
	Update(ctx context.Context, token string, cmd UpdateDocumentCommand) error
	Delete(ctx context.Context, token string) error
}

type ListDocumentQuery struct {
	Name  string
	Token string
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
