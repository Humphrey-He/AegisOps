package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/secret"
)

type SecretHandler struct {
	service *secret.Service
}

func NewSecretHandler(service *secret.Service) *SecretHandler {
	return &SecretHandler{service: service}
}

func (h *SecretHandler) RegisterRoutes(r gin.IRouter) {
	r.GET("/secrets", h.List)
	r.POST("/secrets", h.Create)
	r.GET("/secrets/:id", h.Get)
	r.PATCH("/secrets/:id", h.Update)
	r.DELETE("/secrets/:id", h.Delete)
}

func (h *SecretHandler) List(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.List(c.Request.Context(), c.Query("keyword"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *SecretHandler) Create(c *gin.Context) {
	var req secret.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	Created(c, item)
}

func (h *SecretHandler) Get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *SecretHandler) Update(c *gin.Context) {
	var req secret.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, item)
}

func (h *SecretHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"deleted": true})
}
