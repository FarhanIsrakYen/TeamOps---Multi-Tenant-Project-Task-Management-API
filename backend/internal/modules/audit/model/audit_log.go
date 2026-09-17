package model

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID             int64          `json:"id"`
	OrganizationID uuid.UUID      `json:"organizationId"`
	ActorID        *uuid.UUID     `json:"actorId,omitempty"`
	Action         string         `json:"action"`
	ResourceType   string         `json:"resourceType"`
	ResourceID     string         `json:"resourceId"`
	Metadata       map[string]any `json:"metadata"`
	RequestID      string         `json:"requestId"`
	CreatedAt      time.Time      `json:"createdAt"`
}
