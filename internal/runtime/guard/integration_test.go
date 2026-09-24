package guard

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGuardianDetectsNetworkPromptDuringLongAutomationTask(t *testing.T) {
	h := newGuardianIntegrationHarness(t)
	h.startLongTask()
	h.supervisor.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "detected"})
	if got := h.nextRecovery(t); got != RecoveryNetworkReconnect {
		t.Fatalf("recovery = %s", got)
	}
	if !h.longTaskRunning() {
		t.Fatal("network detection waited for the automation task")
	}
}

func TestGuardianHandlesDueOtherPlaceLoginDuringAutomationTask(t *testing.T) {
	h := newGuardianIntegrationHarness(t)
	h.startLongTask()
	h.supervisor.HandleRuntimeEvent(RuntimeEvent{Name: "other_place_login_reconnect", Phase: "due"})
	if got := h.nextRecovery(t); got != RecoveryOtherPlaceLogin {
		t.Fatalf("recovery = %s", got)
	}
}

func TestGuardianProcessRestartInvalidatesOldTaskResult(t *testing.T) {
	h := newGuardianIntegrationHarness(t)
	old := h.coordinator.Generation()
	h.coordinator.AdvanceGeneration("process_restart")
	if h.coordinator.IsCurrent(old) {
		t.Fatal("old generation remained current")
	}
	if err := h.finishLongTask(old); !errors.Is(err, ErrStaleRuntimeGeneration) {
		t.Fatalf("task completion error = %v", err)
	}
}

func TestGuardianContinuesWithoutAutomationScheduler(t *testing.T) {
	h := newGuardianIntegrationHarness(t)
	h.supervisor.OnRuntimeStatus(readyQQSnapshot())
	status := h.supervisor.Status()
	if !status.Running || status.Process.LastCheckAt == "" {
		t.Fatalf("guardian did not run independently: %#v", status)
	}
}

type guardianIntegrationHarness struct {
	supervisor  *Supervisor
	coordinator *RecoveryCoordinator
	recoveries  chan RecoveryKind
	longTask    chan struct{}
	started     chan struct{}
}

func newGuardianIntegrationHarness(t *testing.T) *guardianIntegrationHarness {
	t.Helper()
	h := &guardianIntegrationHarness{recoveries: make(chan RecoveryKind, 2), longTask: make(chan struct{}), started: make(chan struct{})}
	h.coordinator = NewRecoveryCoordinator(CoordinatorOptions{Run: func(_ context.Context, request RecoveryRequest) RecoveryResult {
		h.recoveries <- request.Kind
		return RecoveryResult{OK: true, Kind: request.Kind}
	}})
	settings := DefaultSettings()
	settings.Enabled = true
	settings.OtherPlaceLoginEnabled = true
	h.supervisor = NewSupervisor(SupervisorOptions{Settings: settings, Coordinator: h.coordinator})
	h.supervisor.Start(context.Background())
	t.Cleanup(h.supervisor.Close)
	return h
}

func (h *guardianIntegrationHarness) startLongTask() {
	go func() { close(h.started); <-h.longTask }()
	<-h.started
}

func (h *guardianIntegrationHarness) longTaskRunning() bool {
	select {
	case <-h.longTask:
		return false
	default:
		return true
	}
}

func (h *guardianIntegrationHarness) finishLongTask(generation uint64) error {
	select {
	case <-h.longTask:
	default:
		close(h.longTask)
	}
	if !h.coordinator.IsCurrent(generation) {
		return ErrStaleRuntimeGeneration
	}
	return nil
}

func (h *guardianIntegrationHarness) nextRecovery(t *testing.T) RecoveryKind {
	t.Helper()
	select {
	case kind := <-h.recoveries:
		return kind
	case <-time.After(time.Second):
		t.Fatal("recovery did not run")
		return ""
	}
}
