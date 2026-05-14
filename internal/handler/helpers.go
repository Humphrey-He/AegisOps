package handler

import (
	"errors"
	"net/http"
	"strconv"

	appErrors "github.com/Humphrey-He/AegisOps/pkg/errors"
	"github.com/Humphrey-He/AegisOps/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || value == 0 {
		response.Error(c, http.StatusBadRequest, "INVALID_PARAM", "invalid "+name)
		return 0, false
	}
	return uint(value), true
}

func pagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appErrors.ErrInvalidCredentials):
		response.Error(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid username or password")
	case errors.Is(err, appErrors.ErrUserDisabled):
		response.Error(c, http.StatusForbidden, "USER_DISABLED", "user is disabled")
	case errors.Is(err, appErrors.ErrUnauthorized):
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
	case errors.Is(err, appErrors.ErrInvalidInput):
		response.Error(c, http.StatusBadRequest, "INVALID_INPUT", "invalid input")
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	default:
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}
