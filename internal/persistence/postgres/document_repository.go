package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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

// ListPapers mengembalikan kertas yang tersedia untuk dokumen baru.
//
// Diurutkan menurut nama lalu ukuran, bukan menurut waktu pembuatan seperti
// dokumen. Ini daftar pilihan yang dibaca manusia, dan enam di antaranya
// bernama sama — "Continuous Form" — sehingga hanya ukuran yang membedakannya.
// Urutan menurut waktu akan menyebarkan keenamnya ke seluruh daftar.
func (r *DocumentRepository) ListPapers(ctx context.Context, query input.ListDocumentPaperQuery) ([]entity.DocumentPaper, error) {
	builder := strings.Builder{}
	builder.WriteString(`
		SELECT
			id,
			token::text,
			name,
			media_type,
			width,
			height,
			unit,
			allow_portrait,
			allow_landscape,
			status,
			created_at,
			updated_at
		FROM document_papers
	`)

	args := make([]any, 0, 1)
	if query.Status != "" {
		args = append(args, query.Status)
		builder.WriteString(fmt.Sprintf(" WHERE status = $%d", len(args)))
	}
	builder.WriteString(" ORDER BY name ASC, width ASC, height ASC")

	rows, err := r.db.QueryContext(ctx, builder.String(), args...)
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

	return papers, rows.Err()
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
		ORDER BY d.created_at DESC
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

	var createdToken string
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO documents (
			document_paper_id,
			parent_id,
			name,
			document_type,
			status,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING token::text
	`, paperID, parentID, document.Name, document.DocumentType, document.Status).Scan(&createdToken)
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

	result, err := r.db.ExecContext(ctx, `
		UPDATE documents
		SET document_paper_id = $1,
			parent_id = $2,
			name = $3,
			document_type = $4,
			status = $5,
			updated_at = NOW()
		WHERE token = $6::uuid
			AND status <> 'deleted'
	`, paperID, parentID, document.Name, document.DocumentType, document.Status, token)
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

func (r *DocumentRepository) GetContent(ctx context.Context, token string) (*entity.DocumentContent, error) {
	var (
		raw     []byte
		version int64
		paper   entity.DocumentPaperSize
	)

	err := r.db.QueryRowContext(ctx, `
		SELECT d.content, d.content_version, paper.width, paper.height, paper.unit
		FROM documents d
		JOIN document_papers paper ON paper.id = d.document_paper_id
		WHERE d.token = $1::uuid
			AND d.status <> 'deleted'
	`, token).Scan(&raw, &version, &paper.Width, &paper.Height, &paper.Unit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "document not found")
		}
		return nil, err
	}

	return &entity.DocumentContent{
		Token:   token,
		Content: json.RawMessage(raw),
		Version: version,
		Paper:   paper,
	}, nil
}

func (r *DocumentRepository) SaveContent(ctx context.Context, token string, content json.RawMessage, fromVersion, toVersion int64) error {
	// content dikirim sebagai string, bukan []byte: lib/pq mengencode []byte
	// sebagai bytea, yang tidak dapat di-assign ke kolom jsonb.
	result, err := r.db.ExecContext(ctx, `
		UPDATE documents
		SET content = $1,
			content_version = $2,
			updated_at = NOW()
		WHERE token = $3::uuid
			AND status <> 'deleted'
			AND content_version = $4
	`, string(content), toVersion, token, fromVersion)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return r.explainSaveContentFailure(ctx, token, fromVersion)
	}

	return nil
}

// saveFailureLookupTimeout memberi pemeriksaan penyebab kegagalan tenggatnya
// sendiri, terlepas dari sisa tenggat pemanggil.
const saveFailureLookupTimeout = 2 * time.Second

// explainSaveContentFailure membedakan dokumen yang hilang dari versi yang sudah
// bergeser, supaya pemanggil tahu apakah perlu memuat ulang atau menyerah.
func (r *DocumentRepository) explainSaveContentFailure(ctx context.Context, token string, fromVersion int64) error {
	// Context sendiri, lepas dari milik pemanggil. Bila UPDATE tadi sudah
	// menghabiskan hampir seluruh tenggatnya, query ini akan gagal karena deadline
	// dan kegagalan permanen justru tersamar menjadi kegagalan sementara — lalu
	// dicoba berulang kali padahal dokumennya memang sudah tidak ada.
	lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveFailureLookupTimeout)
	defer cancel()

	current, err := r.GetContent(lookupCtx, token)
	if err != nil {
		return err
	}

	return domain.NewError(domain.ErrConflict, fmt.Sprintf(
		"document content version mismatch: expected %d, found %d", fromVersion, current.Version,
	))
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
	if err := row.Scan(
		&document.ID,
		&document.Token,
		&document.DocumentPaperID,
		&parentID,
		&document.ParentToken,
		&document.Name,
		&document.DocumentType,
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

	if parentID.Valid {
		document.ParentID = &parentID.Int64
	}

	return nil
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
