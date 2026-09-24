# Guardian Restart Auto-Minimize Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a persisted guardian switch that minimizes only a restarted mini-program host after its runtime is ready and connected.

**Architecture:** Store a false-by-default option in global runtime settings and expose it through the existing guardian save/status API. `App` records a mutex-protected, one-shot pending request after a guardian restart dispatches its launch, then consumes it after the matching runtime reports `Ready && Connected`. The app retrieves current windows from its newly auto-bound host and delegates native minimization to the guard package.

**Tech Stack:** Go, Wails, SQLite, `golang.org/x/sys/windows`, React, TypeScript, Vitest.

---

## File Structure

- `internal/storage/settings.go`: persisted `ProcessGuardAutoMinimizeAfterRestart` setting.
- `internal/storage/storage_test.go`: round-trip and legacy-default coverage.
- `internal/runtime/guard/types.go`, `manager.go`: guardian setting and process-status field returned to the UI.
- `internal/runtime/guard/host.go`, `process_windows.go`, `process_other.go`, `host_test.go`: valid-handle filtering and platform minimization API.
- `app.go`, `app_test.go`: setting propagation, restart queue, stable-status consumption, and events.
- `frontend/src/views/GuardView.tsx`, `GuardView.test.tsx`: process-guardian switch and rendering coverage.

### Task 1: Persist the setting

**Files:**
- Modify: `internal/storage/settings.go:19-225`
- Modify: `internal/storage/storage_test.go:298-420`

- [ ] **Step 1: Write the failing storage assertions**

```go
func TestRuntimeSettingsPersistCompleteGuardianConfiguration(t *testing.T) {
    input.ProcessGuardAutoMinimizeAfterRestart = true
    if err := store.SaveRuntimeSettings(ctx, input); err != nil {
        t.Fatalf("save settings: %v", err)
    }
    got, err := store.LoadRuntimeSettings(ctx)
    if err != nil || !got.ProcessGuardAutoMinimizeAfterRestart {
        t.Fatalf("got = %#v, err = %v", got, err)
    }
}

func TestRuntimeSettingsGuardianDefaults(t *testing.T) {
    got, err := store.LoadRuntimeSettings(ctx)
    if err != nil || got.ProcessGuardAutoMinimizeAfterRestart {
        t.Fatalf("got = %#v, err = %v", got, err)
    }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/storage -run 'TestRuntimeSettingsPersistCompleteGuardianConfiguration|TestRuntimeSettingsGuardianDefaults' -count=1`

Expected: FAIL because `RuntimeSettings` has no `ProcessGuardAutoMinimizeAfterRestart` field.

- [ ] **Step 3: Implement the setting model and storage mapping**

