package handler

import (
	"net/http"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RoleHandler struct {
	db    *gorm.DB
	audit *audit.Service
}

func NewRoleHandler(db *gorm.DB, auditService *audit.Service) *RoleHandler {
	return &RoleHandler{db: db, audit: auditService}
}

type roleRequest struct {
	Name          string `json:"name" binding:"required"`
	Code          string `json:"code" binding:"required"`
	Description   string `json:"description"`
	PermissionIDs []uint `json:"permissionIds"`
}

type permissionRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

func (h *RoleHandler) List(c *gin.Context) {
	page, pageSize := pagination(c)
	var total int64
	var roles []model.Role
	query := h.db.WithContext(c.Request.Context()).Model(&model.Role{})
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("name LIKE ? OR code LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		writeError(c, err)
		return
	}
	if err := query.Preload("Permissions").Offset((page - 1) * pageSize).Limit(pageSize).Order("id desc").Find(&roles).Error; err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, gin.H{"items": roles, "total": total, "page": page, "pageSize": pageSize})
}

func (h *RoleHandler) Create(c *gin.Context) {
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	role := model.Role{Name: req.Name, Code: req.Code, Description: req.Description}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		return replaceRolePermissions(tx, role.ID, req.PermissionIDs)
	})
	if err != nil {
		writeError(c, err)
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "role.create", ResourceType: "role", ResourceID: uintString(role.ID), Result: model.AuditResultSuccess})
	response.Created(c, role)
}

func (h *RoleHandler) Get(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var role model.Role
	if err := h.db.WithContext(c.Request.Context()).Preload("Permissions").First(&role, id).Error; err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, role)
}

func (h *RoleHandler) Update(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var role model.Role
		if err := tx.First(&role, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&role).Updates(map[string]any{
			"name":        req.Name,
			"code":        req.Code,
			"description": req.Description,
		}).Error; err != nil {
			return err
		}
		if req.PermissionIDs != nil {
			if err := syncRolePermissions(tx, id, req.PermissionIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		writeError(c, err)
		return
	}
	var role model.Role
	if err := h.db.WithContext(c.Request.Context()).Preload("Permissions").First(&role, id).Error; err != nil {
		writeError(c, err)
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "role.update", ResourceType: "role", ResourceID: uintString(id), Result: model.AuditResultSuccess})
	response.OK(c, role)
}

func (h *RoleHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&model.Role{}, id).Error; err != nil {
		writeError(c, err)
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "role.delete", ResourceType: "role", ResourceID: uintString(id), Result: model.AuditResultSuccess})
	response.NoContent(c)
}

func (h *RoleHandler) ListPermissions(c *gin.Context) {
	var permissions []model.Permission
	if err := h.db.WithContext(c.Request.Context()).Order("resource asc, action asc, id asc").Find(&permissions).Error; err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, permissions)
}

func (h *RoleHandler) CreatePermission(c *gin.Context) {
	var req permissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	permission := model.Permission{
		Name:        req.Name,
		Code:        req.Code,
		Resource:    req.Resource,
		Action:      req.Action,
		Description: req.Description,
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&permission).Error; err != nil {
		writeError(c, err)
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "permission.create", ResourceType: "permission", ResourceID: uintString(permission.ID), Result: model.AuditResultSuccess})
	response.Created(c, permission)
}

func replaceRolePermissions(tx *gorm.DB, roleID uint, permissionIDs []uint) error {
	return syncRolePermissions(tx, roleID, permissionIDs)
}

func syncRolePermissions(tx *gorm.DB, roleID uint, permissionIDs []uint) error {
	var current []model.RolePermission
	if err := tx.Where("role_id = ?", roleID).Find(&current).Error; err != nil {
		return err
	}
	currentSet := make(map[uint]struct{}, len(current))
	nextSet := make(map[uint]struct{}, len(permissionIDs))
	for _, item := range current {
		currentSet[item.PermissionID] = struct{}{}
	}
	for _, permissionID := range permissionIDs {
		nextSet[permissionID] = struct{}{}
	}
	var removeIDs []uint
	for permissionID := range currentSet {
		if _, ok := nextSet[permissionID]; !ok {
			removeIDs = append(removeIDs, permissionID)
		}
	}
	if len(removeIDs) > 0 {
		if err := tx.Where("role_id = ? AND permission_id IN ?", roleID, removeIDs).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
	}
	for permissionID := range nextSet {
		if _, ok := currentSet[permissionID]; ok {
			continue
		}
		if err := tx.Create(&model.RolePermission{RoleID: roleID, PermissionID: permissionID}).Error; err != nil {
			return err
		}
	}
	return nil
}
