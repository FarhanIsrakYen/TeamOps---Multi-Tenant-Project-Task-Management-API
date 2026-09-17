package handler

import (
	userdto "github.com/example/teamops/backend/internal/modules/users/dto"
	usersvc "github.com/example/teamops/backend/internal/modules/users/service"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *usersvc.Service }

func New(service *usersvc.Service) *Handler { return &Handler{service: service} }

func (h *Handler) Me(c *gin.Context) {
	u, err := h.service.Me(c, c.MustGet("user_id").(uuid.UUID))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, userdto.FromModel(u))
}
