# Runtime Reconnect Process Guard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add reliable reconnect and process-guard recovery for QQ WS, WeChat CDP, and YingYongBao CDP.

**Architecture:** Keep runtime links in Go. Add a small guard domain under `internal/runtime/guard` for process snapshots, host binding, restart state, and launch actions. Wire the guard into `App`, expose Wails methods, then add a frontend guard page using existing Farm_Go shell styles.

**Tech Stack:** Go 1.25, Wails v2, React 18, TypeScript, Vitest, `golang.org/x/sys/windows`, Gorilla WebSocket.

---

## File Structure

- Modify `internal/runtime/qqpatch/patcher.go`: discover broader QQ source roots and patch multiple auto-discovered candidates.
- Modify `internal/runtime/qqpatch/patcher_test.go`: add multi-candidate patch coverage.
- Modify `internal/runtime/wmpf/link.go`: allow CDP link to reset evaluator/context and retry after miniapp reconnects.
- Modify `internal/runtime/wmpf/link_test.go`: add reconnect regression tests with a controllable bridge.
- Create `internal/runtime/guard/types.go`: shared settings, status, candidates, bindings, restart events.
- Create `internal/runtime/guard/manager.go`: process guard state machine.
- Create `internal/runtime/guard/manager_test.go`: state-machine tests.
- Create `internal/runtime/guard/host.go`: platform-independent host binding and restart orchestration.
- Create `internal/runtime/guard/host_test.go`: pure Go host selection/restart tests.
- Create `internal/runtime/guard/process_windows.go`: Windows process/window snapshots, termination, launch actions.
- Create `internal/runtime/guard/process_other.go`: unsupported-platform stubs.
- Create `internal/runtime/guard/launch.go`: QQ/WX/YYB launch request resolution.
- Modify `internal/storage/settings.go`: persist process guard settings.
- Modify `internal/storage/storage_test.go`: verify persistence.
- Modify `app.go`: construct guard manager, feed runtime status/errors, expose Wails methods.
- Modify `app_test.go`: verify App guard methods and QQ patch status behavior.
- Modify `frontend/src/components/AppShell.tsx`: add `守护` tab.
- Modify `frontend/src/App.tsx`: fetch guard status and route guard tab.
- Create `frontend/src/views/GuardView.tsx`: guard UI.
- Create `frontend/src/views/GuardView.test.tsx`: render coverage.
- Modify `frontend/src/style.css`: guard view styles, matching existing visual language.
- Regenerate Wails bindings after Go methods are added.

---

### Task 1: QQ Patch Multi-Candidate Recovery

**Files:**
- Modify: `internal/runtime/qqpatch/patcher_test.go`
- Modify: `internal/runtime/qqpatch/patcher.go`

- [ ] **Step 1: Write the failing multi-candidate test**

Append this test to `internal/runtime/qqpatch/patcher_test.go`:

```go
func TestInstallPatchesAllAutoDiscoveredCandidates(t *testing.T) {
	root := t.TempDir()
	firstDir := filepath.Join(root, "1112386029_1_first")
	secondDir := filepath.Join(root, "1112386029_2_second")
	for _, dir := range []string{firstDir, secondDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "game.js"), []byte("console.log('game');\n"), 0o644); err != nil {
			t.Fatalf("write game.js: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "game.json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write game.json: %v", err)
		}
	}

	result := Install(Options{
		MiniappSrcRoot: root,
		HostScript:     "autoHost();",
		HostVersion:    "farm-go-host-1",
		NoBackup:       true,
	})

	if !result.OK {
		t.Fatalf("expected install ok, got %#v", result)
	}
	for _, target := range []string{filepath.Join(firstDir, "game.js"), filepath.Join(secondDir, "game.js")} {
		content, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read %s: %v", target, err)
		}
		if !strings.Contains(string(content), MarkerStart) || !strings.Contains(string(content), "autoHost();") {
			t.Fatalf("expected %s to be patched, got %q", target, string(content))
		}
	}
	if len(result.TargetPaths) != 2 {
		t.Fatalf("expected both target paths in result, got %#v", result.TargetPaths)
	}
}
```

- [ ] **Step 2: Run RED**

Run: `go test ./internal/runtime/qqpatch -run TestInstallPatchesAllAutoDiscoveredCandidates -count=1`

Expected: FAIL because `Result.TargetPaths` does not exist and only one target is patched.

