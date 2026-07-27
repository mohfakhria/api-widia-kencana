package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/google/uuid"
)

const defaultAssetUploadExpiry = 15 * time.Minute
const defaultAssetPreviewExpiry = 15 * time.Minute
const defaultAssetScope = "assets"

var allowedAssetStatuses = map[string]struct{}{
	"pending":   {},
	"uploading": {},
	"uploaded":  {},
	"failed":    {},
	"deleted":   {},
}

type assetUseCase struct {
	repo    output.AssetRepository
	storage output.ObjectStorage
}

func NewAssetUseCase(repo output.AssetRepository, storage output.ObjectStorage) input.AssetUseCase {
	return &assetUseCase{repo: repo, storage: storage}
}

func (uc *assetUseCase) RequestUpload(ctx context.Context, cmd input.RequestAssetUploadCommand) (*input.AssetUploadRequestResult, error) {
	if uc.storage == nil {
		return nil, domain.NewError(domain.ErrUnavailable, "asset storage is unavailable")
	}
	if cmd.Size <= 0 {
		return nil, domain.NewError(domain.ErrInvalidInput, "asset size must be greater than 0")
	}
	if strings.TrimSpace(cmd.OriginalFilename) == "" {
		return nil, domain.NewError(domain.ErrInvalidInput, "asset filename is required")
	}

	contentType := strings.TrimSpace(cmd.MimeType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	scope := sanitizeAssetScope(cmd.Scope)
	storedFilename := buildStoredAssetFilename(cmd.OriginalFilename)
	objectName := fmt.Sprintf("%s/%s", scope, storedFilename)
	expiresAt := time.Now().Add(defaultAssetUploadExpiry)

	asset := &entity.Asset{
		Bucket:             uc.storage.Bucket(),
		Scope:              scope,
		ObjectName:         objectName,
		OriginalFilename:   strings.TrimSpace(cmd.OriginalFilename),
		StoredFilename:     storedFilename,
		MimeType:           contentType,
		Extension:          strings.TrimPrefix(strings.ToLower(filepath.Ext(cmd.OriginalFilename)), "."),
		Size:               cmd.Size,
		Status:             "pending",
		UploadMethod:       "presigned_put",
		IsPrivate:          true,
		UploadedBy:         cmd.UploadedBy,
		PresignedExpiresAt: &expiresAt,
	}

	created, err := uc.repo.CreatePending(ctx, asset)
	if err != nil {
		return nil, err
	}

	uploadURL, err := uc.storage.PresignPut(ctx, created.ObjectName, defaultAssetUploadExpiry, contentType)
	if err != nil {
		_ = uc.repo.MarkFailed(ctx, created.Token, "presign_failed", err.Error())
		return nil, err
	}

	return &input.AssetUploadRequestResult{
		Asset:     created,
		UploadURL: uploadURL,
		ExpiresAt: expiresAt,
	}, nil
}

func (uc *assetUseCase) CompleteUpload(ctx context.Context, token string, uploadedBy *int64) (*entity.Asset, error) {
	token = strings.TrimSpace(token)
	if err := validateAssetUUIDToken(token, "asset token"); err != nil {
		return nil, err
	}

	asset, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := ensureAssetOwner(asset, uploadedBy); err != nil {
		return nil, err
	}
	if asset.Status == "uploaded" {
		return asset, nil
	}
	if asset.Status != "pending" && asset.Status != "uploading" {
		return nil, domain.NewError(domain.ErrInvalidInput, "asset cannot be completed from current status")
	}
	if asset.PresignedExpiresAt != nil && time.Now().After(*asset.PresignedExpiresAt) {
		_ = uc.repo.MarkFailed(ctx, token, "upload_expired", "asset upload URL expired")
		return nil, domain.NewError(domain.ErrInvalidInput, "asset upload URL expired")
	}

	stored, err := uc.storage.Stat(ctx, asset.ObjectName)
	if err != nil {
		_ = uc.repo.MarkFailed(ctx, token, "object_not_found", err.Error())
		return nil, domain.NewError(domain.ErrNotFound, "uploaded asset object not found")
	}
	if stored.Size != asset.Size {
		_ = uc.repo.MarkFailed(ctx, token, "size_mismatch", "uploaded asset size does not match request")
		return nil, domain.NewError(domain.ErrInvalidInput, "uploaded asset size does not match request")
	}
	if !assetContentTypeMatches(asset.MimeType, stored.ContentType) {
		_ = uc.repo.MarkFailed(ctx, token, "mime_type_mismatch", "uploaded asset MIME type does not match request")
		return nil, domain.NewError(domain.ErrInvalidInput, "uploaded asset MIME type does not match request")
	}

	return uc.repo.MarkUploaded(ctx, token, stored)
}

func (uc *assetUseCase) List(ctx context.Context, query input.ListAssetQuery) ([]entity.Asset, error) {
	query.Status = strings.ToLower(strings.TrimSpace(query.Status))
	query.Scope = sanitizeOptionalAssetScope(query.Scope)
	query.MimeType = strings.TrimSpace(query.MimeType)
	query.Extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(query.Extension)), ".")
	if query.Status != "" {
		if _, ok := allowedAssetStatuses[query.Status]; !ok {
			return nil, domain.NewError(domain.ErrInvalidInput, "invalid asset status")
		}
	}

	return uc.repo.List(ctx, query)
}

