package handler

import (
	"net/http"

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
	UserCount              int64            `json:"userCount"`
	HostCount              int64            `json:"hostCount"`
	DockerNodeCount        int64            `json:"dockerNodeCount"`
	ContainerCount         int64            `json:"containerCount"`
	UnhealthyResourceCount int64            `json:"unhealthyResourceCount"`
	RecentTasks            []model.Task     `json:"recentTasks"`
	RecentAudits           []model.AuditLog `json:"recentAudits"`
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

	if err := h.db.WithContext(ctx).Order("created_at desc").Limit(5).Find(&summary.RecentTasks).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(ctx).Order("id desc").Limit(5).Find(&summary.RecentAudits).Error; err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	OK(c, summary)
}
