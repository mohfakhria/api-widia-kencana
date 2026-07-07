package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type WorkflowStepRepository interface {
	ListByWorkflowStageID(ctx context.Context, workflowStageID int64) ([]entity.WorkflowStep, error)
	GetByID(ctx context.Context, id int64) (*entity.WorkflowStep, error)
	Create(ctx context.Context, step *entity.WorkflowStep) (*entity.WorkflowStep, error)
	Update(ctx context.Context, id int64, step *entity.WorkflowStep) error
	Sort(ctx context.Context, workflowStageID int64, items []entity.WorkflowStep) error
	Delete(ctx context.Context, id int64) error
}
