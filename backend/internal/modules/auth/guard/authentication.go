package guard

import (
	"strings"

	"github.com/example/teamops/backend/internal/platform/observability"
	sharedauth "github.com/example/teamops/backend/internal/shared/auth"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func RequireAuthenticatedUser(tokens *sharedauth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			observability.AuthenticationFailure("missing_bearer")
			response.Error(c, apperror.ErrUnauthorized)
			c.Abort()
			return
		}
		claims, err := tokens.Parse(parts[1])
		if err != nil {
			observability.AuthenticationFailure("invalid_access_token")
			response.Error(c, apperror.ErrUnauthorized)
			c.Abort()
			return
		}
		identity := sharedauth.Identity{UserID: claims.UserID, Email: claims.Email, TokenID: claims.ID}
		c.Set("identity", identity)
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Request = c.Request.WithContext(sharedauth.ContextWithIdentity(c.Request.Context(), identity))
		c.Next()
	}
}
