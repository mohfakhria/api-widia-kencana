package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type PurchaseOrderRepository struct {
	db *sql.DB
}

func NewPurchaseOrderRepository(db *sql.DB) output.PurchaseOrderRepository {
	return &PurchaseOrderRepository{db: db}
}

func (r *PurchaseOrderRepository) UpsertByQuotationID(ctx context.Context, quotationID int64, items []entity.PurchaseOrderItem) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM quotations WHERE id = $1)`, quotationID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return domain.NewError(domain.ErrNotFound, "quotation not found")
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, quotation_id, name, qty, unit, price, total
		FROM purchase_order
		WHERE quotation_id = $1
	`, quotationID)
	if err != nil {
		return err
	}
	defer rows.Close()

	existingByKey := make(map[string]entity.PurchaseOrderItem)
	for rows.Next() {
		var item entity.PurchaseOrderItem
		if err := rows.Scan(&item.ID, &item.QuotationID, &item.Name, &item.Qty, &item.Unit, &item.Price, &item.Total); err != nil {
			return err
		}
		existingByKey[purchaseOrderItemKey(item.Name, item.Unit, item.Price)] = item
	}
	if err := rows.Err(); err != nil {
		return err
	}

	incomingKeys := make(map[string]struct{}, len(items))
	for _, item := range items {
		key := purchaseOrderItemKey(item.Name, item.Unit, item.Price)
		incomingKeys[key] = struct{}{}

		if existing, ok := existingByKey[key]; ok {
			if _, err := tx.ExecContext(ctx, `
				UPDATE purchase_order
				SET qty = $1, price = $2, total = $3
				WHERE id = $4
			`, item.Qty, item.Price, item.Total, existing.ID); err != nil {
				return err
			}
			continue
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO purchase_order (quotation_id, name, qty, unit, price, total)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, quotationID, item.Name, item.Qty, item.Unit, item.Price, item.Total); err != nil {
			return err
		}
	}

	for key, item := range existingByKey {
		if _, keep := incomingKeys[key]; keep {
			continue
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM purchase_order WHERE id = $1`, item.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *PurchaseOrderRepository) GetByQuotationID(ctx context.Context, quotationID int64) ([]entity.PurchaseOrderItem, error) {
	exists, err := r.quotationExists(ctx, r.db, quotationID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.NewError(domain.ErrNotFound, "quotation not found")
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, quotation_id, name, qty, unit, price, total
		FROM purchase_order
		WHERE quotation_id = $1
		ORDER BY id ASC
	`, quotationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]entity.PurchaseOrderItem, 0)
	for rows.Next() {
		var item entity.PurchaseOrderItem
		if err := rows.Scan(&item.ID, &item.QuotationID, &item.Name, &item.Qty, &item.Unit, &item.Price, &item.Total); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *PurchaseOrderRepository) DeleteByQuotationID(ctx context.Context, quotationID int64) error {
	exists, err := r.quotationExists(ctx, r.db, quotationID)
	if err != nil {
		return err
	}
	if !exists {
		return domain.NewError(domain.ErrNotFound, "quotation not found")
	}

	_, err = r.db.ExecContext(ctx, `DELETE FROM purchase_order WHERE quotation_id = $1`, quotationID)
	return err
}

func (r *PurchaseOrderRepository) quotationExists(ctx context.Context, querier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, quotationID int64) (bool, error) {
	var exists bool
	if err := querier.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM quotations WHERE id = $1)`, quotationID).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func purchaseOrderItemKey(name, unit string, price float64) string {
	return fmt.Sprintf("%s|%s|%f", name, unit, price)
}
