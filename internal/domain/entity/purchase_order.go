package entity

type PurchaseOrder struct {
	ID          int64
	QuotationID int64
	Items       []PurchaseOrderDetail
}

type PurchaseOrderDetail struct {
	ID              int64
	PurchaseOrderID int64
	Name            string
	Qty             float64
	Unit            string
	Price           float64
	Total           float64
}
