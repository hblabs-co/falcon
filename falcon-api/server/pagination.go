package server

import (
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Pagination is the envelope every paginated endpoint returns under the
// "pagination" JSON key. Shared across handlers so the iOS client can decode
// it with one model.
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// ParsePage reads ?page=N. Defaults to 1, clamps to a floor of 1 on invalid input.
func ParsePage(c *gin.Context) int {
	if p := c.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			return n
		}
	}
	return 1
}

// Paginate builds the envelope, computing total_pages from total + pageSize.
func Paginate(page, pageSize int, total int64) Pagination {
	return Pagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: int(math.Ceil(float64(total) / float64(pageSize))),
	}
}
