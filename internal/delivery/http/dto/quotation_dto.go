package dto

import (
	"strings"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

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

type QuotationListFilterRequest struct {
	Status  string `form:"status"`
	Project string `form:"project"`
}

type QuotationListResponse struct {
	ID          int64     `json:"id"`
	QuotationNo string    `json:"quotation_no"`
	ClientName  string    `json:"client_name"`
	Project     string    `json:"project"`
	Status      string    `json:"status"`
	Total       float64   `json:"total"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type QuotationListDataResponse struct {
	Quotations []QuotationListResponse `json:"quotations"`
}

type QuotationDetailResponse struct {
	ID            int64                      `json:"id"`
	QuotationNo   string                     `json:"quotation_no"`
	ClientName    string                     `json:"client_name"`
	AttnName      string                     `json:"attn_name"`
	AttnPosition  string                     `json:"attn_position"`
	Address       string                     `json:"address"`
	Project       string                     `json:"project"`
	DiscountType  string                     `json:"discount_type"`
	DiscountValue float64                    `json:"discount_value"`
	SubTotal      float64                    `json:"subtotal"`
	Total         float64                    `json:"total"`
	Notes         []string                   `json:"notes"`
	CreatedAt     time.Time                  `json:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at"`
	Sections      []QuotationSectionResponse `json:"sections"`
}

type QuotationDetailDataResponse struct {
	Quotation QuotationDetailResponse `json:"quotation"`
}

type QuotationSectionResponse struct {
	ID       int64                            `json:"id"`
	Title    string                           `json:"title"`
	Position int                              `json:"position"`
	Items    []QuotationItemResponse          `json:"items"`
	Details  []QuotationSectionDetailResponse `json:"details"`
}

type QuotationItemResponse struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	Qty   float64 `json:"qty"`
	Unit  string  `json:"unit"`
	Price float64 `json:"price"`
	Total float64 `json:"total"`
}

type QuotationSectionDetailResponse struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Position    int    `json:"position"`
}

type QuotationCreatedResponse struct {
	QuotationNo string `json:"quotationNo"`
}

type QuotationCreatedDataResponse struct {
	Quotation QuotationCreatedResponse `json:"quotation"`
}

func (r QuotationRequest) ToCreateQuotationCommand() input.CreateQuotationCommand {
	cmd := input.CreateQuotationCommand{
		ClientName:    r.ClientName,
		AttnName:      r.AttnName,
		AttnPosition:  r.AttnPosition,
		Address:       r.Address,
		Project:       r.Project,
		DiscountType:  r.DiscountType,
		DiscountValue: r.DiscountValue,
		SubTotal:      r.SubTotal,
		Total:         r.Total,
		Notes:         r.Notes,
	}

	for _, section := range r.Sections {
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

func (r QuotationRequest) ToUpdateQuotationCommand() input.UpdateQuotationCommand {
	return input.UpdateQuotationCommand(r.ToCreateQuotationCommand())
}

func (r QuotationListFilterRequest) ToListQuotationQuery() input.ListQuotationQuery {
	return input.ListQuotationQuery{
		Status:  strings.TrimSpace(r.Status),
		Project: strings.TrimSpace(r.Project),
	}
}

func NewQuotationListResponses(quotations []entity.Quotation) QuotationListDataResponse {
	responses := QuotationListDataResponse{
		Quotations: make([]QuotationListResponse, 0, len(quotations)),
	}
	for _, quotation := range quotations {
		responses.Quotations = append(responses.Quotations, QuotationListResponse{
			ID:          quotation.ID,
			QuotationNo: quotation.QuotationNo,
			ClientName:  quotation.ClientName,
			Project:     quotation.Project,
			Status:      quotation.Status,
			Total:       quotation.Total,
			CreatedAt:   quotation.CreatedAt,
			UpdatedAt:   quotation.UpdatedAt,
		})
	}

	return responses
}

func NewQuotationDetailResponse(quotation *entity.Quotation) QuotationDetailDataResponse {
	response := QuotationDetailDataResponse{
		Quotation: QuotationDetailResponse{
			ID:            quotation.ID,
			QuotationNo:   quotation.QuotationNo,
			ClientName:    quotation.ClientName,
			AttnName:      quotation.AttnName,
			AttnPosition:  quotation.AttnPosition,
			Address:       quotation.Address,
			Project:       quotation.Project,
			DiscountType:  quotation.DiscountType,
			DiscountValue: quotation.DiscountValue,
			SubTotal:      quotation.SubTotal,
			Total:         quotation.Total,
			Notes:         quotation.Notes,
			CreatedAt:     quotation.CreatedAt,
			UpdatedAt:     quotation.UpdatedAt,
		},
	}

	for _, section := range quotation.Sections {
		mappedSection := QuotationSectionResponse{
			ID:       section.ID,
			Title:    section.Title,
			Position: section.Position,
		}

		for _, item := range section.Items {
			mappedSection.Items = append(mappedSection.Items, QuotationItemResponse{
				ID:    item.ID,
				Name:  item.Name,
				Qty:   item.Qty,
				Unit:  item.Unit,
				Price: item.Price,
				Total: item.Total,
			})
		}

		for _, detail := range section.Details {
			mappedSection.Details = append(mappedSection.Details, QuotationSectionDetailResponse{
				ID:          detail.ID,
				Description: detail.Description,
				Position:    detail.Position,
			})
		}

		response.Quotation.Sections = append(response.Quotation.Sections, mappedSection)
	}

	return response
}

func NewQuotationCreatedResponse(quotationNo string) QuotationCreatedDataResponse {
	return QuotationCreatedDataResponse{
		Quotation: QuotationCreatedResponse{QuotationNo: quotationNo},
	}
}
