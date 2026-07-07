package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type WorkflowRepository struct {
	db *sql.DB
}

func NewWorkflowRepository(db *sql.DB) output.WorkflowRepository {
	return &WorkflowRepository{db: db}
}

func (r *WorkflowRepository) List(ctx context.Context) ([]entity.Workflow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, status, created_at, updated_at
		FROM workflows
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []entity.Workflow
	for rows.Next() {
		var workflow entity.Workflow
		if err := rows.Scan(&workflow.ID, &workflow.Name, &workflow.Status, &workflow.CreatedAt, &workflow.UpdatedAt); err != nil {
			return nil, err
		}
		workflows = append(workflows, workflow)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return workflows, nil
}

func (r *WorkflowRepository) GetByID(ctx context.Context, id int64) (*entity.Workflow, error) {
	var workflow entity.Workflow
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, status, created_at, updated_at
		FROM workflows
		WHERE id = $1
	`, id).Scan(&workflow.ID, &workflow.Name, &workflow.Status, &workflow.CreatedAt, &workflow.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "workflow not found")
		}
		return nil, err
	}

	stages, err := r.workflowStages(ctx, workflow.ID)
	if err != nil {
		return nil, err
	}
	workflow.Stages = stages

	return &workflow, nil
}

func (r *WorkflowRepository) workflowStages(ctx context.Context, workflowID int64) ([]entity.WorkflowStage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, workflow_id, name, position, status, created_at, updated_at
		FROM workflow_stages
		WHERE workflow_id = $1
		ORDER BY position ASC, id ASC
	`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stages := make([]entity.WorkflowStage, 0)
	for rows.Next() {
		var stage entity.WorkflowStage
		if err := rows.Scan(&stage.ID, &stage.WorkflowID, &stage.Name, &stage.Position, &stage.Status, &stage.CreatedAt, &stage.UpdatedAt); err != nil {
			return nil, err
		}
		stages = append(stages, stage)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range stages {
		steps, err := r.workflowSteps(ctx, stages[i].ID)
		if err != nil {
			return nil, err
		}
		stages[i].Steps = steps
	}

	return stages, nil
}

func (r *WorkflowRepository) workflowSteps(ctx context.Context, workflowStageID int64) ([]entity.WorkflowStep, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, workflow_stage_id, name, position, status, created_at, updated_at
		FROM workflow_steps
		WHERE workflow_stage_id = $1
		ORDER BY position ASC, id ASC
	`, workflowStageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	steps := make([]entity.WorkflowStep, 0)
	for rows.Next() {
		var step entity.WorkflowStep
		if err := rows.Scan(&step.ID, &step.WorkflowStageID, &step.Name, &step.Position, &step.Status, &step.CreatedAt, &step.UpdatedAt); err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return steps, nil
}

func (r *WorkflowRepository) Create(ctx context.Context, workflow *entity.Workflow) (*entity.Workflow, error) {
	var created entity.Workflow
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO workflows (name, status, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		RETURNING id, name, status, created_at, updated_at
	`, workflow.Name, workflow.Status).Scan(&created.ID, &created.Name, &created.Status, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &created, nil
}

func (r *WorkflowRepository) Update(ctx context.Context, id int64, workflow *entity.Workflow) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE workflows
		SET name = $1, status = $2, updated_at = NOW()
		WHERE id = $3
	`, workflow.Name, workflow.Status, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(domain.ErrNotFound, "workflow not found")
	}

	return nil
}

func (r *WorkflowRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE workflows
		SET status = 'deleted', updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(domain.ErrNotFound, "workflow not found")
	}

	return nil
}
