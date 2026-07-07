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

type SortWorkflowStageRequest struct {
	WorkflowID int64                          `json:"workflow_id"`
	Stages     []SortWorkflowStageItemRequest `json:"stages"`
}

type SortWorkflowStageItemRequest struct {
	ID       int64 `json:"id"`
	Position int   `json:"position"`
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

type WorkflowStageDataResponse struct {
	Stage WorkflowStageResponse `json:"stage"`
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

func (r SortWorkflowStageRequest) ToSortWorkflowStageCommand() input.SortWorkflowStageCommand {
	cmd := input.SortWorkflowStageCommand{
		WorkflowID: r.WorkflowID,
		Stages:     make([]input.SortWorkflowStageItemCommand, 0, len(r.Stages)),
	}
	for _, stage := range r.Stages {
		cmd.Stages = append(cmd.Stages, input.SortWorkflowStageItemCommand{
			ID:       stage.ID,
			Position: stage.Position,
		})
	}

	return cmd
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

func NewWorkflowStageDataResponse(stage *entity.WorkflowStage) WorkflowStageDataResponse {
	return WorkflowStageDataResponse{Stage: NewWorkflowStageResponse(stage)}
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
