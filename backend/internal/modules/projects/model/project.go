package model

import (
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Archived       bool      `json:"archived"`
	Version        int       `json:"version"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Filters struct {
	Archived *bool
	Search   string
}
