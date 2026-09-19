package handler

import (
	"net/http"

	"github.com/example/teamops/backend/internal/modules/auth/dto"
	authguard "github.com/example/teamops/backend/internal/modules/auth/guard"
	authsvc "github.com/example/teamops/backend/internal/modules/auth/service"
	"github.com/example/teamops/backend/internal/platform/observability"
	"github.com/example/teamops/backend/internal/shared/errors"
	sharedrequest "github.com/example/teamops/backend/internal/shared/request"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service        *authsvc.Service
	loginProtector *authguard.LoginProtector
}

func New(s *authsvc.Service, loginProtector *authguard.LoginProtector) *Handler {
	return &Handler{service: s, loginProtector: loginProtector}
}
func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
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
	if err := sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	ip := c.ClientIP()
	if err := h.loginProtector.Check(c.Request.Context(), ip, req.Email); err != nil {
		observability.AuthenticationFailure("login_blocked")
		response.Error(c, err)
		return
	}
	out, err := h.service.Login(c.Request.Context(), req.Email, req.Password, c.Request.UserAgent(), ip)
	if err != nil {
		if ae := apperror.As(err); ae.Code != "invalid_credentials" {
			h.loginProtector.Reset(c.Request.Context(), ip, req.Email)
		} else {
			observability.AuthenticationFailure("invalid_credentials")
		}
		response.Error(c, err)
		return
	}
	h.loginProtector.Reset(c.Request.Context(), ip, req.Email)
	response.OK(c, dto.NewTokenResponse(out.AccessToken, out.AccessTokenExpiresAt, out.RefreshToken, out.User))
}
func (h *Handler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	out, err := h.service.Refresh(c.Request.Context(), req.RefreshToken, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		if apperror.As(err).Code == "invalid_refresh_token" {
			observability.AuthenticationFailure("invalid_refresh_token")
		}
		response.Error(c, err)
		return
	}
	response.OK(c, dto.NewTokenResponse(out.AccessToken, out.AccessTokenExpiresAt, out.RefreshToken, out.User))
}
func (h *Handler) Logout(c *gin.Context) {
	var req dto.RefreshRequest
	if err := sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
