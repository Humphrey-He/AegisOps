package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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

type ResourceNavigation struct {
	DetailPath string `json:"detailPath"`
	TasksPath  string `json:"tasksPath"`
	AuditsPath string `json:"auditsPath"`
	AlertsPath string `json:"alertsPath"`
}

type ResourceActionHint struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	Permission string `json:"permission,omitempty"`
	Path       string `json:"path,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type ResourceRisk struct {
	Level              string `json:"level"`
	Summary            string `json:"summary"`
	OpenAlertCount     int64  `json:"openAlertCount"`
	FailedTaskCount    int64  `json:"failedTaskCount"`
	HighRiskAuditCount int64  `json:"highRiskAuditCount"`
	LastFailureReason  string `json:"lastFailureReason,omitempty"`
}

type ResourceContext struct {
	ResourceType  string               `json:"resourceType"`
	ResourceID    string               `json:"resourceId"`
	Summary       *ResourceSummary     `json:"summary,omitempty"`
	Navigation    ResourceNavigation   `json:"navigation"`
	PrimaryAction *ResourceActionHint  `json:"primaryAction,omitempty"`
	Actions       []ResourceActionHint `json:"actions"`
	Risk          ResourceRisk         `json:"risk"`
	RecentTasks   []model.Task         `json:"recentTasks"`
	RecentAudits  []model.AuditLog     `json:"recentAudits"`
	RecentAlerts  []model.AlertEvent   `json:"recentAlerts"`
}

type ResourceSearchItem struct {
	ResourceType string    `json:"resourceType"`
	ResourceID   string    `json:"resourceId"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	Subtitle     string    `json:"subtitle,omitempty"`
	Path         string    `json:"path"`
	UpdatedAt    time.Time `json:"updatedAt"`
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
		Navigation:   resourceNavigation(resourceType, resourceID),
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
	risk, err := h.resourceRisk(c.Request.Context(), resourceType, resourceID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	result.Risk = risk
	result.Actions = resourceActionHints(resourceType, summary.Status, resourceID)
	if len(result.Actions) > 0 {
		result.PrimaryAction = &result.Actions[0]
	}
	OK(c, result)
}

