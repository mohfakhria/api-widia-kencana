package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type ProjectRepository struct {
	db *sql.DB
}

func NewProjectRepository(db *sql.DB) output.ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) List(ctx context.Context) ([]entity.Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, status, created_at, updated_at
		FROM projects
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []entity.Project
	for rows.Next() {
		var project entity.Project
		if err := rows.Scan(&project.ID, &project.Name, &project.Status, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return projects, nil
}

func (r *ProjectRepository) GetByID(ctx context.Context, id int64) (*entity.Project, error) {
	var project entity.Project
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, status, created_at, updated_at
		FROM projects
		WHERE id = $1
	`, id).Scan(&project.ID, &project.Name, &project.Status, &project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "project not found")
		}
		return nil, err
	}

	return &project, nil
}

func (r *ProjectRepository) Create(ctx context.Context, project *entity.Project) (*entity.Project, error) {
	var created entity.Project
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO projects (name, status, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		RETURNING id, name, status, created_at, updated_at
	`, project.Name, project.Status).Scan(&created.ID, &created.Name, &created.Status, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &created, nil
}

func (r *ProjectRepository) Update(ctx context.Context, id int64, project *entity.Project) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE projects
		SET name = $1, status = $2, updated_at = NOW()
		WHERE id = $3
	`, project.Name, project.Status, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(domain.ErrNotFound, "project not found")
	}

	return nil
}

func (r *ProjectRepository) Delete(ctx context.Context, id int64) error {
	// Dihapus, bukan ditandai. Tidak ada tabel lain yang merujuk projects, jadi
	// tidak ada silsilah yang perlu dijaga.
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM projects WHERE id = $1
	`, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(domain.ErrNotFound, "project not found")
	}

	return nil
}

// ── Peserta proyek ──────────────────────────────────────────────────────────

func projectCompanySelectQuery() string {
	return `
		SELECT
			pc.id::text, pc.project_id, pc.company_id::text, pc.role,
			COALESCE(pc.note, ''), pc.created_at, pc.updated_at,
			c.code, c.name, c.legal_name, c.status
		FROM project_companies pc
		JOIN companies c ON c.id = pc.company_id
	`
}

func scanProjectCompany(row interface{ Scan(...any) error }) (*entity.ProjectCompany, error) {
	var (
		item    entity.ProjectCompany
		company entity.Company
	)
	if err := row.Scan(
		&item.ID, &item.ProjectID, &item.CompanyID, &item.Role,
		&item.Note, &item.CreatedAt, &item.UpdatedAt,
		&company.Code, &company.Name, &company.LegalName, &company.Status,
	); err != nil {
		return nil, err
	}
	company.ID = item.CompanyID
	item.Company = &company

	return &item, nil
}

func (r *ProjectRepository) ListCompanies(ctx context.Context, projectID int64) ([]entity.ProjectCompany, error) {
	// Menurut peran lalu nama, bukan menurut waktu didaftarkan: yang dibaca
	// orang di layar adalah "siapa pelanggannya", dan itu tidak ada hubungannya
	// dengan urutan seseorang mengetiknya.
	rows, err := r.db.QueryContext(ctx, projectCompanySelectQuery()+`
		WHERE pc.project_id = $1
		ORDER BY pc.role, LOWER(c.name)
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]entity.ProjectCompany, 0)
	for rows.Next() {
		item, err := scanProjectCompany(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}

	return items, rows.Err()
}

func (r *ProjectRepository) GetCompanyByID(ctx context.Context, id string) (*entity.ProjectCompany, error) {
	item, err := scanProjectCompany(r.db.QueryRowContext(ctx, projectCompanySelectQuery()+` WHERE pc.id = $1::uuid`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "project company not found")
		}

		return nil, err
	}

	return item, nil
}

func (r *ProjectRepository) AddCompany(ctx context.Context, company *entity.ProjectCompany) (*entity.ProjectCompany, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO project_companies (project_id, company_id, role, note)
		VALUES ($1, $2::uuid, $3, $4)
		RETURNING id::text
	`, company.ProjectID, company.CompanyID, company.Role, company.Note).Scan(&id)
	if err != nil {
		return nil, translateProjectConflict(err)
	}

	return r.GetCompanyByID(ctx, id)
}

func (r *ProjectRepository) UpdateCompany(ctx context.Context, id string, company *entity.ProjectCompany) error {
	// company_id sengaja TIDAK ikut diperbarui. Memindahkan keterlibatan ke
	// perusahaan lain bukan penyuntingan melainkan penggantian; membiarkannya
	// membuat satu baris diam-diam berubah menunjuk pihak yang berbeda,
	// membawa serta catatan yang ditulis untuk pihak sebelumnya.
	result, err := r.db.ExecContext(ctx, `
		UPDATE project_companies SET role = $1, note = $2 WHERE id = $3::uuid
	`, company.Role, company.Note, id)
	if err != nil {
		return translateProjectConflict(err)
	}

	return ensureProjectAffected(result, "project company not found")
}

func (r *ProjectRepository) RemoveCompany(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM project_companies WHERE id = $1::uuid`, id)
	if err != nil {
		return err
	}

	return ensureProjectAffected(result, "project company not found")
}

func (r *ProjectRepository) HasCompany(ctx context.Context, projectID int64, companyID string) (bool, error) {
	var ada bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM project_companies WHERE project_id = $1 AND company_id = $2::uuid
		)
	`, projectID, companyID).Scan(&ada)

	return ada, err
}

// ── Lampiran ────────────────────────────────────────────────────────────────

