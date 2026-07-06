package input

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type ProjectUseCase interface {
	List(ctx context.Context) ([]entity.Project, error)
	GetByID(ctx context.Context, id string) (*entity.Project, error)
	Create(ctx context.Context, cmd CreateProjectCommand) (*entity.Project, error)
	Update(ctx context.Context, id string, cmd UpdateProjectCommand) error
	Delete(ctx context.Context, id string) error
}

type CreateProjectCommand struct {
	Name   string
	Status string
}

type UpdateProjectCommand = CreateProjectCommand
