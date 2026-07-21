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

func (r *DocumentRepository) ListPapers(ctx context.Context) ([]entity.DocumentPaper, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, token::text, name, media_type, width, height, unit,
			allow_portrait, allow_landscape, status, created_at, updated_at
		FROM document_papers
		WHERE status = 'active'
		ORDER BY media_type, name, width, height
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var papers []entity.DocumentPaper
	for rows.Next() {
		var paper entity.DocumentPaper
		if err := rows.Scan(
			&paper.ID,
			&paper.Token,
			&paper.Name,
			&paper.MediaType,
			&paper.Width,
			&paper.Height,
			&paper.Unit,
			&paper.AllowPortrait,
			&paper.AllowLandscape,
			&paper.Status,
			&paper.CreatedAt,
			&paper.UpdatedAt,
		); err != nil {
			return nil, err
		}
		papers = append(papers, paper)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return papers, nil
}

func (r *DocumentRepository) ListElements(
	ctx context.Context,
	query input.ListDocumentElementQuery,
) ([]entity.DocumentElement, error) {
	builder := strings.Builder{}
	builder.WriteString(`
		SELECT id, token::text, code, name, renderer_type, renderer_tag,
			content_type, is_container, status, created_at, updated_at
		FROM document_elements
		WHERE status = 'active'
	`)

	args := make([]any, 0)
	if query.Code != "" && query.Code != "all" {
		args = append(args, query.Code)
		builder.WriteString(fmt.Sprintf(" AND code = $%d", len(args)))
	}
	builder.WriteString(`
		ORDER BY id
	`)

	rows, err := r.db.QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var elements []entity.DocumentElement
	for rows.Next() {
		var element entity.DocumentElement
		if err := rows.Scan(
			&element.ID,
			&element.Token,
			&element.Code,
			&element.Name,
			&element.RendererType,
			&element.RendererTag,
			&element.ContentType,
			&element.IsContainer,
			&element.Status,
			&element.CreatedAt,
			&element.UpdatedAt,
		); err != nil {
			return nil, err
		}
		elements = append(elements, element)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return elements, nil
}

func (r *DocumentRepository) ListProperties(ctx context.Context) ([]entity.DocumentProperty, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, token::text, code, name, data_type, input_type,
			default_value, unit, status, created_at, updated_at
		FROM document_properties
		WHERE status = 'active'
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var properties []entity.DocumentProperty
	for rows.Next() {
		var property entity.DocumentProperty
		if err := rows.Scan(
			&property.ID,
			&property.Token,
			&property.Code,
			&property.Name,
			&property.DataType,
			&property.InputType,
			&property.DefaultValue,
			&property.Unit,
			&property.Status,
			&property.CreatedAt,
			&property.UpdatedAt,
		); err != nil {
			return nil, err
		}
		properties = append(properties, property)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return properties, nil
}

func (r *DocumentRepository) ListPropertyOptions(ctx context.Context) ([]entity.DocumentPropertyOption, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			option.id,
			option.token::text,
			option.document_property_id,
			property.token::text,
			property.code,
			option.value,
			option.label,
			option.position,
			option.status,
			option.created_at,
			option.updated_at
		FROM document_property_options option
		JOIN document_properties property ON property.id = option.document_property_id
		WHERE option.status = 'active'
			AND property.status = 'active'
		ORDER BY option.document_property_id, option.position, option.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var options []entity.DocumentPropertyOption
	for rows.Next() {
		var option entity.DocumentPropertyOption
		if err := rows.Scan(
			&option.ID,
			&option.Token,
			&option.DocumentPropertyID,
			&option.PropertyToken,
			&option.PropertyCode,
			&option.Value,
			&option.Label,
			&option.Position,
			&option.Status,
			&option.CreatedAt,
			&option.UpdatedAt,
		); err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return options, nil
}

func (r *DocumentRepository) ListElementProperties(
	ctx context.Context,
	query input.ListDocumentElementPropertyQuery,
) ([]entity.DocumentElementProperty, error) {
	builder := strings.Builder{}
	builder.WriteString(`
		SELECT
			element_property.id,
			element_property.token::text,
			element_property.document_element_id,
			element.token::text,
			element.code,
			element_property.document_property_id,
			element_property.default_value,
			element_property.position,
			element_property.status,
			element_property.created_at,
			element_property.updated_at
		FROM document_element_properties element_property
		JOIN document_elements element ON element.id = element_property.document_element_id
		WHERE element_property.status = 'active'
			AND element.status = 'active'
	`)

	args := make([]any, 0)
	if query.ElementCode != "" {
		args = append(args, query.ElementCode)
		builder.WriteString(fmt.Sprintf(" AND element.code = $%d", len(args)))
	}
	builder.WriteString(`
		ORDER BY element_property.document_element_id, element_property.position, element_property.id
	`)

	rows, err := r.db.QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var elementProperties []entity.DocumentElementProperty
	for rows.Next() {
		var elementProperty entity.DocumentElementProperty
		if err := rows.Scan(
			&elementProperty.ID,
			&elementProperty.Token,
			&elementProperty.DocumentElementID,
			&elementProperty.ElementToken,
			&elementProperty.ElementCode,
			&elementProperty.DocumentPropertyID,
			&elementProperty.DefaultValue,
			&elementProperty.Position,
			&elementProperty.Status,
			&elementProperty.CreatedAt,
			&elementProperty.UpdatedAt,
		); err != nil {
			return nil, err
		}
		elementProperties = append(elementProperties, elementProperty)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return elementProperties, nil
}

func (r *DocumentRepository) List(ctx context.Context, query input.ListDocumentQuery) ([]entity.Document, error) {
	builder := strings.Builder{}
	builder.WriteString(documentSelectQuery())
	builder.WriteString(`
		WHERE d.status <> 'deleted'
	`)

	args := make([]any, 0)
	if query.Name != "" {
		args = append(args, "%"+query.Name+"%")
		builder.WriteString(fmt.Sprintf(" AND d.name ILIKE $%d", len(args)))
	}
	if query.Token != "" {
		args = append(args, query.Token)
		builder.WriteString(fmt.Sprintf(" AND d.token = $%d::uuid", len(args)))
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

	return ensureAffected(result, "document not found")
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

	return ensureAffected(result, "document not found")
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDocument(row rowScanner, document *entity.Document) error {
	var parentID sql.NullInt64
	var settingsRaw string

	err := row.Scan(
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
	)
	if err != nil {
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
	encoded, err := json.Marshal(normalizeDocumentSettingsMap(settings))
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

func decodeDocumentSettings(raw string) (map[string]any, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil, err
	}

	return normalizeDocumentSettingsMap(settings), nil
}

func normalizeDocumentSettingsMap(settings map[string]any) map[string]any {
	if settings == nil {
		return map[string]any{}
	}

	return settings
}

func ensureAffected(result sql.Result, notFoundMessage string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(domain.ErrNotFound, notFoundMessage)
	}

	return nil
}