func projectAttachmentSelectQuery() string {
	return `
		SELECT
			pa.id::text, pa.project_id, pa.asset_id, pa.kind,
			pa.company_id::text, COALESCE(pa.note, ''), pa.created_at, pa.updated_at,
			a.token::text, a.object_name, a.original_filename, a.mime_type, a.size, a.status,
			c.code, c.name
		FROM project_attachments pa
		JOIN assets a ON a.id = pa.asset_id
		LEFT JOIN companies c ON c.id = pa.company_id
	`
}

func scanProjectAttachment(row interface{ Scan(...any) error }) (*entity.ProjectAttachment, error) {
	var (
		item        entity.ProjectAttachment
		asset       entity.Asset
		companyID   sql.NullString
		companyCode sql.NullString
		companyName sql.NullString
	)
	if err := row.Scan(
		&item.ID, &item.ProjectID, &item.AssetID, &item.Kind,
		&companyID, &item.Note, &item.CreatedAt, &item.UpdatedAt,
		&asset.Token, &asset.ObjectName, &asset.OriginalFilename,
		&asset.MimeType, &asset.Size, &asset.Status,
		&companyCode, &companyName,
	); err != nil {
		return nil, err
	}

	asset.ID = item.AssetID
	item.Asset = &asset
	if companyID.Valid {
		item.CompanyID = &companyID.String
		item.Company = &entity.Company{
			ID:   companyID.String,
			Code: companyCode.String,
			Name: companyName.String,
		}
	}

	return &item, nil
}

func (r *ProjectRepository) ListAttachments(ctx context.Context, projectID int64) ([]entity.ProjectAttachment, error) {
	// Terbaru lebih dulu. Lampiran adalah catatan yang bertambah seiring
	// pekerjaan berjalan, dan yang dicari orang hampir selalu yang paling akhir
	// masuk.
	rows, err := r.db.QueryContext(ctx, projectAttachmentSelectQuery()+`
		WHERE pa.project_id = $1
		ORDER BY pa.created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]entity.ProjectAttachment, 0)
	for rows.Next() {
		item, err := scanProjectAttachment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}

	return items, rows.Err()
}

func (r *ProjectRepository) GetAttachmentByID(ctx context.Context, id string) (*entity.ProjectAttachment, error) {
	item, err := scanProjectAttachment(r.db.QueryRowContext(ctx, projectAttachmentSelectQuery()+` WHERE pa.id = $1::uuid`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "project attachment not found")
		}

		return nil, err
	}

	return item, nil
}

func (r *ProjectRepository) AddAttachment(ctx context.Context, attachment *entity.ProjectAttachment) (*entity.ProjectAttachment, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO project_attachments (project_id, asset_id, kind, company_id, note)
		VALUES ($1, $2, $3, $4::uuid, $5)
		RETURNING id::text
	`, attachment.ProjectID, attachment.AssetID, attachment.Kind,
		attachment.CompanyID, attachment.Note).Scan(&id)
	if err != nil {
		return nil, translateProjectConflict(err)
	}

	return r.GetAttachmentByID(ctx, id)
}

func (r *ProjectRepository) UpdateAttachment(ctx context.Context, id string, attachment *entity.ProjectAttachment) error {
	// asset_id tidak ikut, dengan alasan yang sama seperti company_id pada
	// peserta: mengganti berkas di balik sebuah lampiran adalah lampiran yang
	// berbeda, bukan lampiran yang sama dengan isi baru.
	result, err := r.db.ExecContext(ctx, `
		UPDATE project_attachments
		SET kind = $1, company_id = $2::uuid, note = $3
		WHERE id = $4::uuid
	`, attachment.Kind, attachment.CompanyID, attachment.Note, id)
	if err != nil {
		return translateProjectConflict(err)
	}

	return ensureProjectAffected(result, "project attachment not found")
}

// translateProjectConflict mengubah pelanggaran indeks unik dan foreign key
// menjadi galat domain, supaya klien menerima kalimat yang menyebut sebabnya
// alih-alih 500 yang membocorkan nama constraint.
//
// Foreign key ikut diterjemahkan karena perusahaan yang UUID-nya sah tetapi
// tidak ada tidak dapat dicegah lebih awal tanpa membaca tabel companies dari
// usecase proyek — dan pembacaan itu pun tetap menyisakan celah: barisnya dapat
// hilang di antara pemeriksaan dan penyisipan. Database yang menjawab, dan
// jawabannya diterjemahkan di sini.
func translateProjectConflict(err error) error {
	var pgErr *pq.Error
	if !errors.As(err, &pgErr) {
		return err
	}

	switch pgErr.Code {
	case "23505":
		switch pgErr.Constraint {
		case "project_companies_uq_idx":
			return domain.NewError(domain.ErrConflict, "company already has this role in the project")
		case "project_attachments_asset_uq_idx":
			return domain.NewError(domain.ErrConflict, "file is already attached to a project")
		}
	case "23503":
		switch pgErr.Constraint {
		case "fk_project_companies_company", "fk_project_attachments_company":
			return domain.NewError(domain.ErrNotFound, "company not found")
		case "fk_project_attachments_asset":
			return domain.NewError(domain.ErrNotFound, "asset not found")
		case "fk_project_companies_project", "fk_project_attachments_project":
			return domain.NewError(domain.ErrNotFound, "project not found")
		}
	}

	return err
}

func ensureProjectAffected(result sql.Result, pesan string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(domain.ErrNotFound, pesan)
	}

	return nil
}
