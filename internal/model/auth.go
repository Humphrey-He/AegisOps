package model

import "time"

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

type User struct {
	BaseModel
	Username     string     `json:"username" gorm:"size:64;uniqueIndex;not null"`
	PasswordHash string     `json:"-" gorm:"size:255;not null"`
	DisplayName  string     `json:"displayName" gorm:"size:128"`
	Email        string     `json:"email" gorm:"size:255;index"`
	Status       UserStatus `json:"status" gorm:"size:32;not null;default:active"`
	IsAdmin      bool       `json:"isAdmin" gorm:"not null;default:false"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
	Roles        []Role     `json:"roles,omitempty" gorm:"many2many:user_roles;"`
}

type Role struct {
	BaseModel
	Name        string       `json:"name" gorm:"size:64;uniqueIndex;not null"`
	Code        string       `json:"code" gorm:"size:64;uniqueIndex;not null"`
	Description string       `json:"description" gorm:"size:255"`
	Permissions []Permission `json:"permissions,omitempty" gorm:"many2many:role_permissions;"`
}

type Permission struct {
	BaseModel
	Name        string `json:"name" gorm:"size:128;not null"`
	Code        string `json:"code" gorm:"size:128;uniqueIndex;not null"`
	Resource    string `json:"resource" gorm:"size:64;index"`
	Action      string `json:"action" gorm:"size:64;index"`
	Description string `json:"description" gorm:"size:255"`
}

type UserRole struct {
	UserID    uint      `json:"userId" gorm:"primaryKey"`
	RoleID    uint      `json:"roleId" gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
}

type RolePermission struct {
	RoleID       uint      `json:"roleId" gorm:"primaryKey"`
	PermissionID uint      `json:"permissionId" gorm:"primaryKey"`
	CreatedAt    time.Time `json:"createdAt"`
}
