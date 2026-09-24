# Guardian Service Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete Farm_Go's guardian service with concurrent process recovery, scheduled miniapp restart, immediate network reconnect, delayed other-place-login reconnect, and a unified console.

**Architecture:** Keep prompt detection inside the injected runtime script and place process monitoring, persistence, event routing, and destructive recovery coordination in Go. A long-lived `guard.Supervisor` runs independently of the farm scheduler; all detectors run concurrently, while a recovery coordinator serializes restart-class actions and invalidates results from older runtime generations.

**Tech Stack:** Go 1.25, Wails v2, React 18, TypeScript, Vitest, SQLite settings/event storage, QQ WebSocket runtime, WMPF/CDP runtime, embedded JavaScript runtime controller.

---

## Execution Handoff Checkpoint (2026-07-11)

This section is the authoritative continuation state. The task checklists below
remain the original implementation recipe and have not been rewritten to reflect
every review/fix commit.

### Workspace

- Isolated worktree: `E:\desktop\Farm_Go\.worktrees\guardian-service-migration`
- Branch: `guardian-service-migration`
- Main workspace has an unrelated modified `frontend-vite.out.log`; do not touch it.
- Windows race tests are unavailable because Go has `CGO_ENABLED=0`.

### Completed And Reviewed

- Task 1 settings/types: complete and reviewed.
  - `2ce3c3a feat: persist complete guardian settings`
  - `8a5b784 test: cover guardian setting defaults`
- Task 2 process worker/scheduled restart: complete and reviewed.
  - Final fix chain ends at `68326c9 fix: reconcile stale guardian restarts`.
- Task 3 recovery coordinator/generation isolation: complete and reviewed.
  - Final fix chain ends at `49eea84 fix: isolate guardian result callbacks`.
- Task 4 QQ/CDP/WMPF runtime events: complete and quality-approved.
  - Event forwarding starts at `2ca5466`.
  - Lifecycle/ownership fix chain ends at `a45e037`.
  - The remaining review note is Minor only: the test-only WMPF `setEvaluator`
    helper should not be used to reinstall an evaluator owned by a live connect
    handle. There are no production call sites.
- Task 5 runtime-local watchers: complete, spec-approved, and quality-approved.
  - Implementation/fix chain: `8b0b87f`, `02316d5`, `f6a47bf`, `3ec3977`,
    `eaeadb8`.
  - Covers idempotent start/stop, cancellable waits, generation isolation,
    network lifecycle events, reconnect episode deduplication, and delayed
    other-place-login handling.

### Current Task 6 State

Task 6 is implemented and spec-approved:

- `dddcb21 feat: supervise concurrent guardian workers`
- `e146cf6 fix: isolate guardian worker synchronization`

The final quality review found two Important lifecycle races. A fix is currently
present but **uncommitted** in:

- `internal/runtime/guard/supervisor.go`
- `internal/runtime/guard/supervisor_test.go`

Current uncommitted diff summary: approximately 182 inserted lines and 5 removed
lines. Preserve it. It adds:

- `lifecycleMu` plus a lifecycle token to serialize `Start`, `Close`, and
  `UpdateSettings` owner operations;
- running checks before `NoteHealthy` and `NoteRuntimeError` mutate process state;
- recovery eligibility checks for running/master/child switches;
- lifecycle/generation gating before coordinator submission;
- pending recovery cleanup during `Close`;
- deterministic tests for Start-vs-Close, stopped settings updates, disabled or
  stale recovery events, and pending recovery cleanup.

Immediate continuation:

1. Run `gofmt -w internal/runtime/guard/supervisor.go internal/runtime/guard/supervisor_test.go`.
2. Run `go test ./internal/runtime/guard -run 'TestSupervisor(StartAndClose|UpdateSettingsWhileStopped|RecoveryEventsRequire|CloseClears)' -count=1 -timeout=30s`.
3. Run `go test ./internal/runtime/guard -count=1 -timeout=60s`.
4. Inspect the diff for lock ordering. In particular, do not call a coordinator
   method that can reacquire Supervisor lifecycle locks while holding `s.mu`.
5. Run `go test ./... -count=1 -timeout=120s`, `go vet ./internal/runtime/guard`,
   and `git diff --check`.
6. If green, commit only the two Supervisor files with
   `fix: serialize guardian supervisor lifecycle`.

The two acceptance invariants for this uncommitted fix are:

- A stale remainder of `Start` or `UpdateSettings` cannot re-arm/restart Manager
  or Coordinator after `Close` completes; settings changes while stopped leave
  Manager disarmed.
- A runtime event can enter the recovery coordinator only when the same
  Supervisor lifecycle is still running, the master and matching child switch
  are enabled, and its runtime generation is current. `Close` must not leave a
  pending request that executes after a later `Start`.

### Remaining Delivery Scope

After committing Task 6, continue directly with Tasks 7-9 below:

- Task 7: App lifecycle integration, guarded runtime caller, scheduler dispatch
  pause/resume, QQ/WMPF event registration, and account-scoped event storage.
- Task 8: aggregate Wails APIs and the confirmed unified guardian console. Keep
  the UI consistent with existing global primitives and regenerate Wails
  bindings.
