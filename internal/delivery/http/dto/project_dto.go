package dto

import (
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type ProjectRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`

	// Variables adalah objek datar bernilai skalar, dan project_value tinggal di
	// dalamnya. DIISI ORANG — nilai yang mengikat ada pada PO pelanggan, yang
	// masuk sebagai berkas pindaian tanpa angka yang dapat dibaca mesin.
	//
	// MENIMPA seluruhnya, tidak menggabung: bidang yang tidak disertakan saat
	// update akan hilang. Kirim kantongnya selengkapnya.
	Variables map[string]any `json:"variables"`
}

type ProjectResponse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	// Keduanya hanya muncul pada project-detail. Pada daftar ia dihilangkan,
	// bukan dikirim kosong — larik kosong akan terbaca sebagai "proyek ini
	// memang tidak punya peserta".
	Companies   []ProjectCompanyResponse    `json:"companies,omitempty"`
	Attachments []ProjectAttachmentResponse `json:"attachments,omitempty"`
	Documents   []ProjectDocumentResponse   `json:"documents,omitempty"`
	Milestones  []ProjectMilestoneResponse  `json:"milestones,omitempty"`

	// Customers TANPA omitempty, berbeda dari keempat di atas, karena artinya
	// berbeda: yang di atas dihilangkan pada daftar sebab memang tidak dimuat,
	// sedangkan customers selalu dimuat — di daftar maupun detail — sehingga
	// larik kosong di sini benar-benar berarti "belum ada pelanggan terdaftar".
	// Boleh lebih dari satu: dua pelanggan pada satu proyek adalah keadaan sah.
	Customers []ProjectCompanyRefResponse `json:"customers"`

	// Variables selalu objek, tidak pernah null: kolomnya NOT NULL DEFAULT '{}',
	// dan klien yang melakukan iterasi atasnya gagal justru pada proyek yang
	// paling wajar — yang belum punya satu variabel pun.
	Variables map[string]any `json:"variables"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProjectCompanyRequest struct {
	// CompanyID hanya dibaca saat menambah. Memindahkan keterlibatan ke
	// perusahaan lain bukan penyuntingan melainkan penggantian.
	CompanyID string `json:"company_id"`
	Role      string `json:"role"`
	Note      string `json:"note"`
}

type ProjectMilestoneRequest struct {
	// Milestone TEKS BEBAS, dinormalkan backend: huruf dikecilkan, spasi dan
	// titik menjadi hubung. "BAST Ditandatangani" tersimpan sebagai
	// "bast-ditandatangani". Daftar saran ada di project-milestone-suggestions.
	Milestone string `json:"milestone"`
	// ReachedAt kosong berarti sekarang. Boleh mundur maupun maju.
	ReachedAt *time.Time `json:"reached_at"`
	Note      string     `json:"note"`
}

type ProjectDocumentRequest struct {
	// DocumentToken hanya dibaca saat menambah. Mengganti dokumen di balik
	// sebuah kaitan adalah kaitan yang berbeda, bukan kaitan yang sama dengan
	// isi baru.
	DocumentToken string `json:"document_token"`
	Note          string `json:"note"`
}

type ProjectAttachmentRequest struct {
	// AssetToken hanya dibaca saat menambah, dengan alasan yang sama.
	AssetToken string `json:"asset_token"`
	Kind       string `json:"kind"`
	// CompanyID kosong berarti tidak milik siapa-siapa — foto lapangan,
	// misalnya. Yang diisi harus perusahaan yang memang peserta proyek itu.
	CompanyID string `json:"company_id"`
	Note      string `json:"note"`
}

// ProjectCompanyRefResponse adalah perusahaan secukupnya untuk ditampilkan di
// dalam proyek — bukan seluruh barisnya.
//
// Alamat, kontak, dan tanggal berdirinya tidak dibutuhkan siapa pun di layar
// proyek, dan membawanya berarti setiap pembacaan proyek menyeret data yang
// tidak dilihat. Yang butuh selengkapnya membuka company-detail.
type ProjectCompanyRefResponse struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	LegalName string `json:"legal_name,omitempty"`
	Status    string `json:"status,omitempty"`
}

type ProjectCompanyResponse struct {
	ID        string                     `json:"id"`
	ProjectID int64                      `json:"project_id"`
	Role      string                     `json:"role"`
	Note      string                     `json:"note"`
	Company   *ProjectCompanyRefResponse `json:"company,omitempty"`
	CreatedAt time.Time                  `json:"created_at"`
	UpdatedAt time.Time                  `json:"updated_at"`
}

