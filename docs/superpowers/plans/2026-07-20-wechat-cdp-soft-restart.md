# WeChat CDP Soft Restart Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `wechat_cdp` process-kill restart with a synchronous window-close and CDP-reconnect flow that preserves the WMPF host, while leaving QQ and YYB restart behavior unchanged.

**Architecture:** Add a Windows window-close primitive and a dedicated, dependency-injected WeChat restart orchestrator in `internal/runtime/guard`. Route only `wechat_cdp` through it from `App`, wait for a real disconnected-to-ready transition, refresh the same-PID host binding, and teach Guardian that `reconnected` is already a final healthy result.

**Tech Stack:** Go 1.24, `golang.org/x/sys/windows`, Wails bindings, existing runtime/guard managers, Go `testing`.

---

### Task 1: Close Only the Bound Miniapp Window

**Files:**
- Modify: `internal/runtime/guard/process_windows.go:16-24,191-198`
- Modify: `internal/runtime/guard/process_other.go:17-21`
- Modify: `internal/runtime/guard/process_windows_test.go`

- [ ] **Step 1: Write the failing Windows test**

Add a package-level injectable `postWindowClose` seam to the intended API in the test and verify that only visible, non-zero handles are posted:

```go
func TestCloseHostWindowsPostsCloseOnlyToVisibleHandles(t *testing.T) {
	original := postWindowClose
	t.Cleanup(func() { postWindowClose = original })

	var closed []uint64
	postWindowClose = func(hwnd uint64) error {
		closed = append(closed, hwnd)
		return nil
	}

	err := CloseHostWindows([]HostWindowSnapshot{
		{HWND: 10, Visible: true},
		{HWND: 0, Visible: true},
		{HWND: 11, Visible: false},
		{HWND: 12, Visible: true},
	})
	if err != nil {
		t.Fatalf("close windows: %v", err)
	}
	want := []uint64{10, 12}
	if !reflect.DeepEqual(closed, want) {
		t.Fatalf("closed = %#v, want %#v", closed, want)
	}
}
```

Add the callback-error test:

```go
func TestCloseHostWindowsStopsAfterPostError(t *testing.T) {
	original := postWindowClose
	t.Cleanup(func() { postWindowClose = original })

	wantErr := errors.New("post failed")
	var closed []uint64
	postWindowClose = func(hwnd uint64) error {
		closed = append(closed, hwnd)
		return wantErr
	}

	err := CloseHostWindows([]HostWindowSnapshot{{HWND: 10, Visible: true}, {HWND: 12, Visible: true}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if !reflect.DeepEqual(closed, []uint64{10}) {
		t.Fatalf("closed = %#v, want first handle only", closed)
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./internal/runtime/guard -run '^TestCloseHostWindows' -count=1`

Expected: build failure because `postWindowClose` and `CloseHostWindows` do not exist.

- [ ] **Step 3: Implement the minimal Windows and non-Windows APIs**

In `process_windows.go`, load `PostMessageW`, define `WM_CLOSE`, and add:

```go
const wmClose = 0x0010

var postWindowClose = func(hwnd uint64) error {
	result, _, callErr := procPostMessageW.Call(uintptr(hwnd), uintptr(wmClose), 0, 0)
	if result == 0 {
		return fmt.Errorf("PostMessageW WM_CLOSE failed for hwnd %d: %w", hwnd, callErr)
	}
	return nil
}

func CloseHostWindows(snapshots []HostWindowSnapshot) error {
	handles := minimizableWindowHandles(snapshots)
	if len(handles) == 0 {
		return errors.New("visible host window handle is required")
	}
	for _, handle := range handles {
		if err := postWindowClose(handle); err != nil {
			return err
		}
	}
	return nil
}
```

In `process_other.go`, add `CloseHostWindows([]HostWindowSnapshot) error` returning `unsupported_platform`.

- [ ] **Step 4: Run focused and package tests and verify GREEN**

Run: `go test ./internal/runtime/guard -run '^(TestCloseHostWindows|TestMinimizableWindowHandles)' -count=1`

Expected: PASS.

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the window-close primitive**

```powershell
git add internal/runtime/guard/process_windows.go internal/runtime/guard/process_other.go internal/runtime/guard/process_windows_test.go
git commit -m "feat: close bound miniapp windows without stopping host"
```

### Task 2: Add the Dedicated WeChat Restart Orchestrator

**Files:**
- Create: `internal/runtime/guard/wechat_restart.go`
- Create: `internal/runtime/guard/wechat_restart_test.go`