- [ ] **Step 3: Implement minimal multi-target patching**

In `internal/runtime/qqpatch/patcher.go`, add `TargetPaths []string` to `Result`, change automatic resolution to return all discovered candidate paths, and patch every path when `TargetPath` was not explicit. Keep explicit target behavior to one path.

Implementation shape:

```go
type Result struct {
	OK             bool     `json:"ok"`
	Action         string   `json:"action"`
	TargetPath     string   `json:"targetPath,omitempty"`
	TargetPaths    []string `json:"targetPaths,omitempty"`
	BackupPath     string   `json:"backupPath,omitempty"`
	ScriptHash     string   `json:"scriptHash,omitempty"`
	HostVersion    string   `json:"hostVersion,omitempty"`
	CandidatePaths []string `json:"candidatePaths,omitempty"`
	Error          string   `json:"error,omitempty"`
}
```

Use a helper:

```go
func patchTargets(targetPaths []string, bundle string, noBackup bool) ([]string, bool, error) {
	backupPaths := make([]string, 0, len(targetPaths))
	replacedAny := false
	for _, targetPath := range targetPaths {
		backupPath, replaced, err := patchGameJS(targetPath, bundle, noBackup)
		if err != nil {
			return backupPaths, replacedAny, err
		}
		if backupPath != "" {
			backupPaths = append(backupPaths, backupPath)
		}
		replacedAny = replacedAny || replaced
	}
	return backupPaths, replacedAny, nil
}
```

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/runtime/qqpatch -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```powershell
git add internal/runtime/qqpatch/patcher.go internal/runtime/qqpatch/patcher_test.go
git commit -m "fix: patch all qq miniapp candidates"
```

---

### Task 2: CDP Link Reconnect Loop

**Files:**
- Modify: `internal/runtime/wmpf/link_test.go`
- Modify: `internal/runtime/wmpf/link.go`

- [ ] **Step 1: Write failing reconnect test**

Add a fake bridge that can toggle `MiniappConnected` and return a JS context after reconnect:

```go
func TestCDPLinkRetriesAfterMiniappReconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := farmruntime.NewManager()
	bridge := newReconnectBridge()
	evaluator := &fakeEvaluator{value: map[string]any{"hasGameCtl": true}}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge:    bridge,
		Evaluator: evaluator,
	})

	if err := link.Start(ctx); err != nil {
		t.Fatalf("start link: %v", err)
	}
	bridge.setConnected(true)
	if err := link.UpdateRuntimeContext(ctx, cdp.ExecutionContext{ID: 1, Name: "gameContext"}); err != nil {
		t.Fatalf("ready context: %v", err)
	}
	bridge.setConnected(false)
	link.OnMiniappDisconnected(errors.New("miniapp closed"))

	status := manager.Status()
	if status.Phase != farmruntime.PhaseDisconnected || status.Ready {
		t.Fatalf("expected disconnected after close, got %#v", status)
	}

	bridge.setConnected(true)
	link.OnMiniappConnected()

	status = manager.Status()
	if status.Phase != farmruntime.PhaseHandshaking || !status.Connected {
		t.Fatalf("expected handshaking after reconnect, got %#v", status)
	}
}
```

- [ ] **Step 2: Run RED**

Run: `go test ./internal/runtime/wmpf -run TestCDPLinkRetriesAfterMiniappReconnect -count=1`

Expected: FAIL because `OnMiniappDisconnected` and `OnMiniappConnected` do not exist.

- [ ] **Step 3: Implement minimal lifecycle hooks**

Add methods to `CDPLink`:

```go
func (l *CDPLink) OnMiniappDisconnected(err error) {
	l.mu.Lock()
	l.context = cdp.ExecutionContext{}
	l.eval = nil
	l.mu.Unlock()
	lastError := ""
	if err != nil {
		lastError = err.Error()
	}
	l.manager.SetStatus(farmruntime.Status{
		Target:    string(l.Target()),
		Phase:     farmruntime.PhaseDisconnected,
		LastError: lastError,
	})
}

func (l *CDPLink) OnMiniappConnected() {
	l.manager.SetStatus(farmruntime.Status{
		Target:    string(l.Target()),
		Phase:     farmruntime.PhaseHandshaking,
		Connected: true,
	})
}
```

