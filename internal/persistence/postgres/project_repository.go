package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type ProjectRepository struct {
	db *sql.DB
}

func NewProjectRepository(db *sql.DB) output.ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) List(ctx context.Context) ([]entity.Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, status, created_at, updated_at
		FROM projects
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []entity.Project
	for rows.Next() {
		var project entity.Project
		if err := rows.Scan(&project.ID, &project.Name, &project.Status, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return projects, nil
}

func (r *ProjectRepository) GetByID(ctx context.Context, id int64) (*entity.Project, error) {
	var project entity.Project
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, status, created_at, updated_at
		FROM projects
		WHERE id = $1
	`, id).Scan(&project.ID, &project.Name, &project.Status, &project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "project not found")
		}
		return nil, err
	}

	return &project, nil
}

func (r *ProjectRepository) Create(ctx context.Context, project *entity.Project) (*entity.Project, error) {
	var created entity.Project
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO projects (name, status, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		RETURNING id, name, status, created_at, updated_at
	`, project.Name, project.Status).Scan(&created.ID, &created.Name, &created.Status, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &created, nil
}

func (r *ProjectRepository) Update(ctx context.Context, id int64, project *entity.Project) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE projects
		SET name = $1, status = $2, updated_at = NOW()
		WHERE id = $3
	`, project.Name, project.Status, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NewError(domain.ErrNotFound, "project not found")
	}

	return nil
}

func (r *ProjectRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE projects
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
		return domain.NewError(domain.ErrNotFound, "project not found")
	}

	return nil
}
