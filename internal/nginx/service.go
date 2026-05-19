package nginx

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	alertsvc "github.com/Humphrey-He/AegisOps/internal/alert"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/secret"
	tasksvc "github.com/Humphrey-He/AegisOps/internal/task"
)

const (
	defaultConfigPath    = "/etc/nginx/nginx.conf"
	defaultTestCommand   = "nginx -t"
	defaultReloadCommand = "nginx -s reload"
)

type Service struct {
	db      *gorm.DB
	secrets *secret.Service
	tasks   *tasksvc.Service
	alerts  *alertsvc.Service
	timeout time.Duration
	taskMu  sync.Mutex
}

type CreateNodeRequest struct {
	Name          string `json:"name" binding:"required"`
	HostID        string `json:"hostId" binding:"required"`
	Environment   string `json:"environment"`
	ConfigPath    string `json:"configPath"`
	TestCommand   string `json:"testCommand"`
	ReloadCommand string `json:"reloadCommand"`
	Description   string `json:"description"`
	OperatorID    string `json:"-"`
}

type UpdateNodeRequest struct {
	Name          string `json:"name"`
	HostID        string `json:"hostId"`
	Environment   string `json:"environment"`
	ConfigPath    string `json:"configPath"`
	TestCommand   string `json:"testCommand"`
	ReloadCommand string `json:"reloadCommand"`
	Description   string `json:"description"`
	OperatorID    string `json:"-"`
}

type CreateConfigRequest struct {
	Version    string `json:"version" binding:"required"`
	Content    string `json:"content" binding:"required"`
	Message    string `json:"message"`
	Activate   bool   `json:"activate"`
	OperatorID string `json:"-"`
}

type RollbackRequest struct {
	ConfigID   string `json:"configId" binding:"required"`
	OperatorID string `json:"-"`
}

type PublishConfigRequest struct {
	ConfigID   string `json:"configId" binding:"required"`
	OperatorID string `json:"-"`
}

func NewService(db *gorm.DB, secrets *secret.Service, tasks *tasksvc.Service) *Service {
	return &Service{db: db, secrets: secrets, tasks: tasks, timeout: 15 * time.Second}
}

func (s *Service) SetAlertService(alerts *alertsvc.Service) {
	s.alerts = alerts
}

func (s *Service) CreateNode(ctx context.Context, req CreateNodeRequest) (*model.NginxNode, error) {
	if err := s.ensureHost(ctx, req.HostID); err != nil {
		return nil, err
	}
	item := &model.NginxNode{
		ID:            uuid.NewString(),
		Name:          strings.TrimSpace(req.Name),
		HostID:        req.HostID,
		Environment:   strings.TrimSpace(req.Environment),
		ConfigPath:    defaultString(req.ConfigPath, defaultConfigPath),
		TestCommand:   defaultString(req.TestCommand, defaultTestCommand),
		ReloadCommand: defaultString(req.ReloadCommand, defaultReloadCommand),
		Description:   req.Description,
		Status:        model.NginxNodeStatusUnknown,
		CreatedBy:     req.OperatorID,
		UpdatedBy:     req.OperatorID,
	}
	if item.Name == "" {
		return nil, fmt.Errorf("nginx node name is required")
	}
	return item, s.db.WithContext(ctx).Create(item).Error
}

