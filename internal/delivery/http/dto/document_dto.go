package dto

import (
	"strings"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type CreateDocumentRequest struct {
	DocumentPaperToken string         `json:"document_paper_token"`
	ParentToken        string         `json:"parent_token"`
	Name               string         `json:"name"`
	DocumentType       string         `json:"document_type"`
	Settings           map[string]any `json:"settings"`
	Status             string         `json:"status"`
}

type UpdateDocumentRequest struct {
	DocumentPaperToken string         `json:"document_paper_token"`
	ParentToken        string         `json:"parent_token"`
	Name               string         `json:"name"`
	DocumentType       string         `json:"document_type"`
	Settings           map[string]any `json:"settings"`
	Position           int            `json:"position"`
	Status             string         `json:"status"`
}

type DocumentListFilterRequest struct {
	Name  string `form:"name"`
	Token string `form:"token"`
}

type DocumentDetailQueryRequest struct {
	WithLayer bool `form:"with_layer"`
}

type DocumentResponse struct {
	Token        string                        `json:"token"`
	ParentToken  string                        `json:"parent_token"`
	Name         string                        `json:"name"`
	DocumentType string                        `json:"document_type"`
	Settings     map[string]any                `json:"settings"`
	Position     int                           `json:"position"`
	Status       string                        `json:"status"`
	Paper        DocumentPaperResponse         `json:"paper"`
	CreatedAt    time.Time                     `json:"created_at"`
	UpdatedAt    time.Time                     `json:"updated_at"`
	Layers       *DocumentLayerRegionsResponse `json:"layers,omitempty"`
}

type DocumentLayerRegionsResponse struct {
	Header []DocumentLayerTreeResponse `json:"header"`
	Body   []DocumentLayerTreeResponse `json:"body"`
	Footer []DocumentLayerTreeResponse `json:"footer"`
}

type DocumentLayerTreeResponse struct {
	Token       string                      `json:"token"`
	ParentToken string                      `json:"parent_token"`
	Element     DocumentElementResponse     `json:"element"`
	Region      string                      `json:"region"`
	Name        string                      `json:"name"`
	Content     map[string]any              `json:"content"`
	Properties  map[string]any              `json:"properties"`
	Position    int                         `json:"position"`
	Status      string                      `json:"status"`
	CreatedAt   time.Time                   `json:"created_at"`
	UpdatedAt   time.Time                   `json:"updated_at"`
	Children    []DocumentLayerTreeResponse `json:"children"`
}

type DocumentDataResponse struct {
	Document DocumentResponse `json:"document"`
}

type DocumentListResponse struct {
	Documents []DocumentResponse `json:"documents"`
}

func (r CreateDocumentRequest) ToCreateDocumentCommand() input.CreateDocumentCommand {
	return input.CreateDocumentCommand{
		DocumentPaperToken: r.DocumentPaperToken,
		ParentToken:        r.ParentToken,
		Name:               r.Name,
		DocumentType:       r.DocumentType,
		Settings:           r.Settings,
		Status:             r.Status,
	}
}

func (r UpdateDocumentRequest) ToUpdateDocumentCommand() input.UpdateDocumentCommand {
	return input.UpdateDocumentCommand{
		DocumentPaperToken: r.DocumentPaperToken,
		ParentToken:        r.ParentToken,
		Name:               r.Name,
		DocumentType:       r.DocumentType,
		Settings:           r.Settings,
		Position:           r.Position,
		Status:             r.Status,
	}
}

func (r DocumentListFilterRequest) ToListDocumentQuery() input.ListDocumentQuery {
	return input.ListDocumentQuery{
		Name:  strings.TrimSpace(r.Name),
		Token: strings.TrimSpace(r.Token),
	}
}

func (r DocumentDetailQueryRequest) ToGetDocumentQuery() input.GetDocumentQuery {
	return input.GetDocumentQuery{
		WithLayer: r.WithLayer,
	}
}

func NewDocumentResponse(document *entity.Document) DocumentResponse {
	response := DocumentResponse{
		Token:        document.Token,
		ParentToken:  document.ParentToken,
		Name:         document.Name,
		DocumentType: document.DocumentType,
		Settings:     document.Settings,
		Position:     document.Position,
		Status:       document.Status,
		Paper:        NewDocumentPaperResponse(document.Paper),
		CreatedAt:    document.CreatedAt,
		UpdatedAt:    document.UpdatedAt,
	}
	if document.WithLayers {
		response.Layers = NewDocumentLayerRegionsResponse(document.Layers)
	}

	return response
}

func NewDocumentDataResponse(document *entity.Document) DocumentDataResponse {
	return DocumentDataResponse{Document: NewDocumentResponse(document)}
}

func NewDocumentListResponse(documents []entity.Document) DocumentListResponse {
	response := DocumentListResponse{
		Documents: make([]DocumentResponse, 0, len(documents)),
	}
	for _, document := range documents {
		response.Documents = append(response.Documents, NewDocumentResponse(&document))
	}

	return response
}

func NewDocumentLayerRegionsResponse(
	regions entity.DocumentLayerRegions,
) *DocumentLayerRegionsResponse {
	return &DocumentLayerRegionsResponse{
		Header: NewDocumentLayerTreeResponses(regions.Header),
		Body:   NewDocumentLayerTreeResponses(regions.Body),
		Footer: NewDocumentLayerTreeResponses(regions.Footer),
	}
}

func NewDocumentLayerTreeResponses(layers []entity.DocumentLayer) []DocumentLayerTreeResponse {
	responses := make([]DocumentLayerTreeResponse, 0, len(layers))
	for _, layer := range layers {
		responses = append(responses, NewDocumentLayerTreeResponse(&layer))
	}

	return responses
}

func NewDocumentLayerTreeResponse(layer *entity.DocumentLayer) DocumentLayerTreeResponse {
	return DocumentLayerTreeResponse{
		Token:       layer.Token,
		ParentToken: layer.ParentToken,
		Element:     NewDocumentElementResponse(layer.Element),
		Region:      layer.Region,
		Name:        layer.Name,
		Content:     layer.Content,
		Properties:  layer.Properties,
		Position:    layer.Position,
		Status:      layer.Status,
		CreatedAt:   layer.CreatedAt,
		UpdatedAt:   layer.UpdatedAt,
		Children:    NewDocumentLayerTreeResponses(layer.Children),
	}
}
