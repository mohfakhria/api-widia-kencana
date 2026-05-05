package input

import "context"

type PurchaseOrderUseCase interface {
	Upsert(ctx context.Context, cmd UpsertPurchaseOrderCommand) error
	GetByQuotationID(ctx context.Context, quotationID int64) ([]PurchaseOrderItemResult, error)
}

type UpsertPurchaseOrderCommand struct {
	QuotationID int64
	Items       []PurchaseOrderItemCommand
}

type PurchaseOrderItemCommand struct {
	Name  string
	Qty   float64
	Unit  string
	Price float64
}

type PurchaseOrderItemResult struct {
	ID    int64
	Name  string
	Qty   float64
	Unit  string
	Price float64
	Total float64
}
