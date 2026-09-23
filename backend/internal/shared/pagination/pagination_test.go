package pagination

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFromBoundsAndAllowListsSort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name, query string
		want        Params
	}{{"defaults", "", Params{Page: 1, PageSize: 20, Offset: 0, SortBy: "created_at", Order: "desc"}}, {"bounded page size", "?page=3&page_size=999&sort=name&order=asc", Params{Page: 3, PageSize: 100, Offset: 200, SortBy: "name", Order: "asc"}}, {"descending sort prefix", "?page=2&page_size=25&sort=-created_at&order=asc", Params{Page: 2, PageSize: 25, Offset: 25, SortBy: "created_at", Order: "desc"}}, {"legacy aliases", "?pageSize=10&sort=updatedAt", Params{Page: 1, PageSize: 10, Offset: 0, SortBy: "updated_at", Order: "desc"}}, {"invalid values", "?page=-1&page_size=0&sort=unsafe&order=DROP", Params{Page: 1, PageSize: 20, Offset: 0, SortBy: "created_at", Order: "desc"}}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/"+tt.query, nil)
			got := From(c, map[string]bool{"name": true, "created_at": true, "updated_at": true}, "created_at")
			require.Equal(t, tt.want, got)
		})
	}
}
