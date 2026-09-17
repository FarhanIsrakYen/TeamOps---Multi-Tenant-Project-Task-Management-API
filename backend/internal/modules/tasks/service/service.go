package service

import (
	"context"
	"errors"

	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	projectmodel "github.com/example/teamops/backend/internal/modules/projects/model"
	"github.com/example/teamops/backend/internal/modules/tasks/model"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Auditor interface {
	Record(context.Context, uuid.UUID, uuid.UUID, string, string, string, string, map[string]any) error
}
type TaskRepository interface {
	Create(context.Context, model.Task) (model.Task, error)
	Get(context.Context, uuid.UUID) (model.Task, error)
	List(context.Context, uuid.UUID, pagination.Params, model.Filters) ([]model.Task, int64, error)
	Update(context.Context, model.Task) (model.Task, error)
	Delete(context.Context, uuid.UUID) error
}
type CommentRepository interface {
	AddComment(context.Context, uuid.UUID, uuid.UUID, string) (model.Comment, error)
	Comments(context.Context, uuid.UUID) ([]model.Comment, error)
}
type LabelRepository interface {
	CreateLabel(context.Context, uuid.UUID, string, string) (model.Label, error)
	Labels(context.Context, uuid.UUID) ([]model.Label, error)
	SetLabels(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) error
}
type ProjectReader interface {
	Get(context.Context, uuid.UUID) (projectmodel.Project, error)
}
type OrganizationAuthorizer interface {
	RequireOrganizationPermission(context.Context, uuid.UUID, uuid.UUID, orgguard.Permission) (orgmodel.Role, error)
	IsOrganizationMember(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}
type ProjectAuthorizer interface {
	RequireProjectAccess(context.Context, uuid.UUID, projectmodel.Project, orgguard.Permission) error
}
type TaskAuthorizer interface {
	RequireTaskAccess(context.Context, uuid.UUID, model.Task, orgguard.Permission) error
}
type Service struct {
	tasks         TaskRepository
	comments      CommentRepository
	labels        LabelRepository
	projects      ProjectReader
	orgs          OrganizationAuthorizer
	projectAccess ProjectAuthorizer
	taskAccess    TaskAuthorizer
	audit         Auditor
}

func New(tasks TaskRepository, comments CommentRepository, labels LabelRepository, projects ProjectReader, orgs OrganizationAuthorizer, projectAccess ProjectAuthorizer, taskAccess TaskAuthorizer, audit Auditor) *Service {
	return &Service{tasks: tasks, comments: comments, labels: labels, projects: projects, orgs: orgs, projectAccess: projectAccess, taskAccess: taskAccess, audit: audit}
}
func (s *Service) authorizeProject(ctx context.Context, userID, projectID uuid.UUID, permission orgguard.Permission) (projectmodel.Project, error) {
	p, err := s.projects.Get(ctx, projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, apperror.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	err = s.projectAccess.RequireProjectAccess(ctx, userID, p, permission)
	return p, err
}
func (s *Service) authorizeTask(ctx context.Context, userID, taskID uuid.UUID, permission orgguard.Permission) (model.Task, error) {
	t, err := s.tasks.Get(ctx, taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, apperror.ErrNotFound
	}
	if err != nil {
		return t, err
	}
	err = s.taskAccess.RequireTaskAccess(ctx, userID, t, permission)
	return t, err
}
func (s *Service) Create(ctx context.Context, userID, projectID uuid.UUID, t model.Task, requestID string) (model.Task, error) {
	p, err := s.authorizeProject(ctx, userID, projectID, orgguard.CreateUpdateTasks)
	if err != nil {
		return t, err
	}
	t.ProjectID = projectID
	t.OrganizationID = p.OrganizationID
	t.CreatedBy = userID
	if t.AssigneeID != nil {
		member, memberErr := s.orgs.IsOrganizationMember(ctx, p.OrganizationID, *t.AssigneeID)
		if memberErr != nil {
			return t, memberErr
		}
		if !member {
			return t, apperror.New(400, "invalid_assignee", "assignee must be a member of the organization")
		}
	}
	if t.Status == "" {
		t.Status = "TODO"
	}
	if t.Priority == "" {
		t.Priority = "MEDIUM"
	}
	t, err = s.tasks.Create(ctx, t)
	if err == nil {
		_ = s.audit.Record(ctx, p.OrganizationID, userID, "task.created", "task", t.ID.String(), requestID, nil)
	}
	return t, err
}
func (s *Service) List(ctx context.Context, userID, projectID uuid.UUID, p pagination.Params, f model.Filters) ([]model.Task, int64, error) {
	if _, err := s.authorizeProject(ctx, userID, projectID, orgguard.ReadOrganization); err != nil {
		return nil, 0, err
	}
	return s.tasks.List(ctx, projectID, p, f)
}
func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (model.Task, error) {
	return s.authorizeTask(ctx, userID, id, orgguard.ReadOrganization)
}
func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, input model.Task, requestID string) (model.Task, error) {
	current, err := s.authorizeTask(ctx, userID, id, orgguard.CreateUpdateTasks)
	if err != nil {
		return current, err
	}
	if input.AssigneeID != nil {
		member, memberErr := s.orgs.IsOrganizationMember(ctx, current.OrganizationID, *input.AssigneeID)
		if memberErr != nil {
			return current, memberErr
		}
		if !member {
			return current, apperror.New(400, "invalid_assignee", "assignee must be a member of the organization")
		}
	}
	input.ID = id
	updated, err := s.tasks.Update(ctx, input)
	if errors.Is(err, pgx.ErrNoRows) {
		return updated, apperror.New(409, "version_conflict", "task was modified by another request")
	}
	if err == nil {
		_ = s.audit.Record(ctx, current.OrganizationID, userID, "task.updated", "task", id.String(), requestID, map[string]any{"version": updated.Version, "status": updated.Status})
	}
	return updated, err
}
func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID, requestID string) error {
	t, err := s.authorizeTask(ctx, userID, id, orgguard.DeleteTasks)
	if err != nil {
		return err
	}
	if err = s.tasks.Delete(ctx, id); err == nil {
		_ = s.audit.Record(ctx, t.OrganizationID, userID, "task.deleted", "task", id.String(), requestID, nil)
	}
	return err
}
func (s *Service) Comments(ctx context.Context, userID, taskID uuid.UUID) ([]model.Comment, error) {
	if _, err := s.authorizeTask(ctx, userID, taskID, orgguard.ReadOrganization); err != nil {
		return nil, err
	}
	return s.comments.Comments(ctx, taskID)
}
func (s *Service) AddComment(ctx context.Context, userID, taskID uuid.UUID, body, requestID string) (model.Comment, error) {
	t, err := s.authorizeTask(ctx, userID, taskID, orgguard.CommentTasks)
	if err != nil {
		return model.Comment{}, err
	}
	c, err := s.comments.AddComment(ctx, taskID, userID, body)
	if err == nil {
		_ = s.audit.Record(ctx, t.OrganizationID, userID, "comment.created", "comment", c.ID.String(), requestID, map[string]any{"taskId": taskID})
	}
	return c, err
}
func (s *Service) Labels(ctx context.Context, userID, orgID uuid.UUID) ([]model.Label, error) {
	if _, err := s.orgs.RequireOrganizationPermission(ctx, userID, orgID, orgguard.ReadOrganization); err != nil {
		return nil, err
	}
	return s.labels.Labels(ctx, orgID)
}
func (s *Service) CreateLabel(ctx context.Context, userID, orgID uuid.UUID, name, color string) (model.Label, error) {
	if _, err := s.orgs.RequireOrganizationPermission(ctx, userID, orgID, orgguard.ManageLabels); err != nil {
		return model.Label{}, err
	}
	return s.labels.CreateLabel(ctx, orgID, name, color)
}
func (s *Service) SetLabels(ctx context.Context, userID, taskID uuid.UUID, ids []uuid.UUID) error {
	t, err := s.authorizeTask(ctx, userID, taskID, orgguard.CreateUpdateTasks)
	if err != nil {
		return err
	}
	return s.labels.SetLabels(ctx, taskID, t.OrganizationID, ids)
}
