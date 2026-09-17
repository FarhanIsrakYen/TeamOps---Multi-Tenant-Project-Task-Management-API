package service

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/example/teamops/backend/internal/database"
	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	"github.com/example/teamops/backend/internal/modules/organizations/model"
	orgrepo "github.com/example/teamops/backend/internal/modules/organizations/repository"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Auditor interface {
	Record(context.Context, uuid.UUID, uuid.UUID, string, string, string, string, map[string]any) error
}
type Service struct {
	repo  *orgrepo.Repository
	audit Auditor
	tx    database.Transactor
	guard *orgguard.Guard
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func New(r *orgrepo.Repository, a Auditor, tx database.Transactor, guard *orgguard.Guard) *Service {
	return &Service{repo: r, audit: a, tx: tx, guard: guard}
}
func (s *Service) Create(ctx context.Context, userID uuid.UUID, name, slug, requestID string) (model.Organization, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if !slugPattern.MatchString(slug) {
		return model.Organization{}, apperror.New(400, "invalid_slug", "slug must contain lowercase letters, numbers, and single hyphens")
	}
	var o model.Organization
	err := s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		var createErr error
		o, createErr = s.repo.Create(txCtx, userID, strings.TrimSpace(name), slug)
		if createErr != nil {
			return createErr
		}
		return s.audit.Record(txCtx, o.ID, userID, "organization.created", "organization", o.ID.String(), requestID, nil)
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return o, apperror.Wrap(err, 409, "organization_conflict", "organization slug is already in use")
		}
		return o, err
	}
	return o, nil
}
func (s *Service) List(ctx context.Context, userID uuid.UUID, p pagination.Params) ([]model.Organization, int64, error) {
	return s.repo.ListForUser(ctx, userID, p)
}
func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (model.Organization, error) {
	o, err := s.repo.GetForUser(ctx, id, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, apperror.ErrNotFound
	}
	return o, err
}
func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, name, slug string, version int, requestID string) (model.Organization, error) {
	if _, err := s.guard.RequireOrganizationPermission(ctx, userID, id, orgguard.ManageOrganization); err != nil {
		return model.Organization{}, err
	}
	slug = strings.ToLower(strings.TrimSpace(slug))
	if !slugPattern.MatchString(slug) {
		return model.Organization{}, apperror.New(400, "invalid_slug", "slug must contain lowercase letters, numbers, and single hyphens")
	}
	o, err := s.repo.Update(ctx, id, strings.TrimSpace(name), slug, version)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, apperror.New(409, "version_conflict", "organization was modified by another request")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return o, apperror.New(409, "organization_conflict", "organization slug is already in use")
	}
	if err == nil {
		_ = s.audit.Record(ctx, id, userID, "organization.updated", "organization", id.String(), requestID, map[string]any{"version": o.Version})
	}
	return o, err
}
func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := s.guard.RequireOrganizationPermission(ctx, userID, id, orgguard.DeleteOrganization); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}
func (s *Service) Members(ctx context.Context, userID, orgID uuid.UUID) ([]model.Membership, error) {
	if _, err := s.guard.RequireOrganizationPermission(ctx, userID, orgID, orgguard.ReadOrganization); err != nil {
		return nil, err
	}
	return s.repo.ListMembers(ctx, orgID)
}
func (s *Service) AddMember(ctx context.Context, userID, orgID uuid.UUID, email string, role model.Role, requestID string) (model.Membership, error) {
	if _, err := s.guard.RequireOrganizationPermission(ctx, userID, orgID, orgguard.ManageMembers); err != nil {
		return model.Membership{}, err
	}
	if role == model.RoleOwner {
		return model.Membership{}, apperror.New(403, "owner_assignment_forbidden", "the owner role cannot be assigned")
	}
	if role != model.RoleAdmin && role != model.RoleMember && role != model.RoleViewer {
		return model.Membership{}, apperror.New(400, "invalid_role", "role must be ADMIN, MEMBER, or VIEWER")
	}
	m, err := s.repo.AddMember(ctx, orgID, email, role)
	if errors.Is(err, orgrepo.ErrOwnerRoleImmutable) {
		return m, apperror.New(409, "owner_role_immutable", "the organization owner's role cannot be changed")
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return m, apperror.New(404, "user_not_found", "no account exists for that email")
	}
	if err == nil {
		_ = s.audit.Record(ctx, orgID, userID, "member.added", "membership", m.UserID.String(), requestID, map[string]any{"role": role})
	}
	return m, err
}