- Task 9: deterministic concurrency regressions, README behavior notes, one full
  Go/Node/frontend verification pass, and manual desktop/narrow viewport checks.

The user requested accelerated delivery. Do not run repeated `-count=50/100/200`
stress batches. Use one deterministic focused run and one full run. Keep the
required spec review and one quality review per remaining task, fix only
Critical/Important findings, and defer unrelated Minor cleanup.

### Last Known Verification

- Task 5 watcher harness passed, including 50 repeats during its final review.
- `go test ./... -count=1` passed at Task 6 commit `e146cf6`, before the current
  uncommitted lifecycle fix.
- The current uncommitted Task 6 fix was interrupted before verification and must
  not be reported as passing until the commands above complete.

---

## File Structure

- Modify `internal/runtime/guard/types.go`: aggregate settings, worker status, runtime events, and recovery request types.
- Modify `internal/runtime/guard/manager.go`: process worker timer loop, scheduled restart, reconnect grace, asynchronous restart, and circuit recovery.
- Create `internal/runtime/guard/coordinator.go`: recovery serialization, priority, runtime generation, dispatch pause, and stale-result checks.
- Create `internal/runtime/guard/supervisor.go`: master/child lifecycle, runtime watcher synchronization, aggregate status, and event handling.
- Modify `internal/runtime/qqws/adapter.go`: deliver generic runtime events to subscribers.
- Modify `internal/runtime/cdp/client.go`: expose selected Runtime events to the WMPF link.
- Modify `internal/runtime/wmpf/link.go`: install the runtime event bridge and forward guardian events.
- Modify `resources/wmpf/button.js`: emit network reconnect lifecycle events and expose complete watcher state.
- Modify `internal/storage/settings.go`: persist all guardian child settings.
- Modify `app.go`: own the guardian supervisor, wrap runtime calls, coordinate scheduler pause/resume, expose Wails methods, and record account events.
- Modify `frontend/src/views/GuardView.tsx`: implement the confirmed unified console.
- Modify `frontend/src/App.tsx`: load, save, and refresh aggregate guardian state.
- Modify `frontend/src/style.css`: reuse existing global primitives for the unified console and responsive layout.
- Modify associated Go and frontend test files listed in each task.

### Task 1: Persist Complete Guardian Settings And Types

**Files:**
- Modify: `internal/runtime/guard/types.go`
- Modify: `internal/storage/settings.go`
- Modify: `internal/storage/storage_test.go`

- [ ] **Step 1: Write failing settings persistence tests**

Add a test that saves and reloads all child settings:

```go
func TestRuntimeSettingsPersistCompleteGuardianConfiguration(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil { t.Fatal(err) }
	defer store.Close()

	input := RuntimeSettings{
		ProcessGuardEnabled: true,
		ProcessGuardFailureRecoveryEnabled: true,
		ProcessGuardTimeoutThreshold: 3,
		ProcessGuardMonitorIntervalMS: 3000,
		ProcessGuardRestartReconnectGraceSec: 45,
		ProcessGuardMaxRestartsPer10Min: 4,
		ProcessGuardScheduledRestartEnabled: true,
		ProcessGuardScheduledRestartIntervalMin: 60,
		NetworkReconnectEnabled: true,
		NetworkReconnectIntervalMS: 1000,
		NetworkReconnectRecoveryTimeoutMS: 20000,
		OtherPlaceLoginReconnectEnabled: true,
		OtherPlaceLoginCheckIntervalMS: 5000,
		OtherPlaceLoginReconnectDelayMin: 5,
	}
	if err := store.SaveRuntimeSettings(ctx, input); err != nil { t.Fatal(err) }
	got, err := store.LoadRuntimeSettings(ctx)
	if err != nil { t.Fatal(err) }
	if !reflect.DeepEqual(guardianSettingsOnly(got), guardianSettingsOnly(input)) {
		t.Fatalf("guardian settings mismatch: got %#v want %#v", got, input)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storage -run TestRuntimeSettingsPersistCompleteGuardianConfiguration -count=1`

Expected: FAIL because the child setting fields and storage keys do not exist.

- [ ] **Step 3: Add bounded settings and aggregate status types**

Extend `guard.Settings` and add child status types:

