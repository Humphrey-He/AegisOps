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

func RegisterUserRoleAuditRoutes(group *gin.RouterGroup, authMiddleware gin.HandlerFunc, rbacService *rbac.Service, userHandler *UserHandler, roleHandler *RoleHandler, auditHandler *AuditHandler) {
	protected := group.Group("", authMiddleware)
	protected.GET("/users", rbac.RequirePermission(rbacService, "users.view"), userHandler.List)
	protected.POST("/users", rbac.RequirePermission(rbacService, "users.manage"), userHandler.Create)
	protected.GET("/users/:id", rbac.RequirePermission(rbacService, "users.view"), userHandler.Get)
	protected.PATCH("/users/:id", rbac.RequirePermission(rbacService, "users.manage"), userHandler.Update)
	protected.DELETE("/users/:id", rbac.RequirePermission(rbacService, "users.manage"), userHandler.Delete)

	protected.GET("/roles", rbac.RequirePermission(rbacService, "roles.view"), roleHandler.List)
	protected.POST("/roles", rbac.RequirePermission(rbacService, "roles.manage"), roleHandler.Create)
	protected.GET("/roles/:id", rbac.RequirePermission(rbacService, "roles.view"), roleHandler.Get)
	protected.PATCH("/roles/:id", rbac.RequirePermission(rbacService, "roles.manage"), roleHandler.Update)
	protected.DELETE("/roles/:id", rbac.RequirePermission(rbacService, "roles.manage"), roleHandler.Delete)
	protected.GET("/permissions", rbac.RequirePermission(rbacService, "roles.view"), roleHandler.ListPermissions)
	protected.POST("/permissions", rbac.RequirePermission(rbacService, "roles.manage"), roleHandler.CreatePermission)

	protected.GET("/audits", rbac.RequirePermission(rbacService, "audits.view"), auditHandler.List)
	protected.GET("/audits/:id", rbac.RequirePermission(rbacService, "audits.view"), auditHandler.Get)
}
