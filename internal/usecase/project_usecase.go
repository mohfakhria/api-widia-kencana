package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

const defaultProjectStatus = "active"

var allowedProjectStatuses = map[string]struct{}{
	"active":    {},
	"inactive":  {},
	"decline":   {},
	"completed": {},
}

// allowedProjectRoles dan allowedAttachmentKinds mengulang CHECK di migrasinya.
//
// Diulang dengan sengaja: yang di database adalah jaring terakhir, yang di sini
// yang menghasilkan pesan yang dapat dibaca orang. Tanpa yang di sini, peran
// salah ketik dijawab galat constraint Postgres — 500 yang menyebut nama indeks
// dan tidak menyebutkan nilai apa yang sebenarnya boleh.
// projectRoleCustomer disebut namanya karena ia satu-satunya peran yang
// diperlakukan khusus: daftar dan detail proyek menyajikan peserta ber-peran
// ini sebagai field customers tersendiri.
const projectRoleCustomer = "customer"

var allowedProjectRoles = map[string]struct{}{
	projectRoleCustomer: {},
	"end-user":          {},
	"supplier":          {},
}

var allowedAttachmentKinds = map[string]struct{}{
	"contract": {}, "site-photo": {}, "tax-invoice": {},
}

type projectUseCase struct {
	repo      output.ProjectRepository
	assets    output.AssetRepository
	documents output.DocumentRepository
	storage   output.ObjectStorage
	logger    *slog.Logger
}

// Menerima penyimpanan aset karena MENGHAPUS LAMPIRAN ikut menghapus berkasnya.
// Satu berkas milik satu proyek, jadi tidak ada yang perlu dihitung dulu — dan
// membiarkan berkasnya berarti objek yatim yang tidak dapat ditemukan siapa pun
// karena barisnya sudah tidak menunjuk ke sana.
func NewProjectUseCase(
	repo output.ProjectRepository,
	assets output.AssetRepository,
	documents output.DocumentRepository,
	storage output.ObjectStorage,
	logger *slog.Logger,
) input.ProjectUseCase {
	if logger == nil {
		logger = slog.Default()
	}

	// Tujuh jenis pertama sama persis dengan document_type — PO yang datang dari
	// pelanggan tetap sebuah purchase order, CV yang dikirim pemasok tetap sebuah
	// CV — jadi diturunkan, bukan ditulis ulang. Dua daftar yang menyebut hal yang
	// sama pasti berselisih suatu hari.
	//
	// Turunan ini membuat allowedAttachmentKinds ikut bertambah sendiri, tetapi
	// project_attachments_kind_chk di database TIDAK. Jenis dokumen baru menuntut
	// ALTER TABLE di sana pada saat yang sama — lihat README.
	for documentType := range allowedDocumentTypes {
		allowedAttachmentKinds[documentType] = struct{}{}
	}

	return &projectUseCase{
		repo: repo, assets: assets, documents: documents, storage: storage, logger: logger,
	}
}

func (uc *projectUseCase) List(ctx context.Context) ([]entity.Project, error) {
	return uc.repo.List(ctx)
}

func (uc *projectUseCase) GetByID(ctx context.Context, id string) (*entity.Project, error) {
	projectID, err := parseProjectID(id)
	if err != nil {
		return nil, err
	}

	project, err := uc.repo.GetByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if project.Companies, err = uc.repo.ListCompanies(ctx, projectID); err != nil {
		return nil, err
	}
	// Customers DITURUNKAN dari Companies, bukan dibaca kedua kalinya — dua
	// pembacaan atas hal yang sama boleh jadi tidak sepakat di antara keduanya.
	// Daftar proyek mengisinya sendiri di repository, karena di sana Companies
	// memang sengaja tidak dimuat.
	for _, participant := range project.Companies {
		if participant.Role == projectRoleCustomer && participant.Company != nil {
			project.Customers = append(project.Customers, *participant.Company)
		}
	}
	if project.Attachments, err = uc.repo.ListAttachments(ctx, projectID); err != nil {
		return nil, err
	}
	if project.Documents, err = uc.repo.ListDocuments(ctx, projectID); err != nil {
		return nil, err
	}
	if project.Milestones, err = uc.repo.ListMilestones(ctx, projectID); err != nil {
		return nil, err
	}

	return project, nil
}

func (uc *projectUseCase) Create(ctx context.Context, cmd input.CreateProjectCommand) (*entity.Project, error) {
	project := mapProjectCommand(cmd)
	if err := validateProject(project); err != nil {
		return nil, err
	}

	return uc.repo.Create(ctx, project)
}

