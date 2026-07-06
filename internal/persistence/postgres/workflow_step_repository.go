package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type WorkflowStepRepository struct {
	db *sql.DB
}

func NewWorkflowStepRepository(db *sql.DB) output.WorkflowStepRepository {
	return &WorkflowStepRepository{db: db}
}

func (r *WorkflowStepRepository) ListByWorkflowStageID(ctx context.Context, workflowStageID int64) ([]entity.WorkflowStep, error) {
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

	var steps []entity.WorkflowStep
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

func (r *WorkflowStepRepository) GetByID(ctx context.Context, id int64) (*entity.WorkflowStep, error) {
	var step entity.WorkflowStep
	err := r.db.QueryRowContext(ctx, `
		SELECT id, workflow_stage_id, name, position, status, created_at, updated_at
		FROM workflow_steps
		WHERE id = $1
	`, id).Scan(&step.ID, &step.WorkflowStageID, &step.Name, &step.Position, &step.Status, &step.CreatedAt, &step.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "workflow step not found")
		}
		return nil, err
	}

	return &step, nil
}

func (r *WorkflowStepRepository) Create(ctx context.Context, step *entity.WorkflowStep) (*entity.WorkflowStep, error) {
	var created entity.WorkflowStep
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO workflow_steps (workflow_stage_id, name, position, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, workflow_stage_id, name, position, status, created_at, updated_at
	`, step.WorkflowStageID, step.Name, step.Position, step.Status).Scan(&created.ID, &created.WorkflowStageID, &created.Name, &created.Position, &created.Status, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &created, nil
}

func (r *WorkflowStepRepository) Update(ctx context.Context, id int64, step *entity.WorkflowStep) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE workflow_steps
		SET workflow_stage_id = $1, name = $2, position = $3, status = $4, updated_at = NOW()
		WHERE id = $5
	`, step.WorkflowStageID, step.Name, step.Position, step.Status, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(domain.ErrNotFound, "workflow step not found")
	}

	return nil
}

func (r *WorkflowStepRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE workflow_steps
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
		return domain.NewError(domain.ErrNotFound, "workflow step not found")
	}

	return nil
}
