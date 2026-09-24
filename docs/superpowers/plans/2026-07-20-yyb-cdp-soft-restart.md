# YYB CDP Soft Restart Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `yyb_cdp` process-kill restart with a synchronous window-close and final CDP-reconnect flow that preserves the Application Treasure WMPF host.

**Architecture:** Extract the existing WeChat soft-restart state machine into a shared, dependency-injected WMPF CDP helper. Keep platform-specific WeChat and YYB wrappers, route `yyb_cdp` through its wrapper in `App`, and preserve the same host PID while refreshing only recreated window handles after a disconnected-to-ready transition.

**Tech Stack:** Go 1.24, Wails application services, `internal/runtime/guard`, Go `testing`, Vitest, Vite.

---

## File Structure

- Create `internal/runtime/guard/wmpf_restart.go`: shared request and state machine.
- Modify `internal/runtime/guard/wechat_restart.go`: compatibility wrapper over the shared helper.
- Create `internal/runtime/guard/yyb_restart.go`: YYB wrapper over the shared helper.
- Create `internal/runtime/guard/yyb_restart_test.go`: YYB orchestration and failure tests.
- Modify `app.go`: route `yyb_cdp` through final soft restart.
- Modify `app_test.go`: replace the old YYB stop-delay-launch contract.
- Modify `app_guardian_minimize_test.go`: test refreshed-window minimization.

### Task 1: Share the WMPF CDP Restart State Machine

**Files:**
- Create: `internal/runtime/guard/wmpf_restart.go`
- Modify: `internal/runtime/guard/wechat_restart.go`
- Create: `internal/runtime/guard/yyb_restart.go`
- Create: `internal/runtime/guard/yyb_restart_test.go`
- Test: `internal/runtime/guard/wechat_restart_test.go`

- [ ] **Step 1: Write the failing YYB happy-path test**

Create `internal/runtime/guard/yyb_restart_test.go`:

```go
package guard

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRestartYYBMiniappPreservesHostAndWaitsForReady(t *testing.T) {
	registry, request, calls := newYYBRestartFixture(t)
	result, err := RestartYYBMiniapp(request)
	if err != nil {
		t.Fatalf("restart YYB miniapp: %v", err)
	}
	if result.Status != RestartStatusReconnected || result.Stopped || !result.LaunchDispatched {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.OldPID != 42 || result.Binding == nil || result.Binding.PID != 42 || !reflect.DeepEqual(result.Binding.HWNDs, []uint64{421}) {
		t.Fatalf("binding was not refreshed on the preserved host: %#v", result)
	}
	if got, want := *calls, []string{"close:420", "wait:disconnected", "launch", "wait:ready", "refresh"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
	binding, ok := registry.Binding("main")
	if !ok || binding.PID != 42 || !reflect.DeepEqual(binding.HWNDs, []uint64{421}) {
		t.Fatalf("registry binding was not refreshed: %#v, ok=%v", binding, ok)
	}
}

func newYYBRestartFixture(t *testing.T) (*HostBindingRegistry, YYBRestartRequest, *[]string) {
	t.Helper()
	initial := yybHostSnapshot(42, 420)
	registry := NewHostBindingRegistry()
	if _, err := registry.Bind("main", candidateFromSnapshot(initial)); err != nil {
		t.Fatalf("bind initial host: %v", err)
	}
	calls := []string{}
	request := YYBRestartRequest{
		Registry: registry, Owner: "main", Snapshots: []HostProcessSnapshot{initial},
		CloseWindows: func([]HostWindowSnapshot) error {
			calls = append(calls, "close:420")
			return nil
		},
		WaitForDisconnected: func(timeout time.Duration) bool {
			calls = append(calls, "wait:disconnected")
			return timeout == 2*time.Second
		},
		Launch: func(request LaunchRequest) error {
			if request.Mode != "yyb_shortcut" || !strings.Contains(request.Parameters, "launchWithShortcut?pkgname=wx5306c5978fdb76e4") {
				t.Fatalf("launch request = %#v", request)
			}
			calls = append(calls, "launch")
			return nil
		},
		WaitForReady: func(timeout time.Duration) bool {
			calls = append(calls, "wait:ready")
			return timeout == 45*time.Second
		},
		RefreshSnapshots: func() ([]HostProcessSnapshot, error) {
			calls = append(calls, "refresh")
			return []HostProcessSnapshot{yybHostSnapshot(42, 421)}, nil
		},
		CloseTimeout: 2 * time.Second, ReconnectTimeout: 45 * time.Second,
	}
	return registry, request, &calls
}

func yybHostSnapshot(pid int, hwnd uint64) HostProcessSnapshot {
	return HostProcessSnapshot{
		PID: pid, ParentPID: 7176, ProcessName: "WeChatAppEx.exe",
		ExecutablePath: `E:\Program Files\Tencent\Androws\WmpfRuntime\5.10.2700.327\runtime\WeChatAppEx.exe`,
		Windows: []HostWindowSnapshot{{HWND: hwnd, PID: pid, Title: "QQ经典农场", Visible: true}},
	}
}
```

