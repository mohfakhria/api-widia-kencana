package dto

import "github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"

type UpsertPurchaseOrderRequest struct {
	QuotationID int64                      `json:"id"`
	Items       []PurchaseOrderItemRequest `json:"items"`
}

type PurchaseOrderItemRequest struct {
	Name  string  `json:"name"`
	Qty   float64 `json:"qty"`
	Unit  string  `json:"unit"`
	Price float64 `json:"price"`
}

type PurchaseOrderResponse struct {
	QuotationID int64                       `json:"id"`
	Items       []PurchaseOrderItemResponse `json:"items"`
}

type PurchaseOrderItemResponse struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	Qty   float64 `json:"qty"`
	Unit  string  `json:"unit"`
	Price float64 `json:"price"`
	Total float64 `json:"total"`
}

func (r UpsertPurchaseOrderRequest) ToUpsertPurchaseOrderCommand() input.UpsertPurchaseOrderCommand {
	cmd := input.UpsertPurchaseOrderCommand{
		QuotationID: r.QuotationID,
		Items:       make([]input.PurchaseOrderItemCommand, 0, len(r.Items)),
	}

	for _, item := range r.Items {
		cmd.Items = append(cmd.Items, input.PurchaseOrderItemCommand{
			Name:  item.Name,
			Qty:   item.Qty,
			Unit:  item.Unit,
			Price: item.Price,
		})
	}

	return cmd
}

func NewPurchaseOrderResponse(quotationID int64, items []input.PurchaseOrderItemResult) PurchaseOrderResponse {
	response := PurchaseOrderResponse{
		QuotationID: quotationID,
		Items:       make([]PurchaseOrderItemResponse, 0, len(items)),
	}

	for _, item := range items {
		response.Items = append(response.Items, PurchaseOrderItemResponse{
			ID:    item.ID,
			Name:  item.Name,
			Qty:   item.Qty,
			Unit:  item.Unit,
			Price: item.Price,
			Total: item.Total,
		})
	}

	return response
}
