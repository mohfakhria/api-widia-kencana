package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type DocumentHandler struct {
	document input.DocumentUseCase
}

func NewDocumentHandler(document input.DocumentUseCase) *DocumentHandler {
	return &DocumentHandler{document: document}
}

func (h *DocumentHandler) ListPapers(c *gin.Context) {
	papers, err := h.document.ListPapers(c.Request.Context())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewDocumentPapersDataResponse(papers))
}

func (h *DocumentHandler) ListElements(c *gin.Context) {
	var req dto.DocumentElementFilterRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid query parameters")
		return
	}

	elements, err := h.document.ListElements(c.Request.Context(), req.ToListDocumentElementQuery())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewDocumentElementsDataResponse(elements))
}

func (h *DocumentHandler) ListProperties(c *gin.Context) {
	properties, err := h.document.ListProperties(c.Request.Context())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewDocumentPropertiesDataResponse(properties))
}

func (h *DocumentHandler) ListPropertyOptions(c *gin.Context) {
	options, err := h.document.ListPropertyOptions(c.Request.Context())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewDocumentPropertyOptionsDataResponse(options))
}

func (h *DocumentHandler) ListElementProperties(c *gin.Context) {
	var req dto.DocumentElementPropertyFilterRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid query parameters")
		return
	}

	elementProperties, err := h.document.ListElementProperties(
		c.Request.Context(),
		req.ToListDocumentElementPropertyQuery(),
	)
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewDocumentElementPropertiesDataResponse(elementProperties))
}

func (h *DocumentHandler) List(c *gin.Context) {
	var req dto.DocumentListFilterRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid query parameters")
		return
	}

	documents, err := h.document.List(c.Request.Context(), req.ToListDocumentQuery())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewDocumentListResponse(documents))
}

func (h *DocumentHandler) Get(c *gin.Context) {
	document, err := h.document.GetByToken(c.Request.Context(), c.Param("token"))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewDocumentDataResponse(document))
}

func (h *DocumentHandler) Create(c *gin.Context) {
	var req dto.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	document, err := h.document.Create(c.Request.Context(), req.ToCreateDocumentCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Document created successfully", dto.NewDocumentDataResponse(document))
}

func (h *DocumentHandler) Update(c *gin.Context) {
	var req dto.UpdateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.document.Update(c.Request.Context(), c.Param("token"), req.ToUpdateDocumentCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Document updated successfully", nil)
}

func (h *DocumentHandler) Delete(c *gin.Context) {
	if err := h.document.Delete(c.Request.Context(), c.Param("token")); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Document deleted successfully", nil)
}
