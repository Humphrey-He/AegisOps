package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/model"
	nginxsvc "github.com/Humphrey-He/AegisOps/internal/nginx"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
)

type NginxHandler struct {
	service *nginxsvc.Service
	audit   *audit.Service
}

func NewNginxHandler(service *nginxsvc.Service, auditService *audit.Service) *NginxHandler {
	return &NginxHandler{service: service, audit: auditService}
}

func (h *NginxHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/nginx/nodes", rbac.RequirePermission(rbacService, "nginx.view"), h.ListNodes)
	r.POST("/nginx/nodes", rbac.RequirePermission(rbacService, "nginx.manage"), h.CreateNode)
	r.GET("/nginx/nodes/:id", rbac.RequirePermission(rbacService, "nginx.view"), h.GetNode)
	r.PATCH("/nginx/nodes/:id", rbac.RequirePermission(rbacService, "nginx.manage"), h.UpdateNode)
	r.DELETE("/nginx/nodes/:id", rbac.RequirePermission(rbacService, "nginx.manage"), h.DeleteNode)
	r.POST("/nginx/nodes/:id/test", rbac.RequirePermission(rbacService, "nginx.test"), h.TestNode)
	r.POST("/nginx/nodes/:id/reload", rbac.RequirePermission(rbacService, "nginx.reload"), h.ReloadNode)
	r.GET("/nginx/nodes/:id/configs", rbac.RequirePermission(rbacService, "nginx.view"), h.ListConfigs)
	r.POST("/nginx/nodes/:id/configs", rbac.RequirePermission(rbacService, "nginx.manage"), h.CreateConfig)
	r.GET("/nginx/configs/:configId", rbac.RequirePermission(rbacService, "nginx.view"), h.GetConfig)
	r.POST("/nginx/nodes/:id/publish", rbac.RequirePermission(rbacService, "nginx.publish"), h.PublishConfig)
	r.POST("/nginx/nodes/:id/rollback", rbac.RequirePermission(rbacService, "nginx.rollback"), h.Rollback)
}

func (h *NginxHandler) ListNodes(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListNodes(c.Request.Context(), c.Query("keyword"), c.Query("environment"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *NginxHandler) CreateNode(c *gin.Context) {
	var req nginxsvc.CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.CreateNode(c.Request.Context(), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "nginx_node.create", ResourceType: "nginx_node", ResourceID: item.ID, Result: model.AuditResultSuccess})
	Created(c, item)
}

func (h *NginxHandler) GetNode(c *gin.Context) {
	item, err := h.service.GetNode(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *NginxHandler) UpdateNode(c *gin.Context) {
	var req nginxsvc.UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.UpdateNode(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "nginx_node.update", ResourceType: "nginx_node", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *NginxHandler) DeleteNode(c *gin.Context) {
	if err := h.service.DeleteNode(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "nginx_node.delete", ResourceType: "nginx_node", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"deleted": true})
}

func (h *NginxHandler) TestNode(c *gin.Context) {
	taskID, err := h.service.TestTask(c.Request.Context(), c.Param("id"), OperatorID(c))
	if err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "nginx_node.test", ResourceType: "nginx_node", ResourceID: c.Param("id"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "nginx_node.test", ResourceType: "nginx_node", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"ok": true, "taskId": taskID})
}

func (h *NginxHandler) ReloadNode(c *gin.Context) {
	taskID, err := h.service.ReloadTask(c.Request.Context(), c.Param("id"), OperatorID(c))
	if err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "nginx_node.reload", ResourceType: "nginx_node", ResourceID: c.Param("id"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "nginx_node.reload", ResourceType: "nginx_node", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"ok": true, "taskId": taskID})
}

func (h *NginxHandler) ListConfigs(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListConfigs(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *NginxHandler) CreateConfig(c *gin.Context) {
	var req nginxsvc.CreateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.CreateConfig(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "nginx_config.create", ResourceType: "nginx_config", ResourceID: item.ID, Result: model.AuditResultSuccess})
	Created(c, item)
}

func (h *NginxHandler) GetConfig(c *gin.Context) {
	item, err := h.service.GetConfig(c.Request.Context(), c.Param("configId"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *NginxHandler) PublishConfig(c *gin.Context) {
	var req nginxsvc.PublishConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	taskID, err := h.service.PublishConfigTask(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "nginx_config.publish", ResourceType: "nginx_node", ResourceID: c.Param("id"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "nginx_config.publish", ResourceType: "nginx_node", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"ok": true, "taskId": taskID})
}

func (h *NginxHandler) Rollback(c *gin.Context) {
	var req nginxsvc.RollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	taskID, err := h.service.RollbackTask(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "nginx_config.rollback", ResourceType: "nginx_node", ResourceID: c.Param("id"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "nginx_config.rollback", ResourceType: "nginx_node", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"ok": true, "taskId": taskID})
}
