package handler

import (
	"github.com/Humphrey-He/AegisOps/internal/auth"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB           *gorm.DB
	AuthService  *auth.Service
	RBACService  *rbac.Service
	AuditService interface {
	}
}

func RegisterAuthRoutes(group *gin.RouterGroup, authHandler *AuthHandler, authMiddleware gin.HandlerFunc) {
	group.POST("/auth/login", authHandler.Login)
	group.POST("/auth/refresh", authHandler.Refresh)

	protected := group.Group("", authMiddleware)
	protected.POST("/auth/logout", authHandler.Logout)
	protected.GET("/auth/me", authHandler.Me)
}

func RegisterUserRoleAuditRoutes(group *gin.RouterGroup, authMiddleware gin.HandlerFunc, userHandler *UserHandler, roleHandler *RoleHandler, auditHandler *AuditHandler) {
	protected := group.Group("", authMiddleware)
	protected.GET("/users", userHandler.List)
	protected.POST("/users", userHandler.Create)
	protected.GET("/users/:id", userHandler.Get)
	protected.PATCH("/users/:id", userHandler.Update)
	protected.DELETE("/users/:id", userHandler.Delete)

	protected.GET("/roles", roleHandler.List)
	protected.POST("/roles", roleHandler.Create)
	protected.GET("/roles/:id", roleHandler.Get)
	protected.PATCH("/roles/:id", roleHandler.Update)
	protected.DELETE("/roles/:id", roleHandler.Delete)
	protected.GET("/permissions", roleHandler.ListPermissions)
	protected.POST("/permissions", roleHandler.CreatePermission)

	protected.GET("/audits", auditHandler.List)
	protected.GET("/audits/:id", auditHandler.Get)
}
