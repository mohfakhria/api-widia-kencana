package postgres

import (
	"context"
	"database/sql"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type DocumentBuilderMetadataRepository struct {
	db *sql.DB
}

func NewDocumentBuilderMetadataRepository(db *sql.DB) output.DocumentBuilderMetadataRepository {
	return &DocumentBuilderMetadataRepository{db: db}
}

func (r *DocumentBuilderMetadataRepository) ListPapers(ctx context.Context) ([]entity.DocumentPaper, error) {
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

func (r *DocumentBuilderMetadataRepository) ListElements(ctx context.Context) ([]entity.DocumentElement, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, token::text, code, name, renderer_type, renderer_tag,
			content_type, is_container, status, created_at, updated_at
		FROM document_elements
		WHERE status = 'active'
		ORDER BY id
	`)
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

func (r *DocumentBuilderMetadataRepository) ListProperties(ctx context.Context) ([]entity.DocumentProperty, error) {
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

func (r *DocumentBuilderMetadataRepository) ListPropertyOptions(ctx context.Context) ([]entity.DocumentPropertyOption, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, token::text, document_property_id, value, label,
			position, status, created_at, updated_at
		FROM document_property_options
		WHERE status = 'active'
		ORDER BY document_property_id, position, id
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

func (r *DocumentBuilderMetadataRepository) ListElementProperties(ctx context.Context) ([]entity.DocumentElementProperty, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, token::text, document_element_id, document_property_id,
			default_value, position, status, created_at, updated_at
		FROM document_element_properties
		WHERE status = 'active'
		ORDER BY document_element_id, position, id
	`)
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
