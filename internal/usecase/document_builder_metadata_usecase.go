package usecase

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type documentBuilderMetadataUseCase struct {
	repo output.DocumentBuilderMetadataRepository
}

func NewDocumentBuilderMetadataUseCase(repo output.DocumentBuilderMetadataRepository) input.DocumentBuilderMetadataUseCase {
	return &documentBuilderMetadataUseCase{repo: repo}
}

func (uc *documentBuilderMetadataUseCase) Get(ctx context.Context) (*entity.DocumentBuilderMetadata, error) {
	papers, err := uc.repo.ListPapers(ctx)
	if err != nil {
		return nil, err
	}

	elements, err := uc.repo.ListElements(ctx)
	if err != nil {
		return nil, err
	}

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
	elements = attachElementProperties(elements, elementProperties, propertiesByID)

	return &entity.DocumentBuilderMetadata{
		Papers:   papers,
		Elements: elements,
	}, nil
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
