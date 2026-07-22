package dto

import (
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type DocumentLayerRequest struct {
	DocumentToken string         `json:"document_token"`
	ParentToken   string         `json:"parent_token"`
	ElementToken  string         `json:"element_token"`
	Region        string         `json:"region"`
	Name          string         `json:"name"`
	Content       map[string]any `json:"content"`
	Properties    map[string]any `json:"properties"`
	Position      int            `json:"position"`
	Status        string         `json:"status"`
}

type SortDocumentLayerRequest struct {
	DocumentToken string                         `json:"document_token"`
	ParentToken   string                         `json:"parent_token"`
	Region        string                         `json:"region"`
	Layers        []SortDocumentLayerItemRequest `json:"layers"`
}

type SortDocumentLayerItemRequest struct {
	Token    string `json:"token"`
	Position int    `json:"position"`
}

type DeleteDocumentLayerRequest struct {
	DocumentToken string   `json:"document_token"`
	Tokens        []string `json:"tokens"`
}

type DocumentLayerResponse struct {
	Token         string                  `json:"token"`
	DocumentToken string                  `json:"document_token"`
	ParentToken   string                  `json:"parent_token"`
	Element       DocumentElementResponse `json:"element"`
	Region        string                  `json:"region"`
	Name          string                  `json:"name"`
	Content       map[string]any          `json:"content"`
	Properties    map[string]any          `json:"properties"`
	Position      int                     `json:"position"`
	Status        string                  `json:"status"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
}

type DocumentLayerDataResponse struct {
	Layer DocumentLayerResponse `json:"layer"`
}

func (r DocumentLayerRequest) ToCreateDocumentLayerCommand() input.CreateDocumentLayerCommand {
	return input.CreateDocumentLayerCommand{
		DocumentToken: r.DocumentToken,
		ParentToken:   r.ParentToken,
		ElementToken:  r.ElementToken,
		Region:        r.Region,
		Name:          r.Name,
		Content:       r.Content,
		Properties:    r.Properties,
		Position:      r.Position,
		Status:        r.Status,
	}
}

func (r DocumentLayerRequest) ToUpdateDocumentLayerCommand() input.UpdateDocumentLayerCommand {
	return input.UpdateDocumentLayerCommand(r.ToCreateDocumentLayerCommand())
}

func (r SortDocumentLayerRequest) ToSortDocumentLayerCommand() input.SortDocumentLayerCommand {
	cmd := input.SortDocumentLayerCommand{
		DocumentToken: r.DocumentToken,
		ParentToken:   r.ParentToken,
		Region:        r.Region,
		Layers:        make([]input.SortDocumentLayerItemCommand, 0, len(r.Layers)),
	}
	for _, layer := range r.Layers {
		cmd.Layers = append(cmd.Layers, input.SortDocumentLayerItemCommand{
			Token:    layer.Token,
			Position: layer.Position,
		})
	}

	return cmd
}

func (r DeleteDocumentLayerRequest) ToDeleteDocumentLayerCommand(pathToken string) input.DeleteDocumentLayerCommand {
	tokens := make([]string, 0, len(r.Tokens)+1)
	if pathToken != "" {
		tokens = append(tokens, pathToken)
	}
	tokens = append(tokens, r.Tokens...)

	return input.DeleteDocumentLayerCommand{
		DocumentToken: r.DocumentToken,
		Tokens:        tokens,
	}
}

func NewDocumentLayerResponse(layer *entity.DocumentLayer) DocumentLayerResponse {
	return DocumentLayerResponse{
		Token:         layer.Token,
		DocumentToken: layer.DocumentToken,
		ParentToken:   layer.ParentToken,
		Element:       NewDocumentElementResponse(layer.Element),
		Region:        layer.Region,
		Name:          layer.Name,
		Content:       layer.Content,
		Properties:    layer.Properties,
		Position:      layer.Position,
		Status:        layer.Status,
		CreatedAt:     layer.CreatedAt,
		UpdatedAt:     layer.UpdatedAt,
	}
}

func NewDocumentLayerDataResponse(layer *entity.DocumentLayer) DocumentLayerDataResponse {
	return DocumentLayerDataResponse{Layer: NewDocumentLayerResponse(layer)}
}
