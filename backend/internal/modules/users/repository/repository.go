package repository

import (
	"context"

	"github.com/example/teamops/backend/internal/database"
	usermodel "github.com/example/teamops/backend/internal/modules/users/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, email, name, hash string) (usermodel.User, error) {
	var u usermodel.User
	err := database.Executor(ctx, r.db).QueryRow(ctx, `INSERT INTO users(email,name,password_hash) VALUES(lower($1),$2,$3) RETURNING id,email,name,password_hash,created_at,updated_at`, email, name, hash).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *Repository) ByEmail(ctx context.Context, email string) (usermodel.User, error) {
	var u usermodel.User
	err := database.Executor(ctx, r.db).QueryRow(ctx, `SELECT id,email,name,password_hash,created_at,updated_at FROM users WHERE email=lower($1)`, email).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err == pgx.ErrNoRows {
		return u, pgx.ErrNoRows
	}
	return u, err
}

func (r *Repository) ByID(ctx context.Context, id uuid.UUID) (usermodel.User, error) {
	var u usermodel.User
	err := database.Executor(ctx, r.db).QueryRow(ctx, `SELECT id,email,name,password_hash,created_at,updated_at FROM users WHERE id=$1`, id).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}
