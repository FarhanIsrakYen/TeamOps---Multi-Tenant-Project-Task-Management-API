package pagination

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Params struct {
	Page, PageSize, Offset int
	SortBy, Order          string
}
type Meta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

func From(c *gin.Context, allowed map[string]bool, defaultSort string) Params {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	rawSize := c.Query("page_size")
	if rawSize == "" {
		rawSize = c.DefaultQuery("pageSize", "20")
	}
	size, _ := strconv.Atoi(rawSize)
	if page < 1 {
		page = 1
	}
	if page > 1_000_000 {
		page = 1_000_000
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	sort := c.DefaultQuery("sort", defaultSort)
	order := strings.ToLower(c.DefaultQuery("order", "desc"))
	if strings.HasPrefix(sort, "-") {
		order = "desc"
		sort = strings.TrimPrefix(sort, "-")
	} else if strings.HasPrefix(sort, "+") {
		order = "asc"
		sort = strings.TrimPrefix(sort, "+")
	}
	sort = canonicalSort(sort)
	if !allowed[sort] {
		sort = defaultSort
	}
	if order != "asc" {
		order = "desc"
	}
	return Params{Page: page, PageSize: size, Offset: (page - 1) * size, SortBy: sort, Order: order}
}

func canonicalSort(value string) string {
	aliases := map[string]string{
		"createdAt": "created_at",
		"updatedAt": "updated_at",
		"dueAt":     "due_at",
	}
	if canonical, ok := aliases[value]; ok {
		return canonical
	}
	return value
}
func NewMeta(p Params, total int64) Meta {
	pages := int((total + int64(p.PageSize) - 1) / int64(p.PageSize))
	return Meta{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: pages}
}
