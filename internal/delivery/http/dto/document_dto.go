package dto

import (
	"strings"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type DocumentRequest struct {
	DocumentPaperToken string `json:"document_paper_token"`
	ParentToken        string `json:"parent_token"`
	Name               string `json:"name"`
	DocumentType       string `json:"document_type"`
	Status             string `json:"status"`
}

type DocumentListFilterRequest struct {
	Name         string `form:"name"`
	Token        string `form:"token"`
	DocumentType string `form:"document_type"`
	Status       string `form:"status"`
}

type DocumentResponse struct {
	Token        string                `json:"token"`
	ParentToken  string                `json:"parent_token"`
	Name         string                `json:"name"`
	DocumentType string                `json:"document_type"`
	Status       string                `json:"status"`
	Paper        DocumentPaperResponse `json:"paper"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

type DocumentPaperResponse struct {
	Token          string    `json:"token"`
	Name           string    `json:"name"`
	MediaType      string    `json:"media_type"`
	Width          float64   `json:"width"`
	Height         float64   `json:"height"`
	Unit           string    `json:"unit"`
	AllowPortrait  bool      `json:"allow_portrait"`
	AllowLandscape bool      `json:"allow_landscape"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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
		Status:             r.Status,
	}
}

func (r DocumentRequest) ToUpdateDocumentCommand() input.UpdateDocumentCommand {
	return input.UpdateDocumentCommand(r.ToCreateDocumentCommand())
}

func (r DocumentListFilterRequest) ToListDocumentQuery() input.ListDocumentQuery {
	return input.ListDocumentQuery{
		Name:         strings.TrimSpace(r.Name),
		Token:        strings.TrimSpace(r.Token),
		DocumentType: strings.TrimSpace(r.DocumentType),
		Status:       strings.TrimSpace(r.Status),
	}
}

func NewDocumentResponse(document *entity.Document) DocumentResponse {
	return DocumentResponse{
		Token:        document.Token,
		ParentToken:  document.ParentToken,
		Name:         document.Name,
		DocumentType: document.DocumentType,
		Status:       document.Status,
		Paper:        NewDocumentPaperResponse(document.Paper),
		CreatedAt:    document.CreatedAt,
		UpdatedAt:    document.UpdatedAt,
	}
}

func NewDocumentPaperResponse(paper entity.DocumentPaper) DocumentPaperResponse {
	return DocumentPaperResponse{
		Token:          paper.Token,
		Name:           paper.Name,
		MediaType:      paper.MediaType,
		Width:          paper.Width,
		Height:         paper.Height,
		Unit:           paper.Unit,
		AllowPortrait:  paper.AllowPortrait,
		AllowLandscape: paper.AllowLandscape,
		Status:         paper.Status,
		CreatedAt:      paper.CreatedAt,
		UpdatedAt:      paper.UpdatedAt,
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
