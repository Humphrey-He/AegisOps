package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	db *gorm.DB
}

func NewDashboardHandler(db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

type DashboardSummary struct {
	UserCount              int64              `json:"userCount"`
	HostCount              int64              `json:"hostCount"`
	DockerNodeCount        int64              `json:"dockerNodeCount"`
	ContainerCount         int64              `json:"containerCount"`
	UnhealthyResourceCount int64              `json:"unhealthyResourceCount"`
	OpenAlertCount         int64              `json:"openAlertCount"`
	FailedTaskCount        int64              `json:"failedTaskCount"`
	HighRiskAuditCount     int64              `json:"highRiskAuditCount"`
	RecentTasks            []model.Task       `json:"recentTasks"`
	RecentAudits           []model.AuditLog   `json:"recentAudits"`
	OpenAlerts             []model.AlertEvent `json:"openAlerts"`
	FailedTasks            []model.Task       `json:"failedTasks"`
	UnhealthyResources     []ResourceSummary  `json:"unhealthyResources"`
	HighRiskAudits         []model.AuditLog   `json:"highRiskAudits"`
}

type ResourceSummary struct {
	ResourceType string      `json:"resourceType"`
	ResourceID   string      `json:"resourceId"`
	Name         string      `json:"name"`
	Status       string      `json:"status"`
	Endpoint     string      `json:"endpoint,omitempty"`
	UpdatedAt    time.Time   `json:"updatedAt"`
	Resource     interface{} `json:"resource,omitempty"`
}

type ResourceContext struct {
	ResourceType string             `json:"resourceType"`
	ResourceID   string             `json:"resourceId"`
	Summary      *ResourceSummary   `json:"summary,omitempty"`
	RecentTasks  []model.Task       `json:"recentTasks"`
	RecentAudits []model.AuditLog   `json:"recentAudits"`
	RecentAlerts []model.AlertEvent `json:"recentAlerts"`
}

func (h *DashboardHandler) Summary(c *gin.Context) {
	ctx := c.Request.Context()
	var summary DashboardSummary

	if err := h.db.WithContext(ctx).Model(&model.User{}).Count(&summary.UserCount).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(ctx).Model(&model.Host{}).Count(&summary.HostCount).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(ctx).Model(&model.DockerNode{}).Count(&summary.DockerNodeCount).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(ctx).Model(&model.Host{}).
		Where("status = ?", model.HostStatusOffline).
		Count(&summary.UnhealthyResourceCount).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	var offlineDockerNodes int64
	if err := h.db.WithContext(ctx).Model(&model.DockerNode{}).
		Where("status = ?", model.DockerNodeStatusOffline).
		Count(&offlineDockerNodes).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	summary.UnhealthyResourceCount += offlineDockerNodes
	var offlineNginxNodes int64
	if err := h.db.WithContext(ctx).Model(&model.NginxNode{}).
		Where("status = ?", model.NginxNodeStatusOffline).
		Count(&offlineNginxNodes).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	summary.UnhealthyResourceCount += offlineNginxNodes
	var offlineRegistries int64
	if err := h.db.WithContext(ctx).Model(&model.Registry{}).
		Where("status = ?", model.RegistryStatusOffline).
		Count(&offlineRegistries).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	summary.UnhealthyResourceCount += offlineRegistries

	if err := h.db.WithContext(ctx).Model(&model.AlertEvent{}).
		Where("status IN ?", []model.AlertEventStatus{model.AlertEventStatusOpen, model.AlertEventStatusAcked}).
		Count(&summary.OpenAlertCount).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(ctx).Model(&model.Task{}).
		Where("status = ?", model.TaskStatusFailed).
		Count(&summary.FailedTaskCount).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(ctx).Model(&model.AuditLog{}).
		Where("result = ? OR action IN ?", model.AuditResultFailure, highRiskAuditActions()).
		Count(&summary.HighRiskAuditCount).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.db.WithContext(ctx).Order("created_at desc").Limit(5).Find(&summary.RecentTasks).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(ctx).Order("id desc").Limit(5).Find(&summary.RecentAudits).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(ctx).Where("status IN ?", []model.AlertEventStatus{model.AlertEventStatusOpen, model.AlertEventStatusAcked}).
		Order(alertOrderExpr()).
		Limit(5).
		Find(&summary.OpenAlerts).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(ctx).Where("status = ?", model.TaskStatusFailed).
		Order("updated_at desc").
		Limit(5).
		Find(&summary.FailedTasks).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.loadUnhealthyResources(ctx, &summary.UnhealthyResources); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(ctx).Where("result = ? OR action IN ?", model.AuditResultFailure, highRiskAuditActions()).
		Order("created_at desc").
		Limit(5).
		Find(&summary.HighRiskAudits).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	OK(c, summary)
}

func (h *DashboardHandler) ResourceContext(c *gin.Context) {
	resourceType := normalizeResourceType(c.Query("resourceType"))
	resourceID := strings.TrimSpace(c.Query("resourceId"))
	if resourceType == "" || resourceID == "" {
		Error(c, http.StatusBadRequest, "resourceType and resourceId are required")
		return
	}
	summary, err := h.resourceSummary(c.Request.Context(), resourceType, resourceID)
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	limit, _ := Pagination(c)
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	result := ResourceContext{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Summary:      summary,
	}
	if err := h.db.WithContext(c.Request.Context()).Where("target_type = ? AND target_id = ?", resourceType, resourceID).
		Order("created_at desc").
		Limit(limit).
		Find(&result.RecentTasks).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).
		Order("created_at desc").
		Limit(limit).
		Find(&result.RecentAudits).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).
		Order(alertOrderExpr()).
		Limit(limit).
		Find(&result.RecentAlerts).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, result)
}

