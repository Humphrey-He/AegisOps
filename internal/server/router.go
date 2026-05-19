package server

import (
	"context"
	"net/http"

	alertsvc "github.com/Humphrey-He/AegisOps/internal/alert"
	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/auth"
	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/demo"
	dockersvc "github.com/Humphrey-He/AegisOps/internal/docker"
	envsvc "github.com/Humphrey-He/AegisOps/internal/environment"
	exportsvc "github.com/Humphrey-He/AegisOps/internal/exporter"
	"github.com/Humphrey-He/AegisOps/internal/handler"
	healthsvc "github.com/Humphrey-He/AegisOps/internal/healthcheck"
	hostsvc "github.com/Humphrey-He/AegisOps/internal/host"
	"github.com/Humphrey-He/AegisOps/internal/middleware"
	nginxsvc "github.com/Humphrey-He/AegisOps/internal/nginx"
	notificationsvc "github.com/Humphrey-He/AegisOps/internal/notification"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
	registrysvc "github.com/Humphrey-He/AegisOps/internal/registry"
	schedulersvc "github.com/Humphrey-He/AegisOps/internal/scheduler"
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
	if err := demo.Seed(context.Background(), database, cfg.App.Env); err != nil {
		log.Fatal("seed demo data", zap.Error(err))
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
	hostService.SetTaskService(taskService)
	dockerService := dockersvc.NewService(database, secretService)
	dockerService.SetTaskService(taskService)
	environmentService := envsvc.NewService(database)
	notificationService := notificationsvc.NewService(database, secretService)
	notificationService.SetOptions(notificationsvc.Options{TemplateVersion: cfg.Notification.TemplateVersion, PublicBaseURL: cfg.Notification.PublicBaseURL})
	alertService := alertsvc.NewService(database, notificationService)
	healthCheckService := healthsvc.NewService(database, alertService)
	hostService.SetHealthCheckService(healthCheckService)
	terminalService := terminalsvc.NewService(database, secretService)
	registryService := registrysvc.NewService(database, secretService)
	registryService.SetTaskService(taskService)
	nginxService := nginxsvc.NewService(database, secretService, taskService)
	nginxService.SetAlertService(alertService)
	exportService := exportsvc.NewService(database, exportsvc.Options{
		DBDSN:   cfg.Database.DSN,
		AppName: cfg.App.Name,
		AppEnv:  cfg.App.Env,
	})
	schedulerService := schedulersvc.NewService(database)
	var releaseExecutor servicesvc.ReleaseExecutor = servicesvc.NewDockerReleaseExecutor(dockerService)
	if cfg.App.Env == "test" {
		releaseExecutor = servicesvc.NoopReleaseExecutor{}
	}
	serviceService := servicesvc.NewService(database, taskService, releaseExecutor)
	serviceService.SetHealthCheckService(healthCheckService)

	authHandler := handler.NewAuthHandler(authService, auditService)
	userHandler := handler.NewUserHandler(database, authService, auditService)
	roleHandler := handler.NewRoleHandler(database, auditService)
	auditHandler := handler.NewAuditHandler(database)
	dashboardHandler := handler.NewDashboardHandler(database)
	secretHandler := handler.NewSecretHandler(secretService, auditService)
	hostHandler := handler.NewHostHandler(hostService, auditService)
	taskHandler := handler.NewTaskHandler(taskService, database)
	dockerHandler := handler.NewDockerHandler(dockerService, auditService)
	environmentHandler := handler.NewEnvironmentHandler(environmentService, auditService)
	terminalHandler := handler.NewTerminalHandler(terminalService)
	registryHandler := handler.NewRegistryHandler(registryService, auditService)
	serviceHandler := handler.NewServiceHandler(serviceService, auditService)
	nginxHandler := handler.NewNginxHandler(nginxService, auditService)
	notificationHandler := handler.NewNotificationHandler(notificationService, auditService)
	alertHandler := handler.NewAlertHandler(alertService, auditService, database)
	healthCheckHandler := handler.NewHealthCheckHandler(healthCheckService)
	exportHandler := handler.NewExportHandler(exportService, auditService)
	backupHandler := handler.NewBackupHandler(exportService, auditService)
	schedulerHandler := handler.NewSchedulerHandler(schedulerService, auditService)

	handler.RegisterAuthRoutes(api, authHandler, authMiddleware)
	handler.RegisterUserRoleAuditRoutes(api, authMiddleware, rbacService, userHandler, roleHandler, auditHandler)
	protected := api.Group("", authMiddleware)
	protected.GET("/dashboard/summary", rbac.RequirePermission(rbacService, "dashboard.view"), dashboardHandler.Summary)
	protected.GET("/resources/context", rbac.RequirePermission(rbacService, "dashboard.view"), dashboardHandler.ResourceContext)
	protected.GET("/resources/search", rbac.RequirePermission(rbacService, "dashboard.view"), dashboardHandler.ResourceSearch)
	secretHandler.RegisterRoutes(protected, rbacService)
	hostHandler.RegisterRoutes(protected, rbacService)
	taskHandler.RegisterRoutes(protected, rbacService)
	dockerHandler.RegisterRoutes(protected, rbacService)
	environmentHandler.RegisterRoutes(protected, rbacService)
	terminalHandler.RegisterRoutes(protected, rbacService)
	registryHandler.RegisterRoutes(protected, rbacService)
	serviceHandler.RegisterRoutes(protected, rbacService)
	nginxHandler.RegisterRoutes(protected, rbacService)
	notificationHandler.RegisterRoutes(protected, rbacService)
	alertHandler.RegisterRoutes(protected, rbacService)
	healthCheckHandler.RegisterRoutes(protected, rbacService)
	exportHandler.RegisterRoutes(protected, rbacService)
	backupHandler.RegisterRoutes(protected, rbacService)
	schedulerHandler.RegisterRoutes(protected, rbacService)

	return r
}