func (s *Service) ListNodes(ctx context.Context, keyword, environment string, limit, offset int) ([]model.NginxNode, int64, error) {
	var items []model.NginxNode
	var total int64
	query := s.db.WithContext(ctx).Model(&model.NginxNode{}).Preload("Host")
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ? OR config_path LIKE ?", like, like, like)
	}
	if environment != "" {
		query = query.Where("environment = ?", environment)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) GetNode(ctx context.Context, id string) (*model.NginxNode, error) {
	var item model.NginxNode
	if err := s.db.WithContext(ctx).Preload("Host").First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) UpdateNode(ctx context.Context, id string, req UpdateNodeRequest) (*model.NginxNode, error) {
	item, err := s.GetNode(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.HostID != "" && req.HostID != item.HostID {
		if err := s.ensureHost(ctx, req.HostID); err != nil {
			return nil, err
		}
		item.HostID = req.HostID
	}
	if req.Name != "" {
		item.Name = strings.TrimSpace(req.Name)
	}
	item.Environment = strings.TrimSpace(req.Environment)
	if req.ConfigPath != "" {
		item.ConfigPath = req.ConfigPath
	}
	if req.TestCommand != "" {
		item.TestCommand = req.TestCommand
	}
	if req.ReloadCommand != "" {
		item.ReloadCommand = req.ReloadCommand
	}
	item.Description = req.Description
	item.UpdatedBy = req.OperatorID
	if err := s.db.WithContext(ctx).Save(item).Error; err != nil {
		return nil, err
	}
	return s.GetNode(ctx, id)
}

func (s *Service) DeleteNode(ctx context.Context, id string) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.NginxConfigVersion{}).Where("node_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("nginx node has config versions")
	}
	return s.db.WithContext(ctx).Delete(&model.NginxNode{}, "id = ?", id).Error
}

