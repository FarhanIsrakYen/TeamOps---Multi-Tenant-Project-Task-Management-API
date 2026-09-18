package handler

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFiltersFromParsesActorResourceAndDateRange(t *testing.T) {
	t.Parallel()
	actorID := uuid.New()
	from := "2026-01-01T00:00:00Z"
	to := "2026-01-31T23:59:59Z"
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?actor_id="+actorID.String()+"&action=member.role_changed&resource_type=membership&resource_id=target&from="+from+"&to="+to, nil)

	filters, err := filtersFrom(c)
	require.NoError(t, err)
	require.Equal(t, &actorID, filters.ActorUserID)
	require.Equal(t, "member.role_changed", filters.Action)
	require.Equal(t, "membership", filters.ResourceType)
	require.Equal(t, "target", filters.ResourceID)
	require.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), *filters.From)
	require.Equal(t, time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC), *filters.To)
}

func TestFiltersFromRejectsMalformedValues(t *testing.T) {
	t.Parallel()
	for _, query := range []string{"?actor_id=not-a-uuid", "?from=yesterday", "?to=tomorrow"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/"+query, nil)
		_, err := filtersFrom(c)
		require.Error(t, err)
	}
}
