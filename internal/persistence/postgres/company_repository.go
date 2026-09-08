package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type CompanyRepository struct {
	db *sql.DB
}

func NewCompanyRepository(db *sql.DB) output.CompanyRepository {
	return &CompanyRepository{db: db}
}

func companySelectQuery() string {
	return `
		SELECT
			id::text, code, name, legal_name,
			COALESCE(company_type, ''), COALESCE(description, ''), established_date,
			COALESCE(address, ''), COALESCE(email, ''), COALESCE(phone, ''), COALESCE(fax, ''),
			timezone, locale, currency_code, status, created_at, updated_at
		FROM companies
	`
}

func scanCompany(row interface{ Scan(...any) error }) (*entity.Company, error) {
	var company entity.Company
	var established sql.NullTime

	if err := row.Scan(
		&company.ID, &company.Code, &company.Name, &company.LegalName,
		&company.CompanyType, &company.Description, &established,
		&company.Address, &company.Email, &company.Phone, &company.Fax,
		&company.Timezone, &company.Locale, &company.CurrencyCode,
		&company.Status, &company.CreatedAt, &company.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if established.Valid {
		company.EstablishedDate = &established.Time
	}

	return &company, nil
}

func (r *CompanyRepository) List(ctx context.Context, query input.ListCompanyQuery) ([]entity.Company, error) {
	builder := strings.Builder{}
	builder.WriteString(companySelectQuery())
	builder.WriteString(" WHERE 1 = 1")

	args := make([]any, 0, 2)
	if query.Status != "" {
		args = append(args, query.Status)
		fmt.Fprintf(&builder, " AND status = $%d", len(args))
	}
	if query.Search != "" {
		// Satu kata kunci diadu ke tiga kolom sekaligus. ILIKE, bukan LIKE:
		// orang yang mencari "dizkaa" tidak sedang menyebut huruf besar-kecilnya.
		args = append(args, "%"+query.Search+"%")
		fmt.Fprintf(&builder,
			" AND (code ILIKE $%d OR name ILIKE $%d OR legal_name ILIKE $%d)",
			len(args), len(args), len(args))
	}

	// Berdasarkan nama, bukan waktu dibuat. Ini master data yang dibaca manusia
	// dari sebuah pemilih — yang dicari orang letaknya menurut abjad, bukan
	// menurut kapan ia dimasukkan.
	builder.WriteString(" ORDER BY LOWER(name)")

	rows, err := r.db.QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	companies := make([]entity.Company, 0)
	for rows.Next() {
		company, err := scanCompany(rows)
		if err != nil {
			return nil, err
		}
		companies = append(companies, *company)
	}

	return companies, rows.Err()
}

func (r *CompanyRepository) GetByID(ctx context.Context, id string) (*entity.Company, error) {
	company, err := scanCompany(r.db.QueryRowContext(ctx, companySelectQuery()+` WHERE id = $1::uuid`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "company not found")
		}

		return nil, err
	}

	return company, nil
}

func (r *CompanyRepository) Create(ctx context.Context, company *entity.Company) (*entity.Company, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO companies (
			code, name, legal_name, company_type, description, established_date,
			address, email, phone, fax, timezone, locale, currency_code, status
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id::text
	`,
		company.Code, company.Name, company.LegalName, company.CompanyType,
		company.Description, company.EstablishedDate, company.Address,
		company.Email, company.Phone, company.Fax,
		company.Timezone, company.Locale, company.CurrencyCode, company.Status,
	).Scan(&id)
	if err != nil {
		return nil, translateCompanyConflict(err)
	}

	return r.GetByID(ctx, id)
}

func (r *CompanyRepository) Update(ctx context.Context, id string, company *entity.Company) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE companies
		SET code = $1, name = $2, legal_name = $3, company_type = $4,
			description = $5, established_date = $6, address = $7,
			email = $8, phone = $9, fax = $10,
			timezone = $11, locale = $12, currency_code = $13, status = $14
		WHERE id = $15::uuid
	`,
		company.Code, company.Name, company.LegalName, company.CompanyType,
		company.Description, company.EstablishedDate, company.Address,
		company.Email, company.Phone, company.Fax,
		company.Timezone, company.Locale, company.CurrencyCode, company.Status, id,
	)
	if err != nil {
		return translateCompanyConflict(err)
	}

	return ensureCompanyAffected(result, "company not found")
}

func (r *CompanyRepository) Delete(ctx context.Context, id string) error {
	// Kontaknya ikut lewat ON DELETE CASCADE. Menghapus barisnya, bukan
	// menandainya — sejalan dengan seluruh tabel lain di repo ini.
	result, err := r.db.ExecContext(ctx, `DELETE FROM companies WHERE id = $1::uuid`, id)
	if err != nil {
		return err
	}

	return ensureCompanyAffected(result, "company not found")
}

func companyContactSelectQuery() string {
	return `
		SELECT
			id::text, company_id::text,
			COALESCE(salutation, ''), name, COALESCE(position, ''), COALESCE(department, ''),
			COALESCE(email, ''), COALESCE(phone, ''), COALESCE(mobile, ''),
			is_primary, status, created_at, updated_at
		FROM company_contacts
	`
}

