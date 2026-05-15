package exporter

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/redact"
)

const (
	jsonContentType = "application/json"
	zipContentType  = "application/zip"
	csvContentType  = "text/csv; charset=utf-8"
)

type Service struct {
	db        *gorm.DB
	baseDir   string
	backupDir string
	dbDSN     string
	appName   string
	appEnv    string
}

type Options struct {
	BaseDir   string
	BackupDir string
	DBDSN     string
	AppName   string
	AppEnv    string
}

type ResourceRequest struct {
	ResourceType string `json:"resourceType" binding:"required"`
	ResourceID   string `json:"resourceId"`
	Masked       *bool  `json:"masked"`
	OperatorID   string `json:"-"`
}

type RecordsRequest struct {
	RecordType string `json:"recordType" binding:"required"`
	Format     string `json:"format"`
	Masked     *bool  `json:"masked"`
	OperatorID string `json:"-"`
}

type IncidentRequest struct {
	TaskID     string `json:"taskId"`
	ReleaseID  string `json:"releaseId"`
	EventID    string `json:"eventId"`
	Masked     *bool  `json:"masked"`
	OperatorID string `json:"-"`
}

type BackupRequest struct {
	Masked     *bool  `json:"masked"`
	OperatorID string `json:"-"`
}

type Download struct {
	FilePath    string
	FileName    string
	ContentType string
}

type manifest struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Resource    map[string]any `json:"resource,omitempty"`
	Masked      bool           `json:"masked"`
	AppName     string         `json:"appName"`
	AppEnv      string         `json:"appEnv"`
	GeneratedAt time.Time      `json:"generatedAt"`
	Files       []string       `json:"files"`
	Checksum    string         `json:"checksum,omitempty"`
}

func NewService(db *gorm.DB, opts Options) *Service {
	baseDir := firstNonEmpty(opts.BaseDir, filepath.Join("data", "exports"))
	backupDir := firstNonEmpty(opts.BackupDir, filepath.Join("data", "backups"))
	return &Service{
		db:        db,
		baseDir:   baseDir,
		backupDir: backupDir,
		dbDSN:     opts.DBDSN,
		appName:   firstNonEmpty(opts.AppName, "aegisops"),
		appEnv:    opts.AppEnv,
	}
}

func (s *Service) ListJobs(ctx context.Context, limit, offset int) ([]model.ExportJob, int64, error) {
	var items []model.ExportJob
	var total int64
	query := s.db.WithContext(ctx).Model(&model.ExportJob{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) GetJob(ctx context.Context, id string) (*model.ExportJob, error) {
	var item model.ExportJob
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) CreateResourceExport(ctx context.Context, req ResourceRequest) (*model.ExportJob, error) {
	masked := maskedDefault(req.Masked)
	job := s.newJob(model.ExportJobTypeResource, req.ResourceType, req.ResourceID, masked, req.OperatorID)
	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, err
	}
	payload, err := s.resourcePayload(ctx, req.ResourceType, req.ResourceID, masked)
	if err != nil {
		return s.failJob(ctx, job, err)
	}
	fileName := safeFileName(strings.Join(nonEmpty("resource", req.ResourceType, req.ResourceID, timestamp()), "-") + ".json")
	path := filepath.Join(s.baseDir, fileName)
	content := map[string]any{
		"manifest": s.newManifest(job.ID, "resource", masked, []string{"resource.json"}, map[string]any{
			"type": req.ResourceType,
			"id":   req.ResourceID,
		}),
		"resource": payload,
	}
	if err := writeJSON(path, content); err != nil {
		return s.failJob(ctx, job, err)
	}
	return s.finishJob(ctx, job, path, fileName, jsonContentType)
}

func (s *Service) CreateRecordsExport(ctx context.Context, req RecordsRequest) (*model.ExportJob, error) {
	masked := maskedDefault(req.Masked)
	format := strings.ToLower(firstNonEmpty(req.Format, "json"))
	job := s.newJob(model.ExportJobTypeRecords, req.RecordType, "", masked, req.OperatorID)
	job.FiltersJSON = mustJSON(map[string]any{"recordType": req.RecordType, "format": format})
	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, err
	}
	payload, err := s.recordsPayload(ctx, req.RecordType, masked)
	if err != nil {
		return s.failJob(ctx, job, err)
	}
	ext := ".json"
	contentType := jsonContentType
	var data []byte
	if format == "csv" {
		ext = ".csv"
		contentType = csvContentType
		data, err = recordsCSV(req.RecordType, payload)
	} else {
		data, err = redact.JSON(map[string]any{
			"manifest": s.newManifest(job.ID, "records", masked, []string{"records.json"}, map[string]any{"type": req.RecordType}),
			"records":  payload,
		}, false)
	}
	if err != nil {
		return s.failJob(ctx, job, err)
	}
	fileName := safeFileName(strings.Join(nonEmpty("records", req.RecordType, timestamp()), "-") + ext)
	path := filepath.Join(s.baseDir, fileName)
	if err := writeBytes(path, data); err != nil {
		return s.failJob(ctx, job, err)
	}
	return s.finishJob(ctx, job, path, fileName, contentType)
}

