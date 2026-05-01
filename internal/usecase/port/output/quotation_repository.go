package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type QuotationRepository interface {
	GenerateQuotationNo(ctx context.Context) (string, error)
	Create(ctx context.Context, quotation *entity.Quotation) (string, error)
	List(ctx context.Context) ([]entity.Quotation, error)
	GetByID(ctx context.Context, id string) (*entity.Quotation, error)
	Update(ctx context.Context, id string, quotation *entity.Quotation) error
}