```go
type Settings struct {
	Enabled                     bool `json:"enabled"`
	FailureRecoveryEnabled      bool `json:"failureRecoveryEnabled"`
	TimeoutThreshold            int  `json:"timeoutThreshold"`
	MonitorIntervalMS           int  `json:"monitorIntervalMs"`
	RestartReconnectGraceSec    int  `json:"restartReconnectGraceSec"`
	MaxRestartsPer10Min         int  `json:"maxRestartsPer10Min"`
	ScheduledRestartEnabled     bool `json:"scheduledRestartEnabled"`
	ScheduledRestartIntervalMin int  `json:"scheduledRestartIntervalMin"`
	NetworkReconnectEnabled     bool `json:"networkReconnectEnabled"`
	NetworkReconnectIntervalMS  int  `json:"networkReconnectIntervalMs"`
	NetworkRecoveryTimeoutMS    int  `json:"networkRecoveryTimeoutMs"`
	OtherPlaceLoginEnabled      bool `json:"otherPlaceLoginEnabled"`
	OtherPlaceLoginIntervalMS   int  `json:"otherPlaceLoginIntervalMs"`
	OtherPlaceLoginDelayMin     int  `json:"otherPlaceLoginDelayMin"`
}

type WorkerStatus struct {
	Enabled bool `json:"enabled"`
	Running bool `json:"running"`
	Busy bool `json:"busy"`
	LastCheckAt string `json:"lastCheckAt,omitempty"`
	LastHandledAt string `json:"lastHandledAt,omitempty"`
	LastResult string `json:"lastResult,omitempty"`
	LastError string `json:"lastError,omitempty"`
}

type RuntimeEvent struct {
	Name string `json:"name"`
	Phase string `json:"phase"`
	RuntimeTarget string `json:"runtimeTarget,omitempty"`
	AccountKey string `json:"accountKey,omitempty"`
	GID string `json:"gid,omitempty"`
	Handled bool `json:"handled,omitempty"`
	Via string `json:"via,omitempty"`
	FirstDetectedAt int64 `json:"firstDetectedAt,omitempty"`
	HandledAt int64 `json:"handledAt,omitempty"`
	RemainingMS int64 `json:"remainingMs,omitempty"`
	Error string `json:"error,omitempty"`
}

type SupervisorStatus struct {
	Enabled bool `json:"enabled"`
	Running bool `json:"running"`
	Phase Phase `json:"phase"`
	RuntimeTarget string `json:"runtimeTarget"`
	Process Status `json:"process"`
	Network WorkerStatus `json:"network"`
	OtherPlaceLogin WorkerStatus `json:"otherPlaceLogin"`
	RecentEvents []RuntimeEvent `json:"recentEvents"`
}

func DefaultSettings() Settings {
	return Settings{
		FailureRecoveryEnabled: true,
		TimeoutThreshold: 3,
		MonitorIntervalMS: 3000,
		RestartReconnectGraceSec: 45,
		MaxRestartsPer10Min: 4,
		ScheduledRestartIntervalMin: 60,
		NetworkReconnectEnabled: true,
		NetworkReconnectIntervalMS: 1000,
		NetworkRecoveryTimeoutMS: 20000,
		OtherPlaceLoginIntervalMS: 5000,
		OtherPlaceLoginDelayMin: 5,
	}
}
```

Add matching `storage.RuntimeSettings` fields and keys:

```go
"processGuard.failureRecoveryEnabled"
"networkReconnect.enabled"
"networkReconnect.intervalMs"
"networkReconnect.recoveryTimeoutMs"
"otherPlaceLoginReconnect.enabled"
"otherPlaceLoginReconnect.intervalMs"
"otherPlaceLoginReconnect.delayMin"
```

Normalize bounds in one helper: process interval `500..60000`, network interval `300..60000`, recovery timeout `1000..120000`, other-place interval `1000..60000`, delay `0..1440`.

- [ ] **Step 4: Run storage tests**

Run: `go test ./internal/storage -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/runtime/guard/types.go internal/storage/settings.go internal/storage/storage_test.go
git commit -m "feat: persist complete guardian settings"
```

### Task 2: Complete The Process Worker Timer And Scheduled Restart

**Files:**
- Modify: `internal/runtime/guard/manager.go`
- Modify: `internal/runtime/guard/manager_test.go`

- [ ] **Step 1: Write failing timer, grace, and circuit recovery tests**

Use an injected timer clock so tests do not sleep:

```go
func TestManagerRunsScheduledRestartWithoutBeingArmed(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	restarted := make(chan RestartReason, 1)
	m := NewManager(ManagerOptions{
		Settings: Settings{ScheduledRestartEnabled: true, ScheduledRestartIntervalMin: 1},
		Restart: func(reason RestartReason) (RestartResult, error) {
			restarted <- reason
			return RestartResult{Status: "launch_dispatched"}, nil
		},
		Now: clock.Now,
		After: clock.After,
	})
	m.Start(context.Background(), func() RuntimeSnapshot { return RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true} })
	defer m.Close()
	clock.Advance(time.Minute)
	select {
	case reason := <-restarted:
		if !reason.Scheduled { t.Fatalf("expected scheduled reason: %#v", reason) }
	case <-time.After(time.Second):
		t.Fatal("scheduled restart was not triggered")
	}
}

func TestManagerAutomaticallyClosesCircuitAfterWindowExpires(t *testing.T) {
	now := time.Unix(100, 0)
	m := NewManager(ManagerOptions{
		Settings: Settings{Enabled: true, FailureRecoveryEnabled: true, TimeoutThreshold: 1, MaxRestartsPer10Min: 1},
		Restart: func(RestartReason) (RestartResult, error) { return RestartResult{Status: "launch_dispatched"}, nil },
		Now: func() time.Time { return now },
	})
	m.Arm("test")
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	if !m.Status().CircuitOpen { t.Fatal("circuit did not open") }
	now = now.Add(11 * time.Minute)
	m.tick(now, RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	status := m.Status()
	if status.CircuitOpen || status.Phase != PhaseWatching {
		t.Fatalf("circuit did not recover: %#v", status)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/runtime/guard -run 'TestManagerRunsScheduledRestart|TestManagerAutomaticallyClosesCircuit' -count=1`

