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

const defaultProjectStatus = "active"

var allowedProjectStatuses = map[string]struct{}{
	"active":    {},
	"inactive":  {},
	"decline":   {},
	"completed": {},
	"deleted":   {},
}

type projectUseCase struct {
	repo output.ProjectRepository
}

func NewProjectUseCase(repo output.ProjectRepository) input.ProjectUseCase {
	return &projectUseCase{repo: repo}
}

func (uc *projectUseCase) List(ctx context.Context) ([]entity.Project, error) {
	return uc.repo.List(ctx)
}

func (uc *projectUseCase) GetByID(ctx context.Context, id string) (*entity.Project, error) {
	projectID, err := parseProjectID(id)
	if err != nil {
		return nil, err
	}

	return uc.repo.GetByID(ctx, projectID)
}

func (uc *projectUseCase) Create(ctx context.Context, cmd input.CreateProjectCommand) (*entity.Project, error) {
	project := mapProjectCommand(cmd)
	if err := validateProject(project); err != nil {
		return nil, err
	}

	return uc.repo.Create(ctx, project)
}

func (uc *projectUseCase) Update(ctx context.Context, id string, cmd input.UpdateProjectCommand) error {
	projectID, err := parseProjectID(id)
	if err != nil {
		return err
	}

	project := mapProjectCommand(input.CreateProjectCommand(cmd))
	if err := validateProject(project); err != nil {
		return err
	}

	return uc.repo.Update(ctx, projectID, project)
}

func (uc *projectUseCase) Delete(ctx context.Context, id string) error {
	projectID, err := parseProjectID(id)
	if err != nil {
		return err
	}

	return uc.repo.Delete(ctx, projectID)
}

func mapProjectCommand(cmd input.CreateProjectCommand) *entity.Project {
	status := strings.TrimSpace(cmd.Status)
	if status == "" {
		status = defaultProjectStatus
	}

	return &entity.Project{
		Name:   strings.TrimSpace(cmd.Name),
		Status: strings.ToLower(status),
	}
}

func validateProject(project *entity.Project) error {
	if project.Name == "" {
		return domain.NewError(domain.ErrInvalidInput, "project name cannot be empty")
	}
	if _, ok := allowedProjectStatuses[project.Status]; !ok {
		return domain.NewError(domain.ErrInvalidInput, "invalid project status")
	}

	return nil
}

func parseProjectID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.NewError(domain.ErrInvalidInput, "invalid project id")
	}

	return id, nil
}
