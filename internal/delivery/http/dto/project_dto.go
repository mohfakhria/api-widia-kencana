package dto

import (
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type ProjectRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type ProjectResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProjectDataResponse struct {
	Project ProjectResponse `json:"project"`
}

type ProjectListResponse struct {
	Projects []ProjectResponse `json:"projects"`
}

func (r ProjectRequest) ToCreateProjectCommand() input.CreateProjectCommand {
	return input.CreateProjectCommand{
		Name:   r.Name,
		Status: r.Status,
	}
}

func (r ProjectRequest) ToUpdateProjectCommand() input.UpdateProjectCommand {
	return input.UpdateProjectCommand(r.ToCreateProjectCommand())
}

func NewProjectResponse(project *entity.Project) ProjectResponse {
	return ProjectResponse{
		ID:        project.ID,
		Name:      project.Name,
		Status:    project.Status,
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
	}
}

func NewProjectDataResponse(project *entity.Project) ProjectDataResponse {
	return ProjectDataResponse{Project: NewProjectResponse(project)}
}

func NewProjectListResponses(projects []entity.Project) ProjectListResponse {
	responses := ProjectListResponse{
		Projects: make([]ProjectResponse, 0, len(projects)),
	}
	for _, project := range projects {
		responses.Projects = append(responses.Projects, NewProjectResponse(&project))
	}

	return responses
}