func (h *DashboardHandler) ResourceSearch(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	limit, _ := Pagination(c)
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	items := make([]ResourceSearchItem, 0, limit)
	ctx := c.Request.Context()

	if err := h.appendHostSearch(ctx, keyword, limit, &items); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if len(items) < limit {
		if err := h.appendDockerSearch(ctx, keyword, limit, &items); err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if len(items) < limit {
		if err := h.appendNginxSearch(ctx, keyword, limit, &items); err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if len(items) < limit {
		if err := h.appendRegistrySearch(ctx, keyword, limit, &items); err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if len(items) < limit {
		if err := h.appendServiceSearch(ctx, keyword, limit, &items); err != nil {
			Error(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if len(items) > limit {
		items = items[:limit]
	}
	OK(c, PageResult{Items: items, Total: int64(len(items)), Limit: limit, Offset: 0})
}

func (h *DashboardHandler) appendHostSearch(ctx context.Context, keyword string, limit int, items *[]ResourceSearchItem) error {
	var hosts []model.Host
	query := h.db.WithContext(ctx).Model(&model.Host{}).Order("updated_at desc").Limit(remaining(limit, *items))
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR address LIKE ? OR tags LIKE ? OR "+quotedColumn(h.db, "group")+" LIKE ?", like, like, like, like)
	}
	if err := query.Find(&hosts).Error; err != nil {
		return err
	}
	for _, item := range hosts {
		*items = append(*items, ResourceSearchItem{
			ResourceType: "host",
			ResourceID:   item.ID,
			Name:         item.Name,
			Status:       string(item.Status),
			Subtitle:     fmt.Sprintf("%s:%d", item.Address, item.SSHPort),
			Path:         resourceNavigation("host", item.ID).DetailPath,
			UpdatedAt:    item.UpdatedAt,
		})
	}
	return nil
}

func (h *DashboardHandler) appendDockerSearch(ctx context.Context, keyword string, limit int, items *[]ResourceSearchItem) error {
	if remaining(limit, *items) <= 0 {
		return nil
	}
	var nodes []model.DockerNode
	query := h.db.WithContext(ctx).Model(&model.DockerNode{}).Order("updated_at desc").Limit(remaining(limit, *items))
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR endpoint LIKE ? OR description LIKE ?", like, like, like)
	}
	if err := query.Find(&nodes).Error; err != nil {
		return err
	}
	for _, item := range nodes {
		*items = append(*items, ResourceSearchItem{
			ResourceType: "docker_node",
			ResourceID:   item.ID,
			Name:         item.Name,
			Status:       string(item.Status),
			Subtitle:     item.Endpoint,
			Path:         resourceNavigation("docker_node", item.ID).DetailPath,
			UpdatedAt:    item.UpdatedAt,
		})
	}
	return nil
}

func (h *DashboardHandler) appendNginxSearch(ctx context.Context, keyword string, limit int, items *[]ResourceSearchItem) error {
	if remaining(limit, *items) <= 0 {
		return nil
	}
	var nodes []model.NginxNode
	query := h.db.WithContext(ctx).Model(&model.NginxNode{}).Order("updated_at desc").Limit(remaining(limit, *items))
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR config_path LIKE ? OR description LIKE ?", like, like, like)
	}
	if err := query.Find(&nodes).Error; err != nil {
		return err
	}
	for _, item := range nodes {
		*items = append(*items, ResourceSearchItem{
			ResourceType: "nginx_node",
			ResourceID:   item.ID,
			Name:         item.Name,
			Status:       string(item.Status),
			Subtitle:     item.ConfigPath,
			Path:         resourceNavigation("nginx_node", item.ID).DetailPath,
			UpdatedAt:    item.UpdatedAt,
		})
	}
	return nil
}

func (h *DashboardHandler) appendRegistrySearch(ctx context.Context, keyword string, limit int, items *[]ResourceSearchItem) error {
	if remaining(limit, *items) <= 0 {
		return nil
	}
	var registries []model.Registry
	query := h.db.WithContext(ctx).Model(&model.Registry{}).Order("updated_at desc").Limit(remaining(limit, *items))
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR url LIKE ? OR description LIKE ?", like, like, like)
	}
	if err := query.Find(&registries).Error; err != nil {
		return err
	}
	for _, item := range registries {
		*items = append(*items, ResourceSearchItem{
			ResourceType: "registry",
			ResourceID:   item.ID,
			Name:         item.Name,
			Status:       string(item.Status),
			Subtitle:     item.URL,
			Path:         resourceNavigation("registry", item.ID).DetailPath,
			UpdatedAt:    item.UpdatedAt,
		})
	}
	return nil
}

func (h *DashboardHandler) appendServiceSearch(ctx context.Context, keyword string, limit int, items *[]ResourceSearchItem) error {
	if remaining(limit, *items) <= 0 {
		return nil
	}
	var services []model.ServiceDefinition
	query := h.db.WithContext(ctx).Model(&model.ServiceDefinition{}).Order("updated_at desc").Limit(remaining(limit, *items))
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ? OR image LIKE ? OR description LIKE ? OR tags LIKE ?", like, like, like, like, like)
	}
	if err := query.Find(&services).Error; err != nil {
		return err
	}
	for _, item := range services {
		*items = append(*items, ResourceSearchItem{
			ResourceType: "service",
			ResourceID:   item.ID,
			Name:         item.Name,
			Status:       string(item.Status),
			Subtitle:     firstNonEmpty(item.Image, item.Code),
			Path:         resourceNavigation("service", item.ID).DetailPath,
			UpdatedAt:    item.UpdatedAt,
		})
	}
	return nil
}

func remaining(limit int, items []ResourceSearchItem) int {
	if limit <= len(items) {
		return 0
	}
	return limit - len(items)
}

func quotedColumn(database *gorm.DB, name string) string {
	if database != nil && database.Dialector != nil && database.Dialector.Name() == "postgres" {
		return `"` + name + `"`
	}
	return "`" + name + "`"
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
	return resourceSummaryByDB(ctx, h.db, resourceType, resourceID)
}

func resourceSummaryByDB(ctx context.Context, database *gorm.DB, resourceType, resourceID string) (*ResourceSummary, error) {
	switch resourceType {
	case "host":
		var item model.Host
		if err := database.WithContext(ctx).First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return &ResourceSummary{ResourceType: resourceType, ResourceID: item.ID, Name: item.Name, Status: string(item.Status), Endpoint: fmt.Sprintf("%s:%d", item.Address, item.SSHPort), UpdatedAt: item.UpdatedAt, Resource: item}, nil
	case "docker_node":
		var item model.DockerNode
		if err := database.WithContext(ctx).First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return &ResourceSummary{ResourceType: resourceType, ResourceID: item.ID, Name: item.Name, Status: string(item.Status), Endpoint: item.Endpoint, UpdatedAt: item.UpdatedAt, Resource: item}, nil
	case "nginx_node":
		var item model.NginxNode
		if err := database.WithContext(ctx).Preload("Host").First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return &ResourceSummary{ResourceType: resourceType, ResourceID: item.ID, Name: item.Name, Status: string(item.Status), Endpoint: item.ConfigPath, UpdatedAt: item.UpdatedAt, Resource: item}, nil
	case "registry":
		var item model.Registry
		if err := database.WithContext(ctx).First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return &ResourceSummary{ResourceType: resourceType, ResourceID: item.ID, Name: item.Name, Status: string(item.Status), Endpoint: item.URL, UpdatedAt: item.UpdatedAt, Resource: item}, nil
	case "service":
		var item model.ServiceDefinition
		if err := database.WithContext(ctx).First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return &ResourceSummary{ResourceType: resourceType, ResourceID: item.ID, Name: item.Name, Status: string(item.Status), Endpoint: item.Image, UpdatedAt: item.UpdatedAt, Resource: item}, nil
	default:
		return nil, fmt.Errorf("unsupported resource type %q", resourceType)
	}
}

func (h *DashboardHandler) resourceRisk(ctx context.Context, resourceType, resourceID string) (ResourceRisk, error) {
	return resourceRiskByDB(ctx, h.db, resourceType, resourceID)
}

func resourceRiskByDB(ctx context.Context, database *gorm.DB, resourceType, resourceID string) (ResourceRisk, error) {
	var risk ResourceRisk
	if err := database.WithContext(ctx).Model(&model.AlertEvent{}).
		Where("resource_type = ? AND resource_id = ? AND status IN ?", resourceType, resourceID, []model.AlertEventStatus{model.AlertEventStatusOpen, model.AlertEventStatusAcked}).
		Count(&risk.OpenAlertCount).Error; err != nil {
		return risk, err
	}
	if err := database.WithContext(ctx).Model(&model.Task{}).
		Where("target_type = ? AND target_id = ? AND status = ?", resourceType, resourceID, model.TaskStatusFailed).
		Count(&risk.FailedTaskCount).Error; err != nil {
		return risk, err
	}
	if err := database.WithContext(ctx).Model(&model.AuditLog{}).
		Where("resource_type = ? AND resource_id = ? AND (result = ? OR action IN ?)", resourceType, resourceID, model.AuditResultFailure, highRiskAuditActions()).
		Count(&risk.HighRiskAuditCount).Error; err != nil {
		return risk, err
	}
	var failedTask model.Task
	if err := database.WithContext(ctx).Where("target_type = ? AND target_id = ? AND status = ?", resourceType, resourceID, model.TaskStatusFailed).
		Order("updated_at desc").
		First(&failedTask).Error; err != nil && err != gorm.ErrRecordNotFound {
		return risk, err
	} else if err == nil {
		risk.LastFailureReason = firstNonEmpty(failedTask.Error, failedTask.Result, failedTask.Title)
	}
	switch {
	case risk.OpenAlertCount > 0:
		risk.Level = "critical"
		risk.Summary = "存在待处理告警，建议优先确认影响范围。"
	case risk.FailedTaskCount > 0:
		risk.Level = "warning"
		risk.Summary = "存在失败任务，建议查看任务步骤和日志。"
	case risk.HighRiskAuditCount > 0:
		risk.Level = "warning"
		risk.Summary = "存在高风险审计行为，建议复核最近变更。"
	default:
		risk.Level = "normal"
		risk.Summary = "当前资源暂无待处理风险。"
	}
	return risk, nil
}

func resourceNavigation(resourceType, resourceID string) ResourceNavigation {
	detailPath := resourceBasePath(resourceType)
	if resourceID != "" {
		detailPath += "?selected=" + url.QueryEscape(resourceID)
	}
	encodedType := url.QueryEscape(resourceType)
	encodedID := url.QueryEscape(resourceID)
	return ResourceNavigation{
		DetailPath: detailPath,
		TasksPath:  "/tasks?resourceType=" + encodedType + "&resourceId=" + encodedID,
		AuditsPath: "/audits?resourceType=" + encodedType + "&resourceId=" + encodedID,
		AlertsPath: "/alerts/events?resourceType=" + encodedType + "&resourceId=" + encodedID,
	}
}

func resourceBasePath(resourceType string) string {
	switch normalizeResourceType(resourceType) {
	case "host":
		return "/assets/hosts"
	case "docker_node":
		return "/docker/nodes"
	case "nginx_node":
		return "/nginx/nodes"
	case "registry":
		return "/delivery/registries"
	case "service":
		return "/delivery/services"
	default:
		return "/dashboard"
	}
}

func resourceActionHints(resourceType, status, resourceID string) []ResourceActionHint {
	switch normalizeResourceType(resourceType) {
	case "host":
		if status == string(model.HostStatusOnline) {
			return []ResourceActionHint{
				{Key: "open_terminal", Label: "打开终端", Kind: "primary", Permission: "terminal.open", Path: "/terminal/new?hostId=" + url.QueryEscape(resourceID), Reason: "主机在线时优先进入终端排查或操作。"},
				{Key: "test_ssh", Label: "SSH 测试", Kind: "secondary", Permission: "hosts.test", Reason: "重新验证主机连通性。"},
			}
		}
		return []ResourceActionHint{
			{Key: "test_ssh", Label: "SSH 测试", Kind: "primary", Permission: "hosts.test", Reason: "主机未在线时应先确认凭证和网络可达性。"},
		}
	case "docker_node":
		if status == string(model.DockerNodeStatusOnline) {
			return []ResourceActionHint{
				{Key: "view_containers", Label: "查看容器", Kind: "primary", Permission: "docker.view", Path: "/docker/nodes/" + url.PathEscape(resourceID), Reason: "节点在线时优先查看容器运行状态。"},
				{Key: "test_connection", Label: "测试连接", Kind: "secondary", Permission: "docker.test", Reason: "重新验证 Docker API 可达性。"},
			}
		}
		return []ResourceActionHint{
			{Key: "test_connection", Label: "测试连接", Kind: "primary", Permission: "docker.test", Reason: "节点未在线时应先确认 Endpoint、TLS 或网络。"},
		}
	case "nginx_node":
		if status == string(model.NginxNodeStatusOnline) {
			return []ResourceActionHint{
				{Key: "reload", Label: "重载 Nginx", Kind: "primary", Permission: "nginx.reload", Reason: "节点在线时可执行重载或发布配置。"},
				{Key: "test_config", Label: "测试配置", Kind: "secondary", Permission: "nginx.test", Reason: "发布前验证配置语法和命令链路。"},
			}
		}
		return []ResourceActionHint{
			{Key: "test_config", Label: "测试配置", Kind: "primary", Permission: "nginx.test", Reason: "节点异常时应先执行配置测试。"},
		}
	case "registry":
		return []ResourceActionHint{
			{Key: "test_registry", Label: "测试 Registry", Kind: "primary", Permission: "registries.test", Reason: "确认镜像仓库认证和网络可达性。"},
		}
	case "service":
		if status == string(model.ServiceStatusActive) {
			return []ResourceActionHint{
				{Key: "release", Label: "发布服务", Kind: "primary", Permission: "services.release", Reason: "服务启用后可执行发布、升级或回滚。"},
				{Key: "view_tasks", Label: "查看任务", Kind: "secondary", Permission: "tasks.view", Path: "/tasks?resourceType=service&resourceId=" + url.QueryEscape(resourceID), Reason: "跟踪最近发布任务。"},
			}
		}
		return []ResourceActionHint{
			{Key: "edit_service", Label: "完善服务", Kind: "primary", Permission: "services.manage", Reason: "服务未启用时应先补齐镜像、目标节点和配置。"},
		}
	default:
		return nil
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
