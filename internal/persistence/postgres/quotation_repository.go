package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type QuotationRepository struct {
	db *sql.DB
}

func NewQuotationRepository(db *sql.DB) output.QuotationRepository {
	return &QuotationRepository{db: db}
}

func (r *QuotationRepository) GenerateQuotationNo(ctx context.Context) (string, error) {
	var quotationNo string
	err := r.db.QueryRowContext(ctx, "SELECT generate_quotation_no()").Scan(&quotationNo)
	if err != nil {
		return "", err
	}
	return quotationNo, nil
}

func (r *QuotationRepository) Create(ctx context.Context, quotation *entity.Quotation) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	quotationNo, err := r.GenerateQuotationNo(ctx)
	if err != nil {
		return "", err
	}

	notesJSON, err := encodeQuotationNotes(quotation.Notes)
	if err != nil {
		return "", err
	}

	var quotationID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO quotations
		(quotation_no, client_name, attn_name, attn_position, address, project, discount_type, discount_value, total, notes, subtotal, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW(),NOW())
		RETURNING id
	`, quotationNo, quotation.ClientName, quotation.AttnName, quotation.AttnPosition, quotation.Address, quotation.Project, quotation.DiscountType, quotation.DiscountValue, quotation.Total, notesJSON, quotation.SubTotal).Scan(&quotationID)
	if err != nil {
		return "", err
	}

	if err := insertSections(ctx, tx, quotationID, quotation.Sections); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return quotationNo, nil
}

func (r *QuotationRepository) List(ctx context.Context, query input.ListQuotationQuery) ([]entity.Quotation, error) {
	var (
		builder strings.Builder
		args    []any
	)

	builder.WriteString(`
		SELECT id, quotation_no, client_name, project, status, total, created_at, updated_at
		FROM quotations
		WHERE 1=1
	`)

	if query.Status != "" {
		args = append(args, query.Status)
		builder.WriteString(`
			AND EXISTS (
				SELECT 1
				FROM unnest(string_to_array(COALESCE(status, ''), ':')) AS status_step
				WHERE status_step = $`)
		builder.WriteString(strconv.Itoa(len(args)))
		builder.WriteString(`
			)
		`)
	}

	if query.Project != "" {
		args = append(args, "%"+query.Project+"%")
		builder.WriteString(`
			AND COALESCE(project, '') ILIKE $`)
		builder.WriteString(strconv.Itoa(len(args)))
		builder.WriteString(`
		`)
	}

	builder.WriteString(`
		ORDER BY created_at DESC
	`)

	rows, err := r.db.QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var quotations []entity.Quotation
	for rows.Next() {
		var q entity.Quotation
		if err := rows.Scan(&q.ID, &q.QuotationNo, &q.ClientName, &q.Project, &q.Status, &q.Total, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		quotations = append(quotations, q)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return quotations, nil
}

func (r *QuotationRepository) GetByID(ctx context.Context, id string) (*entity.Quotation, error) {
	var (
		q        entity.Quotation
		notesRaw string
	)

	err := r.db.QueryRowContext(ctx, `
		SELECT id, quotation_no, client_name, attn_name, attn_position, address, project,
		       discount_type, discount_value, subtotal, total, notes, created_at, updated_at
		FROM quotations WHERE id = $1
	`, id).Scan(&q.ID, &q.QuotationNo, &q.ClientName, &q.AttnName, &q.AttnPosition, &q.Address, &q.Project, &q.DiscountType, &q.DiscountValue, &q.SubTotal, &q.Total, &notesRaw, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil, err
	}

	notes, err := decodeQuotationNotes(notesRaw)
	if err != nil {
		return nil, err
	}
	q.Notes = notes

	rows, err := r.db.QueryContext(ctx, `SELECT id, title, position FROM quotation_sections WHERE quotation_id=$1 ORDER BY position ASC`, q.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var section entity.QuotationSection
		if err := rows.Scan(&section.ID, &section.Title, &section.Position); err != nil {
			return nil, err
		}

		itemRows, err := r.db.QueryContext(ctx, `SELECT id, name, qty, unit, price FROM quotation_items WHERE section_id=$1`, section.ID)
		if err != nil {
			return nil, err
		}
		for itemRows.Next() {
			var item entity.QuotationItem
			if err := itemRows.Scan(&item.ID, &item.Name, &item.Qty, &item.Unit, &item.Price); err != nil {
				itemRows.Close()
				return nil, err
			}
			item.Total = item.Qty * item.Price
			section.Items = append(section.Items, item)
		}
		itemRows.Close()
		if err := itemRows.Err(); err != nil {
			return nil, err
		}

		detailRows, err := r.db.QueryContext(ctx, `SELECT id, description, position FROM quotation_details WHERE section_id=$1 ORDER BY position`, section.ID)
		if err != nil {
			return nil, err
		}
		for detailRows.Next() {
			var detail entity.QuotationDetail
			if err := detailRows.Scan(&detail.ID, &detail.Description, &detail.Position); err != nil {
				detailRows.Close()
				return nil, err
			}
			section.Details = append(section.Details, detail)
		}
		detailRows.Close()
		if err := detailRows.Err(); err != nil {
			return nil, err
		}

		q.Sections = append(q.Sections, section)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &q, nil
}

func (r *QuotationRepository) Update(ctx context.Context, id string, quotation *entity.Quotation) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	quotationID, err := parseQuotationID(id)
	if err != nil {
		return err
	}

	notesJSON, err := encodeQuotationNotes(quotation.Notes)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE quotations SET
			client_name=$1, attn_name=$2, attn_position=$3, address=$4,
			project=$5, discount_type=$6, discount_value=$7,
			subtotal=$8, total=$9, notes=$10, updated_at=NOW()
		WHERE id=$11
	`, quotation.ClientName, quotation.AttnName, quotation.AttnPosition, quotation.Address, quotation.Project, quotation.DiscountType, quotation.DiscountValue, quotation.SubTotal, quotation.Total, notesJSON, id)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM quotation_sections WHERE quotation_id=$1`, quotationID); err != nil {
		return err
	}

	if err := insertSections(ctx, tx, quotationID, quotation.Sections); err != nil {
		return err
	}

	return tx.Commit()
}

func insertSections(ctx context.Context, tx *sql.Tx, quotationID int64, sections []entity.QuotationSection) error {
	for _, section := range sections {
		var sectionID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO quotation_sections (quotation_id, title, position)
			VALUES ($1,$2,$3) RETURNING id
		`, quotationID, section.Title, section.Position).Scan(&sectionID)
		if err != nil {
			return err
		}

		for _, item := range section.Items {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO quotation_items (section_id, name, qty, unit, price)
				VALUES ($1,$2,$3,$4,$5)
			`, sectionID, item.Name, item.Qty, item.Unit, item.Price)
			if err != nil {
				return err
			}
		}

		for _, detail := range section.Details {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO quotation_details (section_id, description, position)
				VALUES ($1,$2,$3)
			`, sectionID, detail.Description, detail.Position)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func encodeQuotationNotes(notes []string) (string, error) {
	if len(notes) == 0 {
		return "[]", nil
	}

	encoded, err := json.Marshal(notes)
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

func decodeQuotationNotes(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, nil
	}

	var notes []string
	if err := json.Unmarshal([]byte(raw), &notes); err != nil {
		return nil, err
	}

	return notes, nil
}

func parseQuotationID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, domain.NewError(domain.ErrInvalidInput, "invalid quotation id")
	}

	return id, nil
}
