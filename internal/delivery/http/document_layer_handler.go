package http

import (
	"errors"
	"io"
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type DocumentLayerHandler struct {
	layer input.DocumentLayerUseCase
}

func NewDocumentLayerHandler(layer input.DocumentLayerUseCase) *DocumentLayerHandler {
	return &DocumentLayerHandler{layer: layer}
}

func (h *DocumentLayerHandler) Create(c *gin.Context) {
	var req dto.DocumentLayerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	layer, err := h.layer.Create(c.Request.Context(), req.ToCreateDocumentLayerCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Document layer created successfully", dto.NewDocumentLayerDataResponse(layer))
}

func (h *DocumentLayerHandler) Update(c *gin.Context) {
	var req dto.DocumentLayerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.layer.Update(c.Request.Context(), c.Param("token"), req.ToUpdateDocumentLayerCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Document layer updated successfully", nil)
}

func (h *DocumentLayerHandler) Sort(c *gin.Context) {
	var req dto.SortDocumentLayerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.layer.Sort(c.Request.Context(), req.ToSortDocumentLayerCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Document layer sorted successfully", nil)
}

func (h *DocumentLayerHandler) Delete(c *gin.Context) {
	var req dto.DeleteDocumentLayerRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.layer.Delete(c.Request.Context(), req.ToDeleteDocumentLayerCommand(c.Param("token"))); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Document layer deleted successfully", nil)
}
