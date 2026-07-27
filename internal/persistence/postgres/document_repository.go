package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type DocumentRepository struct {
	db *sql.DB
}

func NewDocumentRepository(db *sql.DB) output.DocumentRepository {
	return &DocumentRepository{db: db}
}

func (r *DocumentRepository) List(ctx context.Context, query input.ListDocumentQuery) ([]entity.Document, error) {
	builder := strings.Builder{}
	builder.WriteString(documentSelectQuery())

	args := make([]any, 0)
	if query.Status == "" {
		builder.WriteString(" WHERE d.status <> 'deleted'")
	} else {
		args = append(args, query.Status)
		builder.WriteString(fmt.Sprintf(" WHERE d.status = $%d", len(args)))
	}
	if query.Name != "" {
		args = append(args, "%"+query.Name+"%")
		builder.WriteString(fmt.Sprintf(" AND d.name ILIKE $%d", len(args)))
	}
	if query.Token != "" {
		args = append(args, query.Token)
		builder.WriteString(fmt.Sprintf(" AND d.token = $%d::uuid", len(args)))
	}
	if query.DocumentType != "" {
		args = append(args, query.DocumentType)
		builder.WriteString(fmt.Sprintf(" AND d.document_type = $%d", len(args)))
	}
	builder.WriteString(`
		ORDER BY d.position ASC, d.created_at DESC
	`)

	rows, err := r.db.QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanDocuments(rows)
}

func (r *DocumentRepository) GetByToken(ctx context.Context, token string) (*entity.Document, error) {
	var document entity.Document
	err := scanDocument(r.db.QueryRowContext(ctx, documentSelectQuery()+`
		WHERE d.token = $1::uuid
			AND d.status <> 'deleted'
	`, token), &document)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "document not found")
		}
		return nil, err
	}

	return &document, nil
}

func (r *DocumentRepository) Create(ctx context.Context, document *entity.Document) (*entity.Document, error) {
	paperID, err := r.getDocumentPaperIDByToken(ctx, document.Paper.Token)
	if err != nil {
		return nil, err
	}

	parentID, err := r.getOptionalDocumentIDByToken(ctx, document.ParentToken)
	if err != nil {
		return nil, err
	}
	settings, err := encodeDocumentSettings(document.Settings)
	if err != nil {
		return nil, err
	}

	var createdToken string
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO documents (
			document_paper_id,
			parent_id,
			name,
			document_type,
			settings,
			position,
			status,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, NOW(), NOW())
		RETURNING token::text
	`, paperID, parentID, document.Name, document.DocumentType, settings, document.Position, document.Status).Scan(&createdToken)
	if err != nil {
		return nil, err
	}

	return r.GetByToken(ctx, createdToken)
}

func (r *DocumentRepository) Update(ctx context.Context, token string, document *entity.Document) error {
	paperID, err := r.getDocumentPaperIDByToken(ctx, document.Paper.Token)
	if err != nil {
		return err
	}

	parentID, err := r.getOptionalDocumentIDByToken(ctx, document.ParentToken)
	if err != nil {
		return err
	}
	settings, err := encodeDocumentSettings(document.Settings)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE documents
		SET document_paper_id = $1,
			parent_id = $2,
			name = $3,
			document_type = $4,
			settings = $5::jsonb,
			position = $6,
			status = $7,
			updated_at = NOW()
		WHERE token = $8::uuid
			AND status <> 'deleted'
	`, paperID, parentID, document.Name, document.DocumentType, settings, document.Position, document.Status, token)
	if err != nil {
		return err
	}

	return ensureDocumentAffected(result, "document not found")
}

func (r *DocumentRepository) Delete(ctx context.Context, token string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE documents
		SET status = 'deleted',
			updated_at = NOW()
		WHERE token = $1::uuid
			AND status <> 'deleted'
	`, token)
	if err != nil {
		return err
	}

	return ensureDocumentAffected(result, "document not found")
}

func (r *DocumentRepository) getDocumentPaperIDByToken(ctx context.Context, token string) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		SELECT id
		FROM document_papers
		WHERE token = $1::uuid
			AND status = 'active'
	`, token).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, domain.NewError(domain.ErrNotFound, "document paper not found")
		}
		return 0, err
	}

	return id, nil
}

func (r *DocumentRepository) getOptionalDocumentIDByToken(ctx context.Context, token string) (*int64, error) {
	if token == "" {
		return nil, nil
	}

	var id int64
	err := r.db.QueryRowContext(ctx, `
		SELECT id
		FROM documents
		WHERE token = $1::uuid
			AND status <> 'deleted'
	`, token).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "parent document not found")
		}
		return nil, err
	}

	return &id, nil
}

func documentSelectQuery() string {
	return `
		SELECT
			d.id,
			d.token::text,
			d.document_paper_id,
			d.parent_id,
			COALESCE(parent.token::text, ''),
			d.name,
			d.document_type,
			d.settings::text,
			d.position,
			d.status,
			d.created_at,
			d.updated_at,
			paper.id,
			paper.token::text,
			paper.name,
			paper.media_type,
			paper.width,
			paper.height,
			paper.unit,
			paper.allow_portrait,
			paper.allow_landscape,
			paper.status,
			paper.created_at,
			paper.updated_at
		FROM documents d
		JOIN document_papers paper ON paper.id = d.document_paper_id
		LEFT JOIN documents parent ON parent.id = d.parent_id
	`
}

func scanDocuments(rows *sql.Rows) ([]entity.Document, error) {
	var documents []entity.Document
	for rows.Next() {
		var document entity.Document
		if err := scanDocument(rows, &document); err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return documents, nil
}

type documentRowScanner interface {
	Scan(dest ...any) error
}

func scanDocument(row documentRowScanner, document *entity.Document) error {
	var parentID sql.NullInt64
	var settingsRaw string
	if err := row.Scan(
		&document.ID,
		&document.Token,
		&document.DocumentPaperID,
		&parentID,
		&document.ParentToken,
		&document.Name,
		&document.DocumentType,
		&settingsRaw,
		&document.Position,
		&document.Status,
		&document.CreatedAt,
		&document.UpdatedAt,
		&document.Paper.ID,
		&document.Paper.Token,
		&document.Paper.Name,
		&document.Paper.MediaType,
		&document.Paper.Width,
		&document.Paper.Height,
		&document.Paper.Unit,
		&document.Paper.AllowPortrait,
		&document.Paper.AllowLandscape,
		&document.Paper.Status,
		&document.Paper.CreatedAt,
		&document.Paper.UpdatedAt,
	); err != nil {
		return err
	}

	settings, err := decodeDocumentSettings(settingsRaw)
	if err != nil {
		return err
	}
	document.Settings = settings
	if parentID.Valid {
		document.ParentID = &parentID.Int64
	}

	return nil
}

func encodeDocumentSettings(settings map[string]any) (string, error) {
	if settings == nil {
		settings = map[string]any{}
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		return "", domain.NewError(domain.ErrInvalidInput, "invalid document settings")
	}

	return string(encoded), nil
}

func decodeDocumentSettings(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil, err
	}
	if settings == nil {
		return map[string]any{}, nil
	}

	return settings, nil
}

func ensureDocumentAffected(result sql.Result, message string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(domain.ErrNotFound, message)
	}

	return nil
}
