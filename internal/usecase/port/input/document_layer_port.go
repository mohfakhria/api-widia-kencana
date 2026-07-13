package input

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type DocumentLayerUseCase interface {
	Create(ctx context.Context, cmd CreateDocumentLayerCommand) (*entity.DocumentLayer, error)
	Update(ctx context.Context, token string, cmd UpdateDocumentLayerCommand) error
	Sort(ctx context.Context, cmd SortDocumentLayerCommand) error
	Delete(ctx context.Context, token string) error
}

type CreateDocumentLayerCommand struct {
	DocumentToken string
	ParentToken   string
	ElementToken  string
	Region        string
	Name          string
	Content       map[string]any
	Properties    map[string]any
	Position      int
	Status        string
}

type UpdateDocumentLayerCommand = CreateDocumentLayerCommand

type SortDocumentLayerCommand struct {
	DocumentToken string
	ParentToken   string
	Region        string
	Layers        []SortDocumentLayerItemCommand
}

type SortDocumentLayerItemCommand struct {
	Token    string
	Position int
}
