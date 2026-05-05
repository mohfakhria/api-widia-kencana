package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type QuotationRepository interface {
	GenerateQuotationNo(ctx context.Context) (string, error)
	Create(ctx context.Context, quotation *entity.Quotation) (string, error)
	List(ctx context.Context, query input.ListQuotationQuery) ([]entity.Quotation, error)
	GetByID(ctx context.Context, id string) (*entity.Quotation, error)
	Update(ctx context.Context, id string, quotation *entity.Quotation) error
}
