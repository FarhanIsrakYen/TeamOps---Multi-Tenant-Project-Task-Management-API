package service

import (
	"context"
	"strings"
	"unicode/utf8"

	auditmodel "github.com/example/teamops/backend/internal/modules/audit/model"
	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
)

type Repository interface {
	List(context.Context, uuid.UUID, pagination.Params, auditmodel.Filters) ([]auditmodel.AuditLog, int64, error)
}
type OrganizationAuthorizer interface {
	RequireOrganizationPermission(context.Context, uuid.UUID, uuid.UUID, orgguard.Permission) (orgmodel.Role, error)
}
type Service struct {
	repo Repository
	orgs OrganizationAuthorizer
}

func New(repo Repository, orgs OrganizationAuthorizer) *Service {
	return &Service{repo: repo, orgs: orgs}
}

func (s *Service) List(ctx context.Context, userID, orgID uuid.UUID, page pagination.Params, filters auditmodel.Filters) ([]auditmodel.AuditLog, int64, error) {
	if _, err := s.orgs.RequireOrganizationPermission(ctx, userID, orgID, orgguard.ViewAuditLogs); err != nil {
		return nil, 0, err
	}
	filters.Action = strings.TrimSpace(filters.Action)
	filters.ResourceType = strings.TrimSpace(filters.ResourceType)
	filters.ResourceID = strings.TrimSpace(filters.ResourceID)
	if utf8.RuneCountInString(filters.Action) > 100 || utf8.RuneCountInString(filters.ResourceType) > 100 || utf8.RuneCountInString(filters.ResourceID) > 255 {
		return nil, 0, apperror.ErrValidation
	}
	if filters.From != nil && filters.To != nil && filters.From.After(*filters.To) {
		return nil, 0, apperror.New(400, "invalid_date_range", "from must be before or equal to to")
	}
	return s.repo.List(ctx, orgID, page, filters)
}
