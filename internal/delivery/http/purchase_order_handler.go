package http

import (
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
	var req dto.UpsertPurchaseOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.purchaseOrder.Upsert(c.Request.Context(), req.ToUpsertPurchaseOrderCommand()); err != nil {
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

func (h *PurchaseOrderHandler) DeleteByQuotationID(c *gin.Context) {
	quotationID, err := parsePurchaseOrderQuotationID(c.Param("quotationID"))
	if err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid quotation id")
		return
	}

	if err := h.purchaseOrder.DeleteByQuotationID(c.Request.Context(), quotationID); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Purchase order deleted successfully", nil)
}

func parsePurchaseOrderQuotationID(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}