func (s *Service) CreateIncidentExport(ctx context.Context, req IncidentRequest) (*model.ExportJob, error) {
	masked := maskedDefault(req.Masked)
	resourceID := firstNonEmpty(req.TaskID, req.ReleaseID, req.EventID)
	job := s.newJob(model.ExportJobTypeIncident, "incident", resourceID, masked, req.OperatorID)
	job.FiltersJSON = mustJSON(map[string]any{"taskId": req.TaskID, "releaseId": req.ReleaseID, "eventId": req.EventID})
	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, err
	}
	bundle, err := s.incidentPayload(ctx, req, masked)
	if err != nil {
		return s.failJob(ctx, job, err)
	}
	fileName := safeFileName(strings.Join(nonEmpty("incident", resourceID, timestamp()), "-") + ".zip")
	path := filepath.Join(s.baseDir, fileName)
	files := map[string][]byte{}
	files["manifest.json"], _ = redact.JSON(s.newManifest(job.ID, "incident", masked, []string{"summary.md", "task.json", "task-logs.txt", "audits.json", "alerts.json", "notification-records.json", "health-checks.json", "resource-snapshot.json"}, map[string]any{"id": resourceID}), false)
	files["summary.md"] = []byte(incidentSummary(bundle))
	files["task.json"], _ = redact.JSON(bundle["task"], false)
	files["task-logs.txt"] = []byte(taskLogsText(bundle["task"]))
	files["audits.json"], _ = redact.JSON(bundle["audits"], false)
	files["alerts.json"], _ = redact.JSON(bundle["alerts"], false)
	files["notification-records.json"], _ = redact.JSON(bundle["notifications"], false)
	files["health-checks.json"], _ = redact.JSON(bundle["healthChecks"], false)
	files["resource-snapshot.json"], _ = redact.JSON(bundle["resourceSnapshot"], false)
	if err := writeZip(path, files); err != nil {
		return s.failJob(ctx, job, err)
	}
	return s.finishJob(ctx, job, path, fileName, zipContentType)
}

func (s *Service) DownloadJob(ctx context.Context, id string) (*Download, error) {
	job, err := s.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if job.Status != model.ExportJobStatusSuccess || job.FilePath == "" {
		return nil, fmt.Errorf("export job is not ready")
	}
	if !isPathWithin(job.FilePath, s.baseDir) {
		return nil, fmt.Errorf("invalid export path")
	}
	return &Download{FilePath: job.FilePath, FileName: job.FileName, ContentType: job.ContentType}, nil
}