Expected: FAIL because `Start`, `Close`, `After`, scheduled execution, and automatic circuit recovery are missing.

- [ ] **Step 3: Implement a non-blocking process worker loop**

Add these dependencies and lifecycle methods:

```go
type ManagerOptions struct {
	Settings Settings
	Restart  func(RestartReason) (RestartResult, error)
	Now      func() time.Time
	After    func(time.Duration) <-chan time.Time
}

func (m *Manager) Start(ctx context.Context, snapshot func() RuntimeSnapshot) {
	m.mu.Lock()
	if m.cancel != nil { m.cancel() }
	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.snapshot = snapshot
	m.mu.Unlock()
	go m.loop(runCtx)
}

func (m *Manager) Close() {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	if cancel != nil { cancel() }
}
```

Move restart execution outside `m.mu`. Under the lock, reserve the recovery and capture the request; in a goroutine call `m.restart(reason)`, then reacquire the lock to record success/failure. The loop must:

Add `LastCheckAt string \`json:"lastCheckAt,omitempty"\`` and `ReconnectGraceRemainingMS int64 \`json:"reconnectGraceRemainingMs"\`` to `guard.Status`.

```go
func (m *Manager) tick(now time.Time, snapshot RuntimeSnapshot) {
	m.status.LastCheckAt = now.UTC().Format(time.RFC3339)
	m.trimRestartWindow(now)
	if m.status.CircuitOpen && len(m.restartTimes) < m.settings.MaxRestartsPer10Min {
		m.status.CircuitOpen = false
		m.status.LastActionError = ""
		m.status.Phase = PhaseWatching
	}
	m.updateReconnectGraceLocked(now, snapshot)
	m.maybeScheduleRestartLocked(now, snapshot)
}
```

Scheduled restart must run when its child switch is enabled even if failure recovery is not armed. It must use the same restart quota, recovery reservation, event recording, and reconnect grace.

- [ ] **Step 4: Run guard tests**

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS, including the existing threshold and circuit tests.

- [ ] **Step 5: Commit**

```powershell
git add internal/runtime/guard/manager.go internal/runtime/guard/manager_test.go
git commit -m "feat: run guardian process monitor and scheduled restart"
```

### Task 3: Add Recovery Coordination And Runtime Generations

**Files:**
- Create: `internal/runtime/guard/coordinator.go`
- Create: `internal/runtime/guard/coordinator_test.go`

- [ ] **Step 1: Write failing priority and stale-generation tests**

```go
func TestCoordinatorSerializesRecoveryAndPrefersRuntimeLocalWork(t *testing.T) {
	started := make(chan RecoveryKind, 3)
	release := make(chan struct{})
	c := NewRecoveryCoordinator(CoordinatorOptions{Run: func(ctx context.Context, request RecoveryRequest) RecoveryResult {
		started <- request.Kind
		<-release
		return RecoveryResult{OK: true}
	}})
	defer c.Close()
	c.Submit(RecoveryRequest{Kind: RecoveryProcessRestart})
	c.Submit(RecoveryRequest{Kind: RecoveryOtherPlaceLogin})
	c.Submit(RecoveryRequest{Kind: RecoveryNetworkReconnect})
	if got := <-started; got != RecoveryNetworkReconnect {
		t.Fatalf("first recovery = %s", got)
	}
	close(release)
}

func TestCoordinatorRejectsResultsFromOlderGeneration(t *testing.T) {
	c := NewRecoveryCoordinator(CoordinatorOptions{})
	old := c.Generation()
	c.AdvanceGeneration("process_restart")
	if c.IsCurrent(old) { t.Fatal("old generation remained current") }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/runtime/guard -run 'TestCoordinator' -count=1`

Expected: FAIL because the coordinator does not exist.

- [ ] **Step 3: Implement coordinator queue and generation API**

```go
type RecoveryKind string
const (
	RecoveryNetworkReconnect RecoveryKind = "network_reconnect"
	RecoveryOtherPlaceLogin  RecoveryKind = "other_place_login"
	RecoveryProcessRestart   RecoveryKind = "process_restart"
)

type RecoveryRequest struct {
	Kind RecoveryKind
	Reason string
	RuntimeTarget string
	RequestedAt time.Time
}

type RecoveryResult struct {
	OK bool
	Kind RecoveryKind
	Error string
}

var ErrStaleRuntimeGeneration = errors.New("stale runtime generation")

type RecoveryCoordinator struct {
	mu sync.Mutex
	generation uint64
	pending map[RecoveryKind]RecoveryRequest
	wake chan struct{}
	cancel context.CancelFunc
	run func(context.Context, RecoveryRequest) RecoveryResult
	onPause func(bool)
}
```

Implement a single worker that selects network, then other-place login, then process restart. Coalesce duplicate requests by kind. Call `onPause(true)` only for process restart and always restore it with `defer onPause(false)`. Expose `Generation`, `AdvanceGeneration`, and `IsCurrent`.