func (uc *assetUseCase) GetByToken(ctx context.Context, token string, uploadedBy *int64) (*entity.Asset, error) {
	token = strings.TrimSpace(token)
	if err := validateAssetUUIDToken(token, "asset token"); err != nil {
		return nil, err
	}

	asset, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := ensureAssetOwner(asset, uploadedBy); err != nil {
		return nil, err
	}

	return asset, nil
}

func (uc *assetUseCase) PresignGet(ctx context.Context, token string, uploadedBy *int64) (*input.AssetPresignGetResult, error) {
	token = strings.TrimSpace(token)
	if err := validateAssetUUIDToken(token, "asset token"); err != nil {
		return nil, err
	}

	asset, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := ensureAssetOwner(asset, uploadedBy); err != nil {
		return nil, err
	}
	if asset.Status != "uploaded" {
		return nil, domain.NewError(domain.ErrInvalidInput, "asset is not uploaded")
	}

	url, err := uc.storage.PresignGet(ctx, asset.ObjectName, defaultAssetPreviewExpiry)
	if err != nil {
		return nil, err
	}

	return &input.AssetPresignGetResult{
		Asset:     asset,
		URL:       url,
		ExpiresIn: int64(defaultAssetPreviewExpiry.Seconds()),
	}, nil
}

func (uc *assetUseCase) Delete(ctx context.Context, token string, uploadedBy *int64) error {
	token = strings.TrimSpace(token)
	if err := validateAssetUUIDToken(token, "asset token"); err != nil {
		return err
	}

	asset, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		return err
	}
	if err := ensureAssetOwner(asset, uploadedBy); err != nil {
		return err
	}
	if asset.Status == "uploaded" {
		if err := uc.storage.Delete(ctx, asset.ObjectName); err != nil {
			return err
		}
	}

	return uc.repo.MarkDeleted(ctx, token)
}

func sanitizeAssetScope(scope string) string {
	scope = strings.Trim(strings.ToLower(strings.TrimSpace(scope)), "/")
	if scope == "" {
		return defaultAssetScope
	}

	segments := strings.Split(scope, "/")
	cleaned := make([]string, 0, len(segments))
	for _, segment := range segments {
		clean := sanitizeAssetFilename(segment)
		clean = strings.Trim(clean, ".-_/")
		if clean == "" {
			continue
		}
		cleaned = append(cleaned, clean)
	}
	if len(cleaned) == 0 {
		return defaultAssetScope
	}

	return strings.Join(cleaned, "/")
}

func sanitizeOptionalAssetScope(scope string) string {
	if strings.TrimSpace(scope) == "" {
		return ""
	}

	return sanitizeAssetScope(scope)
}

func ensureAssetOwner(asset *entity.Asset, uploadedBy *int64) error {
	if asset == nil {
		return domain.NewError(domain.ErrNotFound, "asset not found")
	}
	if uploadedBy == nil {
		return domain.NewError(domain.ErrUnauthorized, "Invalid or expired token")
	}
	if asset.UploadedBy == nil {
		return nil
	}
	if *asset.UploadedBy != *uploadedBy {
		return domain.NewError(domain.ErrForbidden, "asset access forbidden")
	}

	return nil
}

func assetContentTypeMatches(expected string, actual string) bool {
	expected = normalizeAssetContentType(expected)
	actual = normalizeAssetContentType(actual)
	return actual == "" || expected == "" || expected == "application/octet-stream" || actual == expected
}

func normalizeAssetContentType(contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	return contentType
}

func validateAssetUUIDToken(token, label string) error {
	if strings.TrimSpace(token) == "" {
		return domain.NewError(domain.ErrInvalidInput, label+" cannot be empty")
	}
	if _, err := uuid.Parse(token); err != nil {
		return domain.NewError(domain.ErrInvalidInput, "invalid "+label)
	}

	return nil
}
