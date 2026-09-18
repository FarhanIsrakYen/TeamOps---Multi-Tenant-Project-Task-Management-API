package dto

import (
	"time"

	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	userdto "github.com/example/teamops/backend/internal/modules/users/dto"
	"github.com/google/uuid"
)

type CreateRequest struct {
	Name string `json:"name" binding:"required,min=2,max=120"`
	Slug string `json:"slug" binding:"required,min=2,max=80"`
}
type UpdateRequest struct {
	Name    string `json:"name" binding:"required,min=2,max=120"`
	Slug    string `json:"slug" binding:"required,min=2,max=80"`
	Version int    `json:"version" binding:"required,min=1"`
}
type AddMemberRequest struct {
	Email string `json:"email" binding:"required,email,max=254"`
	Role  string `json:"role" binding:"required,oneof=ADMIN MEMBER VIEWER"`
}

type OrganizationResponse struct {
	ID        uuid.UUID     `json:"id"`
	Name      string        `json:"name"`
	Slug      string        `json:"slug"`
	CreatedBy uuid.UUID     `json:"createdBy"`
	Version   int           `json:"version"`
	Role      orgmodel.Role `json:"role,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}
type MembershipResponse struct {
	OrganizationID uuid.UUID             `json:"organizationId"`
	UserID         uuid.UUID             `json:"userId"`
	Role           orgmodel.Role         `json:"role"`
	User           *userdto.UserResponse `json:"user,omitempty"`
	CreatedAt      time.Time             `json:"createdAt"`
}

func FromOrganization(o orgmodel.Organization) OrganizationResponse {
	return OrganizationResponse{ID: o.ID, Name: o.Name, Slug: o.Slug, CreatedBy: o.CreatedBy, Version: o.Version, Role: o.Role, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt}
}
func FromOrganizations(items []orgmodel.Organization) []OrganizationResponse {
	out := make([]OrganizationResponse, 0, len(items))
	for _, item := range items {
		out = append(out, FromOrganization(item))
	}
	return out
}
func FromMembership(m orgmodel.Membership) MembershipResponse {
	out := MembershipResponse{OrganizationID: m.OrganizationID, UserID: m.UserID, Role: m.Role, CreatedAt: m.CreatedAt}
	if m.User != nil {
		u := userdto.FromModel(*m.User)
		out.User = &u
	}
	return out
}
func FromMemberships(items []orgmodel.Membership) []MembershipResponse {
	out := make([]MembershipResponse, 0, len(items))
	for _, item := range items {
		out = append(out, FromMembership(item))
	}
	return out
}
