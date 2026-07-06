package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type WorkflowStageRepository struct {
	db *sql.DB
}

func NewWorkflowStageRepository(db *sql.DB) output.WorkflowStageRepository {
	return &WorkflowStageRepository{db: db}
}

func (r *WorkflowStageRepository) ListByWorkflowID(ctx context.Context, workflowID int64) ([]entity.WorkflowStage, error) {
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

	var stages []entity.WorkflowStage
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

	return stages, nil
}

func (r *WorkflowStageRepository) GetByID(ctx context.Context, id int64) (*entity.WorkflowStage, error) {
	var stage entity.WorkflowStage
	err := r.db.QueryRowContext(ctx, `
		SELECT id, workflow_id, name, position, status, created_at, updated_at
		FROM workflow_stages
		WHERE id = $1
	`, id).Scan(&stage.ID, &stage.WorkflowID, &stage.Name, &stage.Position, &stage.Status, &stage.CreatedAt, &stage.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "workflow stage not found")
		}
		return nil, err
	}

	return &stage, nil
}

func (r *WorkflowStageRepository) Create(ctx context.Context, stage *entity.WorkflowStage) (*entity.WorkflowStage, error) {
	var created entity.WorkflowStage
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO workflow_stages (workflow_id, name, position, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, workflow_id, name, position, status, created_at, updated_at
	`, stage.WorkflowID, stage.Name, stage.Position, stage.Status).Scan(&created.ID, &created.WorkflowID, &created.Name, &created.Position, &created.Status, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &created, nil
}

func (r *WorkflowStageRepository) Update(ctx context.Context, id int64, stage *entity.WorkflowStage) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE workflow_stages
		SET workflow_id = $1, name = $2, position = $3, status = $4, updated_at = NOW()
		WHERE id = $5
	`, stage.WorkflowID, stage.Name, stage.Position, stage.Status, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(domain.ErrNotFound, "workflow stage not found")
	}

	return nil
}

func (r *WorkflowStageRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE workflow_stages
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
		return domain.NewError(domain.ErrNotFound, "workflow stage not found")
	}

	return nil
}
