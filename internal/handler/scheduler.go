package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
	schedulersvc "github.com/Humphrey-He/AegisOps/internal/scheduler"
)

type SchedulerHandler struct {
	service *schedulersvc.Service
	audit   *audit.Service
}

func NewSchedulerHandler(service *schedulersvc.Service, auditService *audit.Service) *SchedulerHandler {
	return &SchedulerHandler{service: service, audit: auditService}
}

func (h *SchedulerHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/scheduled-jobs", rbac.RequirePermission(rbacService, "scheduler.view"), h.List)
	r.POST("/scheduled-jobs", rbac.RequirePermission(rbacService, "scheduler.manage"), h.Create)
	r.GET("/scheduled-jobs/:id", rbac.RequirePermission(rbacService, "scheduler.view"), h.Get)
	r.GET("/scheduled-jobs/:id/dispatches", rbac.RequirePermission(rbacService, "scheduler.view"), h.Dispatches)
	r.PATCH("/scheduled-jobs/:id", rbac.RequirePermission(rbacService, "scheduler.manage"), h.Update)
	r.DELETE("/scheduled-jobs/:id", rbac.RequirePermission(rbacService, "scheduler.manage"), h.Delete)
}

func (h *SchedulerHandler) List(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.List(c.Request.Context(), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *SchedulerHandler) Create(c *gin.Context) {
	var req schedulersvc.JobRequest
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "scheduled_job.create", ResourceType: "scheduled_job", ResourceID: item.ID, Result: model.AuditResultSuccess})
	Created(c, item)
}

func (h *SchedulerHandler) Get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *SchedulerHandler) Dispatches(c *gin.Context) {
	if _, err := h.service.Get(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	limit, offset := Pagination(c)
	items, total, err := h.service.ListDispatches(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *SchedulerHandler) Update(c *gin.Context) {
	var req schedulersvc.JobRequest
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "scheduled_job.update", ResourceType: "scheduled_job", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *SchedulerHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "scheduled_job.delete", ResourceType: "scheduled_job", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"deleted": true})
}
