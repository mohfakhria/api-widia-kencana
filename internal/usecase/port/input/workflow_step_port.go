package input

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type WorkflowStepUseCase interface {
	ListByWorkflowStageID(ctx context.Context, workflowStageID string) ([]entity.WorkflowStep, error)
	GetByID(ctx context.Context, id string) (*entity.WorkflowStep, error)
	Create(ctx context.Context, cmd CreateWorkflowStepCommand) (*entity.WorkflowStep, error)
	Update(ctx context.Context, id string, cmd UpdateWorkflowStepCommand) error
	Sort(ctx context.Context, cmd SortWorkflowStepCommand) error
	Delete(ctx context.Context, id string) error
}

type CreateWorkflowStepCommand struct {
	WorkflowStageID int64
	Name            string
	Position        int
	Status          string
}

type UpdateWorkflowStepCommand = CreateWorkflowStepCommand

type SortWorkflowStepCommand struct {
	WorkflowStageID int64
	Steps           []SortWorkflowStepItemCommand
}

type SortWorkflowStepItemCommand struct {
	ID       int64
	Position int
}
