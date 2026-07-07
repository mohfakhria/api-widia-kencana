package input

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type WorkflowStageUseCase interface {
	ListByWorkflowID(ctx context.Context, workflowID string) ([]entity.WorkflowStage, error)
	GetByID(ctx context.Context, id string) (*entity.WorkflowStage, error)
	Create(ctx context.Context, cmd CreateWorkflowStageCommand) (*entity.WorkflowStage, error)
	Update(ctx context.Context, id string, cmd UpdateWorkflowStageCommand) error
	Sort(ctx context.Context, cmd SortWorkflowStageCommand) error
	Delete(ctx context.Context, id string) error
}

type CreateWorkflowStageCommand struct {
	WorkflowID int64
	Name       string
	Position   int
	Status     string
}

type UpdateWorkflowStageCommand = CreateWorkflowStageCommand

type SortWorkflowStageCommand struct {
	WorkflowID int64
	Stages     []SortWorkflowStageItemCommand
}

type SortWorkflowStageItemCommand struct {
	ID       int64
	Position int
}
