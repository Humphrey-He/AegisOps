package handler

import (
	"strconv"

	"github.com/Humphrey-He/AegisOps/internal/auth"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/pkg/response"
	"github.com/gin-gonic/gin"
)

type PageResult struct {
	Items  interface{} `json:"items"`
	Total  int64       `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

func OK(c *gin.Context, data interface{}) {
	response.OK(c, data)
}

func Created(c *gin.Context, data interface{}) {
	response.Created(c, data)
}

func Error(c *gin.Context, status int, message string) {
	response.Error(c, status, "ERROR", message)
}

func Pagination(c *gin.Context) (int, int) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func OperatorID(c *gin.Context) string {
	value, ok := c.Get(auth.CurrentUserKey)
	if !ok {
		return ""
	}
	user, ok := value.(*model.User)
	if !ok || user == nil {
		return ""
	}
	return strconv.FormatUint(uint64(user.ID), 10)
}
