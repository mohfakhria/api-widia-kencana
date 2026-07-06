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
	Delete(ctx context.Context, id string) error
}

type CreateWorkflowStageCommand struct {
	WorkflowID int64
	Name       string
	Position   int
	Status     string
}

type UpdateWorkflowStageCommand = CreateWorkflowStageCommand
