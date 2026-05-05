package usecase

import (
	"context"
	"fmt"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type purchaseOrderUseCase struct {
	repo output.PurchaseOrderRepository
}

func NewPurchaseOrderUseCase(repo output.PurchaseOrderRepository) input.PurchaseOrderUseCase {
	return &purchaseOrderUseCase{repo: repo}
}

func (uc *purchaseOrderUseCase) Upsert(ctx context.Context, cmd input.UpsertPurchaseOrderCommand) error {
	if cmd.QuotationID <= 0 {
		return domain.NewError(domain.ErrInvalidInput, "quotation id must be greater than 0")
	}

	items := make([]entity.PurchaseOrderItem, 0, len(cmd.Items))
	seenKeys := make(map[string]struct{}, len(cmd.Items))
	for i, item := range cmd.Items {
		if item.Name == "" {
			return domain.NewError(domain.ErrInvalidInput, fmt.Sprintf("item %d: name cannot be empty", i+1))
		}
		if item.Qty <= 0 {
			return domain.NewError(domain.ErrInvalidInput, fmt.Sprintf("item %d: qty must be greater than 0", i+1))
		}
		if item.Price < 0 {
			return domain.NewError(domain.ErrInvalidInput, fmt.Sprintf("item %d: price cannot be negative", i+1))
		}
		key := purchaseOrderItemKey(item.Name, item.Unit, item.Price)
		if _, exists := seenKeys[key]; exists {
			return domain.NewError(domain.ErrInvalidInput, fmt.Sprintf("item %d: duplicate purchase order item", i+1))
		}
		seenKeys[key] = struct{}{}

		items = append(items, entity.PurchaseOrderItem{
			QuotationID: cmd.QuotationID,
			Name:        item.Name,
			Qty:         item.Qty,
			Unit:        item.Unit,
			Price:       item.Price,
			Total:       item.Qty * item.Price,
		})
	}

	return uc.repo.UpsertByQuotationID(ctx, cmd.QuotationID, items)
}

func (uc *purchaseOrderUseCase) GetByQuotationID(ctx context.Context, quotationID int64) ([]input.PurchaseOrderItemResult, error) {
	if quotationID <= 0 {
		return nil, domain.NewError(domain.ErrInvalidInput, "quotation id must be greater than 0")
	}

	items, err := uc.repo.GetByQuotationID(ctx, quotationID)
	if err != nil {
		return nil, err
	}

	results := make([]input.PurchaseOrderItemResult, 0, len(items))
	for _, item := range items {
		results = append(results, input.PurchaseOrderItemResult{
			ID:    item.ID,
			Name:  item.Name,
			Qty:   item.Qty,
			Unit:  item.Unit,
			Price: item.Price,
			Total: item.Total,
		})
	}

	return results, nil
}

func purchaseOrderItemKey(name, unit string, price float64) string {
	return fmt.Sprintf("%s|%s|%f", name, unit, price)
}
