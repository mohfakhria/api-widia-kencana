package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type WorkflowStageRepository interface {
	ListByWorkflowID(ctx context.Context, workflowID int64) ([]entity.WorkflowStage, error)
	GetByID(ctx context.Context, id int64) (*entity.WorkflowStage, error)
	Create(ctx context.Context, stage *entity.WorkflowStage) (*entity.WorkflowStage, error)
	Update(ctx context.Context, id int64, stage *entity.WorkflowStage) error
	Sort(ctx context.Context, workflowID int64, items []entity.WorkflowStage) error
	Delete(ctx context.Context, id int64) error
}
