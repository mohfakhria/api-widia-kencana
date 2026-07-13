package http

import (
	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type DocumentBuilderMetadataHandler struct {
	metadata input.DocumentBuilderMetadataUseCase
}

func NewDocumentBuilderMetadataHandler(
	metadata input.DocumentBuilderMetadataUseCase,
) *DocumentBuilderMetadataHandler {
	return &DocumentBuilderMetadataHandler{metadata: metadata}
}

func (h *DocumentBuilderMetadataHandler) Get(c *gin.Context) {
	metadata, err := h.metadata.Get(c.Request.Context())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewDocumentBuilderMetadataDataResponse(metadata))
}