- [ ] **Step 1: Write the successful-flow failing test**

Define a request API whose dependencies make process termination impossible:

```go
type WeChatRestartRequest struct {
	Registry            *HostBindingRegistry
	Owner               string
	Snapshots           []HostProcessSnapshot
	CloseWindows        func([]HostWindowSnapshot) error
	WaitForDisconnected func(time.Duration) bool
	Launch               func(LaunchRequest) error
	WaitForReady         func(time.Duration) bool
	RefreshSnapshots     func() ([]HostProcessSnapshot, error)
	CloseTimeout         time.Duration
	ReconnectTimeout     time.Duration
}
```

The first test binds PID `42` with HWND `420`, supplies a refreshed snapshot for the same PID with HWND `421`, records `close -> disconnected -> launch -> ready -> refresh`, and asserts:

```go
if result.Status != RestartStatusReconnected || result.Stopped || !result.LaunchDispatched {
	t.Fatalf("unexpected result: %#v", result)
}
if result.OldPID != 42 || result.Binding == nil || !reflect.DeepEqual(result.Binding.HWNDs, []uint64{421}) {
	t.Fatalf("binding was not refreshed on the same host: %#v", result)
}
```

- [ ] **Step 2: Run the successful-flow test and verify RED**

Run: `go test ./internal/runtime/guard -run '^TestRestartWeChatMiniappPreservesHostAndWaitsForReady$' -count=1`

Expected: build failure because `WeChatRestartRequest`, `RestartStatusReconnected`, and `RestartWeChatMiniapp` do not exist.

- [ ] **Step 3: Implement the minimal orchestrator**

Create `wechat_restart.go` with:

```go
package guard

import (
	"errors"
	"fmt"
	"time"
)

const RestartStatusReconnected = "reconnected"

type WeChatRestartRequest struct {
	Registry            *HostBindingRegistry
	Owner               string
	Snapshots           []HostProcessSnapshot
	CloseWindows        func([]HostWindowSnapshot) error
	WaitForDisconnected func(time.Duration) bool
	Launch               func(LaunchRequest) error
	WaitForReady         func(time.Duration) bool
	RefreshSnapshots     func() ([]HostProcessSnapshot, error)
	CloseTimeout         time.Duration
	ReconnectTimeout     time.Duration
}

func RestartWeChatMiniapp(request WeChatRestartRequest) (RestartResult, error) {
	if request.Registry == nil {
		return RestartResult{}, errors.New("host binding registry is required")
	}
	owner := normalizeOwner(request.Owner)
	binding, ok := request.Registry.Binding(owner)
	if !ok {
		return RestartResult{}, errors.New("owner has no bound host PID")
	}
	result := RestartResult{Owner: owner, OldPID: binding.PID, Binding: &binding}
	fail := func(err error) (RestartResult, error) {
		result.Status = "restart_failed"
		result.Reason = err.Error()
		return result, err
	}
	preview := PreviewRestart(request.Registry, owner, request.Snapshots)
	if !preview.Allowed {
		return fail(fmt.Errorf("wechat restart unavailable: %s", preview.Reason))
	}
	currentSnapshot, ok := findSnapshotByPID(request.Snapshots, binding.PID)
	if !ok {
		return fail(errors.New("bound WeChat host is no longer running"))
	}
	if request.CloseWindows == nil {
		return fail(errors.New("close windows callback is required"))
	}
	if err := request.CloseWindows(currentSnapshot.Windows); err != nil {
		return fail(fmt.Errorf("close WeChat miniapp window: %w", err))
	}
	if request.WaitForDisconnected == nil {
		return fail(errors.New("disconnect waiter is required"))
	}
	if !request.WaitForDisconnected(request.CloseTimeout) {
		return fail(errors.New("timed out waiting for WeChat CDP disconnect"))
	}
	if request.Launch == nil {
		return fail(errors.New("launch callback is required"))
	}
	launchRequest, err := launchRequestForPlatform("wx", &binding)
	if err != nil {
		return fail(err)
	}
	if err := request.Launch(launchRequest); err != nil {
		return fail(fmt.Errorf("launch WeChat miniapp: %w", err))
	}
	result.LaunchDispatched = true
	if request.WaitForReady == nil {
		return fail(errors.New("ready waiter is required"))
	}
	if !request.WaitForReady(request.ReconnectTimeout) {
		return fail(errors.New("timed out waiting for WeChat CDP readiness"))
	}
	if request.RefreshSnapshots == nil {
		return fail(errors.New("snapshot refresh callback is required"))
	}
	refreshedSnapshots, err := request.RefreshSnapshots()
	if err != nil {
		return fail(fmt.Errorf("refresh WeChat host snapshot: %w", err))
	}
	refreshedSnapshot, ok := findSnapshotByPID(refreshedSnapshots, binding.PID)
	if !ok {
		return fail(errors.New("preserved WeChat host PID disappeared after reconnect"))
	}
	refreshedCandidate := candidateFromSnapshot(refreshedSnapshot)
	if !snapshotMatchesPlatform("wx", refreshedSnapshot) || !hostIdentityMatches(binding, refreshedCandidate) {
		return fail(errors.New("preserved WeChat host identity changed after reconnect"))
	}
	if !hasMiniappWindowTitle(refreshedCandidate.WindowTitles) || len(refreshedCandidate.HWNDs) == 0 {
		return fail(errors.New("reconnected WeChat miniapp window was not found"))
	}
	refreshedBinding, err := request.Registry.Bind(owner, refreshedCandidate)
	if err != nil {
		return fail(fmt.Errorf("refresh WeChat host binding: %w", err))
	}
	result.Status = RestartStatusReconnected
	result.Reason = "WeChat CDP reconnected and became ready"
	result.Binding = &refreshedBinding
	result.Candidates = CandidatesForPlatform("wx", refreshedSnapshots)
	return result, nil
}
```

