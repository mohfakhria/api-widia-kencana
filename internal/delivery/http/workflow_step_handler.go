package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type WorkflowStepHandler struct {
	step input.WorkflowStepUseCase
}

func NewWorkflowStepHandler(step input.WorkflowStepUseCase) *WorkflowStepHandler {
	return &WorkflowStepHandler{step: step}
}

func (h *WorkflowStepHandler) ListByWorkflowStageID(c *gin.Context) {
	steps, err := h.step.ListByWorkflowStageID(c.Request.Context(), c.Param("workflowStageID"))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewWorkflowStepListResponses(steps))
}

func (h *WorkflowStepHandler) Get(c *gin.Context) {
	step, err := h.step.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewWorkflowStepResponse(step))
}

func (h *WorkflowStepHandler) Create(c *gin.Context) {
	var req dto.WorkflowStepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	step, err := h.step.Create(c.Request.Context(), req.ToCreateWorkflowStepCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Workflow step created successfully", dto.NewWorkflowStepResponse(step))
}

func (h *WorkflowStepHandler) Update(c *gin.Context) {
	var req dto.WorkflowStepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid payload")
		return
	}

	if err := h.step.Update(c.Request.Context(), c.Param("id"), req.ToUpdateWorkflowStepCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Workflow step updated successfully", nil)
}

func (h *WorkflowStepHandler) Delete(c *gin.Context) {
	if err := h.step.Delete(c.Request.Context(), c.Param("id")); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Workflow step deleted successfully", nil)
}
