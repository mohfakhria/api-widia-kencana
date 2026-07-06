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

const defaultWorkflowStatus = "active"

var allowedWorkflowStatuses = map[string]struct{}{
	"active":   {},
	"inactive": {},
	"deleted":  {},
}

type workflowUseCase struct {
	repo output.WorkflowRepository
}

func NewWorkflowUseCase(repo output.WorkflowRepository) input.WorkflowUseCase {
	return &workflowUseCase{repo: repo}
}

func (uc *workflowUseCase) List(ctx context.Context) ([]entity.Workflow, error) {
	return uc.repo.List(ctx)
}

func (uc *workflowUseCase) GetByID(ctx context.Context, id string) (*entity.Workflow, error) {
	workflowID, err := parseWorkflowID(id)
	if err != nil {
		return nil, err
	}

	return uc.repo.GetByID(ctx, workflowID)
}

func (uc *workflowUseCase) Create(ctx context.Context, cmd input.CreateWorkflowCommand) (*entity.Workflow, error) {
	workflow := mapWorkflowCommand(cmd)
	if err := validateWorkflow(workflow); err != nil {
		return nil, err
	}

	return uc.repo.Create(ctx, workflow)
}

func (uc *workflowUseCase) Update(ctx context.Context, id string, cmd input.UpdateWorkflowCommand) error {
	workflowID, err := parseWorkflowID(id)
	if err != nil {
		return err
	}

	workflow := mapWorkflowCommand(input.CreateWorkflowCommand(cmd))
	if err := validateWorkflow(workflow); err != nil {
		return err
	}

	return uc.repo.Update(ctx, workflowID, workflow)
}

func (uc *workflowUseCase) Delete(ctx context.Context, id string) error {
	workflowID, err := parseWorkflowID(id)
	if err != nil {
		return err
	}

	return uc.repo.Delete(ctx, workflowID)
}

func mapWorkflowCommand(cmd input.CreateWorkflowCommand) *entity.Workflow {
	status := strings.TrimSpace(cmd.Status)
	if status == "" {
		status = defaultWorkflowStatus
	}

	return &entity.Workflow{
		Name:   strings.TrimSpace(cmd.Name),
		Status: strings.ToLower(status),
	}
}

func validateWorkflow(workflow *entity.Workflow) error {
	if workflow.Name == "" {
		return domain.NewError(domain.ErrInvalidInput, "workflow name cannot be empty")
	}
	if _, ok := allowedWorkflowStatuses[workflow.Status]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid workflow status")
	}

	return nil
}

func parseWorkflowID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.NewError(domain.ErrInvalidInput, "invalid workflow id")
	}

	return id, nil
}
