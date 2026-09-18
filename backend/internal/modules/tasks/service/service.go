package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

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
type UpdateInput struct {
	Title       *string
	Description *string
	AssigneeSet bool
	AssigneeID  *uuid.UUID
	Status      *model.Status
	Priority    *model.Priority
	DueAtSet    bool
	DueAt       *time.Time
	Version     int
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
	t.Title = strings.TrimSpace(t.Title)
	t.Description = strings.TrimSpace(t.Description)
	if t.Status == "" {
		t.Status = model.StatusTODO
	}
	if t.Status != model.StatusTODO {
		return t, apperror.New(400, "invalid_initial_status", "new tasks must start in TODO status")
	}
	if t.Priority == "" {
		t.Priority = model.PriorityMedium
	}
	if err := validateTask(t); err != nil {
		return t, err
	}
	if t.AssigneeID != nil {
		member, memberErr := s.orgs.IsOrganizationMember(ctx, p.OrganizationID, *t.AssigneeID)
		if memberErr != nil {
			return t, memberErr
		}
		if !member {
			return t, apperror.New(400, "invalid_assignee", "assignee must be a member of the organization")
		}
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
	if f.Status != "" && !f.Status.Valid() {
		return nil, 0, apperror.New(400, "invalid_status", "task status filter is invalid")
	}
	if f.Priority != "" && !f.Priority.Valid() {
		return nil, 0, apperror.New(400, "invalid_priority", "task priority filter is invalid")
	}
	f.Search = strings.TrimSpace(f.Search)
	if utf8.RuneCountInString(f.Search) > 200 {
		return nil, 0, apperror.New(400, "invalid_search", "search must not exceed 200 characters")
	}
	return s.tasks.List(ctx, projectID, p, f)
}
func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (model.Task, error) {
	return s.authorizeTask(ctx, userID, id, orgguard.ReadOrganization)
}
func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, input UpdateInput, requestID string) (model.Task, error) {
	current, err := s.authorizeTask(ctx, userID, id, orgguard.CreateUpdateTasks)
	if err != nil {
		return current, err
	}
	if input.Version < 1 {
		return current, apperror.New(400, "invalid_version", "version must be a positive integer")
	}
	if input.Version != current.Version {
		return current, apperror.New(409, "version_conflict", "task was modified by another request")
	}
	if input.Title == nil && input.Description == nil && !input.AssigneeSet && input.Status == nil && input.Priority == nil && !input.DueAtSet {
		return current, apperror.New(400, "empty_update", "at least one task field must be provided")
	}
	updatedTask := current
	if input.Title != nil {
		updatedTask.Title = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		updatedTask.Description = strings.TrimSpace(*input.Description)
	}
	if input.AssigneeSet {
		updatedTask.AssigneeID = input.AssigneeID
	}
	if input.Status != nil {
		if !input.Status.Valid() {
			return current, apperror.New(400, "invalid_status", "task status is invalid")
		}
		if !current.Status.CanTransitionTo(*input.Status) {
			return current, apperror.New(409, "invalid_status_transition", "the requested task status transition is not allowed")
		}
		updatedTask.Status = *input.Status
	}
	if input.Priority != nil {
		updatedTask.Priority = *input.Priority
	}
	if input.DueAtSet {
		updatedTask.DueAt = input.DueAt
	}
	updatedTask.Version = input.Version
	if err := validateTask(updatedTask); err != nil {
		return current, err
	}
	if input.AssigneeSet && input.AssigneeID != nil {
		member, memberErr := s.orgs.IsOrganizationMember(ctx, current.OrganizationID, *input.AssigneeID)
		if memberErr != nil {
			return current, memberErr
		}
		if !member {
			return current, apperror.New(400, "invalid_assignee", "assignee must be a member of the organization")
		}
	}
	updated, err := s.tasks.Update(ctx, updatedTask)
	if errors.Is(err, pgx.ErrNoRows) {
		return updated, apperror.New(409, "version_conflict", "task was modified by another request")
	}
	if err == nil {
		_ = s.audit.Record(ctx, current.OrganizationID, userID, "task.updated", "task", id.String(), requestID, map[string]any{"version": updated.Version, "status": updated.Status})
		if !uuidPointersEqual(current.AssigneeID, updated.AssigneeID) {
			_ = s.audit.Record(ctx, current.OrganizationID, userID, "task.assignment_changed", "task", id.String(), requestID, map[string]any{"oldAssigneeId": current.AssigneeID, "newAssigneeId": updated.AssigneeID})
		}
		if current.Status != updated.Status {
			_ = s.audit.Record(ctx, current.OrganizationID, userID, "task.status_changed", "task", id.String(), requestID, map[string]any{"oldStatus": current.Status, "newStatus": updated.Status})
		}
	}
	return updated, err
}

func uuidPointersEqual(left, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func validateTask(task model.Task) error {
	if count := utf8.RuneCountInString(task.Title); count < 2 || count > 200 {
		return apperror.New(400, "invalid_task_title", "task title must be between 2 and 200 characters")
	}
	if utf8.RuneCountInString(task.Description) > 10000 {
		return apperror.New(400, "invalid_task_description", "task description must not exceed 10000 characters")
	}
	if !task.Status.Valid() {
		return apperror.New(400, "invalid_status", "task status is invalid")
	}
	if !task.Priority.Valid() {
		return apperror.New(400, "invalid_priority", "task priority is invalid")
	}
	return nil
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
	body = strings.TrimSpace(body)
	if count := utf8.RuneCountInString(body); count < 1 || count > 5000 {
		return model.Comment{}, apperror.New(400, "invalid_comment", "comment must be between 1 and 5000 characters")
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

var labelColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (s *Service) CreateLabel(ctx context.Context, userID, orgID uuid.UUID, name, color, requestID string) (model.Label, error) {
	if _, err := s.orgs.RequireOrganizationPermission(ctx, userID, orgID, orgguard.ManageLabels); err != nil {
		return model.Label{}, err
	}
	name = strings.TrimSpace(name)
	if count := utf8.RuneCountInString(name); count < 1 || count > 50 || !labelColorPattern.MatchString(color) {
		return model.Label{}, apperror.New(400, "invalid_label", "label name or color is invalid")
	}
	label, err := s.labels.CreateLabel(ctx, orgID, name, color)
	if err == nil {
		_ = s.audit.Record(ctx, orgID, userID, "label.created", "label", label.ID.String(), requestID, nil)
	}
	return label, err
}
func (s *Service) SetLabels(ctx context.Context, userID, taskID uuid.UUID, ids []uuid.UUID, requestID string) error {
	t, err := s.authorizeTask(ctx, userID, taskID, orgguard.CreateUpdateTasks)
	if err != nil {
		return err
	}
	if len(ids) > 20 {
		return apperror.New(400, "too_many_labels", "a task may have at most 20 labels")
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return apperror.New(400, "invalid_label", "label identifiers must be valid UUIDs")
		}
		if _, duplicate := seen[id]; duplicate {
			return apperror.New(400, "duplicate_label", "label identifiers must be unique")
		}
		seen[id] = struct{}{}
	}
	if err = s.labels.SetLabels(ctx, taskID, t.OrganizationID, ids); err != nil {
		return err
	}
	_ = s.audit.Record(ctx, t.OrganizationID, userID, "task.labels_updated", "task", taskID.String(), requestID, map[string]any{"labelCount": len(ids)})
	return nil
}