type ProjectAttachmentResponse struct {
	ID        string `json:"id"`
	ProjectID int64  `json:"project_id"`
	Kind      string `json:"kind"`
	Note      string `json:"note"`
	// AssetToken yang dipakai membuka berkasnya lewat asset-content. Id
	// numeriknya tidak pernah keluar.
	AssetToken       string                     `json:"asset_token"`
	OriginalFilename string                     `json:"original_filename"`
	MimeType         string                     `json:"mime_type"`
	Size             int64                      `json:"size"`
	Company          *ProjectCompanyRefResponse `json:"company,omitempty"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}

// ProjectDocumentResponse membawa dokumen SECUKUPNYA untuk ditampilkan di dalam
// proyek. Isi kanvasnya tidak ikut — yang membukanya memanggil document-detail.
type ProjectDocumentResponse struct {
	ID        string `json:"id"`
	ProjectID int64  `json:"project_id"`
	Note      string `json:"note"`

	// DocumentToken yang dipakai membuka dokumennya. Id numeriknya tidak pernah
	// keluar.
	DocumentToken string         `json:"document_token"`
	Name          string         `json:"name"`
	DocumentType  string         `json:"document_type"`
	Status        string         `json:"status"`
	Variables     map[string]any `json:"variables"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProjectMilestoneResponse struct {
	ID        string    `json:"id"`
	ProjectID int64     `json:"project_id"`
	Milestone string    `json:"milestone"`
	ReachedAt time.Time `json:"reached_at"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProjectMilestoneDataResponse struct {
	Milestone ProjectMilestoneResponse `json:"project_milestone"`
}

// MilestoneSuggestionResponse adalah SARAN, bukan kosakata tertutup. Tonggak di
// luar daftar ini tetap diterima — tampilkan sebagai pilihan yang dapat
// diketik sendiri, bukan sebagai dropdown tertutup.
type MilestoneSuggestionResponse struct {
	Milestone   string `json:"milestone"`
	Description string `json:"description"`
}

type MilestoneSuggestionListResponse struct {
	Milestones []MilestoneSuggestionResponse `json:"milestones"`
}

type ProjectDocumentDataResponse struct {
	Document ProjectDocumentResponse `json:"project_document"`
}

type ProjectCompanyDataResponse struct {
	Company ProjectCompanyResponse `json:"project_company"`
}

type ProjectAttachmentDataResponse struct {
	Attachment ProjectAttachmentResponse `json:"project_attachment"`
}

func (r ProjectCompanyRequest) ToProjectCompanyCommand() input.ProjectCompanyCommand {
	return input.ProjectCompanyCommand{CompanyID: r.CompanyID, Role: r.Role, Note: r.Note}
}

func (r ProjectMilestoneRequest) ToProjectMilestoneCommand() input.ProjectMilestoneCommand {
	return input.ProjectMilestoneCommand{
		Milestone: r.Milestone, ReachedAt: r.ReachedAt, Note: r.Note,
	}
}

func (r ProjectDocumentRequest) ToProjectDocumentCommand() input.ProjectDocumentCommand {
	return input.ProjectDocumentCommand{DocumentToken: r.DocumentToken, Note: r.Note}
}

func (r ProjectAttachmentRequest) ToProjectAttachmentCommand() input.ProjectAttachmentCommand {
	return input.ProjectAttachmentCommand{
		AssetToken: r.AssetToken, Kind: r.Kind, CompanyID: r.CompanyID, Note: r.Note,
	}
}

func newProjectCompanyRefResponse(company *entity.Company) *ProjectCompanyRefResponse {
	if company == nil {
		return nil
	}

	return &ProjectCompanyRefResponse{
		ID:        company.ID,
		Code:      company.Code,
		Name:      company.Name,
		LegalName: company.LegalName,
		Status:    company.Status,
	}
}

func NewProjectCompanyResponse(item *entity.ProjectCompany) ProjectCompanyResponse {
	return ProjectCompanyResponse{
		ID:        item.ID,
		ProjectID: item.ProjectID,
		Role:      item.Role,
		Note:      item.Note,
		Company:   newProjectCompanyRefResponse(item.Company),
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func NewProjectAttachmentResponse(item *entity.ProjectAttachment) ProjectAttachmentResponse {
	response := ProjectAttachmentResponse{
		ID:        item.ID,
		ProjectID: item.ProjectID,
		Kind:      item.Kind,
		Note:      item.Note,
		Company:   newProjectCompanyRefResponse(item.Company),
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}

	if item.Asset != nil {
		response.AssetToken = item.Asset.Token
		response.OriginalFilename = item.Asset.OriginalFilename
		response.MimeType = item.Asset.MimeType
		response.Size = item.Asset.Size
	}

	return response
}

func NewProjectDocumentResponse(item *entity.ProjectDocument) ProjectDocumentResponse {
	response := ProjectDocumentResponse{
		ID:        item.ID,
		ProjectID: item.ProjectID,
		Note:      item.Note,
		Variables: map[string]any{},
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}

	if item.Document != nil {
		response.DocumentToken = item.Document.Token
		response.Name = item.Document.Name
		response.DocumentType = item.Document.DocumentType
		response.Status = item.Document.Status
		response.Variables = variablesOf(item.Document.Variables)
	}

	return response
}

func NewProjectMilestoneResponse(item *entity.ProjectMilestone) ProjectMilestoneResponse {
	return ProjectMilestoneResponse{
		ID:        item.ID,
		ProjectID: item.ProjectID,
		Milestone: item.Milestone,
		ReachedAt: item.ReachedAt,
		Note:      item.Note,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func NewProjectMilestoneDataResponse(item *entity.ProjectMilestone) ProjectMilestoneDataResponse {
	return ProjectMilestoneDataResponse{Milestone: NewProjectMilestoneResponse(item)}
}

func NewMilestoneSuggestionListResponse(items []input.MilestoneSuggestion) MilestoneSuggestionListResponse {
	response := MilestoneSuggestionListResponse{
		Milestones: make([]MilestoneSuggestionResponse, 0, len(items)),
	}
	for _, item := range items {
		response.Milestones = append(response.Milestones, MilestoneSuggestionResponse{
			Milestone: item.Milestone, Description: item.Description,
		})
	}

	return response
}

func NewProjectDocumentDataResponse(item *entity.ProjectDocument) ProjectDocumentDataResponse {
	return ProjectDocumentDataResponse{Document: NewProjectDocumentResponse(item)}
}

func NewProjectCompanyDataResponse(item *entity.ProjectCompany) ProjectCompanyDataResponse {
	return ProjectCompanyDataResponse{Company: NewProjectCompanyResponse(item)}
}

func NewProjectAttachmentDataResponse(item *entity.ProjectAttachment) ProjectAttachmentDataResponse {
	return ProjectAttachmentDataResponse{Attachment: NewProjectAttachmentResponse(item)}
}

type ProjectDataResponse struct {
	Project ProjectResponse `json:"project"`
}

type ProjectListResponse struct {
	Projects []ProjectResponse `json:"projects"`
}

func (r ProjectRequest) ToCreateProjectCommand() input.CreateProjectCommand {
	return input.CreateProjectCommand{
		Name:      r.Name,
		Status:    r.Status,
		Variables: r.Variables,
	}
}

func (r ProjectRequest) ToUpdateProjectCommand() input.UpdateProjectCommand {
	return input.UpdateProjectCommand(r.ToCreateProjectCommand())
}

func NewProjectResponse(project *entity.Project) ProjectResponse {
	response := ProjectResponse{
		ID:        project.ID,
		Name:      project.Name,
		Status:    project.Status,
		Variables: variablesOf(project.Variables),
		// make, bukan nil: customers dijanjikan selalu larik, tidak pernah null.
		Customers: make([]ProjectCompanyRefResponse, 0, len(project.Customers)),
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
	}

	for index := range project.Customers {
		customer := &project.Customers[index]
		response.Customers = append(response.Customers, ProjectCompanyRefResponse{
			ID:        customer.ID,
			Code:      customer.Code,
			Name:      customer.Name,
			LegalName: customer.LegalName,
			Status:    customer.Status,
		})
	}

	for index := range project.Companies {
		response.Companies = append(response.Companies, NewProjectCompanyResponse(&project.Companies[index]))
	}
	for index := range project.Attachments {
		response.Attachments = append(response.Attachments, NewProjectAttachmentResponse(&project.Attachments[index]))
	}
	for index := range project.Documents {
		response.Documents = append(response.Documents, NewProjectDocumentResponse(&project.Documents[index]))
	}
	for index := range project.Milestones {
		response.Milestones = append(response.Milestones, NewProjectMilestoneResponse(&project.Milestones[index]))
	}

	return response
}

func NewProjectDataResponse(project *entity.Project) ProjectDataResponse {
	return ProjectDataResponse{Project: NewProjectResponse(project)}
}

func NewProjectListResponses(projects []entity.Project) ProjectListResponse {
	responses := ProjectListResponse{
		Projects: make([]ProjectResponse, 0, len(projects)),
	}
	for _, project := range projects {
		responses.Projects = append(responses.Projects, NewProjectResponse(&project))
	}

	return responses
}
