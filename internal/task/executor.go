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

func NewDispatchExecutor(registryTester RegistryTester) DispatchExecutor {
	return func(ctx context.Context, task model.Task, dispatch model.TaskDispatch) (string, error) {
		switch strings.TrimSpace(strings.ToLower(task.Type)) {
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
