package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
	"github.com/Humphrey-He/AegisOps/internal/task"
)

type TaskHandler struct {
	service *task.Service
	db      *gorm.DB
	audit   *audit.Service
}

func NewTaskHandler(service *task.Service, db *gorm.DB) *TaskHandler {
	return &TaskHandler{service: service, db: db}
}

func NewTaskHandlerWithAudit(service *task.Service, db *gorm.DB, auditService *audit.Service) *TaskHandler {
	return &TaskHandler{service: service, db: db, audit: auditService}
}

type TaskContext struct {
	Task           *model.Task                `json:"task"`
	Resource       *ResourceSummary           `json:"resource,omitempty"`
	Navigation     ResourceNavigation         `json:"navigation"`
	Risk           ResourceRisk               `json:"risk"`
	RelatedAudits  []model.AuditLog           `json:"relatedAudits"`
	RelatedAlerts  []model.AlertEvent         `json:"relatedAlerts"`
	Notifications  []model.NotificationRecord `json:"notifications"`
	FailureSummary string                     `json:"failureSummary,omitempty"`
	NextActions    []ResourceActionHint       `json:"nextActions"`
}

func (h *TaskHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/tasks", rbac.RequirePermission(rbacService, "tasks.view"), h.List)
	r.POST("/tasks", rbac.RequirePermission(rbacService, "tasks.create"), h.Create)
	r.GET("/tasks/:id", rbac.RequirePermission(rbacService, "tasks.view"), h.Get)
	r.GET("/tasks/:id/context", rbac.RequirePermission(rbacService, "tasks.view"), h.Context)
	r.POST("/tasks/:id/steps", rbac.RequirePermission(rbacService, "tasks.dispatch"), h.AddStep)
	r.POST("/tasks/:id/logs", rbac.RequirePermission(rbacService, "tasks.dispatch"), h.AddLog)
	r.POST("/tasks/:id/cancel", rbac.RequirePermission(rbacService, "tasks.cancel"), h.Cancel)
	r.POST("/tasks/:id/retry", rbac.RequirePermission(rbacService, "tasks.retry"), h.Retry)
	r.GET("/task-dispatches", rbac.RequirePermission(rbacService, "tasks.view"), h.ListDispatches)
	r.GET("/task-dispatches/:id", rbac.RequirePermission(rbacService, "tasks.view"), h.GetDispatch)
	r.POST("/task-dispatches/:id/cancel", rbac.RequirePermission(rbacService, "tasks.cancel"), h.CancelDispatch)
	r.POST("/task-dispatches/:id/retry", rbac.RequirePermission(rbacService, "tasks.retry"), h.RetryDispatch)
}

func (h *TaskHandler) List(c *gin.Context) {
	limit, offset := Pagination(c)
	targetType := firstQuery(c, "targetType", "resourceType")
	targetID := firstQuery(c, "targetId", "resourceId")
	items, total, err := h.service.ListWithFilter(c.Request.Context(), task.ListFilter{
		Status:     c.Query("status"),
		TargetType: targetType,
		TargetID:   targetID,
	}, limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func firstQuery(c *gin.Context, names ...string) string {
	for _, name := range names {
		if value := c.Query(name); value != "" {
			return value
		}
	}
	return ""
}

func (h *TaskHandler) Create(c *gin.Context) {
	var req task.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.CreatedBy = OperatorID(c)
	item, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	Created(c, item)
}

func (h *TaskHandler) Get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *TaskHandler) Context(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	result := TaskContext{
		Task:           item,
		FailureSummary: firstNonEmpty(item.Error, failedStepError(item), lastErrorLog(item), item.Result),
	}
	if item.TargetType != "" && item.TargetID != "" {
		resourceType := normalizeResourceType(item.TargetType)
		result.Navigation = resourceNavigation(resourceType, item.TargetID)
		summary, err := resourceSummaryByDB(c.Request.Context(), h.db, resourceType, item.TargetID)
		if err == nil {
			result.Resource = summary
			result.NextActions = resourceActionHints(resourceType, summary.Status, item.TargetID)
		}
		risk, err := resourceRiskByDB(c.Request.Context(), h.db, resourceType, item.TargetID)
		if err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		result.Risk = risk
		if err := h.db.WithContext(c.Request.Context()).Where("resource_type = ? AND resource_id = ?", resourceType, item.TargetID).
			Order("created_at desc").
			Limit(10).
			Find(&result.RelatedAudits).Error; err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		if err := h.db.WithContext(c.Request.Context()).Where("(task_id = ?) OR (resource_type = ? AND resource_id = ?)", item.ID, resourceType, item.TargetID).
			Order(alertOrderExpr()).
			Limit(10).
			Find(&result.RelatedAlerts).Error; err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		result.Navigation = ResourceNavigation{
			TasksPath: "/tasks",
		}
	}
	if err := h.db.WithContext(c.Request.Context()).Where("event_id IN (?)",
		h.db.Model(&model.AlertEvent{}).Select("id").Where("task_id = ?", item.ID),
	).Order("created_at desc").Limit(10).Find(&result.Notifications).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, result)
}