- [ ] **Step 4: Run coordinator tests**

Run: `go test ./internal/runtime/guard -run TestCoordinator -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/runtime/guard/coordinator.go internal/runtime/guard/coordinator_test.go
git commit -m "feat: coordinate guardian recovery actions"
```

### Task 4: Forward Guardian Runtime Events On QQ And WMPF

**Files:**
- Modify: `internal/runtime/qqws/adapter.go`
- Modify: `internal/runtime/qqws/adapter_test.go`
- Modify: `internal/runtime/cdp/client.go`
- Modify: `internal/runtime/cdp/client_test.go`
- Modify: `internal/runtime/wmpf/link.go`
- Modify: `internal/runtime/wmpf/link_test.go`

- [ ] **Step 1: Write failing event subscription tests**

For QQ, register a callback, pass a generic `MessageEvent` through `handleEvent`, and assert the payload arrives unchanged:

```go
func TestAdapterForwardsGenericRuntimeEvent(t *testing.T) {
	adapter := &Adapter{}
	received := make(chan map[string]any, 1)
	adapter.OnRuntimeEvent(func(event map[string]any) { received <- event })
	adapter.handleEvent(&clientSession{id: "test"}, Packet{
		Type: MessageEvent,
		Payload: map[string]any{"name": "other_place_login_reconnect", "phase": "waiting"},
	})
	select {
	case event := <-received:
		if event["name"] != "other_place_login_reconnect" || event["phase"] != "waiting" { t.Fatalf("%#v", event) }
	case <-time.After(time.Second):
		t.Fatal("runtime event was not forwarded")
	}
}
```

For CDP, feed a `Runtime.bindingCalled` event named `__qqFarmRuntimeEventBridge` with JSON payload and assert the WMPF link forwards the decoded event.

- [ ] **Step 2: Run focused tests to verify they fail**

Run: `go test ./internal/runtime/qqws ./internal/runtime/cdp ./internal/runtime/wmpf -run 'RuntimeEvent|BindingEvent' -count=1`

Expected: FAIL because generic event subscriptions and binding forwarding do not exist.

- [ ] **Step 3: Add transport-neutral runtime event hooks**

In QQ adapter:

```go
func (a *Adapter) OnRuntimeEvent(handler func(map[string]any)) {
	a.mu.Lock()
	a.runtimeEventHandlers = append(a.runtimeEventHandlers, handler)
	a.mu.Unlock()
}
```

Copy the handler slice before invoking it so callbacks never run under adapter locks.

In CDP client, add `OnEvent(func(method string, params map[string]any))` and call listeners after internal context bookkeeping. In WMPF setup, execute:

```go
_, err := client.Send(ctx, "Runtime.addBinding", map[string]any{"name": "__qqFarmRuntimeEventBridge"}, 3*time.Second)
```

Then inject:

```javascript
globalThis.__qqFarmRuntimeEventBridge = function (event) {
  globalThis.__qqFarmRuntimeEventBridge(JSON.stringify(event));
};
```

Use distinct native binding and JavaScript wrapper names to avoid recursion, for example native `__qqFarmRuntimeEventBinding` and wrapper `__qqFarmRuntimeEventBridge`.

- [ ] **Step 4: Run runtime transport tests**

Run: `go test ./internal/runtime/qqws ./internal/runtime/cdp ./internal/runtime/wmpf -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/runtime/qqws internal/runtime/cdp internal/runtime/wmpf
git commit -m "feat: forward guardian runtime events"
```

### Task 5: Complete Runtime-Local Watcher Events And Idempotency

**Files:**
- Modify: `resources/wmpf/button.js`
- Create: `scripts/test-guardian-runtime-watchers.js`

- [ ] **Step 1: Add a failing deterministic watcher test**

The test loads `button.js` in a VM harness with fake timers and fake UI components. Assert:

```javascript
assert.equal(gameCtl.startReconnectWatcher({ intervalMs: 1000 }).running, true);
assert.equal(gameCtl.startReconnectWatcher({ intervalMs: 1000 }).running, true, "restart is idempotent");
clock.tick(1000);
assert.deepEqual(events.at(-1), {
  name: "network_reconnect",
  phase: "reconnected",
  handled: true,
  via: "server_kickout_ok_node",
});
```

Also assert other-place-login countdown survives a temporarily inactive prompt and handles once when due.

- [ ] **Step 2: Run the script to verify it fails**

Run: `node scripts/test-guardian-runtime-watchers.js`

Expected: FAIL because network reconnect does not emit lifecycle events and the state payload lacks an explicit enabled field.

- [ ] **Step 3: Extend watcher state and lifecycle events**

Add `enabled` and `generation` handling to the network watcher, matching the existing other-place worker. Emit:

```javascript
emitFarmRuntimeEvent('network_reconnect', {
  phase: result && result.handled ? 'reconnected' : 'failed',
  handled: !!(result && result.handled),
  via: result && result.via || null,
  error: result && result.error || null,
  checkedAt: Date.now(),
});
```

Add `setReconnectWatcherEnabled(enabled, opts)` and return complete state from both workers. Preserve existing exported method names for compatibility.

- [ ] **Step 4: Run the watcher script and JavaScript syntax check**