func (s *Service) ListBackups(ctx context.Context, limit, offset int) ([]model.BackupRecord, int64, error) {
	var items []model.BackupRecord
	var total int64
	query := s.db.WithContext(ctx).Model(&model.BackupRecord{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) GetBackup(ctx context.Context, id string) (*model.BackupRecord, error) {
	var item model.BackupRecord
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) CreateBackup(ctx context.Context, req BackupRequest) (*model.BackupRecord, error) {
	masked := maskedDefault(req.Masked)
	record := &model.BackupRecord{
		ID:        uuid.NewString(),
		Type:      model.BackupRecordTypeManual,
		Status:    model.ExportJobStatusRunning,
		Masked:    masked,
		CreatedBy: req.OperatorID,
	}
	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}
	fileName := safeFileName("backup-" + timestamp() + ".zip")
	path := filepath.Join(s.backupDir, fileName)
	manifestValue := s.newManifest(record.ID, "backup", masked, []string{"backup.db", "config-snapshot.json", "manifest.json", "restore-guide.md"}, nil)
	files := map[string][]byte{}
	dbFiles, err := s.databaseFiles()
	if err != nil {
		return s.failBackup(ctx, record, err)
	}
	for name, data := range dbFiles {
		files[name] = data
	}
	files["config-snapshot.json"], _ = redact.JSON(s.configSnapshot(ctx, masked), false)
	files["restore-guide.md"] = []byte(restoreGuide())
	files["manifest.json"], _ = redact.JSON(manifestValue, false)
	if err := writeZip(path, files); err != nil {
		return s.failBackup(ctx, record, err)
	}
	checksum, size, err := checksumFile(path)
	if err != nil {
		return s.failBackup(ctx, record, err)
	}
	manifestValue.Checksum = checksum
	manifestJSON := mustJSON(manifestValue)
	now := time.Now().UTC()
	updates := map[string]any{
		"status":        model.ExportJobStatusSuccess,
		"file_name":     fileName,
		"file_path":     path,
		"file_size":     size,
		"checksum":      checksum,
		"manifest_json": manifestJSON,
		"finished_at":   &now,
	}
	if err := s.db.WithContext(ctx).Model(&model.BackupRecord{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetBackup(ctx, record.ID)
}

func (s *Service) DownloadBackup(ctx context.Context, id string) (*Download, error) {
	record, err := s.GetBackup(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.Status != model.ExportJobStatusSuccess || record.FilePath == "" {
		return nil, fmt.Errorf("backup is not ready")
	}
	if !isPathWithin(record.FilePath, s.backupDir) {
		return nil, fmt.Errorf("invalid backup path")
	}
	return &Download{FilePath: record.FilePath, FileName: record.FileName, ContentType: zipContentType}, nil
}

func (s *Service) newJob(jobType model.ExportJobType, resourceType, resourceID string, masked bool, operatorID string) *model.ExportJob {
	return &model.ExportJob{
		ID:           uuid.NewString(),
		Type:         jobType,
		Status:       model.ExportJobStatusRunning,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Masked:       masked,
		CreatedBy:    operatorID,
	}
}

func (s *Service) finishJob(ctx context.Context, job *model.ExportJob, path, fileName, contentType string) (*model.ExportJob, error) {
	info, err := os.Stat(path)
	if err != nil {
		return s.failJob(ctx, job, err)
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":       model.ExportJobStatusSuccess,
		"file_name":    fileName,
		"file_path":    path,
		"file_size":    info.Size(),
		"content_type": contentType,
		"finished_at":  &now,
	}
	if err := s.db.WithContext(ctx).Model(&model.ExportJob{}).Where("id = ?", job.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetJob(ctx, job.ID)
}

func (s *Service) failJob(ctx context.Context, job *model.ExportJob, err error) (*model.ExportJob, error) {
	now := time.Now().UTC()
	_ = s.db.WithContext(ctx).Model(&model.ExportJob{}).Where("id = ?", job.ID).Updates(map[string]any{
		"status":        model.ExportJobStatusFailed,
		"error_message": err.Error(),
		"finished_at":   &now,
	}).Error
	return s.GetJob(ctx, job.ID)
}

func (s *Service) failBackup(ctx context.Context, record *model.BackupRecord, err error) (*model.BackupRecord, error) {
	now := time.Now().UTC()
	_ = s.db.WithContext(ctx).Model(&model.BackupRecord{}).Where("id = ?", record.ID).Updates(map[string]any{
		"status":        model.ExportJobStatusFailed,
		"error_message": err.Error(),
		"finished_at":   &now,
	}).Error
	return s.GetBackup(ctx, record.ID)
}

func (s *Service) resourcePayload(ctx context.Context, resourceType, resourceID string, masked bool) (any, error) {
	switch strings.TrimSpace(resourceType) {
	case "service":
		var item model.ServiceDefinition
		if err := s.db.WithContext(ctx).First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		var versions []model.ServiceVersion
		var releases []model.ServiceReleaseRecord
		var instances []model.ServiceInstance
		_ = s.db.WithContext(ctx).Where("service_id = ?", item.ID).Find(&versions).Error
		_ = s.db.WithContext(ctx).Where("service_id = ?", item.ID).Find(&releases).Error
		_ = s.db.WithContext(ctx).Where("service_id = ?", item.ID).Find(&instances).Error
		return redact.Struct(map[string]any{"definition": item, "versions": versions, "releases": releases, "instances": instances}, masked), nil
	case "host":
		var item model.Host
		if err := s.db.WithContext(ctx).First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return redact.Struct(item, masked), nil
	case "docker_node":
		var item model.DockerNode
		if err := s.db.WithContext(ctx).First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return redact.Struct(item, masked), nil
	case "registry":
		var item model.Registry
		if err := s.db.WithContext(ctx).First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return redact.Struct(item, masked), nil
	case "nginx_node":
		var item model.NginxNode
		if err := s.db.WithContext(ctx).Preload("Host").First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		var configs []model.NginxConfigVersion
		_ = s.db.WithContext(ctx).Where("node_id = ?", item.ID).Find(&configs).Error
		return redact.Struct(map[string]any{"node": item, "configs": configs}, masked), nil
	case "nginx_config":
		var item model.NginxConfigVersion
		if err := s.db.WithContext(ctx).Preload("NginxNode").First(&item, "id = ?", resourceID).Error; err != nil {
			return nil, err
		}
		return redact.Struct(item, masked), nil
	case "alert_rules":
		var items []model.AlertRule
		if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
			return nil, err
		}
		return redact.Struct(items, masked), nil
	case "notification_channels":
		var items []model.NotificationChannel
		if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
			return nil, err
		}
		return redact.Struct(items, masked), nil
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

func (s *Service) recordsPayload(ctx context.Context, recordType string, masked bool) (any, error) {
	switch strings.TrimSpace(recordType) {
	case "audits":
		var items []model.AuditLog
		err := s.db.WithContext(ctx).Order("created_at DESC").Limit(1000).Find(&items).Error
		return redact.Struct(items, masked), err
	case "tasks":
		var items []model.Task
		err := s.db.WithContext(ctx).Preload("Steps").Preload("Logs").Order("created_at DESC").Limit(200).Find(&items).Error
		return redact.Struct(items, masked), err
	case "alerts":
		var items []model.AlertEvent
		err := s.db.WithContext(ctx).Order("last_triggered_at DESC").Limit(1000).Find(&items).Error
		return redact.Struct(items, masked), err
	case "notifications":
		var items []model.NotificationRecord
		err := s.db.WithContext(ctx).Order("created_at DESC").Limit(1000).Find(&items).Error
		return redact.Struct(items, masked), err
	case "health_checks":
		var serviceChecks []model.ServiceHealthCheck
		var hostChecks []model.HostAvailabilityCheck
		err := s.db.WithContext(ctx).Order("created_at DESC").Limit(1000).Find(&serviceChecks).Error
		if err != nil {
			return nil, err
		}
		err = s.db.WithContext(ctx).Order("created_at DESC").Limit(1000).Find(&hostChecks).Error
		return redact.Struct(map[string]any{"serviceHealthChecks": serviceChecks, "hostAvailabilityChecks": hostChecks}, masked), err
	default:
		return nil, fmt.Errorf("unsupported record type: %s", recordType)
	}
}

func (s *Service) incidentPayload(ctx context.Context, req IncidentRequest, masked bool) (map[string]any, error) {
	taskID := req.TaskID
	if taskID == "" && req.ReleaseID != "" {
		var release model.ServiceReleaseRecord
		if err := s.db.WithContext(ctx).First(&release, "id = ?", req.ReleaseID).Error; err != nil {
			return nil, err
		}
		taskID = release.TaskID
	}
	if taskID == "" && req.EventID != "" {
		var event model.AlertEvent
		if err := s.db.WithContext(ctx).First(&event, "id = ?", req.EventID).Error; err != nil {
			return nil, err
		}
		taskID = event.TaskID
	}
	if taskID == "" {
		return nil, errors.New("taskId, releaseId or eventId is required")
	}
	var task model.Task
	if err := s.db.WithContext(ctx).Preload("Steps").Preload("Logs").First(&task, "id = ?", taskID).Error; err != nil {
		return nil, err
	}
	var audits []model.AuditLog
	_ = s.db.WithContext(ctx).Where("resource_id = ? OR trace_id = ?", task.TargetID, task.ID).Order("created_at DESC").Limit(200).Find(&audits).Error
	var alerts []model.AlertEvent
	_ = s.db.WithContext(ctx).Where("task_id = ?", task.ID).Order("last_triggered_at DESC").Find(&alerts).Error
	var notifications []model.NotificationRecord
	if len(alerts) > 0 {
		eventIDs := make([]string, 0, len(alerts))
		for _, item := range alerts {
			eventIDs = append(eventIDs, item.ID)
		}
		_ = s.db.WithContext(ctx).Where("event_id IN ?", eventIDs).Order("created_at DESC").Find(&notifications).Error
	}
	var serviceChecks []model.ServiceHealthCheck
	var hostChecks []model.HostAvailabilityCheck
	_ = s.db.WithContext(ctx).Where("task_id = ?", task.ID).Find(&serviceChecks).Error
	_ = s.db.WithContext(ctx).Where("task_id = ?", task.ID).Find(&hostChecks).Error
	snapshot, _ := s.snapshotForTask(ctx, task, masked)
	return map[string]any{
		"task":             redact.Struct(task, masked),
		"audits":           redact.Struct(audits, masked),
		"alerts":           redact.Struct(alerts, masked),
		"notifications":    redact.Struct(notifications, masked),
		"healthChecks":     redact.Struct(map[string]any{"service": serviceChecks, "host": hostChecks}, masked),
		"resourceSnapshot": snapshot,
	}, nil
}

func (s *Service) snapshotForTask(ctx context.Context, task model.Task, masked bool) (any, error) {
	switch task.TargetType {
	case "service":
		return s.resourcePayload(ctx, "service", task.TargetID, masked)
	case "host":
		return s.resourcePayload(ctx, "host", task.TargetID, masked)
	case "docker_node":
		return s.resourcePayload(ctx, "docker_node", task.TargetID, masked)
	case "registry":
		return s.resourcePayload(ctx, "registry", task.TargetID, masked)
	case "nginx_node":
		return s.resourcePayload(ctx, "nginx_node", task.TargetID, masked)
	default:
		return map[string]any{"targetType": task.TargetType, "targetId": task.TargetID}, nil
	}
}

func (s *Service) configSnapshot(ctx context.Context, masked bool) any {
	snapshot := map[string]any{}
	var hosts []model.Host
	var dockerNodes []model.DockerNode
	var registries []model.Registry
	var services []model.ServiceDefinition
	var nginxNodes []model.NginxNode
	var alertRules []model.AlertRule
	var notificationChannels []model.NotificationChannel
	_ = s.db.WithContext(ctx).Find(&hosts).Error
	_ = s.db.WithContext(ctx).Find(&dockerNodes).Error
	_ = s.db.WithContext(ctx).Find(&registries).Error
	_ = s.db.WithContext(ctx).Find(&services).Error
	_ = s.db.WithContext(ctx).Find(&nginxNodes).Error
	_ = s.db.WithContext(ctx).Find(&alertRules).Error
	_ = s.db.WithContext(ctx).Find(&notificationChannels).Error
	snapshot["hosts"] = hosts
	snapshot["dockerNodes"] = dockerNodes
	snapshot["registries"] = registries
	snapshot["services"] = services
	snapshot["nginxNodes"] = nginxNodes
	snapshot["alertRules"] = alertRules
	snapshot["notificationChannels"] = notificationChannels
	return redact.Struct(snapshot, masked)
}

func (s *Service) databaseFiles() (map[string][]byte, error) {
	files := map[string][]byte{}
	path := sqlitePath(s.dbDSN)
	if path == "" || path == ":memory:" || strings.HasPrefix(path, "file:") {
		files["backup.db"] = []byte{}
		return files, nil
	}
	dbBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	files["backup.db"] = dbBytes
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecarPath := path + suffix
		data, err := os.ReadFile(sidecarPath)
		if err == nil {
			files["backup.db"+suffix] = data
		}
	}
	return files, nil
}

func (s *Service) newManifest(id, itemType string, masked bool, files []string, resource map[string]any) manifest {
	return manifest{
		ID:          id,
		Type:        itemType,
		Resource:    resource,
		Masked:      masked,
		AppName:     s.appName,
		AppEnv:      s.appEnv,
		GeneratedAt: time.Now().UTC(),
		Files:       files,
	}
}

func maskedDefault(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}

func writeJSON(path string, value any) error {
	data, err := redact.JSON(value, false)
	if err != nil {
		return err
	}
	return writeBytes(path, data)
}

func writeBytes(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func writeZip(path string, files map[string][]byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	zw := zip.NewWriter(file)
	defer zw.Close()
	for name, data := range files {
		writer, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := writer.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func checksumFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func recordsCSV(recordType string, payload any) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"type", "json"}); err != nil {
		return nil, err
	}
	items, ok := payload.([]any)
	if !ok {
		data, _ := json.Marshal(payload)
		_ = writer.Write([]string{recordType, string(data)})
		writer.Flush()
		return buf.Bytes(), writer.Error()
	}
	for _, item := range items {
		data, _ := json.Marshal(item)
		if err := writer.Write([]string{recordType, string(data)}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buf.Bytes(), writer.Error()
}

func incidentSummary(bundle map[string]any) string {
	return "# AegisOps 故障排查包\n\n生成时间：" + time.Now().Format(time.RFC3339) + "\n\n包含任务、步骤、日志、审计、告警、通知和资源快照。\n"
}

func taskLogsText(taskValue any) string {
	data, _ := json.Marshal(taskValue)
	var task model.Task
	_ = json.Unmarshal(data, &task)
	var lines []string
	for _, item := range task.Logs {
		lines = append(lines, fmt.Sprintf("[%s] [%s] %s", item.CreatedAt.Format(time.RFC3339), item.Level, item.Message))
	}
	return strings.Join(lines, "\n")
}

func restoreGuide() string {
	return "# AegisOps 恢复说明\n\n1. 停止 AegisOps 后端服务。\n2. 备份当前数据库文件。\n3. 将备份包中的 `backup.db` 放回 SQLite 数据库路径。\n4. 启动后端服务并检查 `/healthz`。\n\n本期提供备份包和恢复预案，不执行自动恢复。\n"
}

func sqlitePath(dsn string) string {
	if strings.HasPrefix(dsn, "file:") {
		return ""
	}
	if idx := strings.IndexRune(dsn, '?'); idx >= 0 {
		return dsn[:idx]
	}
	return dsn
}

func isPathWithin(path, root string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

func safeFileName(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-", "..", "-")
	return replacer.Replace(value)
}

func timestamp() string {
	return time.Now().UTC().Format("20060102T150405Z")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nonEmpty(values ...string) []string {
	items := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			items = append(items, strings.TrimSpace(value))
		}
	}
	return items
}

func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
