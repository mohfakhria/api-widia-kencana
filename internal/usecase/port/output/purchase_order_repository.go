package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type PurchaseOrderRepository interface {
	UpsertByQuotationID(ctx context.Context, quotationID int64, items []entity.PurchaseOrderItem) error
	GetByQuotationID(ctx context.Context, quotationID int64) ([]entity.PurchaseOrderItem, error)
}