func (h *TaskHandler) AddStep(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		SortOrder int    `json:"sortOrder"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.service.AddStep(c.Request.Context(), c.Param("id"), req.Name, req.SortOrder)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	Created(c, item)
}

func (h *TaskHandler) AddLog(c *gin.Context) {
	var req struct {
		StepID  string             `json:"stepId"`
		Level   model.TaskLogLevel `json:"level"`
		Message string             `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Level == "" {
		req.Level = model.TaskLogLevelInfo
	}
	item, err := h.service.AddLog(c.Request.Context(), c.Param("id"), req.StepID, req.Level, req.Message)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	Created(c, item)
}

func (h *TaskHandler) Cancel(c *gin.Context) {
	if err := h.service.Cancel(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"canceled": true})
}

func (h *TaskHandler) Retry(c *gin.Context) {
	item, err := h.service.Retry(c.Request.Context(), c.Param("id"), OperatorID(c))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	Created(c, item)
}

func (h *TaskHandler) ListDispatches(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListDispatches(c.Request.Context(), task.DispatchFilter{
		Status:         c.Query("status"),
		Source:         c.Query("source"),
		JobID:          c.Query("jobId"),
		TaskID:         c.Query("taskId"),
		ConcurrencyKey: c.Query("concurrencyKey"),
	}, limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *TaskHandler) GetDispatch(c *gin.Context) {
	item, err := h.service.GetDispatch(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *TaskHandler) CancelDispatch(c *gin.Context) {
	item, err := h.service.CancelDispatch(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	h.recordDispatchAudit(c, "task_dispatch.cancel", item.ID, model.AuditResultSuccess, "")
	OK(c, item)
}

func (h *TaskHandler) RetryDispatch(c *gin.Context) {
	item, err := h.service.RetryDispatch(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	h.recordDispatchAudit(c, "task_dispatch.retry", item.ID, model.AuditResultSuccess, "")
	OK(c, item)
}

func (h *TaskHandler) recordDispatchAudit(c *gin.Context, action, resourceID string, result model.AuditResult, message string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{
		Action:       action,
		ResourceType: "task_dispatch",
		ResourceID:   resourceID,
		Result:       result,
		Message:      message,
	})
}

func failedStepError(item *model.Task) string {
	for _, step := range item.Steps {
		if step.Status == model.TaskStatusFailed {
			return firstNonEmpty(step.Error, step.Result, step.Name)
		}
	}
	return ""
}

func lastErrorLog(item *model.Task) string {
	for i := len(item.Logs) - 1; i >= 0; i-- {
		if item.Logs[i].Level == model.TaskLogLevelError {
			return item.Logs[i].Message
		}
	}
	return ""
}
