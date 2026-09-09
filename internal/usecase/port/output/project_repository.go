package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type ProjectRepository interface {
	List(ctx context.Context) ([]entity.Project, error)
	GetByID(ctx context.Context, id int64) (*entity.Project, error)
	Create(ctx context.Context, project *entity.Project) (*entity.Project, error)
	Update(ctx context.Context, id int64, project *entity.Project) error
	Delete(ctx context.Context, id int64) error

	ListCompanies(ctx context.Context, projectID int64) ([]entity.ProjectCompany, error)
	GetCompanyByID(ctx context.Context, id string) (*entity.ProjectCompany, error)
	AddCompany(ctx context.Context, company *entity.ProjectCompany) (*entity.ProjectCompany, error)
	UpdateCompany(ctx context.Context, id string, company *entity.ProjectCompany) error
	RemoveCompany(ctx context.Context, id string) error

	ListAttachments(ctx context.Context, projectID int64) ([]entity.ProjectAttachment, error)
	GetAttachmentByID(ctx context.Context, id string) (*entity.ProjectAttachment, error)
	AddAttachment(ctx context.Context, attachment *entity.ProjectAttachment) (*entity.ProjectAttachment, error)
	UpdateAttachment(ctx context.Context, id string, attachment *entity.ProjectAttachment) error

	ListDocuments(ctx context.Context, projectID int64) ([]entity.ProjectDocument, error)
	GetDocumentByID(ctx context.Context, id string) (*entity.ProjectDocument, error)
	AddDocument(ctx context.Context, document *entity.ProjectDocument) (*entity.ProjectDocument, error)
	UpdateDocument(ctx context.Context, id string, document *entity.ProjectDocument) error

	// RemoveDocument hanya memutus kaitannya. Dokumennya TIDAK dihapus —
	// berbeda dari lampiran, yang berkasnya ikut dibuang.
	RemoveDocument(ctx context.Context, id string) error

	// HasCompany menjawab apakah sebuah perusahaan benar-benar peserta proyek.
	//
	// Ada di sini, bukan sebagai foreign key, karena menjaminnya di database
	// menuntut (project_id, company_id) unik pada project_companies — dan itu
	// berarti satu perusahaan tidak lagi boleh memegang dua peran. Lihat
	// catatannya di migration/project_attachments.sql.
	HasCompany(ctx context.Context, projectID int64, companyID string) (bool, error)
}
