package desktop

import (
	"context"
	"testing"
	"time"

	"Farm_Go/internal/farm/automation"
)

func TestAutomationExecutionGateAllowsGodTasksTogether(t *testing.T) {
	gate := newAutomationExecutionGate()
	firstRelease, err := gate.Acquire(context.Background(), automation.RunModeGod)
	if err != nil {
		t.Fatalf("acquire first god task: %v", err)
	}
	secondRelease, ok := gate.TryAcquire(automation.RunModeGod)
	if !ok {
		t.Fatal("god tasks should share the execution gate")
	}
	secondRelease()
	firstRelease()
}

func TestAutomationExecutionGateGivesWaitingSafeTaskPriority(t *testing.T) {
	gate := newAutomationExecutionGate()
	godRelease, err := gate.Acquire(context.Background(), automation.RunModeGod)
	if err != nil {
		t.Fatalf("acquire god task: %v", err)
	}

	safeReleaseCh := make(chan func(), 1)
	go func() {
		release, acquireErr := gate.Acquire(context.Background(), automation.RunModeSafe)
		if acquireErr == nil {
			safeReleaseCh <- release
		}
	}()
	waitForExecutionGate(t, gate, func() bool { return gate.waitingSafe == 1 })

	if _, ok := gate.TryAcquire(automation.RunModeGod); ok {
		t.Fatal("a waiting safe task should block a new god task")
	}
	godRelease()

	var safeRelease func()
	select {
	case safeRelease = <-safeReleaseCh:
	case <-time.After(time.Second):
		t.Fatal("safe task did not acquire after running god task finished")
	}
	if _, ok := gate.TryAcquire(automation.RunModeGod); ok {
		t.Fatal("god task should wait while a safe task is running")
	}
	safeRelease()
}

func waitForExecutionGate(t *testing.T, gate *automationExecutionGate, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		gate.mu.Lock()
		matched := condition()
		gate.mu.Unlock()
		if matched {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("execution gate did not reach expected state")
}
