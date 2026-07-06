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

func NewWorkflowListResponses(workflows []entity.Workflow) WorkflowListResponse {
	responses := WorkflowListResponse{
		Workflows: make([]WorkflowResponse, 0, len(workflows)),
	}
	for _, workflow := range workflows {
		responses.Workflows = append(responses.Workflows, NewWorkflowResponse(&workflow))
	}

	return responses
}
