package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"

	"github.com/gin-gonic/gin"
)

type QuotationHandler struct {
	quotation input.QuotationUseCase
}

func NewQuotationHandler(quotation input.QuotationUseCase) *QuotationHandler {
	return &QuotationHandler{quotation: quotation}
}

func (h *QuotationHandler) List(c *gin.Context) {
	data, err := h.quotation.List(c.Request.Context())
	if err != nil {
		dto.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	dto.Success(c, "Success", dto.NewQuotationListResponses(data))
}

func (h *QuotationHandler) Get(c *gin.Context) {
	data, err := h.quotation.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		dto.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	dto.Success(c, "Success", dto.NewQuotationDetailResponse(data))
}

func (h *QuotationHandler) Create(c *gin.Context) {
	var req dto.QuotationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusInternalServerError, "Invalid request payload")
		return
	}

	quotationNo, err := h.quotation.Create(c.Request.Context(), req.ToCreateQuotationCommand())
	if err != nil {
		dto.Error(c, http.StatusUnauthorized, "Failed to create quotation: "+err.Error())
		return
	}

	dto.Success(c, "Quotation created successfully", dto.NewQuotationCreatedResponse(quotationNo))
}

func (h *QuotationHandler) Update(c *gin.Context) {
	var req dto.QuotationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid payload")
		return
	}

	if err := h.quotation.Update(c.Request.Context(), c.Param("id"), req.ToUpdateQuotationCommand()); err != nil {
		dto.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	dto.Success(c, "Quotation updated successfully", nil)
}
