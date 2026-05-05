package http

import (
	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type QuotationHandler struct {
	quotation input.QuotationUseCase
}

func NewQuotationHandler(quotation input.QuotationUseCase) *QuotationHandler {
	return &QuotationHandler{quotation: quotation}
}

func (h *QuotationHandler) List(c *gin.Context) {
	var req dto.QuotationListFilterRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		dto.Error(c, 400, "Invalid query parameters")
		return
	}

	data, err := h.quotation.List(c.Request.Context(), req.ToListQuotationQuery())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}
	dto.Success(c, "Success", dto.NewQuotationListResponses(data))
}

func (h *QuotationHandler) Get(c *gin.Context) {
	data, err := h.quotation.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}
	dto.Success(c, "Success", dto.NewQuotationDetailResponse(data))
}

func (h *QuotationHandler) Create(c *gin.Context) {
	var req dto.QuotationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "Invalid request payload")
		return
	}

	quotationNo, err := h.quotation.Create(c.Request.Context(), req.ToCreateQuotationCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), "Failed to create quotation: "+err.Error())
		return
	}

	dto.Success(c, "Quotation created successfully", dto.NewQuotationCreatedResponse(quotationNo))
}

func (h *QuotationHandler) Update(c *gin.Context) {
	var req dto.QuotationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "Invalid payload")
		return
	}

	if err := h.quotation.Update(c.Request.Context(), c.Param("id"), req.ToUpdateQuotationCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Quotation updated successfully", nil)
}