func (uc *projectUseCase) Update(ctx context.Context, id string, cmd input.UpdateProjectCommand) error {
	projectID, err := parseProjectID(id)
	if err != nil {
		return err
	}

	project := mapProjectCommand(input.CreateProjectCommand(cmd))
	if err := validateProject(project); err != nil {
		return err
	}

	return uc.repo.Update(ctx, projectID, project)
}

// Delete membuang berkas setiap lampirannya lebih dulu, baru proyeknya.
//
// ON DELETE CASCADE hanya sampai pada BARIS project_attachments; ia tidak dapat
// menyentuh baris assets, apalagi objek di MinIO. Menghapus proyek tanpa
// langkah ini meninggalkan berkas yang objek dan baris asetnya masih ada tetapi
// tidak lagi ditunjuk siapa pun — tidak muncul di daftar mana pun, dan tidak
// ada satu pun cara menemukannya kembali selain menyisir bucket.
//
// Gagal di tengah jalan sengaja MEMBATALKAN sisanya. Proyeknya masih ada,
// lampiran yang belum sempat dibuang masih tercatat, dan permintaan yang sama
// dapat diulang; yang sudah terlanjur hilang tidak dapat dikembalikan.
func (uc *projectUseCase) Delete(ctx context.Context, id string) error {
	projectID, err := parseProjectID(id)
	if err != nil {
		return err
	}
	if _, err := uc.repo.GetByID(ctx, projectID); err != nil {
		return err
	}

	attachments, err := uc.repo.ListAttachments(ctx, projectID)
	if err != nil {
		return err
	}
	for i := range attachments {
		if err := uc.removeAttachmentFile(ctx, &attachments[i]); err != nil {
			return err
		}
	}

	return uc.repo.Delete(ctx, projectID)
}

func mapProjectCommand(cmd input.CreateProjectCommand) *entity.Project {
	status := strings.TrimSpace(cmd.Status)
	if status == "" {
		status = defaultProjectStatus
	}

	return &entity.Project{
		Name:      strings.TrimSpace(cmd.Name),
		Status:    strings.ToLower(status),
		Variables: cmd.Variables,
	}
}

func validateProject(project *entity.Project) error {
	if project.Name == "" {
		return domain.NewError(domain.ErrInvalidInput, "project name cannot be empty")
	}
	if _, ok := allowedProjectStatuses[project.Status]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid project status")
	}

	// Penjaga yang sama dengan kantong dokumen, hanya berbeda kunci bertipenya.
	// Ia yang menolak project_value negatif — indeks ekspresi di database sudah
	// menolak yang bukan angka, tetapi negatif baginya angka yang sah.
	return projectVariableRules.validate(project.Variables)
}

func parseProjectID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.NewError(domain.ErrInvalidInput, "invalid project id")
	}

	return id, nil
}

// ── Peserta proyek ──────────────────────────────────────────────────────────

func (uc *projectUseCase) AddCompany(ctx context.Context, projectID string, cmd input.ProjectCompanyCommand) (*entity.ProjectCompany, error) {
	id, err := parseProjectID(projectID)
	if err != nil {
		return nil, err
	}

	// Proyeknya dipastikan ada lebih dulu supaya jawabannya "project not found",
	// bukan pelanggaran foreign key yang menyebut nama constraint.
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}

	role := strings.ToLower(strings.TrimSpace(cmd.Role))
	if _, ok := allowedProjectRoles[role]; !ok {
		return nil, domain.NewError(domain.ErrInvalidInput, "invalid project company role")
	}

	companyID, err := parseUUID(cmd.CompanyID, "company id")
	if err != nil {
		return nil, err
	}

	return uc.repo.AddCompany(ctx, &entity.ProjectCompany{
		ProjectID: id,
		CompanyID: companyID,
		Role:      role,
		Note:      strings.TrimSpace(cmd.Note),
	})
}

func (uc *projectUseCase) UpdateCompany(ctx context.Context, id string, cmd input.ProjectCompanyCommand) error {
	participationID, err := parseUUID(id, "project company id")
	if err != nil {
		return err
	}

	role := strings.ToLower(strings.TrimSpace(cmd.Role))
	if _, ok := allowedProjectRoles[role]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid project company role")
	}

	return uc.repo.UpdateCompany(ctx, participationID, &entity.ProjectCompany{
		Role: role,
		Note: strings.TrimSpace(cmd.Note),
	})
}