- [ ] **Step 2: Run the new test to verify RED**

Run:

```powershell
go test ./internal/runtime/guard -run '^TestRestartYYBMiniappPreservesHostAndWaitsForReady$' -count=1
```

Expected: compilation fails because `YYBRestartRequest` and `RestartYYBMiniapp` do not exist.

- [ ] **Step 3: Extract the shared request and orchestration**

Create `internal/runtime/guard/wmpf_restart.go`. Move the existing body from `RestartWeChatMiniapp` into this helper, changing only hard-coded platform and label values:

```go
package guard

import (
	"errors"
	"fmt"
	"time"
)

const RestartStatusReconnected = "reconnected"

type WMPFRestartRequest struct {
	Registry *HostBindingRegistry
	Owner string
	Snapshots []HostProcessSnapshot
	CloseWindows func([]HostWindowSnapshot) error
	WaitForDisconnected func(time.Duration) bool
	Launch func(LaunchRequest) error
	WaitForReady func(time.Duration) bool
	RefreshSnapshots func() ([]HostProcessSnapshot, error)
	CloseTimeout time.Duration
	ReconnectTimeout time.Duration
}

type wmpfRestartProfile struct { platform, label string }

func restartWMPFMiniapp(request WMPFRestartRequest, profile wmpfRestartProfile) (RestartResult, error) {
	if request.Registry == nil { return RestartResult{}, errors.New("host binding registry is required") }
	owner := normalizeOwner(request.Owner)
	binding, ok := request.Registry.Binding(owner)
	if !ok { return RestartResult{}, errors.New("owner has no bound host PID") }
	result := RestartResult{Owner: owner, OldPID: binding.PID, Binding: &binding}
	fail := func(err error) (RestartResult, error) {
		result.Status, result.Reason = "restart_failed", err.Error()
		return result, err
	}
	preview := PreviewRestart(request.Registry, owner, request.Snapshots)
	if !preview.Allowed { return fail(fmt.Errorf("%s restart unavailable: %s", profile.label, preview.Reason)) }
	current, ok := findSnapshotByPID(request.Snapshots, binding.PID)
	if !ok { return fail(fmt.Errorf("bound %s host is no longer running", profile.label)) }
	if request.CloseWindows == nil { return fail(errors.New("close windows callback is required")) }
	if err := request.CloseWindows(current.Windows); err != nil { return fail(fmt.Errorf("close %s miniapp window: %w", profile.label, err)) }
	if request.WaitForDisconnected == nil { return fail(errors.New("disconnect waiter is required")) }
	if !request.WaitForDisconnected(request.CloseTimeout) { return fail(fmt.Errorf("timed out waiting for %s CDP disconnect", profile.label)) }
	if request.Launch == nil { return fail(errors.New("launch callback is required")) }
	launchRequest, err := launchRequestForPlatform(profile.platform, &binding)
	if err != nil { return fail(err) }
	if err := request.Launch(launchRequest); err != nil { return fail(fmt.Errorf("launch %s miniapp: %w", profile.label, err)) }
	result.LaunchDispatched = true
	if request.WaitForReady == nil { return fail(errors.New("ready waiter is required")) }
	if !request.WaitForReady(request.ReconnectTimeout) { return fail(fmt.Errorf("timed out waiting for %s CDP readiness", profile.label)) }
	if request.RefreshSnapshots == nil { return fail(errors.New("snapshot refresh callback is required")) }
	refreshed, err := request.RefreshSnapshots()
	if err != nil { return fail(fmt.Errorf("refresh %s host snapshot: %w", profile.label, err)) }
	refreshedSnapshot, ok := findSnapshotByPID(refreshed, binding.PID)
	if !ok { return fail(fmt.Errorf("preserved %s host PID disappeared after reconnect", profile.label)) }
	candidate := candidateFromSnapshot(refreshedSnapshot)
	if !snapshotMatchesPlatform(profile.platform, refreshedSnapshot) || !hostIdentityMatches(binding, candidate) {
		return fail(fmt.Errorf("preserved %s host identity changed after reconnect", profile.label))
	}
	if !hasMiniappWindowTitle(candidate.WindowTitles) || len(candidate.HWNDs) == 0 {
		return fail(fmt.Errorf("reconnected %s miniapp window was not found", profile.label))
	}
	refreshedBinding, err := request.Registry.Bind(owner, candidate)
	if err != nil { return fail(fmt.Errorf("refresh %s host binding: %w", profile.label, err)) }
	result.Status = RestartStatusReconnected
	result.Reason = profile.label + " CDP reconnected and became ready"
	result.Binding = &refreshedBinding
	result.Candidates = CandidatesForPlatform(profile.platform, refreshed)
	return result, nil
}
```

