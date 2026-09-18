package service

import (
	"context"
	"testing"
	"time"

	auditmodel "github.com/example/teamops/backend/internal/modules/audit/model"
	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type auditListRepository struct{ calls int }

func (r *auditListRepository) List(context.Context, uuid.UUID, pagination.Params, auditmodel.Filters) ([]auditmodel.AuditLog, int64, error) {
	r.calls++
	return []auditmodel.AuditLog{}, 0, nil
}

type auditAuthorizer struct{ role orgmodel.Role }

func (a auditAuthorizer) RequireOrganizationPermission(_ context.Context, _, _ uuid.UUID, permission orgguard.Permission) (orgmodel.Role, error) {
	if permission == orgguard.ViewAuditLogs && (a.role == orgmodel.RoleOwner || a.role == orgmodel.RoleAdmin) {
		return a.role, nil
	}
	return a.role, apperror.ErrForbidden
}

func TestOnlyOwnerAndAdminCanListAuditLogs(t *testing.T) {
	t.Parallel()
	for _, role := range []orgmodel.Role{orgmodel.RoleOwner, orgmodel.RoleAdmin, orgmodel.RoleMember, orgmodel.RoleViewer} {
		t.Run(string(role), func(t *testing.T) {
			repository := &auditListRepository{}
			service := New(repository, auditAuthorizer{role: role})
			_, _, err := service.List(context.Background(), uuid.New(), uuid.New(), pagination.Params{Page: 1, PageSize: 20}, auditmodel.Filters{})
			if role == orgmodel.RoleOwner || role == orgmodel.RoleAdmin {
				require.NoError(t, err)
				require.Equal(t, 1, repository.calls)
			} else {
				require.ErrorIs(t, err, apperror.ErrForbidden)
				require.Zero(t, repository.calls)
			}
		})
	}
}

func TestAuditLogDateRangeValidation(t *testing.T) {
	t.Parallel()
	now := time.Now()
	later := now.Add(time.Hour)
	repository := &auditListRepository{}
	service := New(repository, auditAuthorizer{role: orgmodel.RoleOwner})

	_, _, err := service.List(context.Background(), uuid.New(), uuid.New(), pagination.Params{Page: 1, PageSize: 20}, auditmodel.Filters{From: &later, To: &now})
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "invalid_date_range", appErr.Code)
	require.Zero(t, repository.calls)
}
