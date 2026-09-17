package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	usermodel "github.com/example/teamops/backend/internal/modules/users/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTokenResponseNeverSerializesPasswordHash(t *testing.T) {
	t.Parallel()
	user := usermodel.User{
		ID:           uuid.New(),
		Email:        "person@example.com",
		Name:         "Person",
		PasswordHash: "must-never-leave-the-service",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	encoded, err := json.Marshal(NewTokenResponse("access", time.Now(), "refresh", user))
	require.NoError(t, err)
	require.NotContains(t, strings.ToLower(string(encoded)), "password")
	require.NotContains(t, string(encoded), user.PasswordHash)
}
