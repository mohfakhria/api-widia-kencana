package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type AssetHandler struct {
	asset input.AssetUseCase
}

func NewAssetHandler(asset input.AssetUseCase) *AssetHandler {
	return &AssetHandler{asset: asset}
}

func (h *AssetHandler) RequestUpload(c *gin.Context) {
	var req dto.AssetUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	result, err := h.asset.RequestUpload(c.Request.Context(), req.ToRequestAssetUploadCommand(currentUserID(c)))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Asset upload URL generated successfully", dto.NewAssetUploadRequestResponse(result))
}

func (h *AssetHandler) CompleteUpload(c *gin.Context) {
	asset, err := h.asset.CompleteUpload(c.Request.Context(), c.Param("token"), currentUserID(c))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Asset upload completed successfully", dto.NewAssetDataResponse(asset))
}

func (h *AssetHandler) List(c *gin.Context) {
	var req dto.AssetListFilterRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid query parameters")
		return
	}

	query := req.ToListAssetQuery()
	query.UploadedBy = currentUserID(c)
	assets, err := h.asset.List(c.Request.Context(), query)
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewAssetListResponse(assets))
}

func (h *AssetHandler) Get(c *gin.Context) {
	asset, err := h.asset.GetByToken(c.Request.Context(), c.Param("token"), currentUserID(c))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewAssetDataResponse(asset))
}

func (h *AssetHandler) PresignGet(c *gin.Context) {
	result, err := h.asset.PresignGet(c.Request.Context(), c.Param("token"), currentUserID(c))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Asset URL generated successfully", dto.NewAssetPresignGetResponse(result))
}

func (h *AssetHandler) Delete(c *gin.Context) {
	if err := h.asset.Delete(c.Request.Context(), c.Param("token"), currentUserID(c)); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Asset deleted successfully", nil)
}

func currentUserID(c *gin.Context) *int64 {
	userID := c.GetInt64("userIDInt")
	if userID == 0 {
		return nil
	}

	return &userID
}
