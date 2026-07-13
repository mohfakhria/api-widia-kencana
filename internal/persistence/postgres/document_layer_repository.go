package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/lib/pq"
	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type DocumentLayerRepository struct {
	db *sql.DB
}

func NewDocumentLayerRepository(db *sql.DB) output.DocumentLayerRepository {
	return &DocumentLayerRepository{db: db}
}

func (r *DocumentLayerRepository) Create(
	ctx context.Context,
	layer *entity.DocumentLayer,
) (*entity.DocumentLayer, error) {
	documentID, err := r.getDocumentIDByToken(ctx, layer.DocumentToken)
	if err != nil {
		return nil, err
	}

	parentID, err := r.getOptionalLayerIDByToken(ctx, documentID, layer.ParentToken)
	if err != nil {
		return nil, err
	}

	elementID, err := r.getElementIDByToken(ctx, layer.Element.Token)
	if err != nil {
		return nil, err
	}

	content, err := json.Marshal(layer.Content)
	if err != nil {
		return nil, err
	}
	properties, err := json.Marshal(layer.Properties)
	if err != nil {
		return nil, err
	}

	var createdToken string
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO document_layers (
			document_id,
			parent_id,
			document_element_id,
			region,
			name,
			content,
			properties,
			position,
			status,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9, NOW(), NOW())
		RETURNING token::text
	`, documentID, parentID, elementID, layer.Region, layer.Name, string(content), string(properties), layer.Position, layer.Status).Scan(&createdToken)
	if err != nil {
		return nil, err
	}

	return r.GetByToken(ctx, createdToken)
}

func (r *DocumentLayerRepository) Update(
	ctx context.Context,
	token string,
	layer *entity.DocumentLayer,
) error {
	documentID, err := r.getDocumentIDByToken(ctx, layer.DocumentToken)
	if err != nil {
		return err
	}

	parentID, err := r.getOptionalLayerIDByToken(ctx, documentID, layer.ParentToken)
	if err != nil {
		return err
	}

	elementID, err := r.getElementIDByToken(ctx, layer.Element.Token)
	if err != nil {
		return err
	}

	content, err := json.Marshal(layer.Content)
	if err != nil {
		return err
	}
	properties, err := json.Marshal(layer.Properties)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE document_layers
		SET document_id = $1,
			parent_id = $2,
			document_element_id = $3,
			region = $4,
			name = $5,
			content = $6::jsonb,
			properties = $7::jsonb,
			position = $8,
			status = $9,
			updated_at = NOW()
		WHERE token = $10::uuid
			AND status <> 'deleted'
	`, documentID, parentID, elementID, layer.Region, layer.Name, string(content), string(properties), layer.Position, layer.Status, token)
	if err != nil {
		return err
	}

	return ensureAffected(result, "document layer not found")
}

func (r *DocumentLayerRepository) Sort(
	ctx context.Context,
	documentToken string,
	parentToken string,
	region string,
	layers []entity.DocumentLayer,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	documentID, err := getDocumentIDByTokenTx(ctx, tx, documentToken)
	if err != nil {
		return err
	}

	parentID, err := getOptionalLayerIDByTokenTx(ctx, tx, documentID, parentToken)
	if err != nil {
		return err
	}

	tokens := make([]string, 0, len(layers))
	for _, layer := range layers {
		tokens = append(tokens, layer.Token)
	}

	var matched int
	if parentID == nil {
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM document_layers
			WHERE document_id = $1
				AND parent_id IS NULL
				AND region = $2
				AND status <> 'deleted'
				AND token::text = ANY($3)
		`, documentID, region, pq.Array(tokens)).Scan(&matched)
	} else {
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM document_layers
			WHERE document_id = $1
				AND parent_id = $2
				AND region = $3
				AND status <> 'deleted'
				AND token::text = ANY($4)
		`, documentID, *parentID, region, pq.Array(tokens)).Scan(&matched)
	}
	if err != nil {
		return err
	}
	if matched != len(layers) {
		return domain.NewError(domain.ErrInvalidInput, "all document layers must belong to the same document group")
	}

	for _, layer := range layers {
		if _, err := tx.ExecContext(ctx, `
			UPDATE document_layers
			SET position = $1, updated_at = NOW()
			WHERE token = $2::uuid
				AND status <> 'deleted'
		`, layer.Position, layer.Token); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *DocumentLayerRepository) Delete(ctx context.Context, token string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		WITH RECURSIVE layer_tree AS (
			SELECT id
			FROM document_layers
			WHERE token = $1::uuid
				AND status <> 'deleted'

			UNION ALL

			SELECT child.id
			FROM document_layers child
			JOIN layer_tree parent ON parent.id = child.parent_id
			WHERE child.status <> 'deleted'
		)
		UPDATE document_layers
		SET status = 'deleted',
			updated_at = NOW()
		WHERE id IN (SELECT id FROM layer_tree)
	`, token)
	if err != nil {
		return err
	}
	if err := ensureAffected(result, "document layer not found"); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *DocumentLayerRepository) GetByToken(
	ctx context.Context,
	token string,
) (*entity.DocumentLayer, error) {
	var layer entity.DocumentLayer
	if err := scanDocumentLayer(r.db.QueryRowContext(ctx, documentLayerSelectQuery()+`
		WHERE layer.token = $1::uuid
			AND layer.status <> 'deleted'
	`, token), &layer); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "document layer not found")
		}
		return nil, err
	}

	return &layer, nil
}

func (r *DocumentLayerRepository) getDocumentIDByToken(ctx context.Context, token string) (int64, error) {
	return getDocumentIDByTokenQuery(ctx, r.db, token)
}

func (r *DocumentLayerRepository) getOptionalLayerIDByToken(
	ctx context.Context,
	documentID int64,
	token string,
) (*int64, error) {
	return getOptionalLayerIDByTokenQuery(ctx, r.db, documentID, token)
}

func (r *DocumentLayerRepository) getElementIDByToken(ctx context.Context, token string) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		SELECT id
		FROM document_elements
		WHERE token = $1::uuid
			AND status = 'active'
	`, token).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, domain.NewError(domain.ErrNotFound, "document element not found")
		}
		return 0, err
	}

	return id, nil
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getDocumentIDByTokenTx(ctx context.Context, tx *sql.Tx, token string) (int64, error) {
	return getDocumentIDByTokenQuery(ctx, tx, token)
}

