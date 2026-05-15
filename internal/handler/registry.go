package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
	registrysvc "github.com/Humphrey-He/AegisOps/internal/registry"
)

type RegistryHandler struct {
	service *registrysvc.Service
	audit   *audit.Service
}

func NewRegistryHandler(service *registrysvc.Service, auditService *audit.Service) *RegistryHandler {
	return &RegistryHandler{service: service, audit: auditService}
}

func (h *RegistryHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/registries", rbac.RequirePermission(rbacService, "registries.view"), h.List)
	r.POST("/registries", rbac.RequirePermission(rbacService, "registries.manage"), h.Create)
	r.GET("/registries/:id", rbac.RequirePermission(rbacService, "registries.view"), h.Get)
	r.PATCH("/registries/:id", rbac.RequirePermission(rbacService, "registries.manage"), h.Update)
	r.DELETE("/registries/:id", rbac.RequirePermission(rbacService, "registries.manage"), h.Delete)
	r.POST("/registries/:id/test", rbac.RequirePermission(rbacService, "registries.manage"), h.Test)
	r.GET("/registries/:id/repositories", rbac.RequirePermission(rbacService, "registries.view"), h.Repositories)
	r.GET("/registries/:id/repositories/:repo/tags", rbac.RequirePermission(rbacService, "registries.view"), h.Tags)
	r.GET("/registries/:id/repositories/:repo/manifests/:reference", rbac.RequirePermission(rbacService, "registries.view"), h.Manifest)
}

func (h *RegistryHandler) List(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.List(c.Request.Context(), c.Query("keyword"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *RegistryHandler) Create(c *gin.Context) {
	var req registrysvc.CreateRequest
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "registry.create", ResourceType: "registry", ResourceID: item.ID, Result: model.AuditResultSuccess})
	Created(c, item)
}

func (h *RegistryHandler) Get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *RegistryHandler) Update(c *gin.Context) {
	var req registrysvc.UpdateRequest
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "registry.update", ResourceType: "registry", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *RegistryHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "registry.delete", ResourceType: "registry", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"deleted": true})
}

func (h *RegistryHandler) Test(c *gin.Context) {
	if err := h.service.Test(c.Request.Context(), c.Param("id")); err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "registry.test", ResourceType: "registry", ResourceID: c.Param("id"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "registry.test", ResourceType: "registry", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"connected": true})
}

func (h *RegistryHandler) Repositories(c *gin.Context) {
	result, err := h.service.Repositories(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, result)
}

func (h *RegistryHandler) Tags(c *gin.Context) {
	result, err := h.service.Tags(c.Request.Context(), c.Param("id"), wildcardParam(c, "repo"))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, result)
}

func (h *RegistryHandler) Manifest(c *gin.Context) {
	result, err := h.service.Manifest(c.Request.Context(), c.Param("id"), wildcardParam(c, "repo"), c.Param("reference"))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, result)
}

func wildcardParam(c *gin.Context, name string) string {
	value := c.Param(name)
	if value == "" {
		value = c.Param("*" + name)
	}
	if len(value) > 0 && value[0] == '/' {
		value = value[1:]
	}
	return value
}
