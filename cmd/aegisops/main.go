package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	alertsvc "github.com/Humphrey-He/AegisOps/internal/alert"
	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	dockersvc "github.com/Humphrey-He/AegisOps/internal/docker"
	healthsvc "github.com/Humphrey-He/AegisOps/internal/healthcheck"
	hostsvc "github.com/Humphrey-He/AegisOps/internal/host"
	"github.com/Humphrey-He/AegisOps/internal/logger"
	"github.com/Humphrey-He/AegisOps/internal/metrics"
	"github.com/Humphrey-He/AegisOps/internal/model"
	nginxsvc "github.com/Humphrey-He/AegisOps/internal/nginx"
	notificationsvc "github.com/Humphrey-He/AegisOps/internal/notification"
	registrysvc "github.com/Humphrey-He/AegisOps/internal/registry"
	schedulersvc "github.com/Humphrey-He/AegisOps/internal/scheduler"
	secretsvc "github.com/Humphrey-He/AegisOps/internal/secret"
	servicesvc "github.com/Humphrey-He/AegisOps/internal/service"
	"github.com/Humphrey-He/AegisOps/internal/server"
	tasksvc "github.com/Humphrey-He/AegisOps/internal/task"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log, err := logger.New(cfg.App.Env)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = log.Sync()
	}()

	database, err := db.Open(cfg.Database)
	if err != nil {
		log.Fatal("open database", zap.Error(err))
	}
	if err := db.AutoMigrate(database); err != nil {
		log.Fatal("auto migrate database", zap.Error(err))
	}

	router := server.NewRouter(cfg, database, log)
	schedulerService := schedulersvc.NewService(database)
	secretService, err := secretsvc.NewService(database, cfg.Security.SecretKey)
	if err != nil {
		log.Fatal("initialize secret service", zap.Error(err))
	}
	hostService := hostsvc.NewService(database, secretService)
	dockerService := dockersvc.NewService(database, secretService)
	registryService := registrysvc.NewService(database, secretService)
	taskService := tasksvc.NewService(database)
	notificationService := notificationsvc.NewService(database, secretService)
	notificationService.SetOptions(notificationsvc.Options{TemplateVersion: cfg.Notification.TemplateVersion, PublicBaseURL: cfg.Notification.PublicBaseURL})
	alertService := alertsvc.NewService(database, notificationService)
	healthCheckService := healthsvc.NewService(database, alertService)
	hostService.SetHealthCheckService(healthCheckService)
	nginxService := nginxsvc.NewService(database, secretService, taskService)
	nginxService.SetAlertService(alertService)
	releaseExecutor := servicesvc.NewDockerReleaseExecutor(dockerService)
	serviceService := servicesvc.NewService(database, taskService, releaseExecutor)
	serviceService.SetHealthCheckService(healthCheckService)
	serviceService.SetSecretService(secretService)
	dispatchWorker := tasksvc.NewWorker(taskService)
	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	defer stopScheduler()
	go schedulerService.Run(schedulerCtx, schedulersvc.RunOptions{
		Interval: time.Minute,
		Limit:    20,
		OnError: func(err error) {
			log.Warn("scheduled job dispatch failed", zap.Error(err))
		},
		OnDispatch: func(dispatches []model.TaskDispatch) {
			log.Info("scheduled jobs dispatched", zap.Int("count", len(dispatches)))
		},
	})
	go dispatchWorker.Run(schedulerCtx, tasksvc.WorkerOptions{
		Interval: time.Minute,
		Limit:    20,
		Owner:    "aegisops-api",
		Executor: tasksvc.NewDispatchExecutor(tasksvc.DispatchExecutorOptions{
			Registry: registryService,
			Host:     hostService,
			Docker:   dockerService,
			Service:  serviceService,
			Nginx:    nginxService,
		}),
		OnError: func(err error) {
			log.Warn("task dispatch worker failed", zap.Error(err))
		},
		OnComplete: func(dispatch model.TaskDispatch) {
			log.Info("task dispatch processed", zap.String("dispatchID", dispatch.ID), zap.String("status", string(dispatch.Status)))
		},
	})
	httpServer := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("aegisops api listening", zap.String("addr", cfg.HTTP.Addr))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("http server failed", zap.Error(err))
		}
	}()

	// Prometheus 指标独立端口，无鉴权，靠网络层隔离；enabled=false 可整体关闭。
	var metricsServer *http.Server
	if cfg.Metrics.Enabled {
		metricsServer = &http.Server{
			Addr:              cfg.Metrics.Addr,
			Handler:           metrics.Handler(),
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			log.Info("aegisops metrics listening", zap.String("addr", cfg.Metrics.Addr))
			if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatal("metrics server failed", zap.Error(err))
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	stopScheduler()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error("http server shutdown failed", zap.Error(err))
	}
	if metricsServer != nil {
		if err := metricsServer.Shutdown(ctx); err != nil {
			log.Error("metrics server shutdown failed", zap.Error(err))
		}
	}
}
