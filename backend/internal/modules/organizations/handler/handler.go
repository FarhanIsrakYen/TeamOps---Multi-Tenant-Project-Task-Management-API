package handler

import (
	"net/http"

	"github.com/example/teamops/backend/internal/modules/organizations/dto"
	"github.com/example/teamops/backend/internal/modules/organizations/model"
	orgsvc "github.com/example/teamops/backend/internal/modules/organizations/service"
	apperror "github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/pagination"
	sharedrequest "github.com/example/teamops/backend/internal/shared/request"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *orgsvc.Service }

func New(s *orgsvc.Service) *Handler { return &Handler{service: s} }
func IDs(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
	uid, ok := c.Get("user_id")
	if !ok {
		return uuid.Nil, uuid.Nil, apperror.ErrUnauthorized
	}
	oid, err := uuid.Parse(c.Param("organizationId"))
	if err != nil {
		return uuid.Nil, uuid.Nil, apperror.ErrValidation
	}
	return uid.(uuid.UUID), oid, nil
}
func (h *Handler) List(c *gin.Context) {
	uid := c.MustGet("user_id").(uuid.UUID)
	p := pagination.From(c, map[string]bool{"name": true, "created_at": true}, "created_at")
	items, total, err := h.service.List(c, uid, p)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.List(c, dto.FromOrganizations(items), pagination.NewMeta(p, total))
}
func (h *Handler) Create(c *gin.Context) {
	var req dto.CreateRequest
	if err := sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	uid := c.MustGet("user_id").(uuid.UUID)
	o, err := h.service.Create(c, uid, req.Name, req.Slug, c.GetString("request_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, dto.FromOrganization(o))
}
func (h *Handler) Get(c *gin.Context) {
	uid, oid, err := IDs(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	o, err := h.service.Get(c, uid, oid)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dto.FromOrganization(o))
}
func (h *Handler) Update(c *gin.Context) {
	uid, oid, err := IDs(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	var req dto.UpdateRequest
	if err = sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	o, err := h.service.Update(c, uid, oid, req.Name, req.Slug, req.Version, c.GetString("request_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dto.FromOrganization(o))
}
func (h *Handler) Delete(c *gin.Context) {
	uid, oid, err := IDs(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err = h.service.Delete(c, uid, oid, c.GetString("request_id")); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) Members(c *gin.Context) {
	uid, oid, err := IDs(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	items, err := h.service.Members(c, uid, oid)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dto.FromMemberships(items))
}
func (h *Handler) AddMember(c *gin.Context) {
	uid, oid, err := IDs(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	var req dto.AddMemberRequest
	if err = sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	m, err := h.service.AddMember(c, uid, oid, req.Email, model.Role(req.Role), c.GetString("request_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, dto.FromMembership(m))
}

func (h *Handler) RemoveMember(c *gin.Context) {
	uid, oid, err := IDs(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	memberID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		response.Error(c, apperror.ErrValidation)
		return
	}
	if err = h.service.RemoveMember(c, uid, oid, memberID, c.GetString("request_id")); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
