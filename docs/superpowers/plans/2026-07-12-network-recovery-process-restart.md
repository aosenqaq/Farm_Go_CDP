# Network Recovery Process Restart Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restart the bound miniapp host when the runtime reconnect watcher reports a terminal network recovery failure.

**Architecture:** The watcher emits one `network_reconnect:failed` event per disconnected-popup episode, so the event directly reserves an automatic restart via the existing process manager. The new manager method retains enable checks, single-flight protection, reconnect grace, restart history, and circuit breaking; `detected` and `waiting` remain local recovery only.

**Tech Stack:** Go, existing `internal/runtime/guard` manager and supervisor tests.

---

## File Structure

- Modify: `internal/runtime/guard/manager.go` - add the terminal-recovery automatic-restart entry point.
- Modify: `internal/runtime/guard/manager_test.go` - prove one terminal failure restarts despite a threshold of three.
- Modify: `internal/runtime/guard/supervisor.go` - forward only terminal network recovery failures.
- Modify: `internal/runtime/guard/supervisor_test.go` - prove `detected` does not restart but `failed` does.
- Modify: `docs/superpowers/specs/2026-07-12-network-recovery-process-restart-design.md` - correct the threshold assumption.

### Task 1: Add the Manager Entry Point

**Files:**
- Modify: `internal/runtime/guard/manager_test.go`
- Modify: `internal/runtime/guard/manager.go`

- [ ] **Step 1: Write the failing test**

Add after `TestManagerTriggersRestartAfterTimeoutThreshold`:

```go
func TestManagerRestartsAfterTerminalNetworkRecoveryFailure(t *testing.T) {
	restarted := make(chan RestartReason, 1)
	settings := enabledSettings()
	settings.TimeoutThreshold = 3
	manager := NewManager(ManagerOptions{Settings: settings, Restart: func(reason RestartReason) (RestartResult, error) {
		restarted <- reason
		return RestartResult{Status: "launch_dispatched"}, nil
	}})
	manager.Arm("test")

	if !manager.RestartAfterNetworkRecoveryFailure(RuntimeSnapshot{RuntimeTarget: "qq_ws"}, "reconnect timeout") {
		t.Fatal("terminal network recovery failure was not accepted")
	}
	select {
	case reason := <-restarted:
		if reason.Manual || reason.Scheduled || reason.Reason != "network reconnect failed: reconnect timeout" {
			t.Fatalf("restart reason = %#v", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal network recovery failure did not restart")
	}
}
```

- [ ] **Step 2: Verify red**

Run: `go test ./internal/runtime/guard -run TestManagerRestartsAfterTerminalNetworkRecoveryFailure -count=1`

Expected: FAIL because `RestartAfterNetworkRecoveryFailure` does not exist.

- [ ] **Step 3: Add the minimal implementation**

Add after `NoteRuntimeError`:

```go
func (m *Manager) RestartAfterNetworkRecoveryFailure(snapshot RuntimeSnapshot, detail string) bool {
	detail = strings.TrimSpace(detail)
	reason := "network reconnect failed"
	if detail != "" { reason += ": " + detail }

	m.mu.Lock()
	runtimeTarget := resolveRuntimeTarget(snapshot)
	m.status.RuntimeTarget = runtimeTarget
	if !m.armed || !m.settings.Enabled || !m.settings.FailureRecoveryEnabled || m.restartInFlight || !m.reconnectGraceUntil.IsZero() {
		m.mu.Unlock()
		return false
	}
	now := m.now()
	m.trimRestartWindowLocked(now)
	m.maybeCloseCircuitLocked()
	m.status.LastTimeoutAt = now.UTC().Format(time.RFC3339)
	m.status.LastReason = reason
	if m.status.CircuitOpen || len(m.restartTimes) >= m.settings.MaxRestartsPer10Min {
		notification := m.openCircuitLocked(true)
		m.queueLifecycleNotificationsLocked(notification)
		m.mu.Unlock()
		m.deliverLifecycleNotifications()
		return false
	}
	job, _, _, notification := m.reserveRestartLocked(RestartReason{RuntimeTarget: runtimeTarget, Reason: reason, Snapshot: snapshot})
	m.queueLifecycleNotificationsLocked(notification)
	m.mu.Unlock()
	m.deliverLifecycleNotifications()
	if job == nil { return false }
	go m.executeRestart(*job)
	return true
}
```

