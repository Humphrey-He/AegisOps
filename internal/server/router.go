package server

import (
	"context"
	"net/http"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/auth"
	"github.com/Humphrey-He/AegisOps/internal/config"
	dockersvc "github.com/Humphrey-He/AegisOps/internal/docker"
	"github.com/Humphrey-He/AegisOps/internal/handler"
	hostsvc "github.com/Humphrey-He/AegisOps/internal/host"
	"github.com/Humphrey-He/AegisOps/internal/middleware"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
	registrysvc "github.com/Humphrey-He/AegisOps/internal/registry"
	secretsvc "github.com/Humphrey-He/AegisOps/internal/secret"
	servicesvc "github.com/Humphrey-He/AegisOps/internal/service"
	tasksvc "github.com/Humphrey-He/AegisOps/internal/task"
	terminalsvc "github.com/Humphrey-He/AegisOps/internal/terminal"
	"github.com/Humphrey-He/AegisOps/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func NewRouter(cfg *config.Config, database *gorm.DB, log *zap.Logger) http.Handler {
	if cfg.App.Env == "prod" || cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.CORS())
	r.Use(middleware.Logger(log))

	r.GET("/healthz", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := database.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			response.Error(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "database is not ready")
			return
		}
		response.OK(c, gin.H{"status": "ready"})
	})

	api := r.Group("/api")
	api.GET("/healthz", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})

	auditService := audit.NewService(database)
	authService := auth.NewService(database, auth.Config{
		JWTSecret:         cfg.Security.JWTSecret,
		AccessTokenTTL:    cfg.Security.AccessTokenTTL,
		RefreshTokenTTL:   cfg.Security.RefreshTokenTTL,
		InitialAdminUser:  cfg.Admin.Username,
		InitialAdminPass:  cfg.Admin.Password,
		InitialAdminEmail: cfg.Admin.Email,
	})
	if _, err := authService.InitAdmin(context.Background()); err != nil {
		log.Fatal("initialize admin user", zap.Error(err))
	}
	authMiddleware := auth.Middleware(authService)

	rbacService := rbac.NewService(database)
	_ = rbacService

	secretService, err := secretsvc.NewService(database, cfg.Security.SecretKey)
	if err != nil {
		log.Fatal("initialize secret service", zap.Error(err))
	}
	hostService := hostsvc.NewService(database, secretService)
	taskService := tasksvc.NewService(database)
	dockerService := dockersvc.NewService(database, secretService)
	dockerService.SetTaskService(taskService)
	terminalService := terminalsvc.NewService(database, secretService)
	registryService := registrysvc.NewService(database, secretService)
	var releaseExecutor servicesvc.ReleaseExecutor = servicesvc.NewDockerReleaseExecutor(dockerService)
	if cfg.App.Env == "test" {
		releaseExecutor = servicesvc.NoopReleaseExecutor{}
	}
	serviceService := servicesvc.NewService(database, taskService, releaseExecutor)

	authHandler := handler.NewAuthHandler(authService, auditService)
	userHandler := handler.NewUserHandler(database, authService, auditService)
	roleHandler := handler.NewRoleHandler(database, auditService)
	auditHandler := handler.NewAuditHandler(database)
	dashboardHandler := handler.NewDashboardHandler(database)
	secretHandler := handler.NewSecretHandler(secretService, auditService)
	hostHandler := handler.NewHostHandler(hostService, auditService)
	taskHandler := handler.NewTaskHandler(taskService)
	dockerHandler := handler.NewDockerHandler(dockerService, auditService)
	terminalHandler := handler.NewTerminalHandler(terminalService)
	registryHandler := handler.NewRegistryHandler(registryService, auditService)
	serviceHandler := handler.NewServiceHandler(serviceService, auditService)

	handler.RegisterAuthRoutes(api, authHandler, authMiddleware)
	handler.RegisterUserRoleAuditRoutes(api, authMiddleware, rbacService, userHandler, roleHandler, auditHandler)
	protected := api.Group("", authMiddleware)
	protected.GET("/dashboard/summary", rbac.RequirePermission(rbacService, "dashboard.view"), dashboardHandler.Summary)
	secretHandler.RegisterRoutes(protected, rbacService)
	hostHandler.RegisterRoutes(protected, rbacService)
	taskHandler.RegisterRoutes(protected, rbacService)
	dockerHandler.RegisterRoutes(protected, rbacService)
	terminalHandler.RegisterRoutes(protected, rbacService)
	registryHandler.RegisterRoutes(protected, rbacService)
	serviceHandler.RegisterRoutes(protected, rbacService)

	return r
}