Then extend `connectRuntimeWhenReady` so it does not `return` permanently on context wait errors caused by absent miniapp; it should clear evaluator and keep polling until `ctx.Done()`.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/runtime/wmpf -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```powershell
git add internal/runtime/wmpf/link.go internal/runtime/wmpf/link_test.go
git commit -m "fix: retry cdp runtime reconnect"
```

---

### Task 3: Guard Types And Settings

**Files:**
- Create: `internal/runtime/guard/types.go`
- Modify: `internal/storage/settings.go`
- Modify: `internal/storage/storage_test.go`

- [ ] **Step 1: Write failing storage test**

Add to `internal/storage/storage_test.go`:

```go
func TestRuntimeSettingsPersistProcessGuard(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	input := RuntimeSettings{
		DefaultTarget: "qq_ws",
		CurrentTarget: "qq_ws",
		AutoStart: true,
		CDPPort: 62000,
		WMPFDebugPort: 9420,
		ProcessGuardEnabled: true,
		ProcessGuardTimeoutThreshold: 2,
		ProcessGuardMonitorIntervalMS: 1500,
		ProcessGuardRestartReconnectGraceSec: 30,
		ProcessGuardMaxRestartsPer10Min: 3,
		ProcessGuardScheduledRestartEnabled: true,
		ProcessGuardScheduledRestartIntervalMin: 45,
	}
	if err := store.SaveRuntimeSettings(ctx, input); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	got, err := store.LoadRuntimeSettings(ctx)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if !got.ProcessGuardEnabled || got.ProcessGuardTimeoutThreshold != 2 || got.ProcessGuardScheduledRestartIntervalMin != 45 {
		t.Fatalf("process guard settings were not persisted: %#v", got)
	}
}
```

- [ ] **Step 2: Run RED**

Run: `go test ./internal/storage -run TestRuntimeSettingsPersistProcessGuard -count=1`

Expected: FAIL because the settings fields do not exist.

- [ ] **Step 3: Add settings fields and guard types**

Create `internal/runtime/guard/types.go`:

```go
package guard

type Phase string

const (
	PhaseDisabled Phase = "disabled"
	PhaseStandby Phase = "standby"
	PhaseWatching Phase = "watching"
	PhaseDegraded Phase = "degraded"
	PhaseRestarting Phase = "restarting"
	PhaseWaitingReconnect Phase = "waiting_reconnect"
	PhaseCircuitOpen Phase = "circuit_open"
)

type Settings struct {
	Enabled bool `json:"enabled"`
	TimeoutThreshold int `json:"timeoutThreshold"`
	MonitorIntervalMS int `json:"monitorIntervalMs"`
	RestartReconnectGraceSec int `json:"restartReconnectGraceSec"`
	MaxRestartsPer10Min int `json:"maxRestartsPer10Min"`
	ScheduledRestartEnabled bool `json:"scheduledRestartEnabled"`
	ScheduledRestartIntervalMin int `json:"scheduledRestartIntervalMin"`
}

func DefaultSettings() Settings {
	return Settings{
		TimeoutThreshold: 3,
		MonitorIntervalMS: 3000,
		RestartReconnectGraceSec: 45,
		MaxRestartsPer10Min: 4,
		ScheduledRestartIntervalMin: 60,
	}
}
```

Add matching scalar fields to `storage.RuntimeSettings` and persist them under keys like `processGuard.enabled`, `processGuard.timeoutThreshold`, and `processGuard.scheduledRestartIntervalMin`.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/storage -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```powershell
git add internal/runtime/guard/types.go internal/storage/settings.go internal/storage/storage_test.go
git commit -m "feat: persist process guard settings"
```

---

### Task 4: Host Candidate Binding And Restart Core

**Files:**
- Create: `internal/runtime/guard/host.go`
- Create: `internal/runtime/guard/host_test.go`
- Create: `internal/runtime/guard/launch.go`

- [ ] **Step 1: Write failing host binding tests**

Create `internal/runtime/guard/host_test.go`:

