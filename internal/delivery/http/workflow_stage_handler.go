package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type WorkflowStageHandler struct {
	stage input.WorkflowStageUseCase
}

func NewWorkflowStageHandler(stage input.WorkflowStageUseCase) *WorkflowStageHandler {
	return &WorkflowStageHandler{stage: stage}
}

func (h *WorkflowStageHandler) ListByWorkflowID(c *gin.Context) {
	stages, err := h.stage.ListByWorkflowID(c.Request.Context(), c.Param("workflowID"))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewWorkflowStageListResponses(stages))
}

func (h *WorkflowStageHandler) Get(c *gin.Context) {
	stage, err := h.stage.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewWorkflowStageDataResponse(stage))
}

func (h *WorkflowStageHandler) Create(c *gin.Context) {
	var req dto.WorkflowStageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	stage, err := h.stage.Create(c.Request.Context(), req.ToCreateWorkflowStageCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Workflow stage created successfully", dto.NewWorkflowStageDataResponse(stage))
}

func (h *WorkflowStageHandler) Update(c *gin.Context) {
	var req dto.WorkflowStageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid payload")
		return
	}

	if err := h.stage.Update(c.Request.Context(), c.Param("id"), req.ToUpdateWorkflowStageCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Workflow stage updated successfully", nil)
}

func (h *WorkflowStageHandler) Sort(c *gin.Context) {
	var req dto.SortWorkflowStageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid payload")
		return
	}

	if err := h.stage.Sort(c.Request.Context(), req.ToSortWorkflowStageCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Workflow stage sorted successfully", nil)
}

func (h *WorkflowStageHandler) Delete(c *gin.Context) {
	if err := h.stage.Delete(c.Request.Context(), c.Param("id")); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Workflow stage deleted successfully", nil)
}
