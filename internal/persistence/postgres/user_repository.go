package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) output.UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, email, password, role
		FROM users
		WHERE email = $1
	`, email)

	var user entity.User
	if err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.Role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "user not found")
		}
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*entity.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, email, role
		FROM users
		WHERE id = $1
	`, id)

	var user entity.User
	if err := row.Scan(&user.ID, &user.Name, &user.Email, &user.Role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(domain.ErrNotFound, "user not found")
		}
		return nil, err
	}

	return &user, nil
}