func (s *Service) CreateConfig(ctx context.Context, nodeID string, req CreateConfigRequest) (*model.NginxConfigVersion, error) {
	if _, err := s.GetNode(ctx, nodeID); err != nil {
		return nil, err
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("nginx config content is required")
	}
	sum := sha256.Sum256([]byte(content))
	item := &model.NginxConfigVersion{
		ID:        uuid.NewString(),
		NodeID:    nodeID,
		Version:   strings.TrimSpace(req.Version),
		Content:   content,
		Checksum:  hex.EncodeToString(sum[:]),
		Status:    model.NginxConfigStatusDraft,
		Message:   req.Message,
		CreatedBy: req.OperatorID,
	}
	if item.Version == "" {
		return nil, fmt.Errorf("nginx config version is required")
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.Activate {
			if err := tx.Model(&model.NginxConfigVersion{}).Where("node_id = ?", nodeID).Update("status", model.NginxConfigStatusDraft).Error; err != nil {
				return err
			}
			item.Status = model.NginxConfigStatusActive
		}
		return tx.Create(item).Error
	})
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListConfigs(ctx context.Context, nodeID string, limit, offset int) ([]model.NginxConfigVersion, int64, error) {
	var items []model.NginxConfigVersion
	var total int64
	query := s.db.WithContext(ctx).Model(&model.NginxConfigVersion{}).Where("node_id = ?", nodeID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) GetConfig(ctx context.Context, id string) (*model.NginxConfigVersion, error) {
	var item model.NginxConfigVersion
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) TestTask(ctx context.Context, nodeID, operatorID string) (string, error) {
	return s.runCommandTask(ctx, tasksvc.CreateRequest{
		Type:       "nginx.node.test",
		Title:      "test nginx node " + nodeID,
		TargetType: "nginx_node",
		TargetID:   nodeID,
		CreatedBy:  operatorID,
		Steps: []tasksvc.CreateStepRequest{
			{Name: "load nginx node", SortOrder: 1},
			{Name: "run nginx test command", SortOrder: 2},
			{Name: "record nginx status", SortOrder: 3},
		},
	}, func(ctx context.Context, runner *nginxTaskRunner) error {
		var node *model.NginxNode
		if err := runner.Step("load nginx node", func(stepID string) error {
			var err error
			node, err = s.GetNode(ctx, nodeID)
			return err
		}); err != nil {
			return err
		}
		if err := runner.Step("run nginx test command", func(stepID string) error {
			output, err := s.runHostCommand(ctx, node.HostID, node.TestCommand)
			runner.Log(stepID, model.TaskLogLevelInfo, output)
			if err != nil {
				s.recordNodeTestResult(ctx, nodeID, model.NginxNodeStatusOffline)
			}
			return err
		}); err != nil {
			return err
		}
		return runner.Step("record nginx status", func(stepID string) error {
			return s.recordNodeTestResult(ctx, nodeID, model.NginxNodeStatusOnline)
		})
	})
}

func (s *Service) ReloadTask(ctx context.Context, nodeID, operatorID string) (string, error) {
	return s.runCommandTask(ctx, tasksvc.CreateRequest{
		Type:       "nginx.node.reload",
		Title:      "reload nginx node " + nodeID,
		TargetType: "nginx_node",
		TargetID:   nodeID,
		CreatedBy:  operatorID,
		Steps: []tasksvc.CreateStepRequest{
			{Name: "load nginx node", SortOrder: 1},
			{Name: "run nginx test command", SortOrder: 2},
			{Name: "run nginx reload command", SortOrder: 3},
		},
	}, func(ctx context.Context, runner *nginxTaskRunner) error {
		var node *model.NginxNode
		if err := runner.Step("load nginx node", func(stepID string) error {
			var err error
			node, err = s.GetNode(ctx, nodeID)
			return err
		}); err != nil {
			return err
		}
		if err := runner.Step("run nginx test command", func(stepID string) error {
			output, err := s.runHostCommand(ctx, node.HostID, node.TestCommand)
			runner.Log(stepID, model.TaskLogLevelInfo, output)
			return err
		}); err != nil {
			return err
		}
		return runner.Step("run nginx reload command", func(stepID string) error {
			output, err := s.runHostCommand(ctx, node.HostID, node.ReloadCommand)
			runner.Log(stepID, model.TaskLogLevelInfo, output)
			return err
		})
	})
}

func (s *Service) PublishConfigTask(ctx context.Context, nodeID string, req PublishConfigRequest) (string, error) {
	return s.applyConfigTask(ctx, nodeID, req.ConfigID, req.OperatorID, "nginx.config.publish", "publish nginx config "+nodeID)
}

func (s *Service) RollbackTask(ctx context.Context, nodeID string, req RollbackRequest) (string, error) {
	return s.applyConfigTask(ctx, nodeID, req.ConfigID, req.OperatorID, "nginx.config.rollback", "rollback nginx config "+nodeID)
}

func (s *Service) applyConfigTask(ctx context.Context, nodeID, configID, operatorID, taskType, title string) (string, error) {
	return s.runCommandTask(ctx, tasksvc.CreateRequest{
		Type:       taskType,
		Title:      title,
		TargetType: "nginx_node",
		TargetID:   nodeID,
		CreatedBy:  operatorID,
		Steps: []tasksvc.CreateStepRequest{
			{Name: "load config version", SortOrder: 1},
			{Name: "backup remote config", SortOrder: 2},
			{Name: "write remote config", SortOrder: 3},
			{Name: "run nginx test command", SortOrder: 4},
			{Name: "run nginx reload command", SortOrder: 5},
			{Name: "activate config version", SortOrder: 6},
		},
	}, func(ctx context.Context, runner *nginxTaskRunner) error {
		var node *model.NginxNode
		var config *model.NginxConfigVersion
		var backupPath string
		if err := runner.Step("load config version", func(stepID string) error {
			var err error
			node, err = s.GetNode(ctx, nodeID)
			if err != nil {
				return err
			}
			config, err = s.GetConfig(ctx, configID)
			if err != nil {
				return err
			}
			if config.NodeID != nodeID {
				return fmt.Errorf("config version does not belong to nginx node")
			}
			return nil
		}); err != nil {
			return err
		}
		if err := runner.Step("backup remote config", func(stepID string) error {
			var err error
			backupPath, err = s.writeRemoteConfig(ctx, node.HostID, node.ConfigPath, config.Content)
			if err != nil {
				return err
			}
			runner.Log(stepID, model.TaskLogLevelInfo, "remote config backup: "+backupPath)
			return nil
		}); err != nil {
			return err
		}
		if err := runner.Step("write remote config", func(stepID string) error {
			runner.Log(stepID, model.TaskLogLevelInfo, "wrote config version "+config.Version+" to "+node.ConfigPath)
			return nil
		}); err != nil {
			return err
		}
		restore := func(stepID string, reason error) error {
			restoreOutput, restoreErr := s.runHostCommand(ctx, node.HostID, "cp "+shellQuote(backupPath)+" "+shellQuote(node.ConfigPath))
			runner.Log(stepID, model.TaskLogLevelWarn, restoreOutput)
			if restoreErr != nil {
				return fmt.Errorf("%w; restore remote config failed: %v", reason, restoreErr)
			}
			runner.Log(stepID, model.TaskLogLevelWarn, "restored remote config from backup after failure")
			return reason
		}
		if err := runner.Step("run nginx test command", func(stepID string) error {
			output, err := s.runHostCommand(ctx, node.HostID, node.TestCommand)
			runner.Log(stepID, model.TaskLogLevelInfo, output)
			if err != nil {
				return restore(stepID, err)
			}
			return nil
		}); err != nil {
			return err
		}
		if err := runner.Step("run nginx reload command", func(stepID string) error {
			output, err := s.runHostCommand(ctx, node.HostID, node.ReloadCommand)
			runner.Log(stepID, model.TaskLogLevelInfo, output)
			if err != nil {
				return restore(stepID, err)
			}
			return nil
		}); err != nil {
			return err
		}
		return runner.Step("activate config version", func(stepID string) error {
			if err := s.activateConfig(ctx, nodeID, configID); err != nil {
				return err
			}
			runner.Log(stepID, model.TaskLogLevelInfo, "activated config version "+config.Version)
			return nil
		})
	})
}

func (s *Service) activateConfig(ctx context.Context, nodeID, configID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.NginxConfigVersion{}).Where("node_id = ?", nodeID).Update("status", model.NginxConfigStatusDraft).Error; err != nil {
			return err
		}
		return tx.Model(&model.NginxConfigVersion{}).Where("id = ? AND node_id = ?", configID, nodeID).Update("status", model.NginxConfigStatusActive).Error
	})
}

