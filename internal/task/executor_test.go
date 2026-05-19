package task

import (
	"context"
	"errors"
	"testing"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

type fakeRegistryTester struct {
	calledID string
	err      error
}

func (f *fakeRegistryTester) Test(_ context.Context, id string) error {
	f.calledID = id
	return f.err
}

type fakeHostTester struct {
	calledID string
	err      error
}

func (f *fakeHostTester) TestSSH(_ context.Context, id string) error {
	f.calledID = id
	return f.err
}

type fakeDockerTester struct {
	calledID string
	err      error
}

func (f *fakeDockerTester) TestConnection(_ context.Context, id string) error {
	f.calledID = id
	return f.err
}

func TestDispatchExecutorRunsRegistryTest(t *testing.T) {
	t.Parallel()

	tester := &fakeRegistryTester{}
	executor := NewDispatchExecutor(DispatchExecutorOptions{Registry: tester})
	result, err := executor(context.Background(), model.Task{
		Type:     "registry.test",
		TargetID: "registry-1",
	}, model.TaskDispatch{ID: "dispatch-1"})
	if err != nil {
		t.Fatalf("registry executor err = %v", err)
	}
	if tester.calledID != "registry-1" {
		t.Fatalf("registry tester called with %q, want registry-1", tester.calledID)
	}
	if result == "" {
		t.Fatal("registry executor result is empty")
	}
}

func TestDispatchExecutorRunsHostSSHTest(t *testing.T) {
	t.Parallel()

	tester := &fakeHostTester{}
	executor := NewDispatchExecutor(DispatchExecutorOptions{Host: tester})
	result, err := executor(context.Background(), model.Task{
		Type:     "host.ssh.test",
		TargetID: "host-1",
	}, model.TaskDispatch{ID: "dispatch-1"})
	if err != nil {
		t.Fatalf("host executor err = %v", err)
	}
	if tester.calledID != "host-1" {
		t.Fatalf("host tester called with %q, want host-1", tester.calledID)
	}
	if result == "" {
		t.Fatal("host executor result is empty")
	}
}

func TestDispatchExecutorRunsDockerNodeTest(t *testing.T) {
	t.Parallel()

	tester := &fakeDockerTester{}
	executor := NewDispatchExecutor(DispatchExecutorOptions{Docker: tester})
	result, err := executor(context.Background(), model.Task{
		Type:     "docker.node.test",
		TargetID: "docker-1",
	}, model.TaskDispatch{ID: "dispatch-1"})
	if err != nil {
		t.Fatalf("docker executor err = %v", err)
	}
	if tester.calledID != "docker-1" {
		t.Fatalf("docker tester called with %q, want docker-1", tester.calledID)
	}
	if result == "" {
		t.Fatal("docker executor result is empty")
	}
}

func TestDispatchExecutorReturnsDockerFailure(t *testing.T) {
	t.Parallel()

	tester := &fakeDockerTester{err: errors.New("docker failed")}
	executor := NewDispatchExecutor(DispatchExecutorOptions{Docker: tester})
	_, err := executor(context.Background(), model.Task{
		Type:     "docker.node.test",
		TargetID: "docker-1",
	}, model.TaskDispatch{ID: "dispatch-1"})
	if err == nil || err.Error() != "docker failed" {
		t.Fatalf("docker executor err = %v, want docker failed", err)
	}
}

func TestDispatchExecutorRejectsMissingDockerTarget(t *testing.T) {
	t.Parallel()

	executor := NewDispatchExecutor(DispatchExecutorOptions{Docker: &fakeDockerTester{}})
	_, err := executor(context.Background(), model.Task{Type: "docker.node.test"}, model.TaskDispatch{ID: "dispatch-1"})
	if err == nil || err.Error() != "docker.node.test targetId is required" {
		t.Fatalf("docker missing target err = %v", err)
	}
}

func TestDispatchExecutorRejectsUnconfiguredDocker(t *testing.T) {
	t.Parallel()

	executor := NewDispatchExecutor(DispatchExecutorOptions{})
	_, err := executor(context.Background(), model.Task{Type: "docker.node.test", TargetID: "docker-1"}, model.TaskDispatch{ID: "dispatch-1"})
	if err == nil || err.Error() != "docker executor is not configured" {
		t.Fatalf("docker unconfigured err = %v", err)
	}
}

func TestDispatchExecutorReturnsHostFailure(t *testing.T) {
	t.Parallel()

	tester := &fakeHostTester{err: errors.New("ssh failed")}
	executor := NewDispatchExecutor(DispatchExecutorOptions{Host: tester})
	_, err := executor(context.Background(), model.Task{
		Type:     "host.ssh.test",
		TargetID: "host-1",
	}, model.TaskDispatch{ID: "dispatch-1"})
	if err == nil || err.Error() != "ssh failed" {
		t.Fatalf("host executor err = %v, want ssh failed", err)
	}
}

func TestDispatchExecutorRejectsMissingHostTarget(t *testing.T) {
	t.Parallel()

	executor := NewDispatchExecutor(DispatchExecutorOptions{Host: &fakeHostTester{}})
	_, err := executor(context.Background(), model.Task{Type: "host.ssh.test"}, model.TaskDispatch{ID: "dispatch-1"})
	if err == nil || err.Error() != "host.ssh.test targetId is required" {
		t.Fatalf("host missing target err = %v", err)
	}
}

func TestDispatchExecutorRejectsUnconfiguredHost(t *testing.T) {
	t.Parallel()

	executor := NewDispatchExecutor(DispatchExecutorOptions{})
	_, err := executor(context.Background(), model.Task{Type: "host.ssh.test", TargetID: "host-1"}, model.TaskDispatch{ID: "dispatch-1"})
	if err == nil || err.Error() != "host executor is not configured" {
		t.Fatalf("host unconfigured err = %v", err)
	}
}

func TestDispatchExecutorReturnsRegistryFailure(t *testing.T) {
	t.Parallel()

	tester := &fakeRegistryTester{err: errors.New("registry down")}
	executor := NewDispatchExecutor(DispatchExecutorOptions{Registry: tester})
	_, err := executor(context.Background(), model.Task{
		Type:     "registry.test",
		TargetID: "registry-1",
	}, model.TaskDispatch{ID: "dispatch-1"})
	if err == nil || err.Error() != "registry down" {
		t.Fatalf("registry executor err = %v, want registry down", err)
	}
}

func TestDispatchExecutorFallsBackToDefault(t *testing.T) {
	t.Parallel()

	executor := NewDispatchExecutor(DispatchExecutorOptions{})
	result, err := executor(context.Background(), model.Task{Type: "scheduled.noop"}, model.TaskDispatch{ID: "dispatch-1"})
	if err != nil {
		t.Fatalf("noop executor err = %v", err)
	}
	if result == "" {
		t.Fatal("noop executor result is empty")
	}
}
