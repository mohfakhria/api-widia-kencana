package usecase

import (
	"context"
	"fmt"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type quotationUseCase struct {
	repo output.QuotationRepository
}

func NewQuotationUseCase(repo output.QuotationRepository) input.QuotationUseCase {
	return &quotationUseCase{repo: repo}
}

func (uc *quotationUseCase) List(ctx context.Context) ([]entity.Quotation, error) {
	return uc.repo.List(ctx)
}

func (uc *quotationUseCase) GetByID(ctx context.Context, id string) (*entity.Quotation, error) {
	return uc.repo.GetByID(ctx, id)
}

func (uc *quotationUseCase) Create(ctx context.Context, cmd input.CreateQuotationCommand) (string, error) {
	quotation := uc.mapCommand(cmd)
	if err := validateQuotation(quotation); err != nil {
		return "", err
	}
	return uc.repo.Create(ctx, quotation)
}

func (uc *quotationUseCase) Update(ctx context.Context, id string, cmd input.UpdateQuotationCommand) error {
	quotation := uc.mapCommand(input.CreateQuotationCommand(cmd))
	if err := validateQuotation(quotation); err != nil {
		return err
	}
	return uc.repo.Update(ctx, id, quotation)
}

func (uc *quotationUseCase) mapCommand(cmd input.CreateQuotationCommand) *entity.Quotation {
	quotation := &entity.Quotation{
		ClientName:    cmd.ClientName,
		AttnName:      cmd.AttnName,
		AttnPosition:  cmd.AttnPosition,
		Address:       cmd.Address,
		Project:       cmd.Project,
		DiscountType:  cmd.DiscountType,
		DiscountValue: cmd.DiscountValue,
		SubTotal:      cmd.SubTotal,
		Total:         cmd.Total,
		Notes:         cmd.Notes,
	}

	for _, section := range cmd.Sections {
		mappedSection := entity.QuotationSection{
			Title:    section.Title,
			Position: section.Position,
		}

		for _, item := range section.Items {
			mappedSection.Items = append(mappedSection.Items, entity.QuotationItem{
				Name:  item.Name,
				Qty:   item.Qty,
				Unit:  item.Unit,
				Price: item.Price,
			})
		}

		for _, detail := range section.Details {
			mappedSection.Details = append(mappedSection.Details, entity.QuotationDetail{
				Description: detail.Description,
				Position:    detail.Position,
			})
		}

		quotation.Sections = append(quotation.Sections, mappedSection)
	}

	return quotation
}

func validateQuotation(q *entity.Quotation) error {
	if q.ClientName == "" {
		return domain.NewError(domain.ErrInvalidInput, "client name cannot be empty")
	}
	if q.Project == "" {
		return domain.NewError(domain.ErrInvalidInput, "project name cannot be empty")
	}
	if len(q.Sections) == 0 {
		return domain.NewError(domain.ErrInvalidInput, "at least one section is required")
	}

	for i, section := range q.Sections {
		if section.Title == "" {
			return domain.NewError(domain.ErrInvalidInput, fmt.Sprintf("section %d: title cannot be empty", i+1))
		}
		if len(section.Items) == 0 && len(section.Details) == 0 {
			return domain.NewError(domain.ErrInvalidInput, fmt.Sprintf("section %d: must have either items or details", i+1))
		}

		for j, item := range section.Items {
			if item.Name == "" {
				return domain.NewError(domain.ErrInvalidInput, fmt.Sprintf("section %d item %d: name cannot be empty", i+1, j+1))
			}
			if item.Qty <= 0 {
				return domain.NewError(domain.ErrInvalidInput, fmt.Sprintf("section %d item %d: qty must be greater than 0", i+1, j+1))
			}
			if item.Price < 0 {
				return domain.NewError(domain.ErrInvalidInput, fmt.Sprintf("section %d item %d: price cannot be negative", i+1, j+1))
			}
		}

		for j, detail := range section.Details {
			if detail.Description == "" {
				return domain.NewError(domain.ErrInvalidInput, fmt.Sprintf("section %d detail %d: description cannot be empty", i+1, j+1))
			}
		}
	}

	return nil
}
