package usecase

import (
	"context"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/google/uuid"
)

const defaultDocumentStatus = "draft"
const defaultDocumentType = "custom"

var allowedDocumentStatuses = map[string]struct{}{
	"draft":    {},
	"active":   {},
	"inactive": {},
	"archived": {},
	"deleted":  {},
}

type documentUseCase struct {
	repo output.DocumentRepository
}

func NewDocumentUseCase(repo output.DocumentRepository) input.DocumentUseCase {
	return &documentUseCase{repo: repo}
}

func (uc *documentUseCase) ListPapers(ctx context.Context) ([]entity.DocumentPaper, error) {
	return uc.repo.ListPapers(ctx)
}

func (uc *documentUseCase) ListElements(ctx context.Context) ([]entity.DocumentElement, error) {
	return uc.repo.ListElements(ctx)
}

func (uc *documentUseCase) ListProperties(ctx context.Context) ([]entity.DocumentProperty, error) {
	return uc.repo.ListProperties(ctx)
}

func (uc *documentUseCase) ListPropertyOptions(ctx context.Context) ([]entity.DocumentPropertyOption, error) {
	return uc.repo.ListPropertyOptions(ctx)
}

func (uc *documentUseCase) ListElementProperties(ctx context.Context) ([]entity.DocumentElementProperty, error) {
	properties, err := uc.repo.ListProperties(ctx)
	if err != nil {
		return nil, err
	}

	options, err := uc.repo.ListPropertyOptions(ctx)
	if err != nil {
		return nil, err
	}

	elementProperties, err := uc.repo.ListElementProperties(ctx)
	if err != nil {
		return nil, err
	}

	propertiesByID := mapPropertiesByID(properties, options)
	for idx := range elementProperties {
		property, ok := propertiesByID[elementProperties[idx].DocumentPropertyID]
		if !ok {
			continue
		}
		elementProperties[idx].Property = property
	}

	return elementProperties, nil
}

func (uc *documentUseCase) List(ctx context.Context) ([]entity.Document, error) {
	return uc.repo.List(ctx)
}

func (uc *documentUseCase) GetByToken(ctx context.Context, token string) (*entity.Document, error) {
	token = strings.TrimSpace(token)
	if err := validateUUIDToken(token, "document token"); err != nil {
		return nil, err
	}

	return uc.repo.GetByToken(ctx, token)
}

func (uc *documentUseCase) Create(ctx context.Context, cmd input.CreateDocumentCommand) (*entity.Document, error) {
	document := mapDocumentCommand(cmd)
	if err := validateDocument(document); err != nil {
		return nil, err
	}

	return uc.repo.Create(ctx, document)
}

func (uc *documentUseCase) Update(ctx context.Context, token string, cmd input.UpdateDocumentCommand) error {
	token = strings.TrimSpace(token)
	if err := validateUUIDToken(token, "document token"); err != nil {
		return err
	}

	document := mapDocumentCommand(input.CreateDocumentCommand(cmd))
	if err := validateDocument(document); err != nil {
		return err
	}

	return uc.repo.Update(ctx, token, document)
}

func (uc *documentUseCase) Delete(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if err := validateUUIDToken(token, "document token"); err != nil {
		return err
	}

	return uc.repo.Delete(ctx, token)
}

func mapDocumentCommand(cmd input.CreateDocumentCommand) *entity.Document {
	status := strings.ToLower(strings.TrimSpace(cmd.Status))
	if status == "" {
		status = defaultDocumentStatus
	}

	documentType := strings.ToLower(strings.TrimSpace(cmd.DocumentType))
	if documentType == "" {
		documentType = defaultDocumentType
	}

	return &entity.Document{
		Paper: entity.DocumentPaper{
			Token: strings.TrimSpace(cmd.DocumentPaperToken),
		},
		ParentToken:  strings.TrimSpace(cmd.ParentToken),
		Name:         strings.TrimSpace(cmd.Name),
		DocumentType: documentType,
		Position:     cmd.Position,
		Status:       status,
	}
}

func validateDocument(document *entity.Document) error {
	if err := validateUUIDToken(document.Paper.Token, "document paper token"); err != nil {
		return err
	}
	if err := validateOptionalUUIDToken(document.ParentToken, "parent token"); err != nil {
		return err
	}
	if document.Name == "" {
		return domain.NewError(domain.ErrInvalidInput, "document name cannot be empty")
	}
	if document.Position < 0 {
		return domain.NewError(domain.ErrInvalidInput, "document position cannot be negative")
	}
	if _, ok := allowedDocumentStatuses[document.Status]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid document status")
	}

	return nil
}

func validateUUIDToken(token, label string) error {
	if strings.TrimSpace(token) == "" {
		return domain.NewError(domain.ErrInvalidInput, label+" cannot be empty")
	}
	if _, err := uuid.Parse(token); err != nil {
		return domain.NewError(domain.ErrInvalidInput, "invalid "+label)
	}

	return nil
}

func validateOptionalUUIDToken(token, label string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}

	return validateUUIDToken(token, label)
}

func mapPropertiesByID(
	properties []entity.DocumentProperty,
	options []entity.DocumentPropertyOption,
) map[int64]entity.DocumentProperty {
	optionsByPropertyID := make(map[int64][]entity.DocumentPropertyOption)
	for _, option := range options {
		optionsByPropertyID[option.DocumentPropertyID] = append(optionsByPropertyID[option.DocumentPropertyID], option)
	}

	propertiesByID := make(map[int64]entity.DocumentProperty)
	for _, property := range properties {
		property.Options = optionsByPropertyID[property.ID]
		propertiesByID[property.ID] = property
	}

	return propertiesByID
}

func attachElementProperties(
	elements []entity.DocumentElement,
	elementProperties []entity.DocumentElementProperty,
	propertiesByID map[int64]entity.DocumentProperty,
) []entity.DocumentElement {
	propertiesByElementID := make(map[int64][]entity.DocumentElementProperty)
	for _, elementProperty := range elementProperties {
		property, ok := propertiesByID[elementProperty.DocumentPropertyID]
		if !ok {
			continue
		}
		elementProperty.Property = property
		propertiesByElementID[elementProperty.DocumentElementID] = append(
			propertiesByElementID[elementProperty.DocumentElementID],
			elementProperty,
		)
	}

	for idx := range elements {
		elements[idx].Properties = propertiesByElementID[elements[idx].ID]
	}

	return elements
}
