package input

import (
	"context"
	"io"
)

type PurchaseOrderUseCase interface {
	Upsert(ctx context.Context, cmd UpsertPurchaseOrderCommand) error
	GetByQuotationID(ctx context.Context, quotationID int64) ([]PurchaseOrderItemResult, error)
}

type UpsertPurchaseOrderCommand struct {
	QuotationID int64
	Items       []PurchaseOrderItemCommand
	Asset       *PurchaseOrderAssetUploadCommand
}

type PurchaseOrderAssetUploadCommand struct {
	Reader           io.Reader
	Size             int64
	OriginalFilename string
	ContentType      string
	Category         string
	UploadedBy       *int64
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
