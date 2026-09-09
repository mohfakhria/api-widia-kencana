package dto

import (
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type ProjectRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
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
	CreatedAt   time.Time                   `json:"created_at"`
	UpdatedAt   time.Time                   `json:"updated_at"`
}

type ProjectCompanyRequest struct {
	// CompanyID hanya dibaca saat menambah. Memindahkan keterlibatan ke
	// perusahaan lain bukan penyuntingan melainkan penggantian.
	CompanyID string `json:"company_id"`
	Role      string `json:"role"`
	Note      string `json:"note"`
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

type ProjectCompanyDataResponse struct {
	Company ProjectCompanyResponse `json:"project_company"`
}

type ProjectAttachmentDataResponse struct {
	Attachment ProjectAttachmentResponse `json:"project_attachment"`
}

func (r ProjectCompanyRequest) ToProjectCompanyCommand() input.ProjectCompanyCommand {
	return input.ProjectCompanyCommand{CompanyID: r.CompanyID, Role: r.Role, Note: r.Note}
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
		Name:   r.Name,
		Status: r.Status,
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
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
	}

	for index := range project.Companies {
		response.Companies = append(response.Companies, NewProjectCompanyResponse(&project.Companies[index]))
	}
	for index := range project.Attachments {
		response.Attachments = append(response.Attachments, NewProjectAttachmentResponse(&project.Attachments[index]))
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
