package dto

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"

	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

var purchaseOrderItemFieldPattern = regexp.MustCompile(`^items\[(\d+)]\.(name|qty|unit|price)$`)

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

type PurchaseOrderDataResponse struct {
	PurchaseOrder PurchaseOrderResponse `json:"purchase_order"`
}

type PurchaseOrderItemResponse struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	Qty   float64 `json:"qty"`
	Unit  string  `json:"unit"`
	Price float64 `json:"price"`
	Total float64 `json:"total"`
}

func NewUpsertPurchaseOrderRequest(values url.Values) (UpsertPurchaseOrderRequest, error) {
	quotationID, err := strconv.ParseInt(values.Get("id"), 10, 64)
	if err != nil {
		return UpsertPurchaseOrderRequest{}, fmt.Errorf("invalid quotation id")
	}

	itemsByIndex := make(map[int]*PurchaseOrderItemRequest)
	for key, fieldValues := range values {
		matches := purchaseOrderItemFieldPattern.FindStringSubmatch(key)
		if len(matches) == 0 || len(fieldValues) == 0 {
			continue
		}

		index, _ := strconv.Atoi(matches[1])
		item := itemsByIndex[index]
		if item == nil {
			item = &PurchaseOrderItemRequest{}
			itemsByIndex[index] = item
		}

		if err := setPurchaseOrderItemField(item, matches[2], fieldValues[0]); err != nil {
			return UpsertPurchaseOrderRequest{}, fmt.Errorf("invalid %s", key)
		}
	}

	indexes := make([]int, 0, len(itemsByIndex))
	for index := range itemsByIndex {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	items := make([]PurchaseOrderItemRequest, 0, len(indexes))
	for _, index := range indexes {
		items = append(items, *itemsByIndex[index])
	}

	return UpsertPurchaseOrderRequest{QuotationID: quotationID, Items: items}, nil
}

func setPurchaseOrderItemField(item *PurchaseOrderItemRequest, field, value string) error {
	switch field {
	case "name":
		item.Name = value
	case "unit":
		item.Unit = value
	case "qty":
		qty, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		item.Qty = qty
	case "price":
		price, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		item.Price = price
	}

	return nil
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

func NewPurchaseOrderResponse(quotationID int64, items []input.PurchaseOrderItemResult) PurchaseOrderDataResponse {
	response := PurchaseOrderDataResponse{
		PurchaseOrder: PurchaseOrderResponse{
			QuotationID: quotationID,
			Items:       make([]PurchaseOrderItemResponse, 0, len(items)),
		},
	}

	for _, item := range items {
		response.PurchaseOrder.Items = append(response.PurchaseOrder.Items, PurchaseOrderItemResponse{
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
