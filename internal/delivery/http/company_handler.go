package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type CompanyHandler struct {
	company input.CompanyUseCase
}

func NewCompanyHandler(company input.CompanyUseCase) *CompanyHandler {
	return &CompanyHandler{company: company}
}

func (h *CompanyHandler) List(c *gin.Context) {
	var req dto.CompanyListFilterRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid query parameters")
		return
	}

	companies, err := h.company.List(c.Request.Context(), req.ToListCompanyQuery())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewCompanyListResponse(companies))
}

// Get menyertakan kontaknya; List tidak.
//
// Daftar dipakai pemilih di layar, dan menyertakan seluruh kontak di sana berarti
// satu kueri tambahan per baris untuk data yang tidak dilihat siapa pun sampai
// satu perusahaan benar-benar dibuka.
func (h *CompanyHandler) Get(c *gin.Context) {
	company, err := h.company.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewCompanyDataResponse(company))
}

func (h *CompanyHandler) Create(c *gin.Context) {
	var req dto.CompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	cmd, err := req.ToCompanyCommand()
	if err != nil {
		// Tanggal yang tidak terbaca dijawab di sini, bukan diteruskan sebagai
		// nil yang berarti "tidak diisi" — pemanggil yang salah format akan
		// mengira tanggalnya tersimpan.
		dto.Error(c, http.StatusBadRequest, "established_date must be formatted as YYYY-MM-DD")
		return
	}

	company, err := h.company.Create(c.Request.Context(), cmd)
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Company created successfully", dto.NewCompanyDataResponse(company))
}

func (h *CompanyHandler) Update(c *gin.Context) {
	var req dto.CompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	cmd, err := req.ToCompanyCommand()
	if err != nil {
		dto.Error(c, http.StatusBadRequest, "established_date must be formatted as YYYY-MM-DD")
		return
	}

	if err := h.company.Update(c.Request.Context(), c.Param("id"), cmd); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Company updated successfully", nil)
}

func (h *CompanyHandler) Delete(c *gin.Context) {
	if err := h.company.Delete(c.Request.Context(), c.Param("id")); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Company deleted successfully", nil)
}

// ListContacts menyajikan kontak satu perusahaan tanpa data perusahaannya.
//
// company-detail sudah membawa keduanya sekaligus; rute ini untuk layar yang
// hanya membutuhkan kontaknya dan memanggilnya berulang.
func (h *CompanyHandler) ListContacts(c *gin.Context) {
	contacts, err := h.company.ListContacts(c.Request.Context(), c.Param("id"))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewCompanyContactListResponse(contacts))
}

func (h *CompanyHandler) AddContact(c *gin.Context) {
	var req dto.CompanyContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	contact, err := h.company.AddContact(c.Request.Context(), c.Param("id"), req.ToCompanyContactCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Company contact created successfully", dto.NewCompanyContactDataResponse(contact))
}

// UpdateContact dan DeleteContact memakai id KONTAK, bukan id perusahaan.
//
// Perusahaannya diambil dari baris kontaknya sendiri, sehingga tidak ada
// kemungkinan pemanggil menyebut pasangan perusahaan-kontak yang tidak
// berhubungan — dan tidak ada pula pemeriksaan tambahan yang bisa terlupa.
func (h *CompanyHandler) UpdateContact(c *gin.Context) {
	var req dto.CompanyContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.company.UpdateContact(c.Request.Context(), c.Param("id"), req.ToCompanyContactCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Company contact updated successfully", nil)
}

func (h *CompanyHandler) DeleteContact(c *gin.Context) {
	if err := h.company.DeleteContact(c.Request.Context(), c.Param("id")); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Company contact deleted successfully", nil)
}
