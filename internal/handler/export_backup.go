package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/exporter"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
)

type ExportHandler struct {
	service *exporter.Service
	audit   *audit.Service
}

type BackupHandler struct {
	service *exporter.Service
	audit   *audit.Service
}

func NewExportHandler(service *exporter.Service, auditService *audit.Service) *ExportHandler {
	return &ExportHandler{service: service, audit: auditService}
}

func NewBackupHandler(service *exporter.Service, auditService *audit.Service) *BackupHandler {
	return &BackupHandler{service: service, audit: auditService}
}

func (h *ExportHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.POST("/exports/resources", rbac.RequirePermission(rbacService, "exports.create"), h.CreateResource)
	r.POST("/exports/records", rbac.RequirePermission(rbacService, "exports.create"), h.CreateRecords)
	r.POST("/exports/incidents", rbac.RequirePermission(rbacService, "exports.create"), h.CreateIncident)
	r.GET("/exports", rbac.RequirePermission(rbacService, "exports.view"), h.List)
	r.GET("/exports/:id", rbac.RequirePermission(rbacService, "exports.view"), h.Get)
	r.GET("/exports/:id/download", rbac.RequirePermission(rbacService, "exports.download"), h.Download)
	r.GET("/tasks/:id/export", rbac.RequirePermission(rbacService, "exports.create"), h.ExportTask)
	r.GET("/services/:id/export", rbac.RequirePermission(rbacService, "exports.create"), h.ExportService)
	r.GET("/nginx/configs/:configId/export", rbac.RequirePermission(rbacService, "exports.create"), h.ExportNginxConfig)
}

func (h *BackupHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.POST("/backups", rbac.RequirePermission(rbacService, "backups.create"), h.Create)
	r.GET("/backups", rbac.RequirePermission(rbacService, "backups.view"), h.List)
	r.GET("/backups/:id", rbac.RequirePermission(rbacService, "backups.view"), h.Get)
	r.GET("/backups/:id/manifest", rbac.RequirePermission(rbacService, "backups.view"), h.Manifest)
	r.GET("/backups/:id/download", rbac.RequirePermission(rbacService, "backups.download"), h.Download)
}

func (h *ExportHandler) CreateResource(c *gin.Context) {
	var req exporter.ResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.CreateResourceExport(c.Request.Context(), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "export.resource", ResourceType: "export", ResourceID: item.ID, Result: auditResult(item)})
	Created(c, item)
}

func (h *ExportHandler) CreateRecords(c *gin.Context) {
	var req exporter.RecordsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.CreateRecordsExport(c.Request.Context(), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "export.records", ResourceType: "export", ResourceID: item.ID, Result: auditResult(item)})
	Created(c, item)
}

func (h *ExportHandler) CreateIncident(c *gin.Context) {
	var req exporter.IncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.CreateIncidentExport(c.Request.Context(), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "export.incident", ResourceType: "export", ResourceID: item.ID, Result: auditResult(item)})
	Created(c, item)
}

func (h *ExportHandler) ExportTask(c *gin.Context) {
	masked := true
	item, err := h.service.CreateIncidentExport(c.Request.Context(), exporter.IncidentRequest{TaskID: c.Param("id"), Masked: &masked, OperatorID: OperatorID(c)})
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	Created(c, item)
}

func (h *ExportHandler) ExportService(c *gin.Context) {
	masked := true
	item, err := h.service.CreateResourceExport(c.Request.Context(), exporter.ResourceRequest{ResourceType: "service", ResourceID: c.Param("id"), Masked: &masked, OperatorID: OperatorID(c)})
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	Created(c, item)
}

func (h *ExportHandler) ExportNginxConfig(c *gin.Context) {
	masked := true
	item, err := h.service.CreateResourceExport(c.Request.Context(), exporter.ResourceRequest{ResourceType: "nginx_config", ResourceID: c.Param("configId"), Masked: &masked, OperatorID: OperatorID(c)})
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	Created(c, item)
}

func (h *ExportHandler) List(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListJobs(c.Request.Context(), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *ExportHandler) Get(c *gin.Context) {
	item, err := h.service.GetJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *ExportHandler) Download(c *gin.Context) {
	download, err := h.service.DownloadJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	c.Header("Content-Type", download.ContentType)
	c.FileAttachment(download.FilePath, download.FileName)
}

func (h *BackupHandler) Create(c *gin.Context) {
	var req exporter.BackupRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			Error(c, http.StatusBadRequest, err.Error())
			return
		}
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.CreateBackup(c.Request.Context(), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{Action: "backup.create", ResourceType: "backup", ResourceID: item.ID, Result: backupAuditResult(item)})
	Created(c, item)
}

func (h *BackupHandler) List(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListBackups(c.Request.Context(), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *BackupHandler) Get(c *gin.Context) {
	item, err := h.service.GetBackup(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *BackupHandler) Manifest(c *gin.Context) {
	item, err := h.service.GetBackup(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, gin.H{"manifest": item.ManifestJSON})
}

func (h *BackupHandler) Download(c *gin.Context) {
	download, err := h.service.DownloadBackup(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	c.Header("Content-Type", download.ContentType)
	c.FileAttachment(download.FilePath, download.FileName)
}

func auditResult(item *model.ExportJob) model.AuditResult {
	if item.Status == model.ExportJobStatusFailed {
		return model.AuditResultFailure
	}
	return model.AuditResultSuccess
}

func backupAuditResult(item *model.BackupRecord) model.AuditResult {
	if item.Status == model.ExportJobStatusFailed {
		return model.AuditResultFailure
	}
	return model.AuditResultSuccess
}