func (uc *projectUseCase) RemoveCompany(ctx context.Context, id string) error {
	participationID, err := parseUUID(id, "project company id")
	if err != nil {
		return err
	}

	return uc.repo.RemoveCompany(ctx, participationID)
}

// ── Lampiran ────────────────────────────────────────────────────────────────

func (uc *projectUseCase) AddAttachment(ctx context.Context, projectID string, cmd input.ProjectAttachmentCommand) (*entity.ProjectAttachment, error) {
	id, err := parseProjectID(projectID)
	if err != nil {
		return nil, err
	}
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}

	kind, err := validateAttachmentKind(cmd.Kind)
	if err != nil {
		return nil, err
	}

	asset, err := uc.assets.GetByToken(ctx, strings.TrimSpace(cmd.AssetToken))
	if err != nil {
		return nil, err
	}

	// Hanya berkas yang isinya memang sudah ada. Menautkan unggahan yang belum
	// selesai menghasilkan lampiran yang tidak dapat dibuka siapa pun — dan
	// yang lebih buruk, penyapu aset akan membuangnya lima belas menit kemudian
	// beserta tautannya.
	if asset.Status != assetStatusUploaded {
		return nil, domain.NewError(domain.ErrInvalidInput, "asset is not uploaded")
	}

	companyID, err := uc.resolveAttachmentCompany(ctx, id, cmd.CompanyID)
	if err != nil {
		return nil, err
	}

	return uc.repo.AddAttachment(ctx, &entity.ProjectAttachment{
		ProjectID: id,
		AssetID:   asset.ID,
		Kind:      kind,
		CompanyID: companyID,
		Note:      strings.TrimSpace(cmd.Note),
	})
}

func (uc *projectUseCase) UpdateAttachment(ctx context.Context, id string, cmd input.ProjectAttachmentCommand) error {
	attachmentID, err := parseUUID(id, "project attachment id")
	if err != nil {
		return err
	}

	existing, err := uc.repo.GetAttachmentByID(ctx, attachmentID)
	if err != nil {
		return err
	}

	kind, err := validateAttachmentKind(cmd.Kind)
	if err != nil {
		return err
	}

	companyID, err := uc.resolveAttachmentCompany(ctx, existing.ProjectID, cmd.CompanyID)
	if err != nil {
		return err
	}

	return uc.repo.UpdateAttachment(ctx, attachmentID, &entity.ProjectAttachment{
		Kind:      kind,
		CompanyID: companyID,
		Note:      strings.TrimSpace(cmd.Note),
	})
}

// RemoveAttachment ikut membuang BERKASNYA, bukan hanya tautannya. Satu berkas
// milik satu proyek — lihat indeks unik pada asset_id — jadi tidak ada pemakai
// lain yang perlu dihitung lebih dulu.
func (uc *projectUseCase) RemoveAttachment(ctx context.Context, id string) error {
	attachmentID, err := parseUUID(id, "project attachment id")
	if err != nil {
		return err
	}

	attachment, err := uc.repo.GetAttachmentByID(ctx, attachmentID)
	if err != nil {
		return err
	}

	return uc.removeAttachmentFile(ctx, attachment)
}

// removeAttachmentFile membuang objeknya lebih dulu, baru baris asetnya.
//
// URUTAN INI YANG MENENTUKAN BENAR-TIDAKNYA, sama seperti AssetSweeper.
// Menghapus baris lebih dulu lalu gagal membuang objeknya berarti objek itu
// yatim selamanya — tanpa satu pun baris yang menunjuk ke sana, tidak ada yang
// bisa menemukannya kembali. Terbalik begini, kegagalan hanya menyisakan
// lampiran yang berkasnya sudah tidak ada, dan itu terlihat.
//
// Baris lampirannya sendiri lenyap lewat ON DELETE CASCADE begitu asetnya
// hilang; tidak ada penghapusan kedua di sini. Dipakai bersama oleh
// RemoveAttachment dan Delete supaya urutan itu hanya ditulis di satu tempat.
func (uc *projectUseCase) removeAttachmentFile(ctx context.Context, attachment *entity.ProjectAttachment) error {
	if attachment.Asset == nil {
		return nil
	}

	if uc.storage != nil {
		if err := uc.storage.Delete(ctx, attachment.Asset.ObjectName); err != nil {
			uc.logger.Error("gagal menghapus berkas lampiran",
				"object_name", attachment.Asset.ObjectName, "error", err)

			return domain.NewError(domain.ErrInternalFailure, "attachment file could not be removed")
		}
	}

	return uc.assets.Delete(ctx, attachment.Asset.Token)
}