```go
package guard

import "testing"

func TestAutoBindRefusesAmbiguousCandidates(t *testing.T) {
	registry := NewHostBindingRegistry()
	candidates := []HostProcessCandidate{
		{PID: 10, ProcessName: "WeChatAppEx.exe", WindowTitles: []string{"QQ经典农场"}, Available: true},
		{PID: 11, ProcessName: "WeChatAppEx.exe", WindowTitles: []string{"QQ经典农场"}, Available: true},
	}
	result, err := registry.AutoBind("main", candidates)
	if err != nil {
		t.Fatalf("auto bind: %v", err)
	}
	if result.Status != "ambiguous" || result.Binding != nil {
		t.Fatalf("expected ambiguous refusal, got %#v", result)
	}
}

func TestRestartTerminatesOnlyBoundPID(t *testing.T) {
	registry := NewHostBindingRegistry()
	_, err := registry.Bind("main", HostProcessCandidate{PID: 20, ProcessName: "QQ.exe", WindowTitles: []string{"QQ经典农场"}, Available: true})
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	var stopped []int
	result, err := RestartBoundHost(RestartRequest{
		Registry: registry,
		Owner: "main",
		Platform: "qq",
		Snapshots: []HostProcessSnapshot{{PID: 20, ProcessName: "QQ.exe", Windows: []HostWindowSnapshot{{PID: 20, Title: "QQ经典农场", Visible: true}}}},
		StopPID: func(pid int) error { stopped = append(stopped, pid); return nil },
		Launch: func(LaunchRequest) error { return nil },
	})
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	if len(stopped) != 1 || stopped[0] != 20 {
		t.Fatalf("expected only bound pid stopped, got %#v", stopped)
	}
	if result.Status != "launch_dispatched" {
		t.Fatalf("unexpected result %#v", result)
	}
}
```

- [ ] **Step 2: Run RED**

Run: `go test ./internal/runtime/guard -run 'TestAutoBind|TestRestart' -count=1`

Expected: FAIL because guard package host types do not exist.

- [ ] **Step 3: Implement pure Go host registry and restart**

Create `host.go` with:

```go
type HostWindowSnapshot struct {
	HWND uint64 `json:"hwnd"`
	PID int `json:"pid"`
	Title string `json:"title"`
	Visible bool `json:"visible"`
}

type HostProcessSnapshot struct {
	PID int `json:"pid"`
	ParentPID int `json:"parentPid,omitempty"`
	ProcessName string `json:"processName"`
	ExecutablePath string `json:"executablePath,omitempty"`
	Windows []HostWindowSnapshot `json:"windows"`
}
```

Implement `HostBindingRegistry`, `Bind`, `Clear`, `AutoBind`, `CandidatesForPlatform`, `PreviewRestart`, and `RestartBoundHost`. Keep the first version conservative: bind exactly one available candidate with a miniapp title, refuse zero or many.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```powershell
git add internal/runtime/guard/host.go internal/runtime/guard/host_test.go internal/runtime/guard/launch.go
git commit -m "feat: add host process binding core"
```

---

### Task 5: Windows Process And Launch Adapter

**Files:**
- Create: `internal/runtime/guard/process_windows.go`
- Create: `internal/runtime/guard/process_other.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Write compile-time adapter test**

Add to `internal/runtime/guard/host_test.go`:

```go
func TestLaunchRequestForPlatform(t *testing.T) {
	cases := map[string]string{
		"qq": "protocol",
		"wechat_cdp": "protocol",
		"yyb_cdp": "yyb_shortcut",
	}
	for platform, want := range cases {
		got := LaunchRequestForPlatform(platform)
		if got.Mode != want {
			t.Fatalf("%s mode = %s, want %s", platform, got.Mode, want)
		}
	}
}
```

- [ ] **Step 2: Run RED**

Run: `go test ./internal/runtime/guard -run TestLaunchRequestForPlatform -count=1`

Expected: FAIL until launch request helper exists.

- [ ] **Step 3: Implement Windows adapter**

Use `golang.org/x/sys/windows` to:

- Enumerate processes with Toolhelp.
- Enumerate windows with `EnumWindows`.
- Read window title and PID.
- Query process executable path where possible.
- Terminate PID with `OpenProcess(PROCESS_TERMINATE)` and `TerminateProcess`.
- Launch protocol or executable with `ShellExecute`.

Create non-Windows stubs returning `unsupported_platform`.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS on logic tests. Platform API tests should be limited to compile-safe helpers.

- [ ] **Step 5: Commit**

Run:

```powershell
git add internal/runtime/guard/process_windows.go internal/runtime/guard/process_other.go internal/runtime/guard/launch.go go.mod go.sum
git commit -m "feat: add windows host process adapter"
```

---

### Task 6: Guard Manager State Machine

**Files:**
- Create: `internal/runtime/guard/manager.go`
- Create: `internal/runtime/guard/manager_test.go`

- [ ] **Step 1: Write failing state tests**

Create `internal/runtime/guard/manager_test.go`:

```go
package guard

