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

type workflowStageUseCase struct {
	repo output.WorkflowStageRepository
}

func NewWorkflowStageUseCase(repo output.WorkflowStageRepository) input.WorkflowStageUseCase {
	return &workflowStageUseCase{repo: repo}
}

func (uc *workflowStageUseCase) ListByWorkflowID(ctx context.Context, workflowID string) ([]entity.WorkflowStage, error) {
	id, err := parseWorkflowStageParentID(workflowID, "invalid workflow id")
	if err != nil {
		return nil, err
	}

	return uc.repo.ListByWorkflowID(ctx, id)
}

func (uc *workflowStageUseCase) GetByID(ctx context.Context, id string) (*entity.WorkflowStage, error) {
	stageID, err := parseWorkflowStageID(id)
	if err != nil {
		return nil, err
	}

	return uc.repo.GetByID(ctx, stageID)
}

func (uc *workflowStageUseCase) Create(ctx context.Context, cmd input.CreateWorkflowStageCommand) (*entity.WorkflowStage, error) {
	stage := mapWorkflowStageCommand(cmd)
	if err := validateWorkflowStage(stage); err != nil {
		return nil, err
	}

	return uc.repo.Create(ctx, stage)
}

func (uc *workflowStageUseCase) Update(ctx context.Context, id string, cmd input.UpdateWorkflowStageCommand) error {
	stageID, err := parseWorkflowStageID(id)
	if err != nil {
		return err
	}

	stage := mapWorkflowStageCommand(input.CreateWorkflowStageCommand(cmd))
	if err := validateWorkflowStage(stage); err != nil {
		return err
	}

	return uc.repo.Update(ctx, stageID, stage)
}

func (uc *workflowStageUseCase) Delete(ctx context.Context, id string) error {
	stageID, err := parseWorkflowStageID(id)
	if err != nil {
		return err
	}

	return uc.repo.Delete(ctx, stageID)
}

func mapWorkflowStageCommand(cmd input.CreateWorkflowStageCommand) *entity.WorkflowStage {
	status := strings.TrimSpace(cmd.Status)
	if status == "" {
		status = defaultWorkflowStatus
	}

	return &entity.WorkflowStage{
		WorkflowID: cmd.WorkflowID,
		Name:       strings.TrimSpace(cmd.Name),
		Position:   cmd.Position,
		Status:     strings.ToLower(status),
	}
}

func validateWorkflowStage(stage *entity.WorkflowStage) error {
	if stage.WorkflowID <= 0 {
		return domain.NewError(domain.ErrInvalidInput, "workflow id must be greater than 0")
	}
	if stage.Name == "" {
		return domain.NewError(domain.ErrInvalidInput, "workflow stage name cannot be empty")
	}
	if stage.Position < 0 {
		return domain.NewError(domain.ErrInvalidInput, "workflow stage position cannot be negative")
	}
	if _, ok := allowedWorkflowStatuses[stage.Status]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid workflow stage status")
	}

	return nil
}

func parseWorkflowStageID(raw string) (int64, error) {
	return parseWorkflowStageParentID(raw, "invalid workflow stage id")
}

func parseWorkflowStageParentID(raw, message string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.NewError(domain.ErrInvalidInput, message)
	}

	return id, nil
}
