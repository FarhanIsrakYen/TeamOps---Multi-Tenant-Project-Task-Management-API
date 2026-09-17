package dto

import (
	"time"

	projectmodel "github.com/example/teamops/backend/internal/modules/projects/model"
	"github.com/google/uuid"
)

type CreateRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=120"`
	Description string `json:"description" binding:"max=5000"`
}
type UpdateRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=2,max=120"`
	Description *string `json:"description" binding:"omitempty,max=5000"`
	Archived    *bool   `json:"archived"`
	Version     int     `json:"version" binding:"required,min=1"`
}
type ProjectResponse struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Archived       bool      `json:"archived"`
	Version        int       `json:"version"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func FromModel(p projectmodel.Project) ProjectResponse {
	return ProjectResponse{ID: p.ID, OrganizationID: p.OrganizationID, Name: p.Name, Description: p.Description, Archived: p.Archived, Version: p.Version, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}
func FromModels(items []projectmodel.Project) []ProjectResponse {
	out := make([]ProjectResponse, 0, len(items))
	for _, item := range items {
		out = append(out, FromModel(item))
	}
	return out
}
