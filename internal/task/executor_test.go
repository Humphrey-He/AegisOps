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

func TestDispatchExecutorRunsRegistryTest(t *testing.T) {
	t.Parallel()

	tester := &fakeRegistryTester{}
	executor := NewDispatchExecutor(tester)
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

func TestDispatchExecutorReturnsRegistryFailure(t *testing.T) {
	t.Parallel()

	tester := &fakeRegistryTester{err: errors.New("registry down")}
	executor := NewDispatchExecutor(tester)
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

	executor := NewDispatchExecutor(nil)
	result, err := executor(context.Background(), model.Task{Type: "scheduled.noop"}, model.TaskDispatch{ID: "dispatch-1"})
	if err != nil {
		t.Fatalf("noop executor err = %v", err)
	}
	if result == "" {
		t.Fatal("noop executor result is empty")
	}
}