- [ ] **Step 4: Replace the WeChat implementation and add the YYB wrapper**

Replace `internal/runtime/guard/wechat_restart.go`:

```go
package guard

type WeChatRestartRequest = WMPFRestartRequest

func RestartWeChatMiniapp(request WeChatRestartRequest) (RestartResult, error) {
	return restartWMPFMiniapp(request, wmpfRestartProfile{platform: "wx", label: "WeChat"})
}
```

Create `internal/runtime/guard/yyb_restart.go`:

```go
package guard

type YYBRestartRequest = WMPFRestartRequest

func RestartYYBMiniapp(request YYBRestartRequest) (RestartResult, error) {
	return restartWMPFMiniapp(request, wmpfRestartProfile{platform: "yyb", label: "YYB"})
}
```

- [ ] **Step 5: Add YYB terminal-failure tests**

Append:

```go
func TestRestartYYBMiniappDisconnectTimeoutDoesNotLaunch(t *testing.T) {
	_, request, calls := newYYBRestartFixture(t)
	request.WaitForDisconnected = func(time.Duration) bool {
		*calls = append(*calls, "wait:disconnected")
		return false
	}
	result, err := RestartYYBMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "disconnect") || result.LaunchDispatched {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	if got, want := *calls, []string{"close:420", "wait:disconnected"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
}

func TestRestartYYBMiniappRejectsReplacementHostPID(t *testing.T) {
	registry, request, _ := newYYBRestartFixture(t)
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		return []HostProcessSnapshot{yybHostSnapshot(99, 990)}, nil
	}
	_, err := RestartYYBMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "PID disappeared") { t.Fatalf("error = %v", err) }
	binding, ok := registry.Binding("main")
	if !ok || binding.PID != 42 || !reflect.DeepEqual(binding.HWNDs, []uint64{420}) {
		t.Fatalf("original binding changed: %#v, ok=%v", binding, ok)
	}
}
```

- [ ] **Step 6: Format, test, and commit the guard layer**

Run:

```powershell
gofmt -w internal/runtime/guard/wmpf_restart.go internal/runtime/guard/wechat_restart.go internal/runtime/guard/yyb_restart.go internal/runtime/guard/yyb_restart_test.go
go test ./internal/runtime/guard -run 'TestRestart(WeChat|YYB)Miniapp' -count=1
git add internal/runtime/guard/wmpf_restart.go internal/runtime/guard/wechat_restart.go internal/runtime/guard/yyb_restart.go internal/runtime/guard/yyb_restart_test.go
git commit -m "feat: add YYB CDP soft restart orchestrator"
```

Expected: all WeChat and YYB restart tests pass; the commit contains only guard-layer files.

### Task 2: Route Application Treasure Through Final Soft Restart

