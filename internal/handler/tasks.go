package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
	"github.com/Humphrey-He/AegisOps/internal/task"
)

type TaskHandler struct {
	service *task.Service
}

func NewTaskHandler(service *task.Service) *TaskHandler {
	return &TaskHandler{service: service}
}

func (h *TaskHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/tasks", rbac.RequirePermission(rbacService, "tasks.view"), h.List)
	r.POST("/tasks", rbac.RequirePermission(rbacService, "tasks.create"), h.Create)
	r.GET("/tasks/:id", rbac.RequirePermission(rbacService, "tasks.view"), h.Get)
	r.POST("/tasks/:id/steps", rbac.RequirePermission(rbacService, "tasks.dispatch"), h.AddStep)
	r.POST("/tasks/:id/logs", rbac.RequirePermission(rbacService, "tasks.dispatch"), h.AddLog)
	r.POST("/tasks/:id/cancel", rbac.RequirePermission(rbacService, "tasks.cancel"), h.Cancel)
	r.POST("/tasks/:id/retry", rbac.RequirePermission(rbacService, "tasks.retry"), h.Retry)
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
