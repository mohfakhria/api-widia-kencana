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

func (h *DocumentHandler) GetMetadata(c *gin.Context) {
	metadata, err := h.document.GetMetadata(c.Request.Context())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewDocumentMetadataDataResponse(metadata))
}

func (h *DocumentHandler) List(c *gin.Context) {
	documents, err := h.document.List(c.Request.Context())
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
	var req dto.DocumentRequest
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
	var req dto.DocumentRequest
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