```go
ProcessGuardAutoMinimizeAfterRestart bool `json:"processGuardAutoMinimizeAfterRestart"`

// Add this key to LoadRuntimeSettings' requested keys and parser.
"processGuard.autoMinimizeAfterRestart",
if value := values["processGuard.autoMinimizeAfterRestart"]; value != "" {
    settings.ProcessGuardAutoMinimizeAfterRestart = value == "true"
}

// Add this entry to SaveRuntimeSettings' values map.
"processGuard.autoMinimizeAfterRestart": strconv.FormatBool(settings.ProcessGuardAutoMinimizeAfterRestart),
```

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/storage -run 'TestRuntimeSettingsPersistCompleteGuardianConfiguration|TestRuntimeSettingsGuardianDefaults' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/storage/settings.go internal/storage/storage_test.go
git commit -m "feat: persist guardian restart minimize setting"
```

### Task 2: Add a focused host-window minimize boundary

**Files:**
- Modify: `internal/runtime/guard/host.go:1-40`
- Modify: `internal/runtime/guard/process_windows.go:18-42`
- Modify: `internal/runtime/guard/process_other.go:5-15`
- Modify: `internal/runtime/guard/host_test.go:1-92`

- [ ] **Step 1: Write the failing platform-neutral eligibility test**

```go
func TestMinimizableWindowHandlesUsesOnlyVisibleWindowsWithHandles(t *testing.T) {
    got := minimizableWindowHandles([]HostWindowSnapshot{
        {HWND: 10, Visible: true},
        {HWND: 0, Visible: true},
        {HWND: 11, Visible: false},
        {HWND: 12, Visible: true},
    })
    want := []uint64{10, 12}
    if !reflect.DeepEqual(got, want) {
        t.Fatalf("handles = %#v, want %#v", got, want)
    }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/runtime/guard -run TestMinimizableWindowHandlesUsesOnlyVisibleWindowsWithHandles -count=1`

Expected: FAIL because `minimizableWindowHandles` is undefined.

- [ ] **Step 3: Implement filtering plus Windows/non-Windows functions**

```go
// host.go
func minimizableWindowHandles(snapshots []HostWindowSnapshot) []uint64 {
    handles := make([]uint64, 0, len(snapshots))
    for _, snapshot := range snapshots {
        if snapshot.Visible && snapshot.HWND != 0 {
            handles = append(handles, snapshot.HWND)
        }
    }
    return handles
}

// process_windows.go
const swMinimize = 6

func MinimizeHostWindows(snapshots []HostWindowSnapshot) error {
    for _, handle := range minimizableWindowHandles(snapshots) {
        windows.ShowWindow(windows.HWND(handle), swMinimize)
    }
    return nil
}

// process_other.go
func MinimizeHostWindows([]HostWindowSnapshot) error { return errors.New("unsupported_platform") }
```

Avoid shadowing the imported `windows` package with a parameter. The app must pass only current windows for its confirmed binding, never title-matched windows.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/runtime/guard -run 'TestMinimizableWindowHandlesUsesOnlyVisibleWindowsWithHandles|TestRestartTerminatesOnlyBoundPID' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/runtime/guard/host.go internal/runtime/guard/host_test.go internal/runtime/guard/process_windows.go internal/runtime/guard/process_other.go
git commit -m "feat: add host window minimize helper"
```

### Task 3: Defer minimization until the restarted runtime is stable

**Files:**
- Modify: `app.go:48-115`, `app.go:2617-2687`, `app.go:2812-2829`, `app.go:3006-3056`, `app.go:3138-3153`, `app.go:3218-3255`
- Modify: `app_test.go` near `TestAppManualRestartRecordsRestartEvent` and `TestSaveGuardianSettingsUpdatesAllWorkers`
- Modify: `internal/runtime/guard/types.go:36-105`
- Modify: `internal/runtime/guard/manager.go:41-92,591-630`

- [ ] **Step 1: Write the failing app tests**

```go
func TestSaveGuardianSettingsPersistsAutoMinimizeAfterRestart(t *testing.T) {
    app := newAuthorizedTestApp(t)
    saved, err := app.SaveGuardianSettings(map[string]any{"autoMinimizeAfterRestart": true})
    if err != nil || !saved.AutoMinimizeAfterRestart {
        t.Fatalf("saved = %#v, err = %v", saved, err)
    }
}

func TestGuardianRestartMinimizesNewBoundHostAfterRuntimeIsStable(t *testing.T) {
    app := newAuthorizedTestApp(t)
    if _, err := app.SaveGuardianSettings(map[string]any{"autoMinimizeAfterRestart": true}); err != nil {
        t.Fatalf("enable auto minimize: %v", err)
    }
    app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
        return []guard.HostProcessSnapshot{{
            PID: 20, ProcessName: "QQ.exe",
            Windows: []guard.HostWindowSnapshot{{PID: 20, HWND: 42, Title: "QQ经典农场", Visible: true}},
        }}, nil
    }
    app.stopHostPID = func(int) error { return nil }
    app.launchHost = func(guard.LaunchRequest) error { return nil }
    var minimized []guard.HostWindowSnapshot
    calls := 0
    app.minimizeHostWindows = func(windows []guard.HostWindowSnapshot) error {
        minimized = append([]guard.HostWindowSnapshot(nil), windows...)
        calls++
        return nil
    }
    if _, err := app.restartHostForRuntimeTarget(string(farmruntime.RuntimeTargetQQWS)); err != nil {
        t.Fatalf("restart host: %v", err)
    }
    app.recordProcessGuardRuntimeStatus(farmruntime.Status{Target: "qq_ws", Ready: true})
    if calls != 0 { t.Fatalf("minimized before stable: %d calls", calls) }
    app.recordProcessGuardRuntimeStatus(farmruntime.Status{Target: "qq_ws", Ready: true, Connected: true})
    if calls != 1 || len(minimized) != 1 || minimized[0].HWND != 42 {
        t.Fatalf("calls = %d, windows = %#v", calls, minimized)
    }
    app.recordProcessGuardRuntimeStatus(farmruntime.Status{Target: "qq_ws", Ready: true, Connected: true})
    if calls != 1 { t.Fatalf("minimize calls = %d, want 1", calls) }
}

func TestGuardianRestartDoesNotQueueMinimizeWhenDisabledOrLaunchFails(t *testing.T) {
    for _, test := range []struct { name string; enabled bool; launchErr error }{
        {name: "disabled", enabled: false},
        {name: "launch fails", enabled: true, launchErr: errors.New("launch failed")},
    } {
        t.Run(test.name, func(t *testing.T) {
            app := newAuthorizedTestApp(t)
            if test.enabled {
                if _, err := app.SaveGuardianSettings(map[string]any{"autoMinimizeAfterRestart": true}); err != nil {
                    t.Fatalf("enable auto minimize: %v", err)
                }
            }
            app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
                return []guard.HostProcessSnapshot{{PID: 20, ProcessName: "QQ.exe", Windows: []guard.HostWindowSnapshot{{PID: 20, HWND: 42, Title: "QQ经典农场", Visible: true}}}}, nil
            }
            app.stopHostPID = func(int) error { return nil }
            app.launchHost = func(guard.LaunchRequest) error { return test.launchErr }
            calls := 0
            app.minimizeHostWindows = func([]guard.HostWindowSnapshot) error { calls++; return nil }
            _, _ = app.restartHostForRuntimeTarget(string(farmruntime.RuntimeTargetQQWS))
            app.recordProcessGuardRuntimeStatus(farmruntime.Status{Target: "qq_ws", Ready: true, Connected: true})
            if calls != 0 { t.Fatalf("minimize calls = %d, want 0", calls) }
        })
    }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test . -run 'TestSaveGuardianSettingsPersistsAutoMinimizeAfterRestart|TestGuardianRestartMinimizesNewBoundHostAfterRuntimeIsStable|TestGuardianRestartDoesNotQueueMinimizeWhenDisabledOrLaunchFails' -count=1`

Expected: FAIL because the setting, pending request, and minimizer dependency do not exist.

- [ ] **Step 3: Implement the smallest complete flow**

```go
type pendingHostMinimize struct{ runtimeTarget string }

type App struct {
    pendingHostMinimizeMu sync.Mutex
    pendingHostMinimize   *pendingHostMinimize
    minimizeHostWindows   func([]guard.HostWindowSnapshot) error
}

// NewApp:
minimizeHostWindows: guard.MinimizeHostWindows,

func (a *App) recordProcessGuardRuntimeStatus(status farmruntime.Status) {
    if status.Ready && status.Connected {
        a.consumePendingHostMinimize(status.Target)
    }
    if a.guardian != nil {
        a.guardian.OnRuntimeStatus(guardianSnapshotFromRuntimeStatus(status))
    }
}
```

Extend `guard.Settings` and `guard.Status`, then have `Manager.Status` copy the setting to its returned process status. Extend `SaveGuardianSettings`, `SaveProcessGuardSettings`, `runtimeSettingsAffectRuntime`, `runtimeSettingsFromConfig`, `guardSettingsFromRuntimeSettings`, and `processGuardSettingsMap` with the same boolean. In `restartHostForRuntimeTarget`, register the pending request only when the result is `launch_dispatched` and the saved setting is true. `consumePendingHostMinimize` must atomically take only a matching target, call `AutoBindHostProcess`, refresh host snapshots, select the current binding PID's visible windows, invoke `minimizeHostWindows` once, and record `guardian.restart_window_minimize` as an info/error event. Clearing the setting must clear any pending request.

- [ ] **Step 4: Verify GREEN and regressions**

Run: `go test . -run 'TestSaveGuardianSettingsPersistsAutoMinimizeAfterRestart|TestGuardianRestartMinimizesNewBoundHostAfterRuntimeIsStable|TestGuardianRestartDoesNotQueueMinimizeWhenDisabledOrLaunchFails|TestAppManualRestartRecordsRestartEvent|TestAppRestartWaitsBeforeRelaunchingMiniapp|TestSaveGuardianSettingsUpdatesAllWorkers' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add app.go app_test.go internal/runtime/guard/types.go internal/runtime/guard/manager.go
git commit -m "feat: minimize restarted guardian host after reconnect"
```

### Task 4: Render the dedicated switch

**Files:**
- Modify: `frontend/src/views/GuardView.tsx:1-150`
- Modify: `frontend/src/views/GuardView.test.tsx:1-70`

- [ ] **Step 1: Write the failing view test**

```tsx
it('renders the restart auto-minimize switch in the process guardian section', () => {
  const html = renderToString(
    <GuardView
      status={{
        enabled: true, phase: 'watching', restartCountInWindow: 0,
        maxRestartsPerWindow: 4, recentRestartEvents: [],
        process: { autoMinimizeAfterRestart: true },
      }}
      onRefresh={() => undefined}
      onLaunch={() => undefined}
      onRestart={() => undefined}
      onSaveSettings={() => undefined}
    />,
  );
  expect(html).toContain('重启后自动最小化窗口');
});
```

- [ ] **Step 2: Verify RED**

Run: `npm test -- --run src/views/GuardView.test.tsx`

Working directory: `frontend`

Expected: FAIL because the label and process DTO field do not exist.

- [ ] **Step 3: Render the switch with existing callbacks**

```tsx
type GuardStatusDto = {
  process?: {
    armed?: boolean;
    scheduledRestartEnabled?: boolean;
    autoMinimizeAfterRestart?: boolean;
    reconnectGraceRemainingMs?: number;
  };
};

<button
  className={status.process?.autoMinimizeAfterRestart ? 'guard-switch guard-switch-on' : 'guard-switch'}
  type="button"
  onClick={() => onSaveSettings?.({ autoMinimizeAfterRestart: !status.process?.autoMinimizeAfterRestart })}
>
  <Power size={15} />
  <span>重启后自动最小化窗口</span>
</button>
```

Place the switch in the `进程异常与定时重启` panel beneath its enable switch. Reuse `onSaveSettings` and existing switch CSS; do not create a separate API or UI state store.

- [ ] **Step 4: Verify GREEN**

Run: `npm test -- --run src/views/GuardView.test.tsx`

Working directory: `frontend`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add frontend/src/views/GuardView.tsx frontend/src/views/GuardView.test.tsx
git commit -m "feat: add guardian restart minimize toggle"
```

### Task 5: Full verification

**Files:** Verify only.

- [ ] **Step 1: Run all Go tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Run frontend build and tests**

Run: `npm test -- --run`

Then run: `npm run build`

Working directory: `frontend`

Expected: both commands PASS.

- [ ] **Step 3: Windows smoke test**

1. Enable `重启后自动最小化窗口` in `进程异常与定时重启`.
2. Restart a valid bound mini-program host and verify it stays visible while reconnecting.
3. Verify it minimizes only after Farm_Go reports the target as ready and connected.
4. Restore it, refresh status, and verify it is not minimized again.
5. Disable the switch, restart again, and verify it remains visible after recovery.