**Files:**
- Modify: `app.go:3297-3345`
- Modify: `app_test.go:2019-2040,2237-2289`
- Modify: `app_guardian_minimize_test.go`

- [ ] **Step 1: Replace the old YYB restart expectation with a failing soft-restart test**

Replace `TestAppYYBRestartStillUsesBoundStopDelayLaunch`:

```go
func TestAppYYBRestartPreservesHostAndWaitsForReconnect(t *testing.T) {
	app := newAuthorizedTestApp(t)
	var calls []string
	configureYYBSoftRestartTest(t, app, &calls)
	result := app.RestartHostProcess()
	if result.Status != guard.RestartStatusReconnected || result.Stopped || !result.LaunchDispatched {
		t.Fatalf("expected final YYB reconnect, got %#v", result)
	}
	if result.Binding == nil || result.Binding.PID != 42 || !reflect.DeepEqual(result.Binding.HWNDs, []uint64{421}) {
		t.Fatalf("expected refreshed same-PID binding, got %#v", result.Binding)
	}
	want := []string{"close:420", "wait:disconnected", "launch", "wait:ready", "refresh"}
	if !reflect.DeepEqual(calls, want) { t.Fatalf("calls = %#v, want %#v", calls, want) }
}
```

Add this fixture next to `configureWeChatSoftRestartTest`:

```go
func configureYYBSoftRestartTest(t *testing.T, app *App, calls *[]string) {
	t.Helper()
	app.cfg.Runtime.CurrentTarget = "yyb_cdp"
	snapshotCalls := 0
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
		snapshotCalls++
		hwnd := uint64(420)
		if snapshotCalls >= 3 {
			hwnd = 421
			if calls != nil && snapshotCalls == 3 {
				*calls = append(*calls, "refresh")
			}
		}
		return []guard.HostProcessSnapshot{{
			PID: 42, ParentPID: 7176, ProcessName: "WeChatAppEx.exe",
			ExecutablePath: `E:\Program Files\Tencent\Androws\WmpfRuntime\5.10.2700.327\runtime\WeChatAppEx.exe`,
			Windows: []guard.HostWindowSnapshot{{HWND: hwnd, PID: 42, Title: "QQ经典农场", Visible: true}},
		}}, nil
	}
	app.stopHostPID = func(pid int) error {
		t.Fatalf("YYB soft restart stopped PID %d", pid)
		return nil
	}
	app.closeHostWindows = func(windows []guard.HostWindowSnapshot) error {
		if len(windows) != 1 || windows[0].HWND != 420 {
			t.Fatalf("close windows = %#v", windows)
		}
		if calls != nil { *calls = append(*calls, "close:420") }
		return nil
	}
	waitCalls := 0
	app.waitForRuntimeStatus = func(_ time.Duration, accept func(farmruntime.Status) bool) bool {
		waitCalls++
		status := farmruntime.Status{Target: "yyb_cdp", Phase: farmruntime.PhaseDisconnected}
		label := "wait:disconnected"
		if waitCalls == 2 {
			status = farmruntime.Status{Target: "yyb_cdp", Phase: farmruntime.PhaseReady, Connected: true, Ready: true}
			label = "wait:ready"
		}
		if calls != nil { *calls = append(*calls, label) }
		return accept(status)
	}
	app.launchHost = func(request guard.LaunchRequest) error {
		if request.Mode != "yyb_shortcut" || !strings.Contains(request.Parameters, "launchWithShortcut?pkgname=wx5306c5978fdb76e4") {
			t.Fatalf("launch request = %#v", request)
		}
		if calls != nil { *calls = append(*calls, "launch") }
		return nil
	}
}
```

- [ ] **Step 2: Run the application test to verify RED**

```powershell
go test . -run '^TestAppYYBRestartPreservesHostAndWaitsForReconnect$' -count=1
```

Expected: FAIL because the current path invokes `stopHostPID` and returns `launch_dispatched`.

- [ ] **Step 3: Route both WMPF CDP targets through the shared request**

Replace the WeChat-only branch in `App.restartHostForRuntimeTarget`:

