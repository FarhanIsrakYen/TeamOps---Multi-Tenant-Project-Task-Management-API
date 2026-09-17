package pagination

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestFromBoundsAndAllowListsSort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name, query string
		want        Params
	}{{"defaults", "", Params{Page: 1, PageSize: 20, Offset: 0, SortBy: "createdAt", Order: "desc"}}, {"bounded page size", "?page=3&pageSize=999&sort=name&order=asc", Params{Page: 3, PageSize: 100, Offset: 200, SortBy: "name", Order: "asc"}}, {"invalid values", "?page=-1&pageSize=0&sort=unsafe&order=DROP", Params{Page: 1, PageSize: 20, Offset: 0, SortBy: "createdAt", Order: "desc"}}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/"+tt.query, nil)
			got := From(c, map[string]bool{"name": true, "createdAt": true}, "createdAt")
			require.Equal(t, tt.want, got)
		})
	}
}
