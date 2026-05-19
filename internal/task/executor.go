package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

type RegistryTester interface {
	Test(context.Context, string) error
}

type HostTester interface {
	TestSSH(context.Context, string) error
}

type DockerTester interface {
	TestConnection(context.Context, string) error
}

type ServiceChangeExecutor interface {
	ExecuteServiceChange(context.Context, model.Task, model.TaskDispatch) (string, error)
}

type NginxExecutor interface {
	TestNode(context.Context, string) error
	ReloadNode(context.Context, string) error
	PublishConfig(context.Context, string, string) error
	RollbackConfig(context.Context, string, string) error
}

type DispatchExecutorOptions struct {
	Registry RegistryTester
	Host     HostTester
	Docker   DockerTester
	Service  ServiceChangeExecutor
	Nginx    NginxExecutor
}

func NewDispatchExecutor(opts DispatchExecutorOptions) DispatchExecutor {
	return func(ctx context.Context, task model.Task, dispatch model.TaskDispatch) (string, error) {
		switch strings.TrimSpace(strings.ToLower(task.Type)) {
		case "service.release", "service.upgrade", "service.rollback":
			if opts.Service == nil {
				return "", fmt.Errorf("service release executor is not configured")
			}
			return opts.Service.ExecuteServiceChange(ctx, task, dispatch)
		case "nginx.node.test":
			if opts.Nginx == nil {
				return "", fmt.Errorf("nginx executor is not configured")
			}
			if strings.TrimSpace(task.TargetID) == "" {
				return "", fmt.Errorf("nginx.node.test targetId is required")
			}
			if err := opts.Nginx.TestNode(ctx, task.TargetID); err != nil {
				return "", err
			}
			return fmt.Sprintf("nginx node %s tested by dispatch %s", task.TargetID, dispatch.ID), nil
		case "nginx.node.reload":
			if opts.Nginx == nil {
				return "", fmt.Errorf("nginx executor is not configured")
			}
			if strings.TrimSpace(task.TargetID) == "" {
				return "", fmt.Errorf("nginx.node.reload targetId is required")
			}
			if err := opts.Nginx.ReloadNode(ctx, task.TargetID); err != nil {
				return "", err
			}
			return fmt.Sprintf("nginx node %s reloaded by dispatch %s", task.TargetID, dispatch.ID), nil
		case "nginx.config.publish", "nginx.config.rollback":
			if opts.Nginx == nil {
				return "", fmt.Errorf("nginx executor is not configured")
			}
			if strings.TrimSpace(task.TargetID) == "" {
				return "", fmt.Errorf("%s targetId is required", task.Type)
			}
			configID, err := nginxConfigID(task.Payload)
			if err != nil {
				return "", err
			}
			if strings.EqualFold(task.Type, "nginx.config.rollback") {
				if err := opts.Nginx.RollbackConfig(ctx, task.TargetID, configID); err != nil {
					return "", err
				}
				return fmt.Sprintf("nginx config %s rolled back on node %s by dispatch %s", configID, task.TargetID, dispatch.ID), nil
			}
			if err := opts.Nginx.PublishConfig(ctx, task.TargetID, configID); err != nil {
				return "", err
			}
			return fmt.Sprintf("nginx config %s published on node %s by dispatch %s", configID, task.TargetID, dispatch.ID), nil
		case "docker.node.test":
			if opts.Docker == nil {
				return "", fmt.Errorf("docker executor is not configured")
			}
			if strings.TrimSpace(task.TargetID) == "" {
				return "", fmt.Errorf("docker.node.test targetId is required")
			}
			if err := opts.Docker.TestConnection(ctx, task.TargetID); err != nil {
				return "", err
			}
			return fmt.Sprintf("docker node %s tested by dispatch %s", task.TargetID, dispatch.ID), nil
		case "host.ssh.test":
			if opts.Host == nil {
				return "", fmt.Errorf("host executor is not configured")
			}
			if strings.TrimSpace(task.TargetID) == "" {
				return "", fmt.Errorf("host.ssh.test targetId is required")
			}
			if err := opts.Host.TestSSH(ctx, task.TargetID); err != nil {
				return "", err
			}
			return fmt.Sprintf("host %s ssh tested by dispatch %s", task.TargetID, dispatch.ID), nil
		case "registry.test":
			if opts.Registry == nil {
				return "", fmt.Errorf("registry executor is not configured")
			}
			if strings.TrimSpace(task.TargetID) == "" {
				return "", fmt.Errorf("registry.test targetId is required")
			}
			if err := opts.Registry.Test(ctx, task.TargetID); err != nil {
				return "", err
			}
			return fmt.Sprintf("registry %s tested by dispatch %s", task.TargetID, dispatch.ID), nil
		default:
			return defaultDispatchExecutor(ctx, task, dispatch)
		}
	}
}

func nginxConfigID(payload string) (string, error) {
	var value struct {
		ConfigID string `json:"configId"`
	}
	if err := json.Unmarshal([]byte(payload), &value); err != nil {
		return "", fmt.Errorf("parse nginx payload: %w", err)
	}
	if strings.TrimSpace(value.ConfigID) == "" {
		return "", fmt.Errorf("nginx configId is required")
	}
	return value.ConfigID, nil
}
