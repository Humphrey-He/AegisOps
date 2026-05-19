package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	dockersvc "github.com/Humphrey-He/AegisOps/internal/docker"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
)

type ContainerItem struct {
	ID           string   `json:"id"`
	NodeID       string   `json:"nodeId"`
	Name         string   `json:"name"`
	Image        string   `json:"image"`
	Status       string   `json:"status"`
	Ports        []string `json:"ports"`
	RestartCount int      `json:"restartCount"`
	CreatedAt    string   `json:"createdAt"`
}

type DockerHandler struct {
	service *dockersvc.Service
	audit   *audit.Service
}

func NewDockerHandler(service *dockersvc.Service, auditService *audit.Service) *DockerHandler {
	return &DockerHandler{service: service, audit: auditService}
}

func (h *DockerHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/docker/nodes", rbac.RequirePermission(rbacService, "docker.view"), h.ListNodes)
	r.POST("/docker/nodes", rbac.RequirePermission(rbacService, "docker.manage"), h.CreateNode)
	r.GET("/docker/nodes/:id", rbac.RequirePermission(rbacService, "docker.view"), h.GetNode)
	r.PATCH("/docker/nodes/:id", rbac.RequirePermission(rbacService, "docker.manage"), h.UpdateNode)
	r.DELETE("/docker/nodes/:id", rbac.RequirePermission(rbacService, "docker.manage"), h.DeleteNode)
	r.POST("/docker/nodes/:id/test", rbac.RequirePermission(rbacService, "docker.test"), h.TestConnection)
	r.GET("/docker/nodes/:id/containers", rbac.RequirePermission(rbacService, "docker.view"), h.ListContainers)
	r.GET("/docker/nodes/:id/containers/:containerId/logs", rbac.RequirePermission(rbacService, "docker.view"), h.ContainerLogs)
	r.POST("/docker/nodes/:id/containers/:containerId/start", rbac.RequirePermission(rbacService, "docker.operate"), h.StartContainer)
	r.POST("/docker/nodes/:id/containers/:containerId/stop", rbac.RequirePermission(rbacService, "docker.operate"), h.StopContainer)
	r.POST("/docker/nodes/:id/containers/:containerId/restart", rbac.RequirePermission(rbacService, "docker.operate"), h.RestartContainer)
}

func (h *DockerHandler) ListNodes(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListNodes(c.Request.Context(), c.Query("keyword"), c.Query("environment"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *DockerHandler) CreateNode(c *gin.Context) {
	var req dockersvc.CreateNodeRequest
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "docker_node.create", ResourceType: "docker_node", ResourceID: item.ID, Result: model.AuditResultSuccess})
	Created(c, item)
}

func (h *DockerHandler) GetNode(c *gin.Context) {
	item, err := h.service.GetNode(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *DockerHandler) UpdateNode(c *gin.Context) {
	var req dockersvc.UpdateNodeRequest
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
	_ = h.audit.RecordGin(c, audit.Entry{Action: "docker_node.update", ResourceType: "docker_node", ResourceID: item.ID, Result: model.AuditResultSuccess})
	OK(c, item)
}

func (h *DockerHandler) DeleteNode(c *gin.Context) {
	if err := h.service.DeleteNode(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "docker_node.delete", ResourceType: "docker_node", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"deleted": true})
}

func (h *DockerHandler) TestConnection(c *gin.Context) {
	taskID, err := h.service.TestConnectionTask(c.Request.Context(), c.Param("id"), OperatorID(c))
	if err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "docker_node.test", ResourceType: "docker_node", ResourceID: c.Param("id"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "docker_node.test", ResourceType: "docker_node", ResourceID: c.Param("id"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"connected": true, "taskId": taskID})
}

func (h *DockerHandler) ListContainers(c *gin.Context) {
	items, err := h.service.ListContainers(c.Request.Context(), c.Param("id"), c.Query("all") == "true")
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	result := make([]ContainerItem, 0, len(items))
	for _, item := range items {
		name := item.ID
		if len(item.Names) > 0 {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		result = append(result, ContainerItem{
			ID:        item.ID,
			NodeID:    c.Param("id"),
			Name:      name,
			Image:     item.Image,
			Status:    item.State,
			Ports:     formatContainerPorts(item.Ports),
			CreatedAt: unixTimeString(item.Created),
		})
	}
	OK(c, result)
}

func formatContainerPorts(ports []dockertypes.Port) []string {
	result := make([]string, 0, len(ports))
	for _, port := range ports {
		if port.PublicPort > 0 {
			result = append(result, strings.ToUpper(port.Type)+":"+port.IP+":"+formatUint16(port.PublicPort)+"->"+formatUint16(port.PrivatePort))
			continue
		}
		result = append(result, strings.ToUpper(port.Type)+":"+formatUint16(port.PrivatePort))
	}
	return result
}

func formatUint16(value uint16) string {
	return strconv.FormatUint(uint64(value), 10)
}

func unixTimeString(value int64) string {
	if value <= 0 {
		return ""
	}
	return time.Unix(value, 0).UTC().Format(time.RFC3339)
}

func (h *DockerHandler) ContainerLogs(c *gin.Context) {
	logs, err := h.service.ContainerLogs(c.Request.Context(), c.Param("id"), c.Param("containerId"), c.DefaultQuery("tail", "200"))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"logs": logs})
}

func (h *DockerHandler) StartContainer(c *gin.Context) {
	taskID, err := h.service.StartContainerTask(c.Request.Context(), c.Param("id"), c.Param("containerId"), OperatorID(c))
	if err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "container.start", ResourceType: "container", ResourceID: c.Param("containerId"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "container.start", ResourceType: "container", ResourceID: c.Param("containerId"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"started": true, "taskId": taskID})
}

func (h *DockerHandler) StopContainer(c *gin.Context) {
	taskID, err := h.service.StopContainerTask(c.Request.Context(), c.Param("id"), c.Param("containerId"), OperatorID(c))
	if err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "container.stop", ResourceType: "container", ResourceID: c.Param("containerId"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "container.stop", ResourceType: "container", ResourceID: c.Param("containerId"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"stopped": true, "taskId": taskID})
}

func (h *DockerHandler) RestartContainer(c *gin.Context) {
	taskID, err := h.service.RestartContainerTask(c.Request.Context(), c.Param("id"), c.Param("containerId"), OperatorID(c))
	if err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{Action: "container.restart", ResourceType: "container", ResourceID: c.Param("containerId"), Result: model.AuditResultFailure, Message: err.Error()})
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "container.restart", ResourceType: "container", ResourceID: c.Param("containerId"), Result: model.AuditResultSuccess})
	OK(c, gin.H{"restarted": true, "taskId": taskID})
}
