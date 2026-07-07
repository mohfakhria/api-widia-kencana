package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type WorkflowHandler struct {
	workflow input.WorkflowUseCase
}

func NewWorkflowHandler(workflow input.WorkflowUseCase) *WorkflowHandler {
	return &WorkflowHandler{workflow: workflow}
}

func (h *WorkflowHandler) List(c *gin.Context) {
	workflows, err := h.workflow.List(c.Request.Context())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewWorkflowListResponses(workflows))
}

func (h *WorkflowHandler) Get(c *gin.Context) {
	workflow, err := h.workflow.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewWorkflowDetailResponse(workflow))
}

func (h *WorkflowHandler) Create(c *gin.Context) {
	var req dto.WorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	workflow, err := h.workflow.Create(c.Request.Context(), req.ToCreateWorkflowCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Workflow created successfully", dto.NewWorkflowDataResponse(workflow))
}

func (h *WorkflowHandler) Update(c *gin.Context) {
	var req dto.WorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid payload")
		return
	}

	if err := h.workflow.Update(c.Request.Context(), c.Param("id"), req.ToUpdateWorkflowCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Workflow updated successfully", nil)
}

func (h *WorkflowHandler) Delete(c *gin.Context) {
	if err := h.workflow.Delete(c.Request.Context(), c.Param("id")); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Workflow deleted successfully", nil)
}
