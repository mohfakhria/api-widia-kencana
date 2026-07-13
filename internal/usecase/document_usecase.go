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
	repo      output.DocumentRepository
	layerRepo output.DocumentLayerRepository
}

func NewDocumentUseCase(
	repo output.DocumentRepository,
	layerRepo output.DocumentLayerRepository,
) input.DocumentUseCase {
	return &documentUseCase{repo: repo, layerRepo: layerRepo}
}

func (uc *documentUseCase) ListPapers(ctx context.Context) ([]entity.DocumentPaper, error) {
	return uc.repo.ListPapers(ctx)
}

func (uc *documentUseCase) ListElements(
	ctx context.Context,
	query input.ListDocumentElementQuery,
) ([]entity.DocumentElement, error) {
	query.Code = strings.TrimSpace(query.Code)

	elements, err := uc.repo.ListElements(ctx, query)
	if err != nil {
		return nil, err
	}
	if query.Code == "" {
		return elements, nil
	}

	elementPropertyQuery := input.ListDocumentElementPropertyQuery{}
	if query.Code != "all" {
		elementPropertyQuery.ElementCode = query.Code
	}

	elementProperties, err := uc.ListElementProperties(ctx, elementPropertyQuery)
	if err != nil {
		return nil, err
	}

	return attachElementPropertiesByElement(elements, elementProperties), nil
}

func (uc *documentUseCase) ListProperties(ctx context.Context) ([]entity.DocumentProperty, error) {
	return uc.repo.ListProperties(ctx)
}

func (uc *documentUseCase) ListPropertyOptions(ctx context.Context) ([]entity.DocumentPropertyOption, error) {
	return uc.repo.ListPropertyOptions(ctx)
}

func (uc *documentUseCase) ListElementProperties(
	ctx context.Context,
	query input.ListDocumentElementPropertyQuery,
) ([]entity.DocumentElementProperty, error) {
	query.ElementCode = strings.TrimSpace(query.ElementCode)

	properties, err := uc.repo.ListProperties(ctx)
	if err != nil {
		return nil, err
	}

	options, err := uc.repo.ListPropertyOptions(ctx)
	if err != nil {
		return nil, err
	}

	elementProperties, err := uc.repo.ListElementProperties(ctx, query)
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

func (uc *documentUseCase) List(ctx context.Context, query input.ListDocumentQuery) ([]entity.Document, error) {
	query.Name = strings.TrimSpace(query.Name)
	query.Token = strings.TrimSpace(query.Token)
	if err := validateOptionalUUIDToken(query.Token, "document token"); err != nil {
		return nil, err
	}

	return uc.repo.List(ctx, query)
}

func (uc *documentUseCase) GetByToken(
	ctx context.Context,
	token string,
	query input.GetDocumentQuery,
) (*entity.Document, error) {
	token = strings.TrimSpace(token)
	if err := validateUUIDToken(token, "document token"); err != nil {
		return nil, err
	}

	document, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if !query.WithLayer {
		return document, nil
	}

	layers, err := uc.layerRepo.ListByDocumentToken(ctx, token)
	if err != nil {
		return nil, err
	}

	layers, err = uc.attachMasterElementProperties(ctx, layers)
	if err != nil {
		return nil, err
	}

	document.WithLayers = true
	document.Layers = buildDocumentLayerTree(layers)
	return document, nil
}

func (uc *documentUseCase) attachMasterElementProperties(
	ctx context.Context,
	layers []entity.DocumentLayer,
) ([]entity.DocumentLayer, error) {
	elementProperties, err := uc.ListElementProperties(ctx, input.ListDocumentElementPropertyQuery{})
	if err != nil {
		return nil, err
	}

	propertiesByElementCode := make(map[string][]entity.DocumentElementProperty)
	for _, elementProperty := range elementProperties {
		propertiesByElementCode[elementProperty.ElementCode] = append(
			propertiesByElementCode[elementProperty.ElementCode],
			elementProperty,
		)
	}

	for idx := range layers {
		layers[idx].Element.Properties = propertiesByElementCode[layers[idx].Element.Code]
	}

	return layers, nil
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

func attachElementPropertiesByElement(
	elements []entity.DocumentElement,
	elementProperties []entity.DocumentElementProperty,
) []entity.DocumentElement {
	propertiesByElementID := make(map[int64][]entity.DocumentElementProperty)
	for _, elementProperty := range elementProperties {
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

func buildDocumentLayerTree(layers []entity.DocumentLayer) entity.DocumentLayerRegions {
	regions := entity.DocumentLayerRegions{
		Header: []entity.DocumentLayer{},
		Body:   []entity.DocumentLayer{},
		Footer: []entity.DocumentLayer{},
	}

	layerByID := make(map[int64]*entity.DocumentLayer, len(layers))
	for idx := range layers {
		layers[idx].Children = []entity.DocumentLayer{}
		layerByID[layers[idx].ID] = &layers[idx]
	}

	for idx := range layers {
		layer := &layers[idx]
		if layer.ParentID != nil {
			if parent, ok := layerByID[*layer.ParentID]; ok {
				parent.Children = append(parent.Children, *layer)
				continue
			}
		}

		switch layer.Region {
		case "header":
			regions.Header = append(regions.Header, *layer)
		case "footer":
			regions.Footer = append(regions.Footer, *layer)
		default:
			regions.Body = append(regions.Body, *layer)
		}
	}

	return regions
}