// ── Dokumen proyek ──────────────────────────────────────────────────────────

// AddDocument MENGAITKAN dokumen yang sudah ada, bukan membuatnya.
//
// Dokumen dibuat lewat document-add dan disunting di editor; yang dikirim ke
// sini hanya tokennya. Urutan itu memang begitu di lapangan: penawaran dibuat
// lebih dulu, proyeknya menyusul ketika PO datang.
func (uc *projectUseCase) AddDocument(ctx context.Context, projectID string, cmd input.ProjectDocumentCommand) (*entity.ProjectDocument, error) {
	id, err := parseProjectID(projectID)
	if err != nil {
		return nil, err
	}

	// Proyeknya dipastikan ada lebih dulu supaya jawabannya "project not found",
	// bukan pelanggaran foreign key yang menyebut nama constraint.
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}

	token := strings.TrimSpace(cmd.DocumentToken)
	if err := validateUUIDToken(token, "document token"); err != nil {
		return nil, err
	}

	document, err := uc.documents.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	return uc.repo.AddDocument(ctx, &entity.ProjectDocument{
		ProjectID:  id,
		DocumentID: document.ID,
		Note:       strings.TrimSpace(cmd.Note),
	})
}

func (uc *projectUseCase) UpdateDocument(ctx context.Context, id string, cmd input.ProjectDocumentCommand) error {
	documentID, err := parseUUID(id, "project document id")
	if err != nil {
		return err
	}

	return uc.repo.UpdateDocument(ctx, documentID, &entity.ProjectDocument{
		Note: strings.TrimSpace(cmd.Note),
	})
}

// RemoveDocument HANYA memutus kaitannya; dokumennya tetap hidup.
//
// Sengaja tidak menyerupai RemoveAttachment, yang ikut membuang berkasnya.
// Lampiran adalah berkas yang DITERIMA dan tidak punya hidup di luar proyeknya;
// dokumen DIBUAT sendiri di editor, punya riwayat, induk, dan isinya sendiri.
// Melepasnya dari proyek yang keliru tidak boleh berarti kehilangan pekerjaan —
// dan yang benar-benar ingin membuangnya memanggil document-delete.
func (uc *projectUseCase) RemoveDocument(ctx context.Context, id string) error {
	documentID, err := parseUUID(id, "project document id")
	if err != nil {
		return err
	}

	return uc.repo.RemoveDocument(ctx, documentID)
}

// ── Tonggak proyek ──────────────────────────────────────────────────────────

// maxMilestoneLength sejalan dengan VARCHAR(60) di kolomnya. Yang lebih panjang
// ditolak di sini supaya jawabannya 400 yang menyebut batasnya, bukan galat
// Postgres mentah yang sampai ke klien sebagai 500.
const maxMilestoneLength = 60

// milestoneSuggestions adalah tonggak yang lazim dipakai, urut sesuai alur
// pekerjaan.
//
// SARAN, BUKAN ATURAN. milestone adalah teks bebas, dan yang di luar daftar ini
// tetap diterima — alur tiap proyek tidak sama, dan kosakata tertutup memaksa
// setiap pekerjaan tak lazim menunggu ALTER TABLE.
//
// Hidup di kode, bukan di database, supaya berubah tanpa migrasi. Disajikan
// lewat endpoint supaya frontend tidak menyalinnya lalu ketinggalan.
var milestoneSuggestions = []input.MilestoneSuggestion{
	{Milestone: "penawaran-terkirim", Description: "Penawaran sampai ke pelanggan, bukan draft di editor"},
	{Milestone: "po-diterima", Description: "PO pelanggan diterima"},
	{Milestone: "pekerjaan-dimulai", Description: "Produksi atau pekerjaan lapangan berjalan"},
	{Milestone: "barang-diserahkan", Description: "Serah terima fisik di lokasi"},
	{Milestone: "bast-ditandatangani", Description: "Pelanggan menerima secara resmi"},
	// Sengaja terpisah dari lunas: jarak antara keduanya yang paling sering
	// dikejar orang, dan digabung ia tidak dapat direkonstruksi.
	{Milestone: "faktur-terbit", Description: "Tagihan dikirim ke pelanggan"},
	{Milestone: "lunas", Description: "Pembayaran diterima penuh"},
}

func (uc *projectUseCase) MilestoneSuggestions() []input.MilestoneSuggestion {
	return milestoneSuggestions
}