func (s *Service) runCommandTask(ctx context.Context, req tasksvc.CreateRequest, fn func(context.Context, *nginxTaskRunner) error) (string, error) {
	if s.tasks == nil {
		return "", fmt.Errorf("task service is not configured")
	}
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	task, err := s.tasks.CreateRunning(ctx, req)
	if err != nil {
		return "", err
	}
	runner := newNginxTaskRunner(ctx, s.tasks, task)
	if err := fn(ctx, runner); err != nil {
		_, _ = s.tasks.AddLog(ctx, task.ID, "", model.TaskLogLevelError, err.Error())
		_ = s.tasks.Finish(ctx, task.ID, model.TaskStatusFailed, "", err.Error())
		s.recordFailureEvent(ctx, req.Type, req.TargetType, req.TargetID, task.ID, err)
		return task.ID, err
	}
	_ = s.tasks.Finish(ctx, task.ID, model.TaskStatusSuccess, "ok", "")
	return task.ID, nil
}

type nginxTaskRunner struct {
	ctx    context.Context
	tasks  *tasksvc.Service
	taskID string
	steps  map[string]string
}

func newNginxTaskRunner(ctx context.Context, tasks *tasksvc.Service, task *model.Task) *nginxTaskRunner {
	steps := make(map[string]string, len(task.Steps))
	for _, step := range task.Steps {
		steps[step.Name] = step.ID
	}
	return &nginxTaskRunner{ctx: ctx, tasks: tasks, taskID: task.ID, steps: steps}
}

func (r *nginxTaskRunner) Step(name string, fn func(stepID string) error) error {
	stepID := r.steps[name]
	if stepID != "" {
		_ = r.tasks.UpdateStep(r.ctx, stepID, model.TaskStatusRunning, "", "")
	}
	err := fn(stepID)
	if err != nil {
		if stepID != "" {
			_ = r.tasks.UpdateStep(r.ctx, stepID, model.TaskStatusFailed, "", err.Error())
			r.Log(stepID, model.TaskLogLevelError, err.Error())
		}
		return err
	}
	if stepID != "" {
		_ = r.tasks.UpdateStep(r.ctx, stepID, model.TaskStatusSuccess, "ok", "")
	}
	return nil
}

