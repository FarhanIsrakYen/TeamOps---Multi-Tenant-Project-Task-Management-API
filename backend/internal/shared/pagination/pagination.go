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
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	sort := c.DefaultQuery("sort", defaultSort)
	if !allowed[sort] {
		sort = defaultSort
	}
	order := strings.ToLower(c.DefaultQuery("order", "desc"))
	if order != "asc" {
		order = "desc"
	}
	return Params{Page: page, PageSize: size, Offset: (page - 1) * size, SortBy: sort, Order: order}
}
func NewMeta(p Params, total int64) Meta {
	pages := int((total + int64(p.PageSize) - 1) / int64(p.PageSize))
	return Meta{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: pages}
}