Every failure returns a `RestartResult` with `Owner`, `OldPID`, preserved `Binding`, `Status: "restart_failed"`, and an exact `Reason`, together with the same error. Set `LaunchDispatched` only after `Launch` succeeds. Do not accept or call a `StopPID` dependency, and never clear the registry.

- [ ] **Step 4: Add failing tests for each safety boundary**

Add separate tests with these exact names and assertions:

```text
TestRestartWeChatMiniappCloseFailureDoesNotLaunch
  close callback returns sentinel error; disconnect, launch, ready, and refresh counters remain zero
TestRestartWeChatMiniappDisconnectTimeoutDoesNotLaunch
  WaitForDisconnected returns false; launch counter remains zero; error contains "disconnect"
TestRestartWeChatMiniappLaunchFailureDoesNotWaitForReady
  launch returns sentinel error; ready and refresh counters remain zero
TestRestartWeChatMiniappReadyTimeoutIsFinalFailure
  WaitForReady returns false; result.LaunchDispatched is true; refresh counter remains zero
TestRestartWeChatMiniappRejectsReplacementHostPID
  refresh contains only PID 99; error contains "PID disappeared"; registry binding remains PID 42
TestRestartWeChatMiniappRefreshesSameHostWindow
  refresh contains PID 42 with HWND 421; result.Binding.PID is 42 and HWNDs equals [421]
```

Each test uses call counters or an ordered string slice and asserts no unrequested callback ran.

- [ ] **Step 5: Run the safety tests and verify RED, then complete validation**

Run: `go test ./internal/runtime/guard -run '^TestRestartWeChatMiniapp' -count=1`

Expected before completing validation: at least one new safety test fails for its missing branch.

Add explicit nil-dependency errors, same-PID lookup, process identity comparison, visible farm-window validation, and refreshed binding replacement until all focused tests pass.

- [ ] **Step 6: Run package tests and verify GREEN**

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS, including existing `RestartBoundHost` tests that preserve QQ/YYB behavior.

- [ ] **Step 7: Commit the WeChat orchestrator**

```powershell
git add internal/runtime/guard/wechat_restart.go internal/runtime/guard/wechat_restart_test.go
git commit -m "feat: add host-preserving WeChat restart flow"
```

### Task 3: Treat `reconnected` as a Final Guardian Result

**Files:**
- Modify: `internal/runtime/guard/manager.go:704-759`
- Modify: `internal/runtime/guard/manager_test.go`

- [ ] **Step 1: Write the failing Guardian final-state test**

Add:

```go
func TestManagerReconnectedRestartSkipsReconnectGrace(t *testing.T) {
	settings := enabledSettings()
	manager := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(RestartReason) (RestartResult, error) {
			return RestartResult{Status: RestartStatusReconnected, LaunchDispatched: true}, nil
		},
	})
	manager.Arm("test")

	result, err := manager.ManualRestart(RuntimeSnapshot{RuntimeTarget: "wechat_cdp"}, "operator request")
	if err != nil || result.Status != RestartStatusReconnected {
		t.Fatalf("restart = %#v, err = %v", result, err)
	}
	status := manager.Status()
	if status.Phase != PhaseWatching || status.ReconnectGraceUntil != "" || status.ReconnectGraceRemainingMS != 0 {
		t.Fatalf("reconnected restart entered grace: %#v", status)
	}
}
```

