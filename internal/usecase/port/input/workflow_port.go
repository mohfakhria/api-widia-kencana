package input

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type WorkflowUseCase interface {
	List(ctx context.Context) ([]entity.Workflow, error)
	GetByID(ctx context.Context, id string) (*entity.Workflow, error)
	Create(ctx context.Context, cmd CreateWorkflowCommand) (*entity.Workflow, error)
	Update(ctx context.Context, id string, cmd UpdateWorkflowCommand) error
	Delete(ctx context.Context, id string) error
}

type CreateWorkflowCommand struct {
	Name   string
	Status string
}

type UpdateWorkflowCommand = CreateWorkflowCommand
