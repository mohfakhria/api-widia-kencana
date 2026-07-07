package dto

import (
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
)

type WorkflowStepRequest struct {
	WorkflowStageID int64  `json:"workflow_stage_id"`
	Name            string `json:"name"`
	Position        int    `json:"position"`
	Status          string `json:"status"`
}

type SortWorkflowStepRequest struct {
	WorkflowStageID int64                         `json:"workflow_stage_id"`
	Steps           []SortWorkflowStepItemRequest `json:"steps"`
}

type SortWorkflowStepItemRequest struct {
	ID       int64 `json:"id"`
	Position int   `json:"position"`
}

type WorkflowStepResponse struct {
	ID              int64     `json:"id"`
	WorkflowStageID int64     `json:"workflow_stage_id"`
	Name            string    `json:"name"`
	Position        int       `json:"position"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type WorkflowStepDataResponse struct {
	Step WorkflowStepResponse `json:"step"`
}

type WorkflowStepListResponse struct {
	Steps []WorkflowStepResponse `json:"steps"`
}

func (r WorkflowStepRequest) ToCreateWorkflowStepCommand() input.CreateWorkflowStepCommand {
	return input.CreateWorkflowStepCommand{
		WorkflowStageID: r.WorkflowStageID,
		Name:            r.Name,
		Position:        r.Position,
		Status:          r.Status,
	}
}

func (r WorkflowStepRequest) ToUpdateWorkflowStepCommand() input.UpdateWorkflowStepCommand {
	return input.UpdateWorkflowStepCommand(r.ToCreateWorkflowStepCommand())
}

func (r SortWorkflowStepRequest) ToSortWorkflowStepCommand() input.SortWorkflowStepCommand {
	cmd := input.SortWorkflowStepCommand{
		WorkflowStageID: r.WorkflowStageID,
		Steps:           make([]input.SortWorkflowStepItemCommand, 0, len(r.Steps)),
	}
	for _, step := range r.Steps {
		cmd.Steps = append(cmd.Steps, input.SortWorkflowStepItemCommand{
			ID:       step.ID,
			Position: step.Position,
		})
	}

	return cmd
}

func NewWorkflowStepResponse(step *entity.WorkflowStep) WorkflowStepResponse {
	return WorkflowStepResponse{
		ID:              step.ID,
		WorkflowStageID: step.WorkflowStageID,
		Name:            step.Name,
		Position:        step.Position,
		Status:          step.Status,
		CreatedAt:       step.CreatedAt,
		UpdatedAt:       step.UpdatedAt,
	}
}

func NewWorkflowStepDataResponse(step *entity.WorkflowStep) WorkflowStepDataResponse {
	return WorkflowStepDataResponse{Step: NewWorkflowStepResponse(step)}
}

func NewWorkflowStepListResponses(steps []entity.WorkflowStep) WorkflowStepListResponse {
	responses := WorkflowStepListResponse{
		Steps: make([]WorkflowStepResponse, 0, len(steps)),
	}
	for _, step := range steps {
		responses.Steps = append(responses.Steps, NewWorkflowStepResponse(&step))
	}

	return responses
}