- [ ] **Step 2: Run the test and verify RED**

Run: `go test ./internal/runtime/guard -run '^TestManagerReconnectedRestartSkipsReconnectGrace$' -count=1`

Expected: FAIL because `executeRestart` enters `PhaseWaitingReconnect`.

- [ ] **Step 3: Implement the final-state branch**

Replace the unconditional `m.enterWaitingReconnectLocked(now)` success tail with:

```go
if result.Status == RestartStatusReconnected {
	m.clearReconnectGraceLocked()
	m.setWatchingPhaseLocked()
} else {
	m.enterWaitingReconnectLocked(now)
}
```

Keep restart history, lifecycle notifications, scheduled-result accounting, and QQ/YYB `launch_dispatched` behavior unchanged.

- [ ] **Step 4: Run focused and manager tests and verify GREEN**

Run: `go test ./internal/runtime/guard -run 'TestManager(ReconnectedRestartSkipsReconnectGrace|RestartsAfter|SuccessfulManualRestart)' -count=1`

Expected: PASS.

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the Guardian final state**

```powershell
git add internal/runtime/guard/manager.go internal/runtime/guard/manager_test.go
git commit -m "fix: finish guardian restart after WeChat reconnect"
```

### Task 4: Route Only `wechat_cdp` Through the Soft Restart

**Files:**
- Modify: `app.go:60-96,210-315,2741-2757,3197-3216`
- Modify: `app_test.go:1738-1800`
- Modify: `app_guardian_minimize_test.go`

- [ ] **Step 1: Replace the old WeChat restart test with a failing final-result test**

Set up an authorized app with an initial PID `42`/HWND `420`, a refreshed PID `42`/HWND `421`, and injectable callbacks. The test records this exact order:

```go
want := []string{"close:420", "wait:disconnected", "launch", "wait:ready", "refresh"}
```

It also makes `stopHostPID` fail the test if called and asserts `RestartHostProcess()` returns `RestartStatusReconnected` with the refreshed same-PID binding.

- [ ] **Step 2: Run the app test and verify RED**

Run: `go test . -run '^TestAppWeChatRestartPreservesHostAndWaitsForReconnect$' -count=1`

Expected: FAIL because the current path calls `stopHostPID` and returns `launch_dispatched`.

- [ ] **Step 3: Add injectable App dependencies and condition waiting**

Add fields:

```go
closeHostWindows    func([]guard.HostWindowSnapshot) error
waitForRuntimeStatus func(time.Duration, func(farmruntime.Status) bool) bool
```

Initialize `closeHostWindows` with `guard.CloseHostWindows`. Initialize the wait callback after constructing `app` so it calls a production helper that checks the current status immediately, then uses a short ticker until the predicate passes or the timeout timer fires:

```go
func waitForRuntimeStatus(manager *farmruntime.Manager, timeout time.Duration, accept func(farmruntime.Status) bool) bool {
	if manager == nil || accept == nil {
		return false
	}
	if accept(manager.Status()) {
		return true
	}
	if timeout <= 0 {
		return false
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if accept(manager.Status()) {
				return true
			}
		case <-timer.C:
			return false
		}
	}
}
```

Add focused tests for immediate success, delayed status transition, and timeout using short bounded durations.

- [ ] **Step 4: Implement the `wechat_cdp` branch**

In `restartHostForRuntimeTarget`, keep the current snapshot acquisition, then branch before `RestartBoundHost`:

```go
if farmruntime.RuntimeTarget(runtimeTarget) == farmruntime.RuntimeTargetWeChatCDP {
	settings := a.guard.Settings()
	result, err := guard.RestartWeChatMiniapp(guard.WeChatRestartRequest{
		Registry: a.hosts, Owner: "main", Snapshots: snapshots,
		CloseWindows: a.closeHostWindows,
		WaitForDisconnected: func(timeout time.Duration) bool {
			return a.waitForRuntimeStatus(timeout, func(status farmruntime.Status) bool {
				return status.Target == string(farmruntime.RuntimeTargetWeChatCDP) && !status.Connected
			})
		},
		Launch: a.launchHost,
		WaitForReady: func(timeout time.Duration) bool {
			return a.waitForRuntimeStatus(timeout, func(status farmruntime.Status) bool {
				return status.Target == string(farmruntime.RuntimeTargetWeChatCDP) && status.Connected && status.Ready
			})
		},
		RefreshSnapshots: a.listHostSnapshots,
		CloseTimeout: a.restartLaunchDelay,
		ReconnectTimeout: time.Duration(settings.RestartReconnectGraceSec) * time.Second,
	})
	if err == nil && result.Status == guard.RestartStatusReconnected && settings.AutoMinimizeAfterRestart {
		a.queuePendingHostMinimize(runtimeTarget)
		a.consumePendingHostMinimize(runtimeTarget)
	}
	return result, err
}
```