func (r *nginxTaskRunner) Log(stepID string, level model.TaskLogLevel, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	_, _ = r.tasks.AddLog(r.ctx, r.taskID, stepID, level, message)
}

func (s *Service) recordNodeTestResult(ctx context.Context, nodeID string, status model.NginxNodeStatus) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&model.NginxNode{}).Where("id = ?", nodeID).Updates(map[string]interface{}{
		"status":       status,
		"last_test_at": &now,
	}).Error
}

func (s *Service) recordFailureEvent(ctx context.Context, taskType, resourceType, resourceID, taskID string, err error) {
	if s.alerts == nil || err == nil {
		return
	}
	eventType := ""
	switch taskType {
	case "nginx.node.reload":
		eventType = "nginx_reload_failed"
	case "nginx.config.publish":
		eventType = "nginx_publish_failed"
	}
	if eventType == "" {
		return
	}
	_, _ = s.alerts.CreateEvent(ctx, alertsvc.EventRequest{
		EventType:    eventType,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		TaskID:       taskID,
		Severity:     model.AlertEventSeverityCritical,
		Summary:      eventType,
		Detail:       err.Error(),
		DedupeKey:    eventType + ":" + resourceID,
	})
}

func (s *Service) runHostCommand(ctx context.Context, hostID, command string) (string, error) {
	var host model.Host
	if err := s.db.WithContext(ctx).First(&host, "id = ?", hostID).Error; err != nil {
		return "", err
	}
	secretItem, err := s.secrets.Get(ctx, host.SSHSecretID)
	if err != nil {
		return "", err
	}
	value, err := s.secrets.DecryptValue(ctx, host.SSHSecretID)
	if err != nil {
		return "", err
	}
	authMethod, err := sshAuthMethod(secretItem.Type, value)
	if err != nil {
		return "", err
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(host.Address, fmt.Sprintf("%d", host.SSHPort)), &ssh.ClientConfig{
		User:            host.SSHUser,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         s.timeout,
	})
	if err != nil {
		return "", err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	output, err := session.CombinedOutput(command)
	return strings.TrimSpace(string(output)), err
}

func (s *Service) writeRemoteConfig(ctx context.Context, hostID, path, content string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("nginx config path is required")
	}
	backupPath := fmt.Sprintf("%s.aegisops.%s.bak", path, time.Now().UTC().Format("20060102150405"))
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	script := strings.Join([]string{
		"set -eu",
		"target=" + shellQuote(path),
		"backup=" + shellQuote(backupPath),
		"tmp=\"${target}.aegisops.tmp.$$\"",
		"if [ -f \"$target\" ]; then cp -p \"$target\" \"$backup\" 2>/dev/null || cp \"$target\" \"$backup\"; else : > \"$backup\"; fi",
		"base64 -d > \"$tmp\" <<'AEGISOPS_CONFIG_EOF'",
		encoded,
		"AEGISOPS_CONFIG_EOF",
		"chmod --reference=\"$target\" \"$tmp\" 2>/dev/null || chmod 0644 \"$tmp\"",
		"chown --reference=\"$target\" \"$tmp\" 2>/dev/null || true",
		"mv \"$tmp\" \"$target\"",
		"printf '%s' \"$backup\"",
	}, "\n")
	output, err := s.runHostCommand(ctx, hostID, "sh -c "+shellQuote(script))
	if err != nil {
		return "", err
	}
	output = strings.TrimSpace(output)
	if output == "" {
		output = backupPath
	}
	return output, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (s *Service) ensureHost(ctx context.Context, hostID string) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.Host{}).Where("id = ?", hostID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("host not found")
	}
	return nil
}

func sshAuthMethod(secretType model.SecretType, value string) (ssh.AuthMethod, error) {
	switch secretType {
	case model.SecretTypeSSHPassword:
		return ssh.Password(value), nil
	case model.SecretTypeSSHPrivateKey:
		signer, err := ssh.ParsePrivateKey([]byte(value))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("secret type %s cannot be used for ssh", secretType)
	}
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
