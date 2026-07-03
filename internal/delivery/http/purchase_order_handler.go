package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type PurchaseOrderHandler struct {
	purchaseOrder input.PurchaseOrderUseCase
}

func NewPurchaseOrderHandler(purchaseOrder input.PurchaseOrderUseCase) *PurchaseOrderHandler {
	return &PurchaseOrderHandler{purchaseOrder: purchaseOrder}
}

func (h *PurchaseOrderHandler) Upsert(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid multipart payload")
		return
	}

	req, err := dto.NewUpsertPurchaseOrderRequest(form.Value)
	if err != nil {
		dto.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	cmd := req.ToUpsertPurchaseOrderCommand()

	fileHeader, err := c.FormFile("asset")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		dto.Error(c, http.StatusBadRequest, "Invalid asset file")
		return
	}
	if fileHeader != nil {
		file, err := fileHeader.Open()
		if err != nil {
			dto.Error(c, http.StatusBadRequest, "Invalid asset file")
			return
		}
		defer file.Close()

		userID := c.GetInt64("userIDInt")
		var uploadedBy *int64
		if userID != 0 {
			uploadedBy = &userID
		}

		cmd.Asset = &input.PurchaseOrderAssetUploadCommand{
			Reader:           file,
			Size:             fileHeader.Size,
			OriginalFilename: fileHeader.Filename,
			ContentType:      fileHeader.Header.Get("Content-Type"),
			Category:         "attachment",
			UploadedBy:       uploadedBy,
		}
	}

	if err := h.purchaseOrder.Upsert(c.Request.Context(), cmd); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Purchase order upserted successfully", nil)
}

func (h *PurchaseOrderHandler) GetByQuotationID(c *gin.Context) {
	quotationID, err := parsePurchaseOrderQuotationID(c.Param("quotationID"))
	if err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid quotation id")
		return
	}

	items, err := h.purchaseOrder.GetByQuotationID(c.Request.Context(), quotationID)
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Purchase order", dto.NewPurchaseOrderResponse(quotationID, items))
}

func parsePurchaseOrderQuotationID(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}