Run: `node scripts/test-guardian-runtime-watchers.js`

Run: `node --check resources/wmpf/button.js`

Expected: both commands PASS.

- [ ] **Step 5: Commit**

```powershell
git add resources/wmpf/button.js scripts/test-guardian-runtime-watchers.js
git commit -m "feat: complete runtime guardian watchers"
```

### Task 6: Build The Guardian Supervisor And Runtime Synchronization

**Files:**
- Create: `internal/runtime/guard/supervisor.go`
- Create: `internal/runtime/guard/supervisor_test.go`

- [ ] **Step 1: Write failing lifecycle and synchronization tests**

```go
func TestSupervisorResynchronizesWorkersWheneverRuntimeBecomesReady(t *testing.T) {
	caller := &fakeGuardianCaller{}
	s := NewSupervisor(SupervisorOptions{Caller: caller, Settings: DefaultSettings()})
	s.Start(context.Background())
	defer s.Close()
	s.OnRuntimeStatus(RuntimeSnapshot{RuntimeTarget: "qq_ws", Connected: true, Ready: true})
	if !caller.called("gameCtl.setReconnectWatcherEnabled") || !caller.called("gameCtl.setOtherPlaceLoginReconnectEnabled") {
		t.Fatalf("missing worker sync: %#v", caller.calls)
	}
}

func TestSupervisorRunsWhenAutomationSchedulerIsStopped(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	process := NewManager(ManagerOptions{Settings: Settings{Enabled: true, FailureRecoveryEnabled: true}, Now: clock.Now, After: clock.After})
	s := NewSupervisor(SupervisorOptions{
		Settings: DefaultSettings(),
		Process: process,
		Snapshot: func() RuntimeSnapshot { return RuntimeSnapshot{RuntimeTarget: "qq_ws", Connected: true, Ready: true} },
		After: clock.After,
	})
	s.Start(context.Background())
	defer s.Close()
	clock.Advance(3 * time.Second)
	status := s.Status()
	if status.Process.LastHealthyAt == "" || status.Phase != PhaseWatching {
		t.Fatalf("guardian did not run independently: %#v", status)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/runtime/guard -run TestSupervisor -count=1`

Expected: FAIL because `Supervisor` does not exist.

- [ ] **Step 3: Implement supervisor lifecycle and sync retry**

```go
type SupervisorOptions struct {
	Settings Settings
	Process *Manager
	Coordinator *RecoveryCoordinator
	Caller RuntimeCaller
	Snapshot func() RuntimeSnapshot
	OnEvent func(RuntimeEvent)
	After func(time.Duration) <-chan time.Time
}

type RuntimeCaller interface {
	Call(context.Context, string, []any, time.Duration) (any, error)
}

func (s *Supervisor) OnRuntimeStatus(snapshot RuntimeSnapshot) {
	s.process.OnRuntimeStatus(snapshot)
	if snapshot.Ready {
		s.coordinator.AdvanceGeneration("runtime_ready")
		s.scheduleSync(0)
	}
}
```

Synchronization calls:

```go
gameCtl.setReconnectWatcherEnabled(enabled, {
  intervalMs: settings.NetworkReconnectIntervalMS,
  recoverTimeoutMs: settings.NetworkRecoveryTimeoutMS
})
gameCtl.setOtherPlaceLoginReconnectEnabled(enabled, {
  intervalMs: settings.OtherPlaceLoginIntervalMS,
  reconnectDelayMs: settings.OtherPlaceLoginDelayMin * 60 * 1000
})
```

Retry only readiness/missing-context failures with delays `800ms, 1600ms, 3200ms, 5000ms`; cancel retries on generation change or shutdown.

- [ ] **Step 4: Run all guard tests**

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/runtime/guard/supervisor.go internal/runtime/guard/supervisor_test.go
git commit -m "feat: supervise concurrent guardian workers"
```

### Task 7: Integrate Guardian Lifecycle, Calls, Events, And Scheduler Pause In App

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`

- [ ] **Step 1: Write failing App integration tests**

Cover four behaviors:

```go
func TestAppStartsGuardianIndependentlyFromAutomationScheduler(t *testing.T) {
	app := NewApp()
	app.startup(context.Background())
	defer app.shutdown(context.Background())
	if app.automationScheduler != nil && app.automationScheduler.State().Running { t.Fatal("scheduler unexpectedly running") }
	if app.guardian == nil || !app.guardian.Status().Running { t.Fatal("guardian is not running") }
}

func TestAppRuntimeCallerReportsRestartableErrorsToGuardian(t *testing.T) {
	base := fakeRuntimeCaller{err: context.DeadlineExceeded}
	guardian := newTestGuardian(t)
	caller := guardianRuntimeCaller{base: base, guardian: guardian, generation: guardian.Generation}
	_, _ = caller.Call(context.Background(), "gameCtl.getFarmStatus", nil, time.Second)
	if guardian.Status().Process.TimeoutStreak != 1 { t.Fatalf("%#v", guardian.Status()) }
}

func TestAppProcessRecoveryPausesAndResumesAutomationDispatch(t *testing.T) {
	app := NewApp()
	app.setGuardianDispatchPaused(true)
	blocked := app.executeFarmAutomationTaskForAccount(context.Background(), "own_collect", "auto", "default")
	if blocked.Status != automation.StatusRuntimeBusy { t.Fatalf("%#v", blocked) }
	app.setGuardianDispatchPaused(false)
	if app.guardianDispatchIsPaused() { t.Fatal("dispatch remained paused") }
}

func TestAppRecordsOtherPlaceLoginEventsForCurrentAccount(t *testing.T) {
	app := newAppWithTempStore(t)
	app.currentAccountKey = "gid:10001"
	app.handleGuardianRuntimeEvent(guard.RuntimeEvent{Name: "other_place_login_reconnect", Phase: "waiting", RemainingMS: 60000})
	events, err := app.store.ListRuntimeEventsForAccount(context.Background(), "gid:10001", 10)
	if err != nil { t.Fatal(err) }
	if len(events) != 1 || events[0].Type != "guardian.other_place_login.waiting" { t.Fatalf("%#v", events) }
}
```

