package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"

	"github.com/gin-gonic/gin"
)

type QuotationHandler struct {
	quotation input.QuotationUseCase
}

type QuotationRequest struct {
	ClientName    string                    `json:"client_name"`
	AttnName      string                    `json:"attn_name"`
	AttnPosition  string                    `json:"attn_position"`
	Address       string                    `json:"address"`
	Project       string                    `json:"project"`
	DiscountType  string                    `json:"discount_type"`
	DiscountValue float64                   `json:"discount_value"`
	SubTotal      float64                   `json:"subtotal"`
	Total         float64                   `json:"total"`
	Notes         []string                  `json:"notes"`
	Sections      []QuotationSectionRequest `json:"sections"`
}

type QuotationSectionRequest struct {
	Title    string                   `json:"title"`
	Position int                      `json:"position"`
	Items    []QuotationItemRequest   `json:"items"`
	Details  []QuotationDetailRequest `json:"details"`
}

type QuotationItemRequest struct {
	Name  string  `json:"name"`
	Qty   float64 `json:"qty"`
	Unit  string  `json:"unit"`
	Price float64 `json:"price"`
}

type QuotationDetailRequest struct {
	Description string `json:"description"`
	Position    int    `json:"position"`
}

func NewQuotationHandler(quotation input.QuotationUseCase) *QuotationHandler {
	return &QuotationHandler{quotation: quotation}
}

func (h *QuotationHandler) List(c *gin.Context) {
	data, err := h.quotation.List(c.Request.Context())
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	Success(c, "Success", data)
}

func (h *QuotationHandler) Get(c *gin.Context) {
	data, err := h.quotation.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	Success(c, "Success", data)
}

func (h *QuotationHandler) Create(c *gin.Context) {
	var req QuotationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusInternalServerError, "Invalid request payload")
		return
	}

	quotationNo, err := h.quotation.Create(c.Request.Context(), mapQuotationRequest(req))
	if err != nil {
		Error(c, http.StatusUnauthorized, "Failed to create quotation: "+err.Error())
		return
	}

	Success(c, "Quotation created successfully", gin.H{
		"quotationNo": quotationNo,
	})
}

func (h *QuotationHandler) Update(c *gin.Context) {
	var req QuotationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid payload")
		return
	}

	if err := h.quotation.Update(c.Request.Context(), c.Param("id"), input.UpdateQuotationCommand(mapQuotationRequest(req))); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	Success(c, "Quotation updated successfully", nil)
}

func mapQuotationRequest(req QuotationRequest) input.CreateQuotationCommand {
	cmd := input.CreateQuotationCommand{
		ClientName:    req.ClientName,
		AttnName:      req.AttnName,
		AttnPosition:  req.AttnPosition,
		Address:       req.Address,
		Project:       req.Project,
		DiscountType:  req.DiscountType,
		DiscountValue: req.DiscountValue,
		SubTotal:      req.SubTotal,
		Total:         req.Total,
		Notes:         req.Notes,
	}

	for _, section := range req.Sections {
		mappedSection := input.QuotationSectionInput{
			Title:    section.Title,
			Position: section.Position,
		}
		for _, item := range section.Items {
			mappedSection.Items = append(mappedSection.Items, input.QuotationItemInput{
				Name:  item.Name,
				Qty:   item.Qty,
				Unit:  item.Unit,
				Price: item.Price,
			})
		}
		for _, detail := range section.Details {
			mappedSection.Details = append(mappedSection.Details, input.QuotationDetailInput{
				Description: detail.Description,
				Position:    detail.Position,
			})
		}
		cmd.Sections = append(cmd.Sections, mappedSection)
	}

	return cmd
}