```go
if target == farmruntime.RuntimeTargetWeChatCDP || target == farmruntime.RuntimeTargetYYBCDP {
	settings := a.guard.Settings()
	targetName := string(target)
	request := guard.WMPFRestartRequest{
		Registry: a.hosts, Owner: "main", Snapshots: snapshots,
		CloseWindows: a.closeHostWindows,
		WaitForDisconnected: func(timeout time.Duration) bool {
			return a.waitForRuntimeStatus(timeout, func(status farmruntime.Status) bool {
				return status.Target == targetName && !status.Connected
			})
		},
		Launch: a.launchHost,
		WaitForReady: func(timeout time.Duration) bool {
			return a.waitForRuntimeStatus(timeout, func(status farmruntime.Status) bool {
				return status.Target == targetName && status.Connected && status.Ready
			})
		},
		RefreshSnapshots: a.listHostSnapshots,
		CloseTimeout: a.restartLaunchDelay,
		ReconnectTimeout: time.Duration(settings.RestartReconnectGraceSec) * time.Second,
	}
	restart := guard.RestartWeChatMiniapp
	if target == farmruntime.RuntimeTargetYYBCDP { restart = guard.RestartYYBMiniapp }
	result, restartErr := restart(request)
	if restartErr == nil && result.Status == guard.RestartStatusReconnected && settings.AutoMinimizeAfterRestart {
		a.queuePendingHostMinimize(runtimeTarget)
		a.consumePendingHostMinimize(runtimeTarget)
	}
	return result, restartErr
}
```

- [ ] **Step 4: Add refreshed-window minimization coverage**

Append to `app_guardian_minimize_test.go`:

```go
func TestGuardianYYBRestartMinimizesRefreshedWindowAfterReady(t *testing.T) {
	app := newAuthorizedTestApp(t)
	configureYYBSoftRestartTest(t, app, nil)
	if _, err := app.SaveGuardianSettings(map[string]any{"autoMinimizeAfterRestart": true}); err != nil {
		t.Fatalf("enable auto minimize: %v", err)
	}
	var minimized []guard.HostWindowSnapshot
	app.minimizeHostWindows = func(windows []guard.HostWindowSnapshot) error {
		minimized = append(minimized, windows...)
		return nil
	}
	result := app.RestartHostProcess()
	if result.Status != guard.RestartStatusReconnected {
		t.Fatalf("restart = %#v", result)
	}
	if len(minimized) != 1 || minimized[0].PID != 42 || minimized[0].HWND != 421 {
		t.Fatalf("minimized = %#v, want refreshed HWND 421", minimized)
	}
}
```

- [ ] **Step 5: Format, test, and commit the application route**

```powershell
gofmt -w app.go app_test.go app_guardian_minimize_test.go
go test . -run 'Test(AppYYBRestart|GuardianYYBRestart|AppWeChatRestart|GuardianWeChatRestart)' -count=1
git add app.go app_test.go app_guardian_minimize_test.go
git commit -m "feat: restart YYB CDP through final reconnect"
```

Expected: focused tests pass, and the YYB fixture proves `stopHostPID` is never called.

### Task 3: Regression and Live Verification

**Files:**
- Verify only; no planned source changes.

- [ ] **Step 1: Run all Go tests**

```powershell
go test ./internal/runtime/guard -count=1
go test ./... -count=1
```

Expected: every Go package passes with no failures.

- [ ] **Step 2: Run frontend regression tests and build**

Run from `frontend`:

```powershell
npm test -- --run
npm run build
```

Expected: all Vitest suites and the Vite production build pass; no frontend source changes are introduced.

- [ ] **Step 3: Inspect final scope and whitespace**

```powershell
git diff HEAD~2 --check
git status --short
git log -3 --oneline
```

Expected: no whitespace errors and no unrelated source changes.

- [ ] **Step 4: Perform one live Application Treasure restart**

Trigger Guardian restart while `yyb_cdp` is connected and ready. Capture process and socket state before and after, requiring:

```text
AndrowsStore PID unchanged
WMPF browser and network-service PIDs unchanged
Active renderer rotated and a new preload renderer appeared
Old 9421/62000 sessions disappeared
New 9421/62000 sessions connected with new ephemeral ports
Restart result reconnected
Runtime Connected=true and Ready=true
```

Expected: no Application Treasure or WMPF infrastructure process is terminated, and the call returns only after the new session is ready.
