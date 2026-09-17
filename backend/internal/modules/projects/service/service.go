package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/example/teamops/backend/internal/cache"
	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	"github.com/example/teamops/backend/internal/modules/projects/model"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Auditor interface {
	Record(context.Context, uuid.UUID, uuid.UUID, string, string, string, string, map[string]any) error
}
type Repository interface {
	Create(context.Context, uuid.UUID, string, string) (model.Project, error)
	Get(context.Context, uuid.UUID) (model.Project, error)
	List(context.Context, uuid.UUID, pagination.Params, model.Filters) ([]model.Project, int64, error)
	Update(context.Context, uuid.UUID, string, string, bool, int) (model.Project, error)
	Delete(context.Context, uuid.UUID) error
}
type UpdateInput struct {
	Name        *string
	Description *string
	Archived    *bool
	Version     int
}
type OrganizationAuthorizer interface {
	RequireOrganizationPermission(context.Context, uuid.UUID, uuid.UUID, orgguard.Permission) (orgmodel.Role, error)
}
type ProjectAuthorizer interface {
	RequireProjectAccess(context.Context, uuid.UUID, model.Project, orgguard.Permission) error
}
type Service struct {
	repo   Repository
	orgs   OrganizationAuthorizer
	access ProjectAuthorizer
	cache  *cache.Cache
	audit  Auditor
}

func New(r Repository, o OrganizationAuthorizer, access ProjectAuthorizer, c *cache.Cache, a Auditor) *Service {
	return &Service{repo: r, orgs: o, access: access, cache: c, audit: a}
}
func key(id uuid.UUID) string { return "project:" + id.String() }
func (s *Service) Create(ctx context.Context, userID, orgID uuid.UUID, name, description, requestID string) (model.Project, error) {
	if _, err := s.orgs.RequireOrganizationPermission(ctx, userID, orgID, orgguard.ManageProjects); err != nil {
		return model.Project{}, err
	}
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if err := validateProject(name, description); err != nil {
		return model.Project{}, err
	}
	p, err := s.repo.Create(ctx, orgID, name, description)
	if isUniqueViolation(err) {
		return p, apperror.New(409, "project_name_conflict", "project name is already in use in this organization")
	}
	if err == nil {
		_ = s.audit.Record(ctx, orgID, userID, "project.created", "project", p.ID.String(), requestID, nil)
	}
	return p, err
}
func (s *Service) List(ctx context.Context, userID, orgID uuid.UUID, p pagination.Params, filters model.Filters) ([]model.Project, int64, error) {
	if _, err := s.orgs.RequireOrganizationPermission(ctx, userID, orgID, orgguard.ReadOrganization); err != nil {
		return nil, 0, err
	}
	filters.Search = strings.TrimSpace(filters.Search)
	if utf8.RuneCountInString(filters.Search) > 200 {
		return nil, 0, apperror.New(400, "invalid_search", "search must not exceed 200 characters")
	}
	return s.repo.List(ctx, orgID, p, filters)
}
func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (model.Project, error) {
	var p model.Project
	if ok, _ := s.cache.GetJSON(ctx, key(id), &p); !ok {
		var err error
		p, err = s.repo.Get(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return p, apperror.ErrNotFound
		}
		if err != nil {
			return p, err
		}
		_ = s.cache.SetJSON(ctx, key(id), p, 5*time.Minute)
	}
	if err := s.access.RequireProjectAccess(ctx, userID, p, orgguard.ReadOrganization); err != nil {
		return model.Project{}, err
	}
	return p, nil
}
func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, input UpdateInput, requestID string) (model.Project, error) {
	current, err := s.repo.Get(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return current, apperror.ErrNotFound
	}
	if err != nil {
		return current, err
	}
	if err = s.access.RequireProjectAccess(ctx, userID, current, orgguard.ManageProjects); err != nil {
		return current, err
	}
	if input.Version < 1 {
		return current, apperror.New(400, "invalid_version", "version must be a positive integer")
	}
	name, description, archived := current.Name, current.Description, current.Archived
	if input.Name != nil {
		name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		description = strings.TrimSpace(*input.Description)
	}
	if input.Archived != nil {
		archived = *input.Archived
	}
	if err := validateProject(name, description); err != nil {
		return current, err
	}
	p, err := s.repo.Update(ctx, id, name, description, archived, input.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, apperror.New(409, "version_conflict", "project was modified by another request")
	}
	if isUniqueViolation(err) {
		return p, apperror.New(409, "project_name_conflict", "project name is already in use in this organization")
	}
	if err == nil {
		_ = s.cache.Delete(ctx, key(id))
		_ = s.audit.Record(ctx, p.OrganizationID, userID, "project.updated", "project", id.String(), requestID, map[string]any{"version": p.Version})
	}
	return p, err
}

func validateProject(name, description string) error {
	if count := utf8.RuneCountInString(name); count < 2 || count > 120 {
		return apperror.New(400, "invalid_project_name", "project name must be between 2 and 120 characters")
	}
	if utf8.RuneCountInString(description) > 5000 {
		return apperror.New(400, "invalid_project_description", "project description must not exceed 5000 characters")
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID, requestID string) error {
	p, err := s.repo.Get(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.ErrNotFound
	}
	if err != nil {
		return err
	}
	if err = s.access.RequireProjectAccess(ctx, userID, p, orgguard.ManageProjects); err != nil {
		return err
	}
	if err = s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	_ = s.cache.Delete(ctx, key(id))
	_ = s.audit.Record(ctx, p.OrganizationID, userID, "project.deleted", "project", id.String(), requestID, nil)
	return nil
}
