package handler

import (
	auditdto "github.com/example/teamops/backend/internal/modules/audit/dto"
	auditsvc "github.com/example/teamops/backend/internal/modules/audit/service"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *auditsvc.Service
}

func New(service *auditsvc.Service) *Handler { return &Handler{service: service} }
func (h *Handler) List(c *gin.Context) {
	oid, err := uuid.Parse(c.Param("organizationId"))
	if err != nil {
		response.Error(c, apperror.ErrValidation)
		return
	}
	uid := c.MustGet("user_id").(uuid.UUID)
	p := pagination.From(c, map[string]bool{"createdAt": true}, "createdAt")
	items, total, err := h.service.List(c, uid, oid, p)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.List(c, auditdto.FromModels(items), pagination.NewMeta(p, total))
}
