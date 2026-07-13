package dto

import (
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type DocumentRequest struct {
	DocumentPaperToken string `json:"document_paper_token"`
	ParentToken        string `json:"parent_token"`
	Name               string `json:"name"`
	DocumentType       string `json:"document_type"`
	Position           int    `json:"position"`
	Status             string `json:"status"`
}

type DocumentResponse struct {
	Token        string                `json:"token"`
	ParentToken  string                `json:"parent_token"`
	Name         string                `json:"name"`
	DocumentType string                `json:"document_type"`
	Position     int                   `json:"position"`
	Status       string                `json:"status"`
	Paper        DocumentPaperResponse `json:"paper"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

type DocumentDataResponse struct {
	Document DocumentResponse `json:"document"`
}

type DocumentListResponse struct {
	Documents []DocumentResponse `json:"documents"`
}

func (r DocumentRequest) ToCreateDocumentCommand() input.CreateDocumentCommand {
	return input.CreateDocumentCommand{
		DocumentPaperToken: r.DocumentPaperToken,
		ParentToken:        r.ParentToken,
		Name:               r.Name,
		DocumentType:       r.DocumentType,
		Position:           r.Position,
		Status:             r.Status,
	}
}

func (r DocumentRequest) ToUpdateDocumentCommand() input.UpdateDocumentCommand {
	return input.UpdateDocumentCommand(r.ToCreateDocumentCommand())
}

func NewDocumentResponse(document *entity.Document) DocumentResponse {
	return DocumentResponse{
		Token:        document.Token,
		ParentToken:  document.ParentToken,
		Name:         document.Name,
		DocumentType: document.DocumentType,
		Position:     document.Position,
		Status:       document.Status,
		Paper:        NewDocumentPaperResponse(document.Paper),
		CreatedAt:    document.CreatedAt,
		UpdatedAt:    document.UpdatedAt,
	}
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
