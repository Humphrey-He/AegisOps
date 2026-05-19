package task

import (
	"context"
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

func NewDispatchExecutor(registryTester RegistryTester, hostTester HostTester, dockerTester DockerTester) DispatchExecutor {
	return func(ctx context.Context, task model.Task, dispatch model.TaskDispatch) (string, error) {
		switch strings.TrimSpace(strings.ToLower(task.Type)) {
		case "docker.node.test":
			if dockerTester == nil {
				return "", fmt.Errorf("docker executor is not configured")
			}
			if strings.TrimSpace(task.TargetID) == "" {
				return "", fmt.Errorf("docker.node.test targetId is required")
			}
			if err := dockerTester.TestConnection(ctx, task.TargetID); err != nil {
				return "", err
			}
			return fmt.Sprintf("docker node %s tested by dispatch %s", task.TargetID, dispatch.ID), nil
		case "host.ssh.test":
			if hostTester == nil {
				return "", fmt.Errorf("host executor is not configured")
			}
			if strings.TrimSpace(task.TargetID) == "" {
				return "", fmt.Errorf("host.ssh.test targetId is required")
			}
			if err := hostTester.TestSSH(ctx, task.TargetID); err != nil {
				return "", err
			}
			return fmt.Sprintf("host %s ssh tested by dispatch %s", task.TargetID, dispatch.ID), nil
		case "registry.test":
			if registryTester == nil {
				return "", fmt.Errorf("registry executor is not configured")
			}
			if strings.TrimSpace(task.TargetID) == "" {
				return "", fmt.Errorf("registry.test targetId is required")
			}
			if err := registryTester.Test(ctx, task.TargetID); err != nil {
				return "", err
			}
			return fmt.Sprintf("registry %s tested by dispatch %s", task.TargetID, dispatch.ID), nil
		default:
			return defaultDispatchExecutor(ctx, task, dispatch)
		}
	}
}