func scanCompanyContact(row interface{ Scan(...any) error }) (*entity.CompanyContact, error) {
	var contact entity.CompanyContact
	if err := row.Scan(
		&contact.ID, &contact.CompanyID,
		&contact.Salutation, &contact.Name, &contact.Position, &contact.Department,
		&contact.Email, &contact.Phone, &contact.Mobile,
		&contact.IsPrimary, &contact.Status, &contact.CreatedAt, &contact.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &contact, nil
}

func (r *CompanyRepository) ListContacts(ctx context.Context, companyID string) ([]entity.CompanyContact, error) {
	// Yang utama lebih dulu, lalu menurut nama. Baris "Attn:" mengambil yang
	// pertama, jadi urutannya bukan soal kerapian.
	rows, err := r.db.QueryContext(ctx, companyContactSelectQuery()+`
		WHERE company_id = $1::uuid
		ORDER BY is_primary DESC, LOWER(name)
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := make([]entity.CompanyContact, 0)
	for rows.Next() {
		contact, err := scanCompanyContact(rows)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, *contact)
	}

	return contacts, rows.Err()
}

func (r *CompanyRepository) GetContactByID(ctx context.Context, id string) (*entity.CompanyContact, error) {
	contact, err := scanCompanyContact(r.db.QueryRowContext(ctx, companyContactSelectQuery()+` WHERE id = $1::uuid`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "contact not found")
		}

		return nil, err
	}

	return contact, nil
}

func (r *CompanyRepository) CreateContact(ctx context.Context, contact *entity.CompanyContact) (*entity.CompanyContact, error) {
	var id string
	err := r.inTransaction(ctx, func(tx *sql.Tx) error {
		if err := demoteOtherPrimary(ctx, tx, contact.CompanyID, "", contact.IsPrimary); err != nil {
			return err
		}

		return tx.QueryRowContext(ctx, `
			INSERT INTO company_contacts (
				company_id, salutation, name, position, department,
				email, phone, mobile, is_primary, status
			)
			VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			RETURNING id::text
		`,
			contact.CompanyID, contact.Salutation, contact.Name, contact.Position,
			contact.Department, contact.Email, contact.Phone, contact.Mobile,
			contact.IsPrimary, contact.Status,
		).Scan(&id)
	})
	if err != nil {
		return nil, err
	}

	return r.GetContactByID(ctx, id)
}

func (r *CompanyRepository) UpdateContact(ctx context.Context, id string, contact *entity.CompanyContact) error {
	return r.inTransaction(ctx, func(tx *sql.Tx) error {
		if err := demoteOtherPrimary(ctx, tx, contact.CompanyID, id, contact.IsPrimary); err != nil {
			return err
		}

		result, err := tx.ExecContext(ctx, `
			UPDATE company_contacts
			SET salutation = $1, name = $2, position = $3, department = $4,
				email = $5, phone = $6, mobile = $7, is_primary = $8, status = $9
			WHERE id = $10::uuid
		`,
			contact.Salutation, contact.Name, contact.Position, contact.Department,
			contact.Email, contact.Phone, contact.Mobile,
			contact.IsPrimary, contact.Status, id,
		)
		if err != nil {
			return err
		}

		return ensureCompanyAffected(result, "contact not found")
	})
}

func (r *CompanyRepository) DeleteContact(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM company_contacts WHERE id = $1::uuid`, id)
	if err != nil {
		return err
	}

	return ensureCompanyAffected(result, "contact not found")
}

// demoteOtherPrimary menurunkan kontak utama yang lama, di dalam transaksi yang
// sama dengan penulisan yang menaikkan penggantinya.
//
// Database menolak kontak utama kedua lewat indeks unik parsial, jadi tanpa ini
// setiap "jadikan kontak utama" gagal. Yang penting satu transaksi: dua
// pernyataan terpisah menyisakan saat ketika perusahaan itu tidak punya kontak
// utama sama sekali — dan kalau yang kedua gagal, keadaan itu menetap.
//
// kecuali diisi saat memperbarui, supaya kontak yang memang sudah utama tidak
// menurunkan dirinya sendiri lalu menaikkannya lagi.
func demoteOtherPrimary(ctx context.Context, tx *sql.Tx, companyID, kecuali string, isPrimary bool) error {
	if !isPrimary {
		return nil
	}

	kueri := `UPDATE company_contacts SET is_primary = FALSE WHERE company_id = $1::uuid AND is_primary`
	args := []any{companyID}
	if kecuali != "" {
		kueri += ` AND id <> $2::uuid`
		args = append(args, kecuali)
	}

	_, err := tx.ExecContext(ctx, kueri, args...)

	return err
}

func (r *CompanyRepository) inTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}

// translateCompanyConflict mengubah pelanggaran kode kembar menjadi galat
// domain.
//
// Kodenya wajib dan unik, jadi tabrakan adalah kekeliruan yang paling wajar —
// dua orang menamai pelanggan yang sama. Tanpa terjemahan ini jawabannya galat
// mentah Postgres: 500 yang membocorkan nama constraint dan tidak menyebutkan
// bahwa yang bentrok adalah kodenya.
func translateCompanyConflict(err error) error {
	var pgErr *pq.Error
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}

	if pgErr.Constraint == "companies_code_uq_idx" {
		return domain.NewError(domain.ErrConflict, "company code is already used")
	}

	return err
}

func ensureCompanyAffected(result sql.Result, pesan string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(domain.ErrNotFound, pesan)
	}

	return nil
}
