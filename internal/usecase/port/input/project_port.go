package input

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type ProjectUseCase interface {
	List(ctx context.Context) ([]entity.Project, error)

	// GetByID menyertakan peserta dan lampirannya; List tidak. Lihat alasannya
	// di entity.Project.
	GetByID(ctx context.Context, id string) (*entity.Project, error)
	Create(ctx context.Context, cmd CreateProjectCommand) (*entity.Project, error)
	Update(ctx context.Context, id string, cmd UpdateProjectCommand) error
	Delete(ctx context.Context, id string) error

	AddCompany(ctx context.Context, projectID string, cmd ProjectCompanyCommand) (*entity.ProjectCompany, error)
	UpdateCompany(ctx context.Context, id string, cmd ProjectCompanyCommand) error
	RemoveCompany(ctx context.Context, id string) error

	AddAttachment(ctx context.Context, projectID string, cmd ProjectAttachmentCommand) (*entity.ProjectAttachment, error)
	UpdateAttachment(ctx context.Context, id string, cmd ProjectAttachmentCommand) error

	// RemoveAttachment MENGHAPUS BERKASNYA JUGA, bukan sekadar melepas
	// tautannya. Satu berkas milik satu proyek — indeks unik pada asset_id yang
	// menjaminnya — sehingga tidak ada keraguan siapa lagi yang memakainya.
	RemoveAttachment(ctx context.Context, id string) error
}

type CreateProjectCommand struct {
	Name   string
	Status string
}

type UpdateProjectCommand = CreateProjectCommand

type ProjectCompanyCommand struct {
	// CompanyID hanya dibaca saat menambah. Memindahkan keterlibatan ke
	// perusahaan lain bukan penyuntingan melainkan penggantian: hapus lalu
	// tambahkan, supaya tidak ada baris yang diam-diam berubah menunjuk pihak
	// yang berbeda.
	CompanyID string
	Role      string
	Note      string
}

type ProjectAttachmentCommand struct {
	// AssetToken hanya dibaca saat menambah, dengan alasan yang sama seperti
	// CompanyID di atas.
	AssetToken string
	Kind       string
	// CompanyID kosong berarti tidak milik siapa-siapa — foto lapangan, misalnya.
	CompanyID string
	Note      string
}
