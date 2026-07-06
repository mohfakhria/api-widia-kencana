package usecase

import (
	"context"
	"strconv"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type workflowStepUseCase struct {
	repo output.WorkflowStepRepository
}

func NewWorkflowStepUseCase(repo output.WorkflowStepRepository) input.WorkflowStepUseCase {
	return &workflowStepUseCase{repo: repo}
}

func (uc *workflowStepUseCase) ListByWorkflowStageID(ctx context.Context, workflowStageID string) ([]entity.WorkflowStep, error) {
	id, err := parseWorkflowStepParentID(workflowStageID, "invalid workflow stage id")
	if err != nil {
		return nil, err
	}

	return uc.repo.ListByWorkflowStageID(ctx, id)
}

func (uc *workflowStepUseCase) GetByID(ctx context.Context, id string) (*entity.WorkflowStep, error) {
	stepID, err := parseWorkflowStepID(id)
	if err != nil {
		return nil, err
	}

	return uc.repo.GetByID(ctx, stepID)
}

func (uc *workflowStepUseCase) Create(ctx context.Context, cmd input.CreateWorkflowStepCommand) (*entity.WorkflowStep, error) {
	step := mapWorkflowStepCommand(cmd)
	if err := validateWorkflowStep(step); err != nil {
		return nil, err
	}

	return uc.repo.Create(ctx, step)
}

func (uc *workflowStepUseCase) Update(ctx context.Context, id string, cmd input.UpdateWorkflowStepCommand) error {
	stepID, err := parseWorkflowStepID(id)
	if err != nil {
		return err
	}

	step := mapWorkflowStepCommand(input.CreateWorkflowStepCommand(cmd))
	if err := validateWorkflowStep(step); err != nil {
		return err
	}

	return uc.repo.Update(ctx, stepID, step)
}

func (uc *workflowStepUseCase) Delete(ctx context.Context, id string) error {
	stepID, err := parseWorkflowStepID(id)
	if err != nil {
		return err
	}

	return uc.repo.Delete(ctx, stepID)
}

func mapWorkflowStepCommand(cmd input.CreateWorkflowStepCommand) *entity.WorkflowStep {
	status := strings.TrimSpace(cmd.Status)
	if status == "" {
		status = defaultWorkflowStatus
	}

	return &entity.WorkflowStep{
		WorkflowStageID: cmd.WorkflowStageID,
		Name:            strings.TrimSpace(cmd.Name),
		Position:        cmd.Position,
		Status:          strings.ToLower(status),
	}
}

func validateWorkflowStep(step *entity.WorkflowStep) error {
	if step.WorkflowStageID <= 0 {
		return domain.NewError(domain.ErrInvalidInput, "workflow stage id must be greater than 0")
	}
	if step.Name == "" {
		return domain.NewError(domain.ErrInvalidInput, "workflow step name cannot be empty")
	}
	if step.Position < 0 {
		return domain.NewError(domain.ErrInvalidInput, "workflow step position cannot be negative")
	}
	if _, ok := allowedWorkflowStatuses[step.Status]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid workflow step status")
	}

	return nil
}

func parseWorkflowStepID(raw string) (int64, error) {
	return parseWorkflowStepParentID(raw, "invalid workflow step id")
}

func parseWorkflowStepParentID(raw, message string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.NewError(domain.ErrInvalidInput, message)
	}

	return id, nil
}
