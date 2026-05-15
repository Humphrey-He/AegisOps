package handler

import (
	"net/http"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	authsvc "github.com/Humphrey-He/AegisOps/internal/auth"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserHandler struct {
	db    *gorm.DB
	auth  *authsvc.Service
	audit *audit.Service
}

func NewUserHandler(db *gorm.DB, authService *authsvc.Service, auditService *audit.Service) *UserHandler {
	return &UserHandler{db: db, auth: authService, audit: auditService}
}

type userCreateRequest struct {
	Username    string           `json:"username" binding:"required"`
	Password    string           `json:"password" binding:"required"`
	DisplayName string           `json:"displayName"`
	Email       string           `json:"email"`
	Status      model.UserStatus `json:"status"`
	IsAdmin     bool             `json:"isAdmin"`
	RoleIDs     []uint           `json:"roleIds"`
}

type userUpdateRequest struct {
	Password    *string           `json:"password"`
	DisplayName *string           `json:"displayName"`
	Email       *string           `json:"email"`
	Status      *model.UserStatus `json:"status"`
	IsAdmin     *bool             `json:"isAdmin"`
	RoleIDs     []uint            `json:"roleIds"`
}

func (h *UserHandler) List(c *gin.Context) {
	page, pageSize := pagination(c)
	var total int64
	var users []model.User
	query := h.db.WithContext(c.Request.Context()).Model(&model.User{})
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("username LIKE ? OR display_name LIKE ? OR email LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		writeError(c, err)
		return
	}
	if err := query.Preload("Roles").Offset((page - 1) * pageSize).Limit(pageSize).Order("id desc").Find(&users).Error; err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, gin.H{"items": users, "total": total, "page": page, "pageSize": pageSize})
}

func (h *UserHandler) Create(c *gin.Context) {
	var req userCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	hash, err := h.auth.HashPassword(req.Password)
	if err != nil {
		writeError(c, err)
		return
	}
	if req.Status == "" {
		req.Status = model.UserStatusActive
	}
	user := model.User{
		Username:     req.Username,
		PasswordHash: hash,
		DisplayName:  req.DisplayName,
		Email:        req.Email,
		Status:       req.Status,
		IsAdmin:      req.IsAdmin,
	}
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return replaceUserRoles(tx, user.ID, req.RoleIDs)
	})
	if err != nil {
		writeError(c, err)
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "user.create", ResourceType: "user", ResourceID: uintString(user.ID), Result: model.AuditResultSuccess})
	response.Created(c, user)
}

func (h *UserHandler) Get(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var user model.User
	if err := h.db.WithContext(c.Request.Context()).Preload("Roles").First(&user, id).Error; err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, user)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req userUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.First(&user, id).Error; err != nil {
			return err
		}
		updates := map[string]any{}
		if req.DisplayName != nil {
			updates["display_name"] = *req.DisplayName
		}
		if req.Email != nil {
			updates["email"] = *req.Email
		}
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		if req.IsAdmin != nil {
			updates["is_admin"] = *req.IsAdmin
		}
		if req.Password != nil && *req.Password != "" {
			hash, err := h.auth.HashPassword(*req.Password)
			if err != nil {
				return err
			}
			updates["password_hash"] = hash
		}
		if len(updates) > 0 {
			if err := tx.Model(&user).Updates(updates).Error; err != nil {
				return err
			}
		}
		if req.RoleIDs != nil {
			if err := syncUserRoles(tx, id, req.RoleIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		writeError(c, err)
		return
	}
	var user model.User
	if err := h.db.WithContext(c.Request.Context()).Preload("Roles").First(&user, id).Error; err != nil {
		writeError(c, err)
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "user.update", ResourceType: "user", ResourceID: uintString(id), Result: model.AuditResultSuccess})
	response.OK(c, user)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Delete(&model.User{}, id).Error; err != nil {
		writeError(c, err)
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "user.delete", ResourceType: "user", ResourceID: uintString(id), Result: model.AuditResultSuccess})
	response.NoContent(c)
}

func replaceUserRoles(tx *gorm.DB, userID uint, roleIDs []uint) error {
	return syncUserRoles(tx, userID, roleIDs)
}

func syncUserRoles(tx *gorm.DB, userID uint, roleIDs []uint) error {
	var current []model.UserRole
	if err := tx.Where("user_id = ?", userID).Find(&current).Error; err != nil {
		return err
	}
	currentSet := make(map[uint]struct{}, len(current))
	nextSet := make(map[uint]struct{}, len(roleIDs))
	for _, item := range current {
		currentSet[item.RoleID] = struct{}{}
	}
	for _, roleID := range roleIDs {
		nextSet[roleID] = struct{}{}
	}
	var removeIDs []uint
	for roleID := range currentSet {
		if _, ok := nextSet[roleID]; !ok {
			removeIDs = append(removeIDs, roleID)
		}
	}
	if len(removeIDs) > 0 {
		if err := tx.Where("user_id = ? AND role_id IN ?", userID, removeIDs).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
	}
	for roleID := range nextSet {
		if _, ok := currentSet[roleID]; ok {
			continue
		}
		if err := tx.Create(&model.UserRole{UserID: userID, RoleID: roleID}).Error; err != nil {
			return err
		}
	}
	return nil
}