func getDocumentIDByTokenQuery(ctx context.Context, queryer queryRower, token string) (int64, error) {
	var id int64
	err := queryer.QueryRowContext(ctx, `
		SELECT id
		FROM documents
		WHERE token = $1::uuid
			AND status <> 'deleted'
	`, token).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, domain.NewError(domain.ErrNotFound, "document not found")
		}
		return 0, err
	}

	return id, nil
}

func getOptionalLayerIDByTokenTx(
	ctx context.Context,
	tx *sql.Tx,
	documentID int64,
	token string,
) (*int64, error) {
	return getOptionalLayerIDByTokenQuery(ctx, tx, documentID, token)
}

func getOptionalLayerIDByTokenQuery(
	ctx context.Context,
	queryer queryRower,
	documentID int64,
	token string,
) (*int64, error) {
	if token == "" {
		return nil, nil
	}

	var id int64
	err := queryer.QueryRowContext(ctx, `
		SELECT id
		FROM document_layers
		WHERE token = $1::uuid
			AND document_id = $2
			AND status <> 'deleted'
	`, token, documentID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "parent document layer not found")
		}
		return nil, err
	}

	return &id, nil
}

func documentLayerSelectQuery() string {
	return `
		SELECT
			layer.id,
			layer.token::text,
			layer.document_id,
			document.token::text,
			layer.parent_id,
			COALESCE(parent.token::text, ''),
			layer.document_element_id,
			element.id,
			element.token::text,
			element.code,
			element.name,
			element.renderer_type,
			element.renderer_tag,
			element.content_type,
			element.is_container,
			element.status,
			element.created_at,
			element.updated_at,
			layer.region,
			layer.name,
			layer.content::text,
			layer.properties::text,
			layer.position,
			layer.status,
			layer.created_at,
			layer.updated_at
		FROM document_layers layer
		JOIN documents document ON document.id = layer.document_id
		LEFT JOIN document_layers parent ON parent.id = layer.parent_id
		JOIN document_elements element ON element.id = layer.document_element_id
	`
}

func scanDocumentLayer(row rowScanner, layer *entity.DocumentLayer) error {
	var parentID sql.NullInt64
	var contentRaw string
	var propertiesRaw string

	err := row.Scan(
		&layer.ID,
		&layer.Token,
		&layer.DocumentID,
		&layer.DocumentToken,
		&parentID,
		&layer.ParentToken,
		&layer.DocumentElementID,
		&layer.Element.ID,
		&layer.Element.Token,
		&layer.Element.Code,
		&layer.Element.Name,
		&layer.Element.RendererType,
		&layer.Element.RendererTag,
		&layer.Element.ContentType,
		&layer.Element.IsContainer,
		&layer.Element.Status,
		&layer.Element.CreatedAt,
		&layer.Element.UpdatedAt,
		&layer.Region,
		&layer.Name,
		&contentRaw,
		&propertiesRaw,
		&layer.Position,
		&layer.Status,
		&layer.CreatedAt,
		&layer.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if parentID.Valid {
		layer.ParentID = &parentID.Int64
	}
	if err := json.Unmarshal([]byte(contentRaw), &layer.Content); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(propertiesRaw), &layer.Properties); err != nil {
		return err
	}

	return nil
}