Leave the existing `RestartBoundHost` call byte-for-byte behaviorally equivalent for `qq_ws` and `yyb_cdp`.

- [ ] **Step 5: Update action and minimize semantics with failing tests first**

Add tests proving `RunGuardianAction({Action: "restart"})` returns `OK: true` for `reconnected`, and that auto-minimize runs after the refreshed WeChat binding is ready. Run each new test before implementation and confirm it fails.

Then update the success check to accept both final and dispatched states:

```go
if result.Status == "launch_dispatched" || result.Status == guard.RestartStatusReconnected {
	return guard.ActionResult{OK: true, Status: result.Status}
}
```

For WeChat auto-minimize, use the immediate queue-and-consume block shown in Step 4 because readiness occurred before the synchronous callback returned. Preserve the existing delayed minimize queue for QQ and YYB.

- [ ] **Step 6: Update existing manual restart assertions**

Change the existing WeChat manual restart history test to use close/wait/refresh dependencies and expect `reconnected`. Keep its assertion that exactly one manual `wechat_cdp` restart event is recorded.

- [ ] **Step 7: Run focused app tests and verify GREEN**

Run: `go test . -run 'Test(AppWeChatRestart|AppManualRestartRecordsRestartEvent|RunGuardianAction|GuardianRestart)' -count=1`

Expected: PASS.

Run: `go test . -count=1`

Expected: PASS.

- [ ] **Step 8: Commit the app routing**

```powershell
git add app.go app_test.go app_guardian_minimize_test.go
git commit -m "feat: soft restart WeChat CDP through final readiness"
```

### Task 5: Full Verification and Live Test Preparation

**Files:**
- Modify only if a failing verification exposes a defect in the files already listed above.

- [ ] **Step 1: Format all changed Go files**

Run: `gofmt -w app.go app_test.go app_guardian_minimize_test.go internal/runtime/guard/process_windows.go internal/runtime/guard/process_other.go internal/runtime/guard/process_windows_test.go internal/runtime/guard/wechat_restart.go internal/runtime/guard/wechat_restart_test.go internal/runtime/guard/manager.go internal/runtime/guard/manager_test.go`

- [ ] **Step 2: Run the complete backend suite**

Run: `go test ./... -count=1`

Expected: PASS with zero failed packages.

- [ ] **Step 3: Run race-sensitive restart packages**

Run: `go test -race ./internal/runtime/guard ./internal/runtime/wmpf -count=1`

Expected: PASS with no race reports.

- [ ] **Step 4: Run frontend tests because the Wails call remains synchronously awaited**

Run: `npm test -- --run` from `frontend`.

Expected: PASS. No generated Wails model change is expected because `RestartResult` fields are unchanged.

- [ ] **Step 5: Run build verification**

Run: `go build ./...`

Expected: exit code 0.

- [ ] **Step 6: Audit the final diff and scope**

Run: `git diff --check`

Expected: no output.

Run: `git status --short`

Expected: only the planned implementation files are present before their final commit; no QQ/YYB link implementation files are modified.

- [ ] **Step 7: Start the development server and perform the user-assisted live test**

Capture before/during/after snapshots of the WeChat PID tree and port `9421`, then have the user click the existing restart button once. Verify:

```text
Weixin main PID                       unchanged
WeChatAppEx browser/network/GPU/link unchanged
wmpf-render-type=0 PID               replaced
old 9421 Established connection      removed
new 9421 Established connection      created
button Promise                       resolves only after Connected && Ready
result                               reconnected
```

For a live failure, report the exact failed stage and retain all host processes; do not invoke the old process-kill path.

- [ ] **Step 8: Commit any verification-only corrections**

If verification required a correction, rerun the failing test first, apply only the smallest fix, rerun the full verification commands, and commit only those corrections. If no correction was required, do not create an empty commit.