import (
	"errors"
	"testing"
	"time"
)

func TestManagerTriggersRestartAfterTimeoutThreshold(t *testing.T) {
	var restarts int
	manager := NewManager(ManagerOptions{
		Settings: Settings{Enabled: true, TimeoutThreshold: 2, MonitorIntervalMS: 1000, RestartReconnectGraceSec: 5, MaxRestartsPer10Min: 4},
		Restart: func(RestartReason) (RestartResult, error) {
			restarts++
			return RestartResult{Status: "launch_dispatched"}, nil
		},
		Now: func() time.Time { return time.Unix(100, 0) },
	})
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "wechat_cdp"})
	manager.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "wechat_cdp"})
	status := manager.Status()
	if restarts != 1 || status.Phase != PhaseWaitingReconnect {
		t.Fatalf("expected restart then waiting reconnect, restarts=%d status=%#v", restarts, status)
	}
}

func TestManagerOpensCircuitAfterTooManyRestarts(t *testing.T) {
	var now = time.Unix(100, 0)
	manager := NewManager(ManagerOptions{
		Settings: Settings{Enabled: true, TimeoutThreshold: 1, MonitorIntervalMS: 1000, RestartReconnectGraceSec: 5, MaxRestartsPer10Min: 1},
		Restart: func(RestartReason) (RestartResult, error) { return RestartResult{Status: "launch_dispatched"}, nil },
		Now: func() time.Time { return now },
	})
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	manager.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	if manager.Status().Phase != PhaseCircuitOpen {
		t.Fatalf("expected circuit open, got %#v", manager.Status())
	}
}
```

- [ ] **Step 2: Run RED**

Run: `go test ./internal/runtime/guard -run 'TestManager' -count=1`

Expected: FAIL because manager does not exist.

- [ ] **Step 3: Implement synchronous manager**

Implement:

- `NewManager`
- `UpdateSettings`
- `Arm`
- `Disarm`
- `NoteHealthy`
- `NoteRuntimeError`
- `ManualRestart`
- `Status`

Make restart synchronous in tests and asynchronous only when called by app tickers in the app wiring task.

- [ ] **Step 4: Run GREEN**

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```powershell
git add internal/runtime/guard/manager.go internal/runtime/guard/manager_test.go
git commit -m "feat: add process guard state machine"
```

---

### Task 7: Wire Guard Into App

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`
- Modify: `frontend/wailsjs/go/main/App.d.ts`
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Write failing app tests**

Add to `app_test.go`:

```go
func TestAppExposesProcessGuardStatus(t *testing.T) {
	app := NewApp()
	status := app.ProcessGuardStatus()
	if status.Phase == "" {
		t.Fatalf("expected guard phase, got %#v", status)
	}
}

func TestAppCanSaveProcessGuardSettings(t *testing.T) {
	app := NewApp()
	settings := app.SaveProcessGuardSettings(map[string]any{
		"enabled": true,
		"timeoutThreshold": 2,
	})
	if settings["enabled"] != true {
		t.Fatalf("expected enabled setting, got %#v", settings)
	}
}
```

- [ ] **Step 2: Run RED**

Run: `go test . -run 'TestAppExposesProcessGuardStatus|TestAppCanSaveProcessGuardSettings' -count=1`

Expected: FAIL because App methods do not exist.

- [ ] **Step 3: Add App guard methods**

Add a `guardManager *guard.Manager` and `hostRegistry *guard.HostBindingRegistry` to `App`.

Expose:

```go
func (a *App) ProcessGuardStatus() guard.Status
func (a *App) SaveProcessGuardSettings(input map[string]any) map[string]any
func (a *App) ListHostCandidates() []guard.HostProcessCandidate
func (a *App) BindHostProcess(pid int) guard.HostBinding
func (a *App) ClearHostBinding() bool
func (a *App) PreviewHostRestart() guard.HostRestartPreview
func (a *App) RestartHostProcess() guard.RestartResult
func (a *App) LaunchHostProcess() guard.HostLaunchResult
```

