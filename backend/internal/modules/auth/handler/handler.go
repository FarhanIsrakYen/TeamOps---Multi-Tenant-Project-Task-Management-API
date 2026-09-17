package handler

import (
	"net/http"

	"github.com/example/teamops/backend/internal/modules/auth/dto"
	authsvc "github.com/example/teamops/backend/internal/modules/auth/service"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *authsvc.Service }

func New(s *authsvc.Service) *Handler { return &Handler{service: s} }
func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrValidation)
		return
	}
	out, err := h.service.Register(c.Request.Context(), req.Email, req.Name, req.Password, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, dto.NewTokenResponse(out.AccessToken, out.AccessTokenExpiresAt, out.RefreshToken, out.User))
}
func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrValidation)
		return
	}
	out, err := h.service.Login(c.Request.Context(), req.Email, req.Password, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dto.NewTokenResponse(out.AccessToken, out.AccessTokenExpiresAt, out.RefreshToken, out.User))
}
func (h *Handler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrValidation)
		return
	}
	out, err := h.service.Refresh(c.Request.Context(), req.RefreshToken, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dto.NewTokenResponse(out.AccessToken, out.AccessTokenExpiresAt, out.RefreshToken, out.User))
}
func (h *Handler) Logout(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.ErrValidation)
		return
	}
	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
