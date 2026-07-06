package dto

import (
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type WorkflowStageRequest struct {
	WorkflowID int64  `json:"workflow_id"`
	Name       string `json:"name"`
	Position   int    `json:"position"`
	Status     string `json:"status"`
}

type WorkflowStageResponse struct {
	ID         int64     `json:"id"`
	WorkflowID int64     `json:"workflow_id"`
	Name       string    `json:"name"`
	Position   int       `json:"position"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type WorkflowStageListResponse struct {
	Stages []WorkflowStageResponse `json:"stages"`
}

func (r WorkflowStageRequest) ToCreateWorkflowStageCommand() input.CreateWorkflowStageCommand {
	return input.CreateWorkflowStageCommand{
		WorkflowID: r.WorkflowID,
		Name:       r.Name,
		Position:   r.Position,
		Status:     r.Status,
	}
}

func (r WorkflowStageRequest) ToUpdateWorkflowStageCommand() input.UpdateWorkflowStageCommand {
	return input.UpdateWorkflowStageCommand(r.ToCreateWorkflowStageCommand())
}

func NewWorkflowStageResponse(stage *entity.WorkflowStage) WorkflowStageResponse {
	return WorkflowStageResponse{
		ID:         stage.ID,
		WorkflowID: stage.WorkflowID,
		Name:       stage.Name,
		Position:   stage.Position,
		Status:     stage.Status,
		CreatedAt:  stage.CreatedAt,
		UpdatedAt:  stage.UpdatedAt,
	}
}

func NewWorkflowStageListResponses(stages []entity.WorkflowStage) WorkflowStageListResponse {
	responses := WorkflowStageListResponse{
		Stages: make([]WorkflowStageResponse, 0, len(stages)),
	}
	for _, stage := range stages {
		responses.Stages = append(responses.Stages, NewWorkflowStageResponse(&stage))
	}

	return responses
}
