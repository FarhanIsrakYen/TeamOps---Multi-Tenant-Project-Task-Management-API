package guard

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sharedauth "github.com/example/teamops/backend/internal/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRequireAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := sharedauth.NewTokenManager("this-is-a-long-enough-test-secret-value", "teamops-api", "teamops-web", time.Minute)
	userID := uuid.New()
	accessToken, _, err := manager.AccessToken(userID, "person@example.com")
	require.NoError(t, err)

	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{name: "valid token", header: "Bearer " + accessToken, wantStatus: http.StatusNoContent},
		{name: "missing authorization header", wantStatus: http.StatusUnauthorized},
		{name: "malformed bearer token", header: "Bearer not-a-token", wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/protected", RequireAuthenticatedUser(manager), func(c *gin.Context) {
				identity, ok := sharedauth.IdentityFromContext(c.Request.Context())
				require.True(t, ok)
				require.Equal(t, userID, identity.UserID)
				require.NotEmpty(t, identity.TokenID)
				c.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.header != "" {
				request.Header.Set("Authorization", tt.header)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, tt.wantStatus, response.Code)
		})
	}
}