Add `fakeRuntimeCaller`, `newTestGuardian`, and `newAppWithTempStore` next to the tests; each helper must construct only local fakes and a temporary SQLite store. Extend the pause test with a blocking runner in the final integration task.

- [ ] **Step 2: Run focused App tests to verify they fail**

Run: `go test . -run 'TestAppStartsGuardian|TestAppRuntimeCallerReports|TestAppProcessRecovery|TestAppRecordsOtherPlace' -count=1`

Expected: FAIL because App still owns only `guard.Manager` and does not route runtime events.

- [ ] **Step 3: Replace direct manager ownership with the supervisor**

Add fields:

```go
guardian *guard.Supervisor
guardianCoordinator *guard.RecoveryCoordinator
guardianPauseMu sync.RWMutex
guardianDispatchPaused bool
```

Start the guardian after settings load and before runtime autostart. Close it before stopping the runtime supervisor. Route `manager.OnStatusChange` to `guardian.OnRuntimeStatus`.

Wrap the base runtime caller before the cache:

```go
type guardianRuntimeCaller struct {
	base automation.RuntimeCaller
	guardian *guard.Supervisor
	generation func() uint64
}

func (c guardianRuntimeCaller) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	generation := c.generation()
	value, err := c.base.Call(ctx, method, args, timeout)
	if err != nil { c.guardian.NoteRuntimeError(err); return nil, err }
	c.guardian.NoteHealthy()
	if !c.guardian.IsCurrentGeneration(generation) { return nil, guard.ErrStaleRuntimeGeneration }
	return value, nil
}
```

Before scheduled task execution, return `StatusRuntimeBusy` while dispatch is paused. Existing running calls are canceled through the restart context; stale results are rejected by generation.

Register QQ and WMPF event handlers and translate `network_reconnect` and `other_place_login_reconnect` into account-scoped `eventbus.Event` records.

- [ ] **Step 4: Run App and package tests**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add app.go app_test.go
git commit -m "feat: integrate guardian service lifecycle"
```

### Task 8: Expose Unified Wails API And Build The Confirmed Console

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`
- Modify: `frontend/src/views/GuardView.tsx`
- Modify: `frontend/src/views/GuardView.test.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/App.test.tsx`
- Modify: `frontend/src/style.css`
- Regenerate: `frontend/wailsjs/go/main/App.js`
- Regenerate: `frontend/wailsjs/go/main/App.d.ts`
- Regenerate: `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Write failing API and rendering tests**

Add App API tests for aggregate settings/status and child switches:

```go
func TestSaveGuardianSettingsUpdatesAllWorkers(t *testing.T) {
	app := newTestApp(t)
	_, err := app.SaveGuardianSettings(map[string]any{
		"enabled": true,
		"failureRecoveryEnabled": true,
		"networkReconnectEnabled": true,
		"otherPlaceLoginEnabled": true,
	})
	if err != nil { t.Fatal(err) }
	status := app.GuardianStatus()
	if !status.Enabled || !status.Network.Enabled || !status.OtherPlaceLogin.Enabled { t.Fatalf("%#v", status) }
}
```

Frontend tests must assert the unified page contains:

```tsx
expect(html).toContain('守护服务');
expect(html).toContain('进程异常与定时重启');
expect(html).toContain('网络异常即时重连');
expect(html).toContain('异地登录延时重连');
expect(html).toContain('守护事件');
```

- [ ] **Step 2: Run focused tests to verify they fail**

Run: `go test . -run TestSaveGuardianSettingsUpdatesAllWorkers -count=1`

Run: `npm test -- --run src/views/GuardView.test.tsx src/App.test.tsx`

Workdir for npm command: `frontend`

Expected: FAIL because the aggregate API and unified sections do not exist.

- [ ] **Step 3: Implement aggregate API and unified console**

Expose:

```go
func (a *App) GuardianStatus() guard.SupervisorStatus
func (a *App) SaveGuardianSettings(input map[string]any) (guard.Settings, error)
func (a *App) RunGuardianAction(input guard.ActionRequest) guard.ActionResult
```

Keep `ProcessGuardStatus` and `SaveProcessGuardSettings` as compatibility wrappers until all frontend callers use the new API.

Build `GuardView` as one page with:

- header master switch and aggregate phase;
- five-column summary strip;
- process/scheduled section with advanced settings disclosure;
- network section;
- other-place-login section with countdown;
- unified filtered event table.

Reuse existing `primary-button`, `secondary-button`, `icon-button`, `status-badge`, `settings-form`, `system-log-table`, spacing, typography, and CSS variables. Add only guardian-specific layout classes. At widths below the existing mobile breakpoint, summary and metrics become one column and event rows become horizontally scrollable rather than overlapping.

- [ ] **Step 4: Regenerate bindings and run frontend tests**

Run: `wails generate module`

Run: `npm test -- --run`

Run: `npm run build`

Workdir for npm commands: `frontend`

Expected: tests and production build PASS.

- [ ] **Step 5: Commit**

```powershell
git add app.go app_test.go frontend/src frontend/wailsjs
git commit -m "feat: add unified guardian console"
```

### Task 9: Run Concurrency Regression And End-To-End Verification

**Files:**
- Create: `internal/runtime/guard/integration_test.go`
- Modify: `README.md`

- [ ] **Step 1: Add deterministic integration regressions**

Create tests using fake callers, fake clocks, and blocking channels for these sequences:

```go
func TestGuardianDetectsNetworkPromptDuringLongAutomationTask(t *testing.T) {
	h := newGuardianIntegrationHarness(t)
	h.startLongTask()
	h.emit(RuntimeEvent{Name: "network_reconnect", Phase: "detected"})
	if got := h.nextRecovery(); got != RecoveryNetworkReconnect { t.Fatalf("recovery=%s", got) }
	if !h.longTaskRunning() { t.Fatal("detection waited for the task to finish") }
}

