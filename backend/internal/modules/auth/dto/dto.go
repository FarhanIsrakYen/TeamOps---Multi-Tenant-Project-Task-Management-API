package dto

import (
	"time"

	userdto "github.com/example/teamops/backend/internal/modules/users/dto"
	usermodel "github.com/example/teamops/backend/internal/modules/users/model"
)

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Password string `json:"password" binding:"required,min=12,max=72"`
}
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,max=72"`
}
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required,max=256"`
}

type TokenResponse struct {
	AccessToken          string               `json:"accessToken"`
	AccessTokenExpiresAt time.Time            `json:"accessTokenExpiresAt"`
	RefreshToken         string               `json:"refreshToken"`
	User                 userdto.UserResponse `json:"user"`
}

func NewTokenResponse(accessToken string, expiresAt time.Time, refreshToken string, user usermodel.User) TokenResponse {
	return TokenResponse{AccessToken: accessToken, AccessTokenExpiresAt: expiresAt, RefreshToken: refreshToken, User: userdto.FromModel(user)}
}
