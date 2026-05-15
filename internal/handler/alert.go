package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/alert"
	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
)

type AlertHandler struct {
	service *alert.Service
	audit   *audit.Service
}

func NewAlertHandler(service *alert.Service, auditService *audit.Service) *AlertHandler {
	return &AlertHandler{service: service, audit: auditService}
}

func (h *AlertHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/alert-rules", rbac.RequirePermission(rbacService, "alerts.view"), h.ListRules)
	r.POST("/alert-rules", rbac.RequirePermission(rbacService, "alerts.manage"), h.CreateRule)
	r.PATCH("/alert-rules/:id", rbac.RequirePermission(rbacService, "alerts.manage"), h.UpdateRule)
	r.DELETE("/alert-rules/:id", rbac.RequirePermission(rbacService, "alerts.manage"), h.DeleteRule)
	r.GET("/alerts/events", rbac.RequirePermission(rbacService, "alerts.view"), h.ListEvents)
	r.POST("/alerts/events", rbac.RequirePermission(rbacService, "alerts.manage"), h.CreateEvent)
	r.GET("/alerts/events/:id", rbac.RequirePermission(rbacService, "alerts.view"), h.GetEvent)
	r.POST("/alerts/events/:id/ack", rbac.RequirePermission(rbacService, "alerts.ack"), h.AckEvent)
	r.POST("/alerts/events/:id/resolve", rbac.RequirePermission(rbacService, "alerts.ack"), h.ResolveEvent)
	r.GET("/alerts/records", rbac.RequirePermission(rbacService, "alerts.view"), h.Records)
}

func (h *AlertHandler) ListRules(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListRules(c.Request.Context(), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *AlertHandler) CreateRule(c *gin.Context) {
	var req alert.RuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.CreateRule(c.Request.Context(), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "alert_rule.create", ResourceType: "alert_rule", ResourceID: item.ID, Result: model.AuditResultSuccess})
	Created(c, item)
}

func (h *AlertHandler) UpdateRule(c *gin.Context) {
	var req alert.RuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.UpdateRule(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "alert_rule.update", ResourceType: "alert_rule", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *AlertHandler) DeleteRule(c *gin.Context) {
	if err := h.service.DeleteRule(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "alert_rule.delete", ResourceType: "alert_rule", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"deleted": true})
}

func (h *AlertHandler) ListEvents(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListEvents(c.Request.Context(), c.Query("status"), c.Query("eventType"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *AlertHandler) CreateEvent(c *gin.Context) {
	var req alert.EventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.service.CreateEvent(c.Request.Context(), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "alert_event.create", ResourceType: "alert_event", ResourceID: item.ID, Result: model.AuditResultSuccess})
	Created(c, item)
}

func (h *AlertHandler) GetEvent(c *gin.Context) {
	item, err := h.service.GetEvent(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *AlertHandler) AckEvent(c *gin.Context) {
	item, err := h.service.AckEvent(c.Request.Context(), c.Param("id"), OperatorID(c))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "alert_event.ack", ResourceType: "alert_event", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *AlertHandler) ResolveEvent(c *gin.Context) {
	item, err := h.service.ResolveEvent(c.Request.Context(), c.Param("id"), OperatorID(c))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "alert_event.resolve", ResourceType: "alert_event", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *AlertHandler) Records(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.Records(c.Request.Context(), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}