func (h *DashboardHandler) loadUnhealthyResources(ctx context.Context, items *[]ResourceSummary) error {
	var hosts []model.Host
	if err := h.db.WithContext(ctx).Where("status = ?", model.HostStatusOffline).Order("updated_at desc").Limit(5).Find(&hosts).Error; err != nil {
		return err
	}
	for _, item := range hosts {
		*items = append(*items, ResourceSummary{
			ResourceType: "host",
			ResourceID:   item.ID,
			Name:         item.Name,
			Status:       string(item.Status),
			Endpoint:     fmt.Sprintf("%s:%d", item.Address, item.SSHPort),
			UpdatedAt:    item.UpdatedAt,
			Resource:     item,
		})
	}
	var dockerNodes []model.DockerNode
	if err := h.db.WithContext(ctx).Where("status = ?", model.DockerNodeStatusOffline).Order("updated_at desc").Limit(5).Find(&dockerNodes).Error; err != nil {
		return err
	}
	for _, item := range dockerNodes {
		*items = append(*items, ResourceSummary{
			ResourceType: "docker_node",
			ResourceID:   item.ID,
			Name:         item.Name,
			Status:       string(item.Status),
			Endpoint:     item.Endpoint,
			UpdatedAt:    item.UpdatedAt,
			Resource:     item,
		})
	}
	var nginxNodes []model.NginxNode
	if err := h.db.WithContext(ctx).Where("status = ?", model.NginxNodeStatusOffline).Order("updated_at desc").Limit(5).Find(&nginxNodes).Error; err != nil {
		return err
	}
	for _, item := range nginxNodes {
		*items = append(*items, ResourceSummary{
			ResourceType: "nginx_node",
			ResourceID:   item.ID,
			Name:         item.Name,
			Status:       string(item.Status),
			Endpoint:     item.ConfigPath,
			UpdatedAt:    item.UpdatedAt,
			Resource:     item,
		})
	}
	var registries []model.Registry
	if err := h.db.WithContext(ctx).Where("status = ?", model.RegistryStatusOffline).Order("updated_at desc").Limit(5).Find(&registries).Error; err != nil {
		return err
	}
	for _, item := range registries {
		*items = append(*items, ResourceSummary{
			ResourceType: "registry",
			ResourceID:   item.ID,
			Name:         item.Name,
			Status:       string(item.Status),
			Endpoint:     item.URL,
			UpdatedAt:    item.UpdatedAt,
			Resource:     item,
		})
	}
	if len(*items) > 5 {
		*items = (*items)[:5]
	}
	return nil
}

func (h *DashboardHandler) resourceSummary(ctx context.Context, resourceType, resourceID string) (*ResourceSummary, error) {
	switch resourceType {
	case "host":
		var item model.Host
		if err := h.db.WithContext(ctx).First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return &ResourceSummary{ResourceType: resourceType, ResourceID: item.ID, Name: item.Name, Status: string(item.Status), Endpoint: fmt.Sprintf("%s:%d", item.Address, item.SSHPort), UpdatedAt: item.UpdatedAt, Resource: item}, nil
	case "docker_node":
		var item model.DockerNode
		if err := h.db.WithContext(ctx).First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return &ResourceSummary{ResourceType: resourceType, ResourceID: item.ID, Name: item.Name, Status: string(item.Status), Endpoint: item.Endpoint, UpdatedAt: item.UpdatedAt, Resource: item}, nil
	case "nginx_node":
		var item model.NginxNode
		if err := h.db.WithContext(ctx).Preload("Host").First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return &ResourceSummary{ResourceType: resourceType, ResourceID: item.ID, Name: item.Name, Status: string(item.Status), Endpoint: item.ConfigPath, UpdatedAt: item.UpdatedAt, Resource: item}, nil
	case "registry":
		var item model.Registry
		if err := h.db.WithContext(ctx).First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return &ResourceSummary{ResourceType: resourceType, ResourceID: item.ID, Name: item.Name, Status: string(item.Status), Endpoint: item.URL, UpdatedAt: item.UpdatedAt, Resource: item}, nil
	case "service":
		var item model.ServiceDefinition
		if err := h.db.WithContext(ctx).First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return &ResourceSummary{ResourceType: resourceType, ResourceID: item.ID, Name: item.Name, Status: string(item.Status), Endpoint: item.Image, UpdatedAt: item.UpdatedAt, Resource: item}, nil
	default:
		return nil, fmt.Errorf("unsupported resource type %q", resourceType)
	}
}

func normalizeResourceType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "docker", "docker_node", "dockernode":
		return "docker_node"
	case "nginx", "nginx_node", "nginxnode":
		return "nginx_node"
	case "service_definition":
		return "service"
	default:
		return value
	}
}

func highRiskAuditActions() []string {
	return []string{
		"user.delete",
		"role.delete",
		"secret.delete",
		"secret.read",
		"docker_container.stop",
		"docker_container.restart",
		"service.rollback",
		"nginx_config.rollback",
	}
}

func alertOrderExpr() string {
	return "CASE severity WHEN 'CRITICAL' THEN 0 WHEN 'WARNING' THEN 1 ELSE 2 END, last_triggered_at DESC"
}
