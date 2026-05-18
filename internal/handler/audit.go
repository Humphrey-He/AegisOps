package handler

import (
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuditHandler struct {
	db *gorm.DB
}

func NewAuditHandler(db *gorm.DB) *AuditHandler {
	return &AuditHandler{db: db}
}

func (h *AuditHandler) List(c *gin.Context) {
	page, pageSize := pagination(c)
	var total int64
	var items []model.AuditLog
	query := h.db.WithContext(c.Request.Context()).Model(&model.AuditLog{})
	if username := c.Query("username"); username != "" {
		query = query.Where("username = ?", username)
	}
	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}
	if resourceType := c.Query("resourceType"); resourceType != "" {
		query = query.Where("resource_type = ?", resourceType)
	}
	if resourceID := c.Query("resourceId"); resourceID != "" {
		query = query.Where("resource_id = ?", resourceID)
	}
	if result := c.Query("result"); result != "" {
		query = query.Where("result = ?", result)
	}
	if err := query.Count(&total).Error; err != nil {
		writeError(c, err)
		return
	}
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("id desc").Find(&items).Error; err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (h *AuditHandler) Get(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var item model.AuditLog
	if err := h.db.WithContext(c.Request.Context()).First(&item, id).Error; err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, item)
}
