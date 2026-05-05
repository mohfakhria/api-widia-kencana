package input

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

type QuotationUseCase interface {
	List(ctx context.Context, query ListQuotationQuery) ([]entity.Quotation, error)
	GetByID(ctx context.Context, id string) (*entity.Quotation, error)
	Create(ctx context.Context, cmd CreateQuotationCommand) (string, error)
	Update(ctx context.Context, id string, cmd UpdateQuotationCommand) error
}

type ListQuotationQuery struct {
	Status  string
	Project string
}

type CreateQuotationCommand struct {
	ClientName    string
	AttnName      string
	AttnPosition  string
	Address       string
	Project       string
	DiscountType  string
	DiscountValue float64
	SubTotal      float64
	Total         float64
	Notes         []string
	Sections      []QuotationSectionInput
}

type UpdateQuotationCommand = CreateQuotationCommand

type QuotationSectionInput struct {
	Title    string
	Position int
	Items    []QuotationItemInput
	Details  []QuotationDetailInput
}

type QuotationItemInput struct {
	Name  string
	Qty   float64
	Unit  string
	Price float64
}

type QuotationDetailInput struct {
	Description string
	Position    int
}
