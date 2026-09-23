package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrLabelNotInOrganization = errors.New("label does not belong to task organization")

type Status string

const (
	StatusTODO       Status = "TODO"
	StatusInProgress Status = "IN_PROGRESS"
	StatusInReview   Status = "IN_REVIEW"
	StatusDone       Status = "DONE"
	StatusCancelled  Status = "CANCELLED"
)

func (s Status) Valid() bool {
	switch s {
	case StatusTODO, StatusInProgress, StatusInReview, StatusDone, StatusCancelled:
		return true
	default:
		return false
	}
}

func (s Status) CanTransitionTo(next Status) bool {
	if s == next {
		return true
	}
	switch s {
	case StatusTODO:
		return next == StatusInProgress || next == StatusCancelled
	case StatusInProgress:
		return next == StatusTODO || next == StatusInReview || next == StatusDone || next == StatusCancelled
	case StatusInReview:
		return next == StatusInProgress || next == StatusDone || next == StatusCancelled
	case StatusDone:
		return next == StatusInProgress
	case StatusCancelled:
		return next == StatusTODO
	default:
		return false
	}
}

type Priority string

const (
	PriorityLow    Priority = "LOW"
	PriorityMedium Priority = "MEDIUM"
	PriorityHigh   Priority = "HIGH"
	PriorityUrgent Priority = "URGENT"
)

func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return true
	default:
		return false
	}
}

type Task struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organizationId"`
	ProjectID      uuid.UUID  `json:"projectId"`
	AssigneeID     *uuid.UUID `json:"assigneeId,omitempty"`
	CreatedBy      uuid.UUID  `json:"createdBy"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Status         Status     `json:"status"`
	Priority       Priority   `json:"priority"`
	DueAt          *time.Time `json:"dueAt,omitempty"`
	Version        int        `json:"version"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type Filters struct {
	Status     Status
	Priority   Priority
	Search     string
	AssigneeID *uuid.UUID
}

type Comment struct {
	ID        uuid.UUID `json:"id"`
	TaskID    uuid.UUID `json:"taskId"`
	AuthorID  uuid.UUID `json:"authorId"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Label struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	Name           string    `json:"name"`
	Color          string    `json:"color"`
}
