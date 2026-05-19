package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	envsvc "github.com/Humphrey-He/AegisOps/internal/environment"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
)

type EnvironmentHandler struct {
	service *envsvc.Service
	audit   *audit.Service
}

func NewEnvironmentHandler(service *envsvc.Service, auditService *audit.Service) *EnvironmentHandler {
	return &EnvironmentHandler{service: service, audit: auditService}
}

func (h *EnvironmentHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/environments", rbac.RequirePermission(rbacService, "environments.view"), h.List)
	r.POST("/environments", rbac.RequirePermission(rbacService, "environments.manage"), h.Create)
	r.GET("/environments/:id", rbac.RequirePermission(rbacService, "environments.view"), h.Get)
	r.PATCH("/environments/:id", rbac.RequirePermission(rbacService, "environments.manage"), h.Update)
	r.DELETE("/environments/:id", rbac.RequirePermission(rbacService, "environments.manage"), h.Delete)
}

func (h *EnvironmentHandler) List(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.List(c.Request.Context(), c.Query("keyword"), c.Query("status"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *EnvironmentHandler) Create(c *gin.Context) {
	var req envsvc.CreateRequest
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "environment.create", ResourceType: "environment", ResourceID: item.ID, Result: model.AuditResultSuccess})
	Created(c, item)
}

func (h *EnvironmentHandler) Get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *EnvironmentHandler) Update(c *gin.Context) {
	var req envsvc.UpdateRequest
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "environment.update", ResourceType: "environment", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *EnvironmentHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "environment.delete", ResourceType: "environment", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"deleted": true})
}
