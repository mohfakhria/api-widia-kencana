package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type PurchaseOrderRepository interface {
	UpsertByQuotationID(ctx context.Context, purchaseOrder *entity.PurchaseOrder, attachment *entity.PurchaseOrderAsset) (*entity.PurchaseOrder, error)
	GetByQuotationID(ctx context.Context, quotationID int64) (*entity.PurchaseOrder, error)
}