func TestGuardianHandlesDueOtherPlaceLoginDuringAutomationTask(t *testing.T) {
	h := newGuardianIntegrationHarness(t)
	h.startLongTask()
	h.emit(RuntimeEvent{Name: "other_place_login_reconnect", Phase: "due"})
	if got := h.nextRecovery(); got != RecoveryOtherPlaceLogin { t.Fatalf("recovery=%s", got) }
}

func TestGuardianProcessRestartInvalidatesOldTaskResult(t *testing.T) {
	h := newGuardianIntegrationHarness(t)
	oldGeneration := h.coordinator.Generation()
	h.startLongTask()
	h.submit(RecoveryRequest{Kind: RecoveryProcessRestart})
	h.finishRecovery()
	if h.coordinator.IsCurrent(oldGeneration) { t.Fatal("old generation remained valid") }
	if err := h.finishLongTask(); !errors.Is(err, ErrStaleRuntimeGeneration) { t.Fatalf("err=%v", err) }
}

func TestGuardianContinuesWhenAutomationSchedulerStopped(t *testing.T) {
	h := newGuardianIntegrationHarness(t)
	h.stopScheduler()
	h.advance(3 * time.Second)
	if !h.supervisor.Status().Running || h.supervisor.Status().Process.LastCheckAt == "" {
		t.Fatalf("guardian stopped with scheduler: %#v", h.supervisor.Status())
	}
}
```

Implement `newGuardianIntegrationHarness` in the same file with fake clock channels, a blocking runtime caller, a captured recovery channel, a real `RecoveryCoordinator`, and a real `Supervisor`. Its methods must expose only the operations used above and clean up all goroutines through `t.Cleanup`.

- [ ] **Step 2: Run the integration tests**

Run: `go test ./internal/runtime/guard -run 'TestGuardian' -race -count=1`

Expected: PASS with no race reports.

- [ ] **Step 3: Document operational behavior**

Add a concise README section listing the master switch, three child capabilities, defaults, safe bound-process restart rule, automatic circuit recovery, and the fact that the guardian is independent from the automation scheduler.

- [ ] **Step 4: Run the full verification suite**

Run: `go test ./... -race -count=1`

Run: `node scripts/test-guardian-runtime-watchers.js`

Run: `npm test -- --run`

Run: `npm run build`

Workdir for npm commands: `frontend`

Expected: every command exits `0`; Go race detector reports no races; frontend build produces `frontend/dist`.

- [ ] **Step 5: Commit**

```powershell
git add internal/runtime/guard/integration_test.go README.md
git commit -m "test: verify guardian concurrency and recovery"
```

## Final Manual Verification

- [ ] Enable the master switch with automation scheduler stopped; verify all enabled child workers remain active.
- [ ] Start a long farm task and display a network error prompt; verify prompt recovery begins immediately.
- [ ] Enable other-place-login recovery with a short test delay; verify countdown and one-time reconnect.
- [ ] Trigger repeated restartable failures; verify threshold, grace period, four-per-ten-minute circuit, and automatic circuit recovery.
- [ ] Enable scheduled restart; verify only the bound miniapp host restarts and Farm_Go remains running.
- [ ] Verify QQ WS, WeChat CDP, and YYB CDP reconnect and resynchronize both runtime-local watchers.
- [ ] Verify the unified console matches global styling and has no overlap at desktop and narrow widths.