Update `recordRuntimeStatus` so ready status calls `guard.NoteHealthy` and disconnected/error status calls `guard.NoteRuntimeError` only when guard is armed/enabled.

- [ ] **Step 4: Regenerate Wails bindings**

Run: `wails generate module`

Expected: frontend `wailsjs` files include new App methods and model types.

- [ ] **Step 5: Run GREEN**

Run: `go test . ./internal/runtime/guard ./internal/storage -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```powershell
git add app.go app_test.go frontend/wailsjs
git commit -m "feat: expose process guard app methods"
```

---

### Task 8: Frontend Guard View

**Files:**
- Modify: `frontend/src/components/AppShell.tsx`
- Modify: `frontend/src/App.tsx`
- Create: `frontend/src/views/GuardView.tsx`
- Create: `frontend/src/views/GuardView.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing frontend tests**

Create `frontend/src/views/GuardView.test.tsx`:

```tsx
import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { GuardView } from './GuardView';

describe('GuardView', () => {
  it('renders guard status and restart controls', () => {
    const html = renderToString(
      <GuardView
        status={{
          phase: 'watching',
          runtimeTarget: 'qq_ws',
          timeoutStreak: 1,
          restartCountInWindow: 0,
          maxRestartsPerWindow: 4,
          recentRestartEvents: [],
        }}
        candidates={[]}
        onRefresh={() => undefined}
        onLaunch={() => undefined}
        onRestart={() => undefined}
      />,
    );
    expect(html).toContain('进程守护');
    expect(html).toContain('重启当前小程序');
  });
});
```

- [ ] **Step 2: Run RED**

Run: `cd frontend; npm test -- GuardView.test.tsx`

Expected: FAIL because `GuardView` does not exist.

- [ ] **Step 3: Implement GuardView and tab**

Add `shield` navigation item:

```tsx
import { Activity, FlaskConical, ScrollText, Settings, ShieldCheck } from 'lucide-react';
export type Tab = 'overview' | 'guard' | 'diagnostics' | 'logs' | 'settings';
```

Create `GuardView.tsx` with compact metric rows and buttons. Use existing button classes such as `primary-button` where available.

- [ ] **Step 4: Run GREEN**

Run: `cd frontend; npm test -- GuardView.test.tsx`

Expected: PASS.

- [ ] **Step 5: Build frontend**

Run: `cd frontend; npm run build`

Expected: TypeScript and Vite build succeed.

- [ ] **Step 6: Commit**

Run:

```powershell
git add frontend/src/components/AppShell.tsx frontend/src/App.tsx frontend/src/views/GuardView.tsx frontend/src/views/GuardView.test.tsx frontend/src/style.css
git commit -m "feat: add process guard view"
```

---

### Task 9: End-To-End Verification

**Files:**
- Verify all changed files.

- [ ] **Step 1: Run Go tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Run frontend tests**

Run: `cd frontend; npm test`

Expected: PASS.

- [ ] **Step 3: Run frontend build**

Run: `cd frontend; npm run build`

Expected: PASS.

- [ ] **Step 4: Run Wails build or generate check**

Run: `wails build`

Expected: Build succeeds. If local Wails packaging dependencies block, record the exact failure and keep `go test ./...` plus `npm run build` as verified.

- [ ] **Step 5: Manual smoke checklist**

Manual checks:

- Start Farm_Go.
- Open QQ Farm miniapp, verify QQ WS ready.
- Close and reopen miniapp, verify ready returns.
- Switch to WeChat CDP, open miniapp, verify ready.
- Close and reopen WeChat miniapp, verify ready returns.
- Switch to YYB CDP, open miniapp, verify ready.
- Use Guard tab to bind host, preview restart, restart current miniapp, and observe waiting reconnect then ready.

- [ ] **Step 6: Final commit if any verification-only fixes were needed**

Run:

```powershell
git status --short
```

Expected: only intentional changes remain. Commit any verification fixes with a focused message.

---

## Self-Review Notes

- Spec coverage: QQ multi-candidate patching is Task 1; CDP reconnect is Task 2; guard settings/types are Task 3; process binding/restart is Tasks 4-5; state machine is Task 6; Wails and UI are Tasks 7-8; verification is Task 9.
- No empty work items remain in the plan; each task has concrete files, tests, commands, and expected results.
- The plan intentionally keeps process killing limited to bound candidates and refuses ambiguous candidates.
