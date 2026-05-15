package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/notification"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
)

type NotificationHandler struct {
	service *notification.Service
	audit   *audit.Service
}

func NewNotificationHandler(service *notification.Service, auditService *audit.Service) *NotificationHandler {
	return &NotificationHandler{service: service, audit: auditService}
}

func (h *NotificationHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/notifications/channels", rbac.RequirePermission(rbacService, "notifications.view"), h.ListChannels)
	r.POST("/notifications/channels", rbac.RequirePermission(rbacService, "notifications.manage"), h.CreateChannel)
	r.GET("/notifications/channels/:id", rbac.RequirePermission(rbacService, "notifications.view"), h.GetChannel)
	r.PATCH("/notifications/channels/:id", rbac.RequirePermission(rbacService, "notifications.manage"), h.UpdateChannel)
	r.DELETE("/notifications/channels/:id", rbac.RequirePermission(rbacService, "notifications.manage"), h.DeleteChannel)
	r.POST("/notifications/channels/:id/test", rbac.RequirePermission(rbacService, "notifications.test"), h.TestChannel)
	r.GET("/notifications/records", rbac.RequirePermission(rbacService, "notifications.view"), h.Records)
}

func (h *NotificationHandler) ListChannels(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListChannels(c.Request.Context(), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *NotificationHandler) GetChannel(c *gin.Context) {
	item, err := h.service.GetChannel(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *NotificationHandler) CreateChannel(c *gin.Context) {
	var req notification.ChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.CreateChannel(c.Request.Context(), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "notification_channel.create", ResourceType: "notification_channel", ResourceID: item.ID, Result: model.AuditResultSuccess})
	Created(c, item)
}

func (h *NotificationHandler) UpdateChannel(c *gin.Context) {
	var req notification.ChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.UpdateChannel(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "notification_channel.update", ResourceType: "notification_channel", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *NotificationHandler) DeleteChannel(c *gin.Context) {
	if err := h.service.DeleteChannel(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "notification_channel.delete", ResourceType: "notification_channel", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"deleted": true})
}

func (h *NotificationHandler) TestChannel(c *gin.Context) {
	record, err := h.service.TestChannel(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "notification_channel.test", ResourceType: "notification_channel", ResourceID: c.Param("id"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "notification_channel.test", ResourceType: "notification_channel", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, record)
}

func (h *NotificationHandler) Records(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.Records(c.Request.Context(), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}
