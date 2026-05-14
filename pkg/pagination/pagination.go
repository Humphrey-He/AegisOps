package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

type Page struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int64 `json:"total"`
}

type Result[T any] struct {
	Items []T  `json:"items"`
	Page  Page `json:"page"`
}

func FromQuery(c *gin.Context) (page int, pageSize int, offset int) {
	page = parsePositive(c.DefaultQuery("page", "1"), 1)
	pageSize = parsePositive(c.DefaultQuery("pageSize", "20"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	offset = (page - 1) * pageSize
	return page, pageSize, offset
}

func parsePositive(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
