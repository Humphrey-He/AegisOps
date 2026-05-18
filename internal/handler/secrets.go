package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
	"github.com/Humphrey-He/AegisOps/internal/secret"
)

type SecretHandler struct {
	service *secret.Service
	audit   *audit.Service
	rbac    *rbac.Service
}

func NewSecretHandler(service *secret.Service, auditService *audit.Service) *SecretHandler {
	return &SecretHandler{service: service, audit: auditService}
}

func (h *SecretHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	h.rbac = rbacService
	r.GET("/secrets", rbac.RequirePermission(rbacService, "secrets.view"), h.List)
	r.POST("/secrets", rbac.RequirePermission(rbacService, "secrets.manage"), h.Create)
	r.GET("/secrets/:id", rbac.RequirePermission(rbacService, "secrets.view"), h.Get)
	r.GET("/secrets/:id/references", rbac.RequirePermission(rbacService, "secrets.view"), h.References)
	r.GET("/secrets/:id/read-audits", rbac.RequirePermission(rbacService, "secrets.view"), h.ReadAudits)
	r.PATCH("/secrets/:id", rbac.RequirePermission(rbacService, "secrets.manage"), h.Update)
	r.POST("/secrets/:id/rotate", rbac.RequirePermission(rbacService, "secrets.rotate"), h.Rotate)
	r.DELETE("/secrets/:id", rbac.RequirePermission(rbacService, "secrets.manage"), h.Delete)
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "secret.create", ResourceType: "secret", ResourceID: item.ID, Result: model.AuditResultSuccess})
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
	if req.Value != nil && !h.ensurePermission(c, "secrets.rotate") {
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "secret.update", ResourceType: "secret", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *SecretHandler) Rotate(c *gin.Context) {
	var req secret.RotateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.Rotate(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "secret.rotate", ResourceType: "secret", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *SecretHandler) References(c *gin.Context) {
	items, err := h.service.References(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"items": items, "total": len(items)})
}

func (h *SecretHandler) ensurePermission(c *gin.Context, permissionCode string) bool {
	return ensurePermission(c, h.rbac, permissionCode)
}

func (h *SecretHandler) ReadAudits(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ReadAudits(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *SecretHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "secret.delete", ResourceType: "secret", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"deleted": true})
}