- [ ] **Step 4: Verify green**

Run: `go test ./internal/runtime/guard -run TestManagerRestartsAfterTerminalNetworkRecoveryFailure -count=1`

Expected: PASS.

### Task 2: Escalate Only Terminal Network Events

**Files:**
- Modify: `internal/runtime/guard/supervisor_test.go`
- Modify: `internal/runtime/guard/supervisor.go`

- [ ] **Step 1: Write the failing test**

Add after `TestSupervisorHandlesRuntimeEventsWithCopiesAndReentrantCallback`:

```go
func TestSupervisorRestartsOnlyAfterTerminalNetworkRecoveryFailure(t *testing.T) {
	restarted := make(chan RestartReason, 1)
	settings := supervisorEnabledSettings()
	process := NewManager(ManagerOptions{Settings: settings, Restart: func(reason RestartReason) (RestartResult, error) {
		restarted <- reason
		return RestartResult{Status: "launch_dispatched"}, nil
	}})
	s := NewSupervisor(SupervisorOptions{Settings: settings, Process: process, Snapshot: readyQQSnapshot})
	s.Start(context.Background())
	defer s.Close()

	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "detected", RuntimeTarget: "qq_ws"})
	select { case reason := <-restarted: t.Fatalf("transient event restarted host: %#v", reason); default: }

	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "failed", RuntimeTarget: "qq_ws", Error: "reconnect timeout"})
	select {
	case reason := <-restarted:
		if reason.Reason != "network reconnect failed: reconnect timeout" { t.Fatalf("restart reason = %#v", reason) }
	case <-time.After(time.Second):
		t.Fatal("terminal network recovery failure did not restart host")
	}
}
```

- [ ] **Step 2: Verify red**

Run: `go test ./internal/runtime/guard -run TestSupervisorRestartsOnlyAfterTerminalNetworkRecoveryFailure -count=1`

Expected: FAIL because `HandleRuntimeEvent` does not escalate `network_reconnect:failed`.

- [ ] **Step 3: Connect the terminal event**

In `Supervisor.HandleRuntimeEvent`, add a `restartDetail` variable outside the mutex. Set it only when the normalized name is `network_reconnect`, the normalized phase is `failed`, and `recoveryEligibleLocked(RecoveryNetworkReconnect)` is true. Preserve `event.Error` after `strings.TrimSpace`; substitute `"reconnect_failed"` when empty. After the existing recovery submission, add:

```go
if restartDetail != "" {
	s.process.RestartAfterNetworkRecoveryFailure(s.currentSnapshot(), restartDetail)
}
```

- [ ] **Step 4: Verify focused tests**

Run: `go test ./internal/runtime/guard -run 'TestManagerRestartsAfterTerminalNetworkRecoveryFailure|TestSupervisorRestartsOnlyAfterTerminalNetworkRecoveryFailure' -count=1`

Expected: PASS.

### Task 3: Align Specification and Verify

**Files:**
- Modify: `docs/superpowers/specs/2026-07-12-network-recovery-process-restart-design.md`

- [ ] **Step 1: Correct the escalation bullet**

Replace the threshold-specific bullet with:

```markdown
- The watcher emits one terminal `failed` event for a disconnected-popup episode. That event immediately reserves an automatic host restart; it does not wait for `TimeoutThreshold` because no repeated event will arrive for the same episode.
```

- [ ] **Step 2: Run the package regression suite**

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS.

- [ ] **Step 3: Review and commit only feature files**

Run: `git diff --check && git status --short`

Expected: no whitespace errors; leave the pre-existing `resources/gameConfig.bundle.zip` modification out of the commit.

Run: `git add internal/runtime/guard/manager.go internal/runtime/guard/manager_test.go internal/runtime/guard/supervisor.go internal/runtime/guard/supervisor_test.go docs/superpowers/specs/2026-07-12-network-recovery-process-restart-design.md; git commit -m "fix: restart host after terminal network recovery failure"`
