package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type ProjectHandler struct {
	project input.ProjectUseCase
}

func NewProjectHandler(project input.ProjectUseCase) *ProjectHandler {
	return &ProjectHandler{project: project}
}

func (h *ProjectHandler) List(c *gin.Context) {
	projects, err := h.project.List(c.Request.Context())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewProjectListResponses(projects))
}

func (h *ProjectHandler) Get(c *gin.Context) {
	project, err := h.project.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Success", dto.NewProjectDataResponse(project))
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var req dto.ProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	project, err := h.project.Create(c.Request.Context(), req.ToCreateProjectCommand())
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project created successfully", dto.NewProjectDataResponse(project))
}

func (h *ProjectHandler) Update(c *gin.Context) {
	var req dto.ProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid payload")
		return
	}

	if err := h.project.Update(c.Request.Context(), c.Param("id"), req.ToUpdateProjectCommand()); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project updated successfully", nil)
}

func (h *ProjectHandler) Delete(c *gin.Context) {
	if err := h.project.Delete(c.Request.Context(), c.Param("id")); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Project deleted successfully", nil)
}
