package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
	servicesvc "github.com/Humphrey-He/AegisOps/internal/service"
)

type ServiceHandler struct {
	service *servicesvc.Service
	audit   *audit.Service
}

func NewServiceHandler(service *servicesvc.Service, auditService *audit.Service) *ServiceHandler {
	return &ServiceHandler{service: service, audit: auditService}
}

func (h *ServiceHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/services", rbac.RequirePermission(rbacService, "services.view"), h.List)
	r.POST("/services", rbac.RequirePermission(rbacService, "services.manage"), h.Create)
	r.GET("/services/:id", rbac.RequirePermission(rbacService, "services.view"), h.Get)
	r.PATCH("/services/:id", rbac.RequirePermission(rbacService, "services.manage"), h.Update)
	r.DELETE("/services/:id", rbac.RequirePermission(rbacService, "services.manage"), h.Delete)
	r.GET("/services/:id/instances", rbac.RequirePermission(rbacService, "services.view"), h.Instances)
	r.GET("/services/:id/releases", rbac.RequirePermission(rbacService, "services.view"), h.Releases)
	r.GET("/services/:id/history", rbac.RequirePermission(rbacService, "services.view"), h.Releases)
	r.GET("/services/:id/versions", rbac.RequirePermission(rbacService, "services.view"), h.Versions)
	r.POST("/services/:id/releases", rbac.RequirePermission(rbacService, "services.release"), h.Release)
	r.POST("/services/:id/upgrades", rbac.RequirePermission(rbacService, "services.release"), h.Upgrade)
	r.POST("/services/:id/rollbacks", rbac.RequirePermission(rbacService, "services.rollback"), h.Rollback)
}

func (h *ServiceHandler) List(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.List(c.Request.Context(), c.Query("keyword"), c.Query("status"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *ServiceHandler) Create(c *gin.Context) {
	var req servicesvc.CreateRequest
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "service.create", ResourceType: "service", ResourceID: item.ID, Result: model.AuditResultSuccess})
	Created(c, item)
}

func (h *ServiceHandler) Get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *ServiceHandler) Update(c *gin.Context) {
	var req servicesvc.UpdateRequest
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "service.update", ResourceType: "service", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *ServiceHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "service.delete", ResourceType: "service", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"deleted": true})
}

func (h *ServiceHandler) Instances(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.Instances(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *ServiceHandler) Releases(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.Releases(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *ServiceHandler) Versions(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.Versions(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *ServiceHandler) Release(c *gin.Context) {
	var req servicesvc.ReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	result, err := h.service.Release(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "service.release", ResourceType: "service", ResourceID: c.Param("id"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "service.release", ResourceType: "service", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	Created(c, result)
}

func (h *ServiceHandler) Upgrade(c *gin.Context) {
	var req servicesvc.ReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	result, err := h.service.Upgrade(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "service.upgrade", ResourceType: "service", ResourceID: c.Param("id"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "service.upgrade", ResourceType: "service", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	Created(c, result)
}

func (h *ServiceHandler) Rollback(c *gin.Context) {
	var req servicesvc.RollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	result, err := h.service.Rollback(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "service.rollback", ResourceType: "service", ResourceID: c.Param("id"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "service.rollback", ResourceType: "service", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	Created(c, result)
}
