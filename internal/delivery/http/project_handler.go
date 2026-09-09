package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type ProjectHandler struct {
	project input.ProjectUseCase
}

func NewProjectHandler(project input.ProjectUseCase) *ProjectHandler {
	return &ProjectHandler{project: project}
}

func (h *ProjectHandler) List(c *gin.Context) {
	projects, err := h.project.List(c.Request.Context())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewProjectListResponses(projects))
}

func (h *ProjectHandler) Get(c *gin.Context) {
	project, err := h.project.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewProjectDataResponse(project))
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var req dto.ProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	project, err := h.project.Create(c.Request.Context(), req.ToCreateProjectCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project created successfully", dto.NewProjectDataResponse(project))
}

func (h *ProjectHandler) Update(c *gin.Context) {
	var req dto.ProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid payload")
		return
	}

	if err := h.project.Update(c.Request.Context(), c.Param("id"), req.ToUpdateProjectCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project updated successfully", nil)
}

func (h *ProjectHandler) Delete(c *gin.Context) {
	if err := h.project.Delete(c.Request.Context(), c.Param("id")); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project deleted successfully", nil)
}

// ── Peserta proyek ──────────────────────────────────────────────────────────

func (h *ProjectHandler) AddCompany(c *gin.Context) {
	var req dto.ProjectCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	item, err := h.project.AddCompany(c.Request.Context(), c.Param("id"), req.ToProjectCompanyCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project company added successfully", dto.NewProjectCompanyDataResponse(item))
}

// UpdateCompany dan RemoveCompany memakai id KETERLIBATAN, bukan id proyek —
// perusahaannya diambil dari baris keterlibatan itu sendiri.
func (h *ProjectHandler) UpdateCompany(c *gin.Context) {
	var req dto.ProjectCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.project.UpdateCompany(c.Request.Context(), c.Param("id"), req.ToProjectCompanyCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project company updated successfully", nil)
}

func (h *ProjectHandler) RemoveCompany(c *gin.Context) {
	if err := h.project.RemoveCompany(c.Request.Context(), c.Param("id")); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project company removed successfully", nil)
}

// ── Lampiran ────────────────────────────────────────────────────────────────

// AddAttachment MENAUTKAN berkas yang sudah diunggah, bukan menerimanya.
//
// Unggahannya memakai alur aset yang sudah ada — asset-upload-request dengan
// group 'documents/<jenis>', PUT ke presigned URL, lalu asset-upload-complete.
// Yang dikirim ke sini hanya tokennya.
func (h *ProjectHandler) AddAttachment(c *gin.Context) {
	var req dto.ProjectAttachmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	item, err := h.project.AddAttachment(c.Request.Context(), c.Param("id"), req.ToProjectAttachmentCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project attachment added successfully", dto.NewProjectAttachmentDataResponse(item))
}

func (h *ProjectHandler) UpdateAttachment(c *gin.Context) {
	var req dto.ProjectAttachmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.project.UpdateAttachment(c.Request.Context(), c.Param("id"), req.ToProjectAttachmentCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project attachment updated successfully", nil)
}

// RemoveAttachment MENGHAPUS BERKASNYA JUGA, bukan sekadar melepas tautannya.
//
// Satu berkas milik satu proyek — indeks unik pada asset_id yang menjaminnya —
// sehingga tidak ada keraguan siapa lagi yang memakainya.
func (h *ProjectHandler) RemoveAttachment(c *gin.Context) {
	if err := h.project.RemoveAttachment(c.Request.Context(), c.Param("id")); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project attachment removed successfully", nil)
}

// ── Dokumen proyek ──────────────────────────────────────────────────────────

// AddDocument MENGAITKAN dokumen yang sudah ada, bukan membuatnya.
//
// Dokumen dibuat lewat document-add dan disunting di editor; yang dikirim ke
// sini hanya tokennya.
func (h *ProjectHandler) AddDocument(c *gin.Context) {
	var req dto.ProjectDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	item, err := h.project.AddDocument(c.Request.Context(), c.Param("id"), req.ToProjectDocumentCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project document added successfully", dto.NewProjectDocumentDataResponse(item))
}

func (h *ProjectHandler) UpdateDocument(c *gin.Context) {
	var req dto.ProjectDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.project.UpdateDocument(c.Request.Context(), c.Param("id"), req.ToProjectDocumentCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project document updated successfully", nil)
}

// RemoveDocument HANYA memutus kaitannya; dokumennya tetap hidup.
//
// Sengaja tidak menyerupai RemoveAttachment, yang ikut membuang berkasnya.
// Yang benar-benar ingin membuang dokumennya memanggil document-delete.
func (h *ProjectHandler) RemoveDocument(c *gin.Context) {
	if err := h.project.RemoveDocument(c.Request.Context(), c.Param("id")); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project document removed successfully", nil)
}

// ── Tonggak proyek ──────────────────────────────────────────────────────────

// MilestoneSuggestions menyodorkan tonggak yang lazim dipakai.
//
// SARAN, bukan kosakata tertutup: `milestone` adalah teks bebas, dan yang di
// luar daftar ini tetap diterima. Ada supaya frontend tidak menyalin daftarnya
// lalu ketinggalan ketika ia berubah.
func (h *ProjectHandler) MilestoneSuggestions(c *gin.Context) {
	dto.Success(c, "Success", dto.NewMilestoneSuggestionListResponse(h.project.MilestoneSuggestions()))
}

func (h *ProjectHandler) AddMilestone(c *gin.Context) {
	var req dto.ProjectMilestoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	item, err := h.project.AddMilestone(c.Request.Context(), c.Param("id"), req.ToProjectMilestoneCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project milestone added successfully", dto.NewProjectMilestoneDataResponse(item))
}

func (h *ProjectHandler) UpdateMilestone(c *gin.Context) {
	var req dto.ProjectMilestoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.project.UpdateMilestone(c.Request.Context(), c.Param("id"), req.ToProjectMilestoneCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project milestone updated successfully", nil)
}

func (h *ProjectHandler) RemoveMilestone(c *gin.Context) {
	if err := h.project.RemoveMilestone(c.Request.Context(), c.Param("id")); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project milestone removed successfully", nil)
}
