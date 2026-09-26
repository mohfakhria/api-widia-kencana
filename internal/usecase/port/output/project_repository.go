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

	// SumQuotationAmounts menjumlahkan grand_total SELURUH penawaran tiap
	// proyek, dikunci id proyeknya — satu kueri untuk seluruh daftar; proyek
	// tanpa penawaran ber-angka tidak punya baris, dan pembacanya memakai nol.
	// Penawaran tanpa grand_total dilewati, bukan dihitung nol. Revisi TIDAK
	// dikecualikan: seluruh status ikut terjumlah, apa pun keadaannya —
	// aturan sementara yang diminta pemilik repo. SumQuotationAmount melayani
	// satu proyek dengan aturan yang sama persis, untuk project-detail.
	SumQuotationAmounts(ctx context.Context) (map[int64]float64, error)
	SumQuotationAmount(ctx context.Context, projectID int64) (float64, error)

	// LatestMilestones mengembalikan nama tonggak TERAKHIR tiap proyek —
	// reached_at terbesar, created_at sebagai pemutus seri — satu kueri untuk
	// seluruh daftar. Proyek tanpa tonggak tidak punya baris. Detail tidak
	// memakai ini: tonggaknya sudah dimuat ListMilestones dengan urutan yang
	// pemutus serinya sama, jadi elemen terakhirnya adalah jawaban yang sama.
	LatestMilestones(ctx context.Context) (map[int64]string, error)

	ListDocuments(ctx context.Context, projectID int64) ([]entity.ProjectDocument, error)
	GetDocumentByID(ctx context.Context, id string) (*entity.ProjectDocument, error)
	AddDocument(ctx context.Context, document *entity.ProjectDocument) (*entity.ProjectDocument, error)
	UpdateDocument(ctx context.Context, id string, document *entity.ProjectDocument) error

	// RemoveDocument hanya memutus kaitannya. Dokumennya TIDAK dihapus —
	// berbeda dari lampiran, yang berkasnya ikut dibuang.
	RemoveDocument(ctx context.Context, id string) error

	ListMilestones(ctx context.Context, projectID int64) ([]entity.ProjectMilestone, error)
	AddMilestone(ctx context.Context, milestone *entity.ProjectMilestone) (*entity.ProjectMilestone, error)
	UpdateMilestone(ctx context.Context, id string, milestone *entity.ProjectMilestone) error
	RemoveMilestone(ctx context.Context, id string) error

	// HasCompany menjawab apakah sebuah perusahaan benar-benar peserta proyek.
	//
	// Ada di sini, bukan sebagai foreign key, karena menjaminnya di database
	// menuntut (project_id, company_id) unik pada project_companies — dan itu
	// berarti satu perusahaan tidak lagi boleh memegang dua peran. Lihat
	// catatannya di migration/project_attachments.sql.
	HasCompany(ctx context.Context, projectID int64, companyID string) (bool, error)
}
