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

const defaultDocumentLayerRegion = "body"
const defaultDocumentLayerStatus = "active"

var allowedDocumentLayerRegions = map[string]struct{}{
	"header": {},
	"body":   {},
	"footer": {},
}

var allowedDocumentLayerStatuses = map[string]struct{}{
	"active":  {},
	"deleted": {},
}

type documentLayerUseCase struct {
	repo output.DocumentLayerRepository
}

func NewDocumentLayerUseCase(repo output.DocumentLayerRepository) input.DocumentLayerUseCase {
	return &documentLayerUseCase{repo: repo}
}

func (uc *documentLayerUseCase) Create(
	ctx context.Context,
	cmd input.CreateDocumentLayerCommand,
) (*entity.DocumentLayer, error) {
	layer := mapDocumentLayerCommand(cmd)
	if err := validateDocumentLayer(layer); err != nil {
		return nil, err
	}

	return uc.repo.Create(ctx, layer)
}

func (uc *documentLayerUseCase) Update(
	ctx context.Context,
	token string,
	cmd input.UpdateDocumentLayerCommand,
) error {
	token = strings.TrimSpace(token)
	if err := validateDocumentLayerToken(token, "document layer token"); err != nil {
		return err
	}

	layer := mapDocumentLayerCommand(input.CreateDocumentLayerCommand(cmd))
	if err := validateDocumentLayer(layer); err != nil {
		return err
	}

	return uc.repo.Update(ctx, token, layer)
}

func (uc *documentLayerUseCase) Sort(ctx context.Context, cmd input.SortDocumentLayerCommand) error {
	cmd.DocumentToken = strings.TrimSpace(cmd.DocumentToken)
	cmd.ParentToken = strings.TrimSpace(cmd.ParentToken)
	cmd.Region = normalizeDocumentLayerRegion(cmd.Region)

	if err := validateDocumentLayerToken(cmd.DocumentToken, "document token"); err != nil {
		return err
	}
	if err := validateOptionalDocumentLayerToken(cmd.ParentToken, "parent layer token"); err != nil {
		return err
	}
	if _, ok := allowedDocumentLayerRegions[cmd.Region]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid document layer region")
	}
	if len(cmd.Layers) == 0 {
		return domain.NewError(domain.ErrInvalidInput, "at least one document layer is required")
	}

	seenToken := make(map[string]struct{}, len(cmd.Layers))
	seenPosition := make(map[int]struct{}, len(cmd.Layers))
	layers := make([]entity.DocumentLayer, 0, len(cmd.Layers))
	for _, item := range cmd.Layers {
		token := strings.TrimSpace(item.Token)
		if err := validateDocumentLayerToken(token, "document layer token"); err != nil {
			return err
		}
		if item.Position < 0 {
			return domain.NewError(domain.ErrInvalidInput, "document layer position cannot be negative")
		}
		if _, ok := seenToken[token]; ok {
			return domain.NewError(domain.ErrInvalidInput, "duplicate document layer token")
		}
		if _, ok := seenPosition[item.Position]; ok {
			return domain.NewError(domain.ErrInvalidInput, "duplicate document layer position")
		}

		seenToken[token] = struct{}{}
		seenPosition[item.Position] = struct{}{}
		layers = append(layers, entity.DocumentLayer{
			Token:    token,
			Position: item.Position,
		})
	}

	return uc.repo.Sort(ctx, cmd.DocumentToken, cmd.ParentToken, cmd.Region, layers)
}

func (uc *documentLayerUseCase) Delete(ctx context.Context, cmd input.DeleteDocumentLayerCommand) error {
	documentToken := strings.TrimSpace(cmd.DocumentToken)
	if err := validateOptionalDocumentLayerToken(documentToken, "document token"); err != nil {
		return err
	}

	tokens, err := normalizeDeleteDocumentLayerTokens(cmd.Tokens)
	if err != nil {
		return err
	}

	return uc.repo.Delete(ctx, documentToken, tokens)
}

func mapDocumentLayerCommand(cmd input.CreateDocumentLayerCommand) *entity.DocumentLayer {
	region := normalizeDocumentLayerRegion(cmd.Region)
	status := strings.ToLower(strings.TrimSpace(cmd.Status))
	if status == "" {
		status = defaultDocumentLayerStatus
	}

	content := cmd.Content
	if content == nil {
		content = map[string]any{}
	}
	properties := cmd.Properties
	if properties == nil {
		properties = map[string]any{}
	}

	return &entity.DocumentLayer{
		DocumentToken: strings.TrimSpace(cmd.DocumentToken),
		ParentToken:   strings.TrimSpace(cmd.ParentToken),
		Element: entity.DocumentElement{
			Token: strings.TrimSpace(cmd.ElementToken),
		},
		Region:     region,
		Name:       strings.TrimSpace(cmd.Name),
		Content:    content,
		Properties: properties,
		Position:   cmd.Position,
		Status:     status,
	}
}

func validateDocumentLayer(layer *entity.DocumentLayer) error {
	if err := validateDocumentLayerToken(layer.DocumentToken, "document token"); err != nil {
		return err
	}
	if err := validateOptionalDocumentLayerToken(layer.ParentToken, "parent layer token"); err != nil {
		return err
	}
	if err := validateDocumentLayerToken(layer.Element.Token, "document element token"); err != nil {
		return err
	}
	if _, ok := allowedDocumentLayerRegions[layer.Region]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid document layer region")
	}
	if layer.Name == "" {
		return domain.NewError(domain.ErrInvalidInput, "document layer name cannot be empty")
	}
	if layer.Position < 0 {
		return domain.NewError(domain.ErrInvalidInput, "document layer position cannot be negative")
	}
	if _, ok := allowedDocumentLayerStatuses[layer.Status]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid document layer status")
	}

	return nil
}

func normalizeDocumentLayerRegion(region string) string {
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "" {
		return defaultDocumentLayerRegion
	}

	return region
}

func validateDocumentLayerToken(token, label string) error {
	if strings.TrimSpace(token) == "" {
		return domain.NewError(domain.ErrInvalidInput, label+" cannot be empty")
	}
	if _, err := uuid.Parse(token); err != nil {
		return domain.NewError(domain.ErrInvalidInput, "invalid "+label)
	}

	return nil
}

func validateOptionalDocumentLayerToken(token, label string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}

	return validateDocumentLayerToken(token, label)
}

func normalizeDeleteDocumentLayerTokens(tokens []string) ([]string, error) {
	unique := make(map[string]struct{}, len(tokens))
	normalized := make([]string, 0, len(tokens))

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if err := validateDocumentLayerToken(token, "document layer token"); err != nil {
			return nil, err
		}
		if _, exists := unique[token]; exists {
			continue
		}

		unique[token] = struct{}{}
		normalized = append(normalized, token)
	}

	if len(normalized) == 0 {
		return nil, domain.NewError(domain.ErrInvalidInput, "at least one document layer token is required")
	}

	return normalized, nil
}