func (uc *projectUseCase) AddMilestone(ctx context.Context, projectID string, cmd input.ProjectMilestoneCommand) (*entity.ProjectMilestone, error) {
	id, err := parseProjectID(projectID)
	if err != nil {
		return nil, err
	}
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}

	milestone, err := mapMilestoneCommand(cmd)
	if err != nil {
		return nil, err
	}
	milestone.ProjectID = id

	return uc.repo.AddMilestone(ctx, milestone)
}

func (uc *projectUseCase) UpdateMilestone(ctx context.Context, id string, cmd input.ProjectMilestoneCommand) error {
	milestoneID, err := parseUUID(id, "project milestone id")
	if err != nil {
		return err
	}

	milestone, err := mapMilestoneCommand(cmd)
	if err != nil {
		return err
	}

	return uc.repo.UpdateMilestone(ctx, milestoneID, milestone)
}

func (uc *projectUseCase) RemoveMilestone(ctx context.Context, id string) error {
	milestoneID, err := parseUUID(id, "project milestone id")
	if err != nil {
		return err
	}

	return uc.repo.RemoveMilestone(ctx, milestoneID)
}

// mapMilestoneCommand MENORMALKAN nama tonggaknya, dan itu bagian terpenting
// dari seluruh berkas ini.
//
// milestone adalah teks bebas yang dipakai sebagai penyaring lintas proyek.
// Tanpa penyatuan ejaan, 'BAST', 'B.A.S.T', dan 'Bast' menjadi tiga tonggak
// berbeda — dan saringan "proyek yang sudah BAST" mengembalikan sepertiganya
// tanpa ada yang menyadari dua pertiganya hilang. Database tidak dapat
// mencegahnya; di sinilah satu-satunya tempat yang bisa.
func mapMilestoneCommand(cmd input.ProjectMilestoneCommand) (*entity.ProjectMilestone, error) {
	milestone := slugify(cmd.Milestone)
	if milestone == "" {
		return nil, domain.NewError(domain.ErrInvalidInput, "milestone has no usable characters")
	}
	if len(milestone) > maxMilestoneLength {
		return nil, domain.NewError(domain.ErrInvalidInput,
			fmt.Sprintf("milestone cannot exceed %d characters", maxMilestoneLength))
	}

	// Kosong berarti sekarang. Waktu yang dikirim TIDAK dibatasi harus masa lalu:
	// tonggak yang dijadwalkan — jatuh tempo faktur, rencana serah terima —
	// adalah pemakaian yang sah, dan menolaknya berarti memaksa orang menunggu
	// tanggalnya lewat sebelum boleh mencatatnya.
	reachedAt := time.Now()
	if cmd.ReachedAt != nil {
		reachedAt = *cmd.ReachedAt
	}

	return &entity.ProjectMilestone{
		Milestone: milestone,
		ReachedAt: reachedAt,
		Note:      strings.TrimSpace(cmd.Note),
	}, nil
}

// resolveAttachmentCompany memastikan pemilik berkas memang peserta proyek itu.
//
// Kosong berarti tidak milik siapa-siapa — foto lapangan, misalnya — dan itu
// keadaan yang sah.
//
// Aturannya di sini, bukan di database: menjaminnya dengan foreign key gabungan
// menuntut (project_id, company_id) unik pada project_companies, dan itu berarti
// satu perusahaan tidak lagi boleh memegang dua peran. Lihat catatannya di
// migration/project_attachments.sql.
func (uc *projectUseCase) resolveAttachmentCompany(ctx context.Context, projectID int64, raw string) (*string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	companyID, err := parseUUID(raw, "company id")
	if err != nil {
		return nil, err
	}

	peserta, err := uc.repo.HasCompany(ctx, projectID, companyID)
	if err != nil {
		return nil, err
	}
	if !peserta {
		return nil, domain.NewError(domain.ErrInvalidInput, "company is not involved in this project")
	}

	return &companyID, nil
}

func validateAttachmentKind(raw string) (string, error) {
	kind := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := allowedAttachmentKinds[kind]; !ok {
		return "", domain.NewError(domain.ErrInvalidInput, "invalid attachment kind")
	}

	return kind, nil
}

// parseUUID memastikan yang datang memang UUID, supaya yang bukan dijawab 400
// yang menyebut fieldnya — bukan 500 dari Postgres yang menyebut galat sintaks
// tipe.
func parseUUID(raw, label string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", domain.NewError(domain.ErrInvalidInput, label+" is required")
	}
	if _, err := uuid.Parse(id); err != nil {
		return "", domain.NewError(domain.ErrInvalidInput, "invalid "+label)
	}

	return id, nil
}
