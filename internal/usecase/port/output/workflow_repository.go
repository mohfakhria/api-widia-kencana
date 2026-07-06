package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type WorkflowRepository interface {
	List(ctx context.Context) ([]entity.Workflow, error)
	GetByID(ctx context.Context, id int64) (*entity.Workflow, error)
	Create(ctx context.Context, workflow *entity.Workflow) (*entity.Workflow, error)
	Update(ctx context.Context, id int64, workflow *entity.Workflow) error
	Delete(ctx context.Context, id int64) error
}
