package response

import (
	"net/http"

	"github.com/Humphrey-He/AegisOps/internal/middleware"
	"github.com/gin-gonic/gin"
)

type Body[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
	TraceID string `json:"traceId"`
}

func OK[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, Body[T]{
		Code:    "OK",
		Message: "ok",
		Data:    data,
		TraceID: middleware.TraceID(c),
	})
}

func Created[T any](c *gin.Context, data T) {
	c.JSON(http.StatusCreated, Body[T]{
		Code:    "OK",
		Message: "created",
		Data:    data,
		TraceID: middleware.TraceID(c),
	})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func Error(c *gin.Context, status int, code string, message string) {
	c.JSON(status, Body[any]{
		Code:    code,
		Message: message,
		TraceID: middleware.TraceID(c),
	})
}
