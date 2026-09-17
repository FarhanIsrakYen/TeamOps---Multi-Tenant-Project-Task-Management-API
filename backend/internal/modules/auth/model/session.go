package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrRefreshSessionInvalid = errors.New("refresh session is invalid, expired, or reused")

type Session struct {
	ID        uuid.UUID
	FamilyID  uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	RevokedAt *time.Time
}
