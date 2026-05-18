package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/alert"
	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
)

type AlertHandler struct {
	service *alert.Service
	audit   *audit.Service
	db      *gorm.DB
}

func NewAlertHandler(service *alert.Service, auditService *audit.Service, db *gorm.DB) *AlertHandler {
	return &AlertHandler{service: service, audit: auditService, db: db}
}

type AlertEventContext struct {
	Event         *model.AlertEvent          `json:"event"`
	Task          *model.Task                `json:"task,omitempty"`
	Resource      *ResourceSummary           `json:"resource,omitempty"`
	Navigation    ResourceNavigation         `json:"navigation"`
	Risk          ResourceRisk               `json:"risk"`
	RelatedAudits []model.AuditLog           `json:"relatedAudits"`
	Notifications []model.NotificationRecord `json:"notifications"`
	NextActions   []ResourceActionHint       `json:"nextActions"`
}

func (h *AlertHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/alert-rules", rbac.RequirePermission(rbacService, "alerts.view"), h.ListRules)
	r.POST("/alert-rules", rbac.RequirePermission(rbacService, "alerts.manage"), h.CreateRule)
	r.PATCH("/alert-rules/:id", rbac.RequirePermission(rbacService, "alerts.manage"), h.UpdateRule)
	r.DELETE("/alert-rules/:id", rbac.RequirePermission(rbacService, "alerts.manage"), h.DeleteRule)
	r.GET("/alerts/events", rbac.RequirePermission(rbacService, "alerts.view"), h.ListEvents)
	r.POST("/alerts/events", rbac.RequirePermission(rbacService, "alerts.manage"), h.CreateEvent)
	r.GET("/alerts/events/:id", rbac.RequirePermission(rbacService, "alerts.view"), h.GetEvent)
	r.GET("/alerts/events/:id/context", rbac.RequirePermission(rbacService, "alerts.view"), h.Context)
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
	items, total, err := h.service.ListEvents(
		c.Request.Context(),
		c.Query("status"),
		c.Query("eventType"),
		c.Query("resourceType"),
		c.Query("resourceId"),
		limit,
		offset,
	)
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

func (h *AlertHandler) Context(c *gin.Context) {
	event, err := h.service.GetEvent(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	resourceType := normalizeResourceType(event.ResourceType)
	result := AlertEventContext{
		Event: event,
	}
	if resourceType != "" && event.ResourceID != "" {
		result.Navigation = resourceNavigation(resourceType, event.ResourceID)
		summary, err := resourceSummaryByDB(c.Request.Context(), h.db, resourceType, event.ResourceID)
		if err == nil {
			result.Resource = summary
			result.NextActions = resourceActionHints(resourceType, summary.Status, event.ResourceID)
		}
		risk, err := resourceRiskByDB(c.Request.Context(), h.db, resourceType, event.ResourceID)
		if err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		result.Risk = risk
		if err := h.db.WithContext(c.Request.Context()).Where("resource_type = ? AND resource_id = ?", resourceType, event.ResourceID).
			Order("created_at desc").
			Limit(10).
			Find(&result.RelatedAudits).Error; err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if event.TaskID != "" {
		var taskItem model.Task
		if err := h.db.WithContext(c.Request.Context()).Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, created_at ASC")
		}).Preload("Logs", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).First(&taskItem, "id = ?", event.TaskID).Error; err != nil && err != gorm.ErrRecordNotFound {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		} else if err == nil {
			result.Task = &taskItem
		}
	}
	if err := h.db.WithContext(c.Request.Context()).Where("event_id = ?", event.ID).
		Order("created_at desc").
		Limit(10).
		Find(&result.Notifications).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, result)
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
