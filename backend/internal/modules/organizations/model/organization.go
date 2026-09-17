package model

import (
	"time"

	usermodel "github.com/example/teamops/backend/internal/modules/users/model"
	"github.com/google/uuid"
)

type Role string

const (
	RoleOwner  Role = "OWNER"
	RoleAdmin  Role = "ADMIN"
	RoleMember Role = "MEMBER"
	RoleViewer Role = "VIEWER"
)

type Organization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedBy uuid.UUID `json:"createdBy"`
	Version   int       `json:"version"`
	Role      Role      `json:"role,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Membership struct {
	OrganizationID uuid.UUID       `json:"organizationId"`
	UserID         uuid.UUID       `json:"userId"`
	Role           Role            `json:"role"`
	User           *usermodel.User `json:"user,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
}
