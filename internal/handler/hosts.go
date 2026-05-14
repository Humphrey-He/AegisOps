package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/host"
)

type HostHandler struct {
	service *host.Service
}

func NewHostHandler(service *host.Service) *HostHandler {
	return &HostHandler{service: service}
}

func (h *HostHandler) RegisterRoutes(r gin.IRouter) {
	r.GET("/hosts", h.List)
	r.POST("/hosts", h.Create)
	r.GET("/hosts/:id", h.Get)
	r.PATCH("/hosts/:id", h.Update)
	r.DELETE("/hosts/:id", h.Delete)
	r.POST("/hosts/:id/test-ssh", h.TestSSH)
}

func (h *HostHandler) List(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.List(c.Request.Context(), c.Query("keyword"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *HostHandler) Create(c *gin.Context) {
	var req host.CreateRequest
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

func (h *HostHandler) Get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *HostHandler) Update(c *gin.Context) {
	var req host.UpdateRequest
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

func (h *HostHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"deleted": true})
}

func (h *HostHandler) TestSSH(c *gin.Context) {
	if err := h.service.TestSSH(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"connected": true})
}
