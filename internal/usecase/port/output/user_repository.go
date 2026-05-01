package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	FindByID(ctx context.Context, id string) (*entity.User, error)
}
