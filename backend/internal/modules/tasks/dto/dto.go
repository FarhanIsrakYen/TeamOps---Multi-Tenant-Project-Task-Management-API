package dto

import (
	"bytes"
	"encoding/json"
	"time"

	taskmodel "github.com/example/teamops/backend/internal/modules/tasks/model"
	"github.com/google/uuid"
)

type CreateRequest struct {
	Title       string             `json:"title" binding:"required,min=2,max=200"`
	Description string             `json:"description" binding:"max=10000"`
	AssigneeID  *uuid.UUID         `json:"assigneeId"`
	Status      taskmodel.Status   `json:"status" binding:"omitempty,oneof=TODO"`
	Priority    taskmodel.Priority `json:"priority" binding:"omitempty,oneof=LOW MEDIUM HIGH URGENT"`
	DueAt       *time.Time         `json:"dueAt"`
}
type UpdateRequest struct {
	Title       *string             `json:"title" binding:"omitempty,min=2,max=200"`
	Description *string             `json:"description" binding:"omitempty,max=10000"`
	AssigneeID  NullableUUID        `json:"assigneeId"`
	Status      *taskmodel.Status   `json:"status" binding:"omitempty,oneof=TODO IN_PROGRESS IN_REVIEW DONE CANCELLED"`
	Priority    *taskmodel.Priority `json:"priority" binding:"omitempty,oneof=LOW MEDIUM HIGH URGENT"`
	DueAt       NullableTime        `json:"dueAt"`
	Version     int                 `json:"version" binding:"required,min=1"`
}

type NullableUUID struct {
	Set   bool
	Value *uuid.UUID
}

func (v *NullableUUID) UnmarshalJSON(data []byte) error {
	v.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		v.Value = nil
		return nil
	}
	var value uuid.UUID
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	v.Value = &value
	return nil
}

type NullableTime struct {
	Set   bool
	Value *time.Time
}

func (v *NullableTime) UnmarshalJSON(data []byte) error {
	v.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		v.Value = nil
		return nil
	}
	var value time.Time
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	v.Value = &value
	return nil
}

type CommentRequest struct {
	Body string `json:"body" binding:"required,min=1,max=5000"`
}
type LabelRequest struct {
	Name  string `json:"name" binding:"required,min=1,max=50"`
	Color string `json:"color" binding:"required,hexcolor"`
}
type SetLabelsRequest struct {
	LabelIDs []uuid.UUID `json:"labelIds" binding:"max=20"`
}

type TaskResponse struct {
	ID             uuid.UUID          `json:"id"`
	OrganizationID uuid.UUID          `json:"organizationId"`
	ProjectID      uuid.UUID          `json:"projectId"`
	AssigneeID     *uuid.UUID         `json:"assigneeId,omitempty"`
	CreatedBy      uuid.UUID          `json:"createdBy"`
	Title          string             `json:"title"`
	Description    string             `json:"description"`
	Status         taskmodel.Status   `json:"status"`
	Priority       taskmodel.Priority `json:"priority"`
	DueAt          *time.Time         `json:"dueAt,omitempty"`
	Version        int                `json:"version"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}
type CommentResponse struct {
	ID        uuid.UUID `json:"id"`
	TaskID    uuid.UUID `json:"taskId"`
	AuthorID  uuid.UUID `json:"authorId"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type LabelResponse struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	Name           string    `json:"name"`
	Color          string    `json:"color"`
}

func FromTask(t taskmodel.Task) TaskResponse {
	return TaskResponse{ID: t.ID, OrganizationID: t.OrganizationID, ProjectID: t.ProjectID, AssigneeID: t.AssigneeID, CreatedBy: t.CreatedBy, Title: t.Title, Description: t.Description, Status: t.Status, Priority: t.Priority, DueAt: t.DueAt, Version: t.Version, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}
func FromTasks(items []taskmodel.Task) []TaskResponse {
	out := make([]TaskResponse, 0, len(items))
	for _, item := range items {
		out = append(out, FromTask(item))
	}
	return out
}
func FromComment(c taskmodel.Comment) CommentResponse {
	return CommentResponse{ID: c.ID, TaskID: c.TaskID, AuthorID: c.AuthorID, Body: c.Body, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}
func FromComments(items []taskmodel.Comment) []CommentResponse {
	out := make([]CommentResponse, 0, len(items))
	for _, item := range items {
		out = append(out, FromComment(item))
	}
	return out
}
func FromLabel(l taskmodel.Label) LabelResponse {
	return LabelResponse{ID: l.ID, OrganizationID: l.OrganizationID, Name: l.Name, Color: l.Color}
}
func FromLabels(items []taskmodel.Label) []LabelResponse {
	out := make([]LabelResponse, 0, len(items))
	for _, item := range items {
		out = append(out, FromLabel(item))
	}
	return out
}
