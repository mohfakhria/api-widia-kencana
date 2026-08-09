package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type AssetRepository struct {
	db *sql.DB
}

func NewAssetRepository(db *sql.DB) output.AssetRepository {
	return &AssetRepository{db: db}
}

func (r *AssetRepository) CreatePending(ctx context.Context, asset *entity.Asset) (*entity.Asset, error) {
	var token string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO assets (
			bucket, scope, object_name, original_filename, stored_filename, mime_type,
			extension, size, etag, checksum_sha256, status, upload_method, is_private,
			uploaded_by, presigned_expires_at, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,NOW(),NOW())
		RETURNING token::text
	`, asset.Bucket, asset.Scope, asset.ObjectName, asset.OriginalFilename, asset.StoredFilename,
		asset.MimeType, asset.Extension, asset.Size, asset.ETag, asset.ChecksumSHA256,
		asset.Status, asset.UploadMethod, asset.IsPrivate, asset.UploadedBy,
		asset.PresignedExpiresAt).Scan(&token)
	if err != nil {
		return nil, err
	}

	return r.GetByToken(ctx, token)
}

func (r *AssetRepository) GetByToken(ctx context.Context, token string) (*entity.Asset, error) {
	var asset entity.Asset
	err := scanAsset(r.db.QueryRowContext(ctx, assetSelectQuery()+`
		WHERE token = $1::uuid
			AND deleted_at IS NULL
	`, token), &asset)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "asset not found")
		}
		return nil, err
	}

	return &asset, nil
}

func (r *AssetRepository) List(ctx context.Context, query input.ListAssetQuery) ([]entity.Asset, error) {
	builder := strings.Builder{}
	builder.WriteString(assetSelectQuery())
	builder.WriteString(`
		WHERE deleted_at IS NULL
	`)

	args := make([]any, 0)
	if query.Status != "" {
		args = append(args, query.Status)
		builder.WriteString(fmt.Sprintf(" AND status = $%d", len(args)))
	} else {
		builder.WriteString(" AND status <> 'deleted'")
	}
	if query.Scope != "" {
		args = append(args, query.Scope)
		builder.WriteString(fmt.Sprintf(" AND scope = $%d", len(args)))
	}
	if query.MimeType != "" {
		args = append(args, query.MimeType)
		builder.WriteString(fmt.Sprintf(" AND mime_type = $%d", len(args)))
	}
	if query.Extension != "" {
		args = append(args, query.Extension)
		builder.WriteString(fmt.Sprintf(" AND extension = $%d", len(args)))
	}
	if query.UploadedBy != nil {
		args = append(args, *query.UploadedBy)
		builder.WriteString(fmt.Sprintf(" AND uploaded_by = $%d", len(args)))
	}
	builder.WriteString(`
		ORDER BY created_at DESC
	`)

	rows, err := r.db.QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets []entity.Asset
	for rows.Next() {
		var asset entity.Asset
		if err := scanAsset(rows, &asset); err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return assets, nil
}

func (r *AssetRepository) MarkUploaded(ctx context.Context, token string, stored *output.StoredObject) (*entity.Asset, error) {
	if stored == nil {
		return nil, domain.NewError(domain.ErrInvalidInput, "stored asset metadata is required")
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE assets
		SET status = 'uploaded',
			size = $1,
			etag = $2,
			mime_type = COALESCE(NULLIF($3, ''), mime_type),
			uploaded_at = NOW(),
			failed_at = NULL,
			failure_code = NULL,
			failure_message = NULL
		WHERE token = $4::uuid
			AND status IN ('pending', 'uploading')
			AND deleted_at IS NULL
	`, stored.Size, stored.ETag, stored.ContentType, token)
	if err != nil {
		return nil, err
	}
	if err := ensureAssetAffected(result, "asset not found"); err != nil {
		return nil, err
	}

	return r.GetByToken(ctx, token)
}

