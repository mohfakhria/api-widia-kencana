package dto

import (
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type WorkflowRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type WorkflowResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WorkflowDetailResponse struct {
	ID        int64                       `json:"id"`
	Name      string                      `json:"name"`
	Status    string                      `json:"status"`
	CreatedAt time.Time                   `json:"created_at"`
	UpdatedAt time.Time                   `json:"updated_at"`
	Stages    []WorkflowStageTreeResponse `json:"stages"`
}

type WorkflowDataResponse struct {
	Workflow WorkflowResponse `json:"workflow"`
}

type WorkflowDetailDataResponse struct {
	Workflow WorkflowDetailResponse `json:"workflow"`
}

type WorkflowStageTreeResponse struct {
	ID         int64                      `json:"id"`
	WorkflowID int64                      `json:"workflow_id"`
	Name       string                     `json:"name"`
	Position   int                        `json:"position"`
	Status     string                     `json:"status"`
	CreatedAt  time.Time                  `json:"created_at"`
	UpdatedAt  time.Time                  `json:"updated_at"`
	Steps      []WorkflowStepTreeResponse `json:"steps"`
}

type WorkflowStepTreeResponse struct {
	ID              int64     `json:"id"`
	WorkflowStageID int64     `json:"workflow_stage_id"`
	Name            string    `json:"name"`
	Position        int       `json:"position"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type WorkflowListResponse struct {
	Workflows []WorkflowResponse `json:"workflows"`
}

func (r WorkflowRequest) ToCreateWorkflowCommand() input.CreateWorkflowCommand {
	return input.CreateWorkflowCommand{
		Name:   r.Name,
		Status: r.Status,
	}
}

func (r WorkflowRequest) ToUpdateWorkflowCommand() input.UpdateWorkflowCommand {
	return input.UpdateWorkflowCommand(r.ToCreateWorkflowCommand())
}

func NewWorkflowResponse(workflow *entity.Workflow) WorkflowResponse {
	return WorkflowResponse{
		ID:        workflow.ID,
		Name:      workflow.Name,
		Status:    workflow.Status,
		CreatedAt: workflow.CreatedAt,
		UpdatedAt: workflow.UpdatedAt,
	}
}

func NewWorkflowDataResponse(workflow *entity.Workflow) WorkflowDataResponse {
	return WorkflowDataResponse{Workflow: NewWorkflowResponse(workflow)}
}

func NewWorkflowDetailResponse(workflow *entity.Workflow) WorkflowDetailDataResponse {
	return WorkflowDetailDataResponse{Workflow: newWorkflowDetailObjectResponse(workflow)}
}

func newWorkflowDetailObjectResponse(workflow *entity.Workflow) WorkflowDetailResponse {
	response := WorkflowDetailResponse{
		ID:        workflow.ID,
		Name:      workflow.Name,
		Status:    workflow.Status,
		CreatedAt: workflow.CreatedAt,
		UpdatedAt: workflow.UpdatedAt,
		Stages:    make([]WorkflowStageTreeResponse, 0, len(workflow.Stages)),
	}

	for _, stage := range workflow.Stages {
		mappedStage := WorkflowStageTreeResponse{
			ID:         stage.ID,
			WorkflowID: stage.WorkflowID,
			Name:       stage.Name,
			Position:   stage.Position,
			Status:     stage.Status,
			CreatedAt:  stage.CreatedAt,
			UpdatedAt:  stage.UpdatedAt,
			Steps:      make([]WorkflowStepTreeResponse, 0, len(stage.Steps)),
		}

		for _, step := range stage.Steps {
			mappedStage.Steps = append(mappedStage.Steps, WorkflowStepTreeResponse{
				ID:              step.ID,
				WorkflowStageID: step.WorkflowStageID,
				Name:            step.Name,
				Position:        step.Position,
				Status:          step.Status,
				CreatedAt:       step.CreatedAt,
				UpdatedAt:       step.UpdatedAt,
			})
		}

		response.Stages = append(response.Stages, mappedStage)
	}

	return response
}

func NewWorkflowListResponses(workflows []entity.Workflow) WorkflowListResponse {
	responses := WorkflowListResponse{
		Workflows: make([]WorkflowResponse, 0, len(workflows)),
	}
	for _, workflow := range workflows {
		responses.Workflows = append(responses.Workflows, NewWorkflowResponse(&workflow))
	}

	return responses
}
