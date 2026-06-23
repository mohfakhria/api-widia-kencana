package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

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

func (r *PurchaseOrderRepository) UpsertByQuotationID(ctx context.Context, purchaseOrder *entity.PurchaseOrder, attachment *entity.PurchaseOrderAsset) (*entity.PurchaseOrder, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var quotationStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM quotations WHERE id = $1 FOR UPDATE`, purchaseOrder.QuotationID).Scan(&quotationStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewError(domain.ErrNotFound, "quotation not found")
		}
		return nil, err
	}

	headerID, err := r.findPurchaseOrderHeaderID(ctx, tx, purchaseOrder.QuotationID)
	if err != nil {
		return nil, err
	}

	if len(purchaseOrder.Items) == 0 {
		if headerID != 0 {
			if _, err := tx.ExecContext(ctx, `DELETE FROM purchase_order WHERE id = $1`, headerID); err != nil {
				return nil, err
			}
		}

		purchaseOrder.ID = 0
		return purchaseOrder, r.syncQuotationStatusAndCommit(ctx, tx, quotationStatus, purchaseOrder.QuotationID, false)
	}

	if headerID == 0 {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO purchase_order (quotation_id)
			VALUES ($1)
			RETURNING id
		`, purchaseOrder.QuotationID).Scan(&headerID)
		if err != nil {
			return nil, err
		}
	}
	purchaseOrder.ID = headerID

	rows, err := tx.QueryContext(ctx, `
		SELECT id, purchase_order_id, name, qty, unit, price, total
		FROM purchase_order_detail
		WHERE purchase_order_id = $1
	`, headerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	existingByKey := make(map[string]entity.PurchaseOrderDetail)
	for rows.Next() {
		var item entity.PurchaseOrderDetail
		if err := rows.Scan(&item.ID, &item.PurchaseOrderID, &item.Name, &item.Qty, &item.Unit, &item.Price, &item.Total); err != nil {
			return nil, err
		}
		existingByKey[purchaseOrderItemKey(item.Name, item.Unit, item.Price)] = item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	incomingKeys := make(map[string]struct{}, len(purchaseOrder.Items))
	for _, item := range purchaseOrder.Items {
		key := purchaseOrderItemKey(item.Name, item.Unit, item.Price)
		incomingKeys[key] = struct{}{}

		if existing, ok := existingByKey[key]; ok {
			if _, err := tx.ExecContext(ctx, `
				UPDATE purchase_order_detail
				SET qty = $1, price = $2, total = $3
				WHERE id = $4
			`, item.Qty, item.Price, item.Total, existing.ID); err != nil {
				return nil, err
			}
			continue
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO purchase_order_detail (purchase_order_id, name, qty, unit, price, total)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, headerID, item.Name, item.Qty, item.Unit, item.Price, item.Total); err != nil {
			return nil, err
		}
	}

	for key, item := range existingByKey {
		if _, keep := incomingKeys[key]; keep {
			continue
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM purchase_order_detail WHERE id = $1`, item.ID); err != nil {
			return nil, err
		}
	}

	if attachment != nil {
		if err := r.createAssetAttachment(ctx, tx, headerID, attachment); err != nil {
			return nil, err
		}
	}

	return purchaseOrder, r.syncQuotationStatusAndCommit(ctx, tx, quotationStatus, purchaseOrder.QuotationID, true)
}

func (r *PurchaseOrderRepository) GetByQuotationID(ctx context.Context, quotationID int64) (*entity.PurchaseOrder, error) {
	exists, err := r.quotationExists(ctx, r.db, quotationID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.NewError(domain.ErrNotFound, "quotation not found")
	}

	purchaseOrder := &entity.PurchaseOrder{
		QuotationID: quotationID,
		Items:       make([]entity.PurchaseOrderDetail, 0),
	}

	err = r.db.QueryRowContext(ctx, `
		SELECT id
		FROM purchase_order
		WHERE quotation_id = $1
	`, quotationID).Scan(&purchaseOrder.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return purchaseOrder, nil
		}
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, purchase_order_id, name, qty, unit, price, total
		FROM purchase_order_detail
		WHERE purchase_order_id = $1
		ORDER BY id ASC
	`, purchaseOrder.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item entity.PurchaseOrderDetail
		if err := rows.Scan(&item.ID, &item.PurchaseOrderID, &item.Name, &item.Qty, &item.Unit, &item.Price, &item.Total); err != nil {
			return nil, err
		}
		purchaseOrder.Items = append(purchaseOrder.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return purchaseOrder, nil
}

func (r *PurchaseOrderRepository) findPurchaseOrderHeaderID(ctx context.Context, tx *sql.Tx, quotationID int64) (int64, error) {
	var headerID int64
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM purchase_order
		WHERE quotation_id = $1
		FOR UPDATE
	`, quotationID).Scan(&headerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}

	return headerID, nil
}

func (r *PurchaseOrderRepository) syncQuotationStatusAndCommit(ctx context.Context, tx *sql.Tx, currentStatus string, quotationID int64, hasItems bool) error {
	nextStatus := syncQuotationStatus(currentStatus, "po", hasItems)
	if nextStatus != currentStatus {
		if _, err := tx.ExecContext(ctx, `
			UPDATE quotations
			SET status = $1, updated_at = NOW()
			WHERE id = $2
		`, nextStatus, quotationID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *PurchaseOrderRepository) createAssetAttachment(ctx context.Context, tx *sql.Tx, purchaseOrderID int64, attachment *entity.PurchaseOrderAsset) error {
	if attachment.Asset == nil {
		return nil
	}

	asset := attachment.Asset
	err := tx.QueryRowContext(ctx, `
		INSERT INTO assets (
			bucket, object_name, original_filename, stored_filename, mime_type,
			extension, size, etag, is_private, uploaded_by, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
		RETURNING id, created_at, updated_at
	`, asset.Bucket, asset.ObjectName, asset.OriginalFilename, asset.StoredFilename, asset.MimeType,
		asset.Extension, asset.Size, asset.ETag, asset.IsPrivate, asset.UploadedBy).
		Scan(&asset.ID, &asset.CreatedAt, &asset.UpdatedAt)
	if err != nil {
		return err
	}

	attachment.PurchaseOrderID = purchaseOrderID
	attachment.AssetID = asset.ID
	return tx.QueryRowContext(ctx, `
		INSERT INTO purchase_order_assets (purchase_order_id, asset_id, category, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, created_at
	`, purchaseOrderID, asset.ID, attachment.Category).Scan(&attachment.ID, &attachment.CreatedAt)
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

func syncQuotationStatus(currentStatus, targetStatus string, shouldExist bool) string {
	if targetStatus == "" {
		return currentStatus
	}

	parts := strings.Split(currentStatus, ":")
	filtered := make([]string, 0, len(parts)+1)
	found := false

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == targetStatus {
			found = true
			if !shouldExist {
				continue
			}
		}
		filtered = append(filtered, part)
	}

	if shouldExist && !found {
		filtered = append(filtered, targetStatus)
	}

	return strings.Join(filtered, ":")
}
