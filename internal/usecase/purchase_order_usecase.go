package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/google/uuid"
)

type purchaseOrderUseCase struct {
	repo    output.PurchaseOrderRepository
	storage output.ObjectStorage
}

func NewPurchaseOrderUseCase(repo output.PurchaseOrderRepository, storage output.ObjectStorage) input.PurchaseOrderUseCase {
	return &purchaseOrderUseCase{
		repo:    repo,
		storage: storage,
	}
}

func (uc *purchaseOrderUseCase) Upsert(ctx context.Context, cmd input.UpsertPurchaseOrderCommand) error {
	if cmd.QuotationID <= 0 {
		return domain.NewError(domain.ErrInvalidInput, "quotation id must be greater than 0")
	}
	if cmd.Asset != nil && len(cmd.Items) == 0 {
		return domain.NewError(domain.ErrInvalidInput, "asset upload requires at least one purchase order item")
	}

	purchaseOrder := &entity.PurchaseOrder{
		QuotationID: cmd.QuotationID,
		Items:       make([]entity.PurchaseOrderDetail, 0, len(cmd.Items)),
	}
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

		purchaseOrder.Items = append(purchaseOrder.Items, entity.PurchaseOrderDetail{
			Name:  item.Name,
			Qty:   item.Qty,
			Unit:  item.Unit,
			Price: item.Price,
			Total: item.Qty * item.Price,
		})
	}

	var attachment *entity.PurchaseOrderAsset
	var uploadedObjectName string
	if cmd.Asset != nil {
		var err error
		attachment, uploadedObjectName, err = uc.buildUploadedAssetAttachment(ctx, cmd.QuotationID, cmd.Asset)
		if err != nil {
			return err
		}
	}

	if _, err := uc.repo.UpsertByQuotationID(ctx, purchaseOrder, attachment); err != nil {
		if uploadedObjectName != "" {
			_ = uc.storage.Delete(ctx, uploadedObjectName)
		}
		return err
	}

	return nil
}

func (uc *purchaseOrderUseCase) GetByQuotationID(ctx context.Context, quotationID int64) ([]input.PurchaseOrderItemResult, error) {
	if quotationID <= 0 {
		return nil, domain.NewError(domain.ErrInvalidInput, "quotation id must be greater than 0")
	}

	purchaseOrder, err := uc.repo.GetByQuotationID(ctx, quotationID)
	if err != nil {
		return nil, err
	}

	results := make([]input.PurchaseOrderItemResult, 0, len(purchaseOrder.Items))
	for _, item := range purchaseOrder.Items {
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

func (uc *purchaseOrderUseCase) buildUploadedAssetAttachment(ctx context.Context, quotationID int64, cmd *input.PurchaseOrderAssetUploadCommand) (*entity.PurchaseOrderAsset, string, error) {
	if uc.storage == nil {
		return nil, "", domain.NewError(domain.ErrUnavailable, "asset storage is unavailable")
	}
	if cmd.Reader == nil {
		return nil, "", domain.NewError(domain.ErrInvalidInput, "asset file is required")
	}
	if cmd.Size <= 0 {
		return nil, "", domain.NewError(domain.ErrInvalidInput, "asset file cannot be empty")
	}
	if strings.TrimSpace(cmd.OriginalFilename) == "" {
		return nil, "", domain.NewError(domain.ErrInvalidInput, "asset filename is required")
	}

	contentType := strings.TrimSpace(cmd.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	category := strings.TrimSpace(cmd.Category)
	if category == "" {
		category = "attachment"
	}

	storedFilename := buildStoredAssetFilename(cmd.OriginalFilename)
	objectName := fmt.Sprintf("purchase-orders/%d/%s", quotationID, storedFilename)
	stored, err := uc.storage.Upload(ctx, output.UploadObject{
		ObjectName:  objectName,
		Reader:      cmd.Reader,
		Size:        cmd.Size,
		ContentType: contentType,
	})
	if err != nil {
		return nil, "", err
	}

	asset := &entity.Asset{
		Bucket:           stored.Bucket,
		ObjectName:       stored.ObjectName,
		OriginalFilename: cmd.OriginalFilename,
		StoredFilename:   storedFilename,
		MimeType:         contentType,
		Extension:        strings.TrimPrefix(strings.ToLower(filepath.Ext(cmd.OriginalFilename)), "."),
		Size:             stored.Size,
		ETag:             stored.ETag,
		IsPrivate:        true,
		UploadedBy:       cmd.UploadedBy,
	}

	return &entity.PurchaseOrderAsset{
		Category: category,
		Asset:    asset,
	}, stored.ObjectName, nil
}

func buildStoredAssetFilename(originalFilename string) string {
	filename := sanitizeFilename(filepath.Base(originalFilename))
	if filename == "" || filename == "." {
		filename = "asset"
	}

	return uuid.NewString() + "-" + filename
}

func sanitizeFilename(filename string) string {
	var builder strings.Builder
	for _, r := range filename {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
		case r == '.', r == '-', r == '_':
			builder.WriteRune(r)
		case unicode.IsSpace(r):
			builder.WriteRune('-')
		}
	}

	return builder.String()
}
