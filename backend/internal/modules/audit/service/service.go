package service

import (
	"context"

	auditmodel "github.com/example/teamops/backend/internal/modules/audit/model"
	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
)

type Repository interface {
	List(context.Context, uuid.UUID, pagination.Params) ([]auditmodel.AuditLog, int64, error)
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

func (s *Service) List(ctx context.Context, userID, orgID uuid.UUID, page pagination.Params) ([]auditmodel.AuditLog, int64, error) {
	if _, err := s.orgs.RequireOrganizationPermission(ctx, userID, orgID, orgguard.ViewAuditLogs); err != nil {
		return nil, 0, err
	}
	return s.repo.List(ctx, orgID, page)
}
