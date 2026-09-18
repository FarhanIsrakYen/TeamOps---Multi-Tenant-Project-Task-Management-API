package model

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID             int64          `json:"id"`
	OrganizationID *uuid.UUID     `json:"organizationId,omitempty"`
	ActorUserID    *uuid.UUID     `json:"actorUserId,omitempty"`
	Action         string         `json:"action"`
	ResourceType   string         `json:"resourceType"`
	ResourceID     string         `json:"resourceId"`
	Metadata       map[string]any `json:"metadata"`
	RequestID      string         `json:"requestId"`
	IPAddress      string         `json:"ipAddress,omitempty"`
	UserAgent      string         `json:"userAgent,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
}

type Entry struct {
	OrganizationID *uuid.UUID
	ActorUserID    *uuid.UUID
	Action         string
	ResourceType   string
	ResourceID     string
	Metadata       map[string]any
	RequestID      string
	IPAddress      string
	UserAgent      string
}

type Filters struct {
	ActorUserID  *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   string
	From         *time.Time
	To           *time.Time
}
