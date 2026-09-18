package dto

import (
	"time"

	auditmodel "github.com/example/teamops/backend/internal/modules/audit/model"
	"github.com/google/uuid"
)

type LogResponse struct {
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

func FromModels(logs []auditmodel.AuditLog) []LogResponse {
	out := make([]LogResponse, 0, len(logs))
	for _, log := range logs {
		out = append(out, LogResponse{ID: log.ID, OrganizationID: log.OrganizationID, ActorUserID: log.ActorUserID, Action: log.Action, ResourceType: log.ResourceType, ResourceID: log.ResourceID, Metadata: log.Metadata, RequestID: log.RequestID, IPAddress: log.IPAddress, UserAgent: log.UserAgent, CreatedAt: log.CreatedAt})
	}
	return out
}
