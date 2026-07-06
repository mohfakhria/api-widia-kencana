package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type ProjectRepository interface {
	List(ctx context.Context) ([]entity.Project, error)
	GetByID(ctx context.Context, id int64) (*entity.Project, error)
	Create(ctx context.Context, project *entity.Project) (*entity.Project, error)
	Update(ctx context.Context, id int64, project *entity.Project) error
	Delete(ctx context.Context, id int64) error
}