func (r *AssetRepository) MarkDeleted(ctx context.Context, token string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE assets
		SET status = 'deleted',
			deleted_at = NOW()
		WHERE token = $1::uuid
			AND deleted_at IS NULL
	`, token)
	if err != nil {
		return err
	}

	return ensureAssetAffected(result, "asset not found")
}

// FindExpired menyusun kembali indeks assets_pending_expiry_idx: ketiga syarat
// di bawah persis syarat parsial indeks itu, sehingga sapuan berkala tidak
// pernah menyusuri seluruh tabel.
//
// presigned_expires_at IS NOT NULL disebut eksplisit. Aset yang dibuat lewat
// jalur lain — tanpa presigned URL — tidak punya tenggat, dan tidak boleh ikut
// tersapu hanya karena statusnya kebetulan pending.
//
// Batas waktunya diterima sebagai argumen, bukan memakai NOW() milik database.
// Yang menentukan kedaluwarsa adalah jam yang sama dengan yang menuliskan
// tenggatnya, dan tenggat itu dihitung di Go saat presigned diterbitkan.
func (r *AssetRepository) FindExpired(ctx context.Context, before time.Time, limit int) ([]entity.Asset, error) {
	rows, err := r.db.QueryContext(ctx, assetSelectQuery()+`
		WHERE deleted_at IS NULL
			AND status IN ('pending', 'uploading')
			AND presigned_expires_at IS NOT NULL
			AND presigned_expires_at < $1
		ORDER BY presigned_expires_at
		LIMIT $2
	`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assets := make([]entity.Asset, 0, limit)
	for rows.Next() {
		var asset entity.Asset
		if err := scanAsset(rows, &asset); err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}

	return assets, rows.Err()
}

func (r *AssetRepository) MarkFailed(ctx context.Context, token string, code string, message string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE assets
		SET status = 'failed',
			failed_at = NOW(),
			failure_code = $1,
			failure_message = $2
		WHERE token = $3::uuid
			AND deleted_at IS NULL
	`, code, message, token)
	if err != nil {
		return err
	}

	return ensureAssetAffected(result, "asset not found")
}

func assetSelectQuery() string {
	return `
		SELECT
			id,
			token::text,
			bucket,
			scope,
			object_name,
			original_filename,
			stored_filename,
			mime_type,
			extension,
			size,
			etag,
			checksum_sha256,
			status,
			upload_method,
			is_private,
			uploaded_by,
			presigned_expires_at,
			uploaded_at,
			failed_at,
			deleted_at,
			failure_code,
			failure_message,
			created_at,
			updated_at
		FROM assets
	`
}

type assetRowScanner interface {
	Scan(dest ...any) error
}

func scanAsset(row assetRowScanner, asset *entity.Asset) error {
	var (
		checksumSHA256     sql.NullString
		uploadedBy         sql.NullInt64
		presignedExpiresAt sql.NullTime
		uploadedAt         sql.NullTime
		failedAt           sql.NullTime
		deletedAt          sql.NullTime
		failureCode        sql.NullString
		failureMessage     sql.NullString
	)
	if err := row.Scan(
		&asset.ID,
		&asset.Token,
		&asset.Bucket,
		&asset.Scope,
		&asset.ObjectName,
		&asset.OriginalFilename,
		&asset.StoredFilename,
		&asset.MimeType,
		&asset.Extension,
		&asset.Size,
		&asset.ETag,
		&checksumSHA256,
		&asset.Status,
		&asset.UploadMethod,
		&asset.IsPrivate,
		&uploadedBy,
		&presignedExpiresAt,
		&uploadedAt,
		&failedAt,
		&deletedAt,
		&failureCode,
		&failureMessage,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	); err != nil {
		return err
	}

	if checksumSHA256.Valid {
		asset.ChecksumSHA256 = &checksumSHA256.String
	}
	if uploadedBy.Valid {
		asset.UploadedBy = &uploadedBy.Int64
	}
	if presignedExpiresAt.Valid {
		asset.PresignedExpiresAt = &presignedExpiresAt.Time
	}
	if uploadedAt.Valid {
		asset.UploadedAt = &uploadedAt.Time
	}
	if failedAt.Valid {
		asset.FailedAt = &failedAt.Time
	}
	if deletedAt.Valid {
		asset.DeletedAt = &deletedAt.Time
	}
	if failureCode.Valid {
		asset.FailureCode = &failureCode.String
	}
	if failureMessage.Valid {
		asset.FailureMessage = &failureMessage.String
	}

	return nil
}

func ensureAssetAffected(result sql.Result, message string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(domain.ErrNotFound, message)
	}

	return nil
}
