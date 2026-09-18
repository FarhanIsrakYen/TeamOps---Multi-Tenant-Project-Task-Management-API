package handler

import (
	"net/http"
	"strconv"

	"github.com/example/teamops/backend/internal/modules/projects/dto"
	"github.com/example/teamops/backend/internal/modules/projects/model"
	projectsvc "github.com/example/teamops/backend/internal/modules/projects/service"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/pagination"
	sharedrequest "github.com/example/teamops/backend/internal/shared/request"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *projectsvc.Service }

func New(s *projectsvc.Service) *Handler { return &Handler{service: s} }
func parse(c *gin.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, apperror.ErrValidation
	}
	return id, nil
}
func (h *Handler) Create(c *gin.Context) {
	oid, err := parse(c, "organizationId")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req dto.CreateRequest
	if err = sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	p, err := h.service.Create(c.Request.Context(), c.MustGet("user_id").(uuid.UUID), oid, req.Name, req.Description, c.GetString("request_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, dto.FromModel(p))
}
func (h *Handler) List(c *gin.Context) {
	oid, err := parse(c, "organizationId")
	if err != nil {
		response.Error(c, err)
		return
	}
	p := pagination.From(c, map[string]bool{"name": true, "created_at": true, "updated_at": true}, "created_at")
	var archived *bool
	if raw, ok := c.GetQuery("archived"); ok {
		v, e := strconv.ParseBool(raw)
		if e != nil {
			response.Error(c, apperror.ErrValidation)
			return
		}
		archived = &v
	}
	filters := model.Filters{Archived: archived, Search: c.Query("search")}
	items, total, err := h.service.List(c.Request.Context(), c.MustGet("user_id").(uuid.UUID), oid, p, filters)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.List(c, dto.FromModels(items), pagination.NewMeta(p, total))
}
func (h *Handler) Get(c *gin.Context) {
	id, err := parse(c, "projectId")
	if err != nil {
		response.Error(c, err)
		return
	}
	p, err := h.service.Get(c.Request.Context(), c.MustGet("user_id").(uuid.UUID), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dto.FromModel(p))
}
func (h *Handler) Update(c *gin.Context) {
	id, err := parse(c, "projectId")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req dto.UpdateRequest
	if err = sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	input := projectsvc.UpdateInput{Name: req.Name, Description: req.Description, Archived: req.Archived, Version: req.Version}
	p, err := h.service.Update(c.Request.Context(), c.MustGet("user_id").(uuid.UUID), id, input, c.GetString("request_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dto.FromModel(p))
}
func (h *Handler) Delete(c *gin.Context) {
	id, err := parse(c, "projectId")
	if err != nil {
		response.Error(c, err)
		return
	}
	if err = h.service.Delete(c.Request.Context(), c.MustGet("user_id").(uuid.UUID), id, c.GetString("request_id")); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
