package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/host"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
)

type HostHandler struct {
	service *host.Service
	audit   *audit.Service
}

func NewHostHandler(service *host.Service, auditService *audit.Service) *HostHandler {
	return &HostHandler{service: service, audit: auditService}
}

func (h *HostHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/hosts", rbac.RequirePermission(rbacService, "hosts.view"), h.List)
	r.POST("/hosts", rbac.RequirePermission(rbacService, "hosts.manage"), h.Create)
	r.GET("/hosts/:id", rbac.RequirePermission(rbacService, "hosts.view"), h.Get)
	r.PATCH("/hosts/:id", rbac.RequirePermission(rbacService, "hosts.manage"), h.Update)
	r.DELETE("/hosts/:id", rbac.RequirePermission(rbacService, "hosts.manage"), h.Delete)
	r.POST("/hosts/:id/test-ssh", rbac.RequirePermission(rbacService, "hosts.manage"), h.TestSSH)
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "host.create", ResourceType: "host", ResourceID: item.ID, Result: model.AuditResultSuccess})
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "host.update", ResourceType: "host", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *HostHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "host.delete", ResourceType: "host", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"deleted": true})
}

func (h *HostHandler) TestSSH(c *gin.Context) {
	if err := h.service.TestSSH(c.Request.Context(), c.Param("id")); err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "host.test_ssh", ResourceType: "host", ResourceID: c.Param("id"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "host.test_ssh", ResourceType: "host", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"connected": true})
}
