package service

import (
	"context"
	"errors"

	usermodel "github.com/example/teamops/backend/internal/modules/users/model"
	apperror "github.com/example/teamops/backend/internal/shared/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository interface {
	ByID(context.Context, uuid.UUID) (usermodel.User, error)
}

type Service struct{ users Repository }

func New(users Repository) *Service { return &Service{users: users} }

func (s *Service) Me(ctx context.Context, userID uuid.UUID) (usermodel.User, error) {
	u, err := s.users.ByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, apperror.ErrNotFound
	}
	return u, err
}
