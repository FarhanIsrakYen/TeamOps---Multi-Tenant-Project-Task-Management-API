package guard

import (
	"context"
	"errors"

	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type MembershipReader interface {
	Role(context.Context, uuid.UUID, uuid.UUID) (orgmodel.Role, error)
}

type Guard struct{ memberships MembershipReader }

type Permission string

const (
	ReadOrganization   Permission = "organization:read"
	ManageOrganization Permission = "organization:manage"
	DeleteOrganization Permission = "organization:delete"
	ManageMembers      Permission = "members:manage"
	ManageProjects     Permission = "projects:manage"
	CreateUpdateTasks  Permission = "tasks:create-update"
	DeleteTasks        Permission = "tasks:delete"
	CommentTasks       Permission = "tasks:comment"
	ManageLabels       Permission = "labels:manage"
	ViewAuditLogs      Permission = "audit:read"
)

func New(memberships MembershipReader) *Guard { return &Guard{memberships: memberships} }

func (g *Guard) RequireOrganizationRole(ctx context.Context, userID, orgID uuid.UUID, allowed ...orgmodel.Role) (orgmodel.Role, error) {
	role, err := g.memberships.Role(ctx, orgID, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperror.ErrForbidden
	}
	if err != nil {
		return "", err
	}
	for _, candidate := range allowed {
		if role == candidate {
			return role, nil
		}
	}
	return role, apperror.ErrForbidden
}

func (g *Guard) RequireOrganizationMember(ctx context.Context, userID, orgID uuid.UUID) (orgmodel.Role, error) {
	return g.RequireOrganizationRole(ctx, userID, orgID, orgmodel.RoleOwner, orgmodel.RoleAdmin, orgmodel.RoleMember, orgmodel.RoleViewer)
}

func (g *Guard) RequireOrganizationPermission(ctx context.Context, userID, orgID uuid.UUID, permission Permission) (orgmodel.Role, error) {
	role, err := g.RequireOrganizationMember(ctx, userID, orgID)
	if err != nil {
		return "", err
	}
	if !allows(role, permission) {
		return role, apperror.ErrForbidden
	}
	return role, nil
}

func allows(role orgmodel.Role, permission Permission) bool {
	switch role {
	case orgmodel.RoleOwner:
		switch permission {
		case ReadOrganization, ManageOrganization, DeleteOrganization, ManageMembers, ManageProjects, CreateUpdateTasks, DeleteTasks, CommentTasks, ManageLabels, ViewAuditLogs:
			return true
		}
	case orgmodel.RoleAdmin:
		switch permission {
		case ReadOrganization, ManageOrganization, ManageMembers, ManageProjects, CreateUpdateTasks, DeleteTasks, CommentTasks, ManageLabels, ViewAuditLogs:
			return true
		}
	case orgmodel.RoleMember:
		return permission == ReadOrganization || permission == CreateUpdateTasks || permission == CommentTasks
	case orgmodel.RoleViewer:
		return permission == ReadOrganization
	}
	return false
}

func (g *Guard) IsOrganizationMember(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	_, err := g.RequireOrganizationMember(ctx, userID, orgID)
	if errors.Is(err, apperror.ErrForbidden) {
		return false, nil
	}
	return err == nil, err
}
