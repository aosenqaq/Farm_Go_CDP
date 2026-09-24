# QQ WS Miniapp Restart Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `qq_ws` stop-delay-launch restart with a synchronous, identity-safe QQEX subtree restart that preserves the QQ main process and succeeds only after a new WebSocket instance is ready and rebound.

**Architecture:** Add a dedicated dependency-injected QQ restart orchestrator in `internal/runtime/guard`. It strictly identifies the titled QQ Farm root under a QQ parent, closes the window first, safely cleans only a revalidated old subtree when graceful exit times out, launches once, requires a new runtime instance, and refreshes the binding to the replacement PID and HWND. Route only `qq_ws` through this path from `App`; WeChat and YYB behavior remain unchanged.

**Tech Stack:** Go 1.24, Wails, `golang.org/x/sys/windows`, existing runtime/guard managers, Go `testing`, PowerShell for live process and TCP verification.

---

## File Map

- Create `internal/runtime/guard/qq_restart.go`: strict QQ miniapp identity, process-tree helpers, close observations, verified timeout cleanup, launch-only recovery, final readiness, and binding refresh.
- Create `internal/runtime/guard/qq_restart_test.go`: deterministic unit coverage for identity, normal restart, launch-only recovery, cleanup safety, errors, and final binding.
- Modify `app.go`: inject the close-observation callback, poll runtime plus process snapshots, and route only `qq_ws` into `RestartQQMiniapp`.
- Modify `app_test.go`: replace the legacy QQ stop-delay-launch expectation with final reconnect, launch-only, final event, and routing tests.
- Modify `app_guardian_minimize_test.go`: assert QQ minimizes only the refreshed replacement window and never minimizes on failure or when disabled.

### Task 1: Add Strict QQ Miniapp Identity And Tree Queries

**Files:**
- Create: `internal/runtime/guard/qq_restart.go`
- Create: `internal/runtime/guard/qq_restart_test.go`

- [ ] **Step 1: Write failing tests for strict root selection**

Create `qq_restart_test.go` with the package and imports, then add:

```go
package guard

import "testing"

func TestQQMiniappCandidatesRequireTitledRootUnderQQParent(t *testing.T) {
	snapshots := []HostProcessSnapshot{
		{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", Windows: []HostWindowSnapshot{{PID: 10, HWND: 100, Title: "QQ", Visible: true}}},
		{PID: 20, ParentPID: 10, ProcessName: "QQ.exe", Windows: []HostWindowSnapshot{{PID: 20, HWND: 200, Title: "QQ经典农场", Visible: true}}},
		{PID: 21, ParentPID: 20, ProcessName: "QQ.exe"},
		{PID: 22, ParentPID: 10, ProcessName: "QQ.exe", Windows: []HostWindowSnapshot{{PID: 22, HWND: 220, Title: "hidden farm", Visible: false}}},
		{PID: 30, ParentPID: 1, ProcessName: "QQ.exe", Windows: []HostWindowSnapshot{{PID: 30, HWND: 300, Title: "QQ经典农场", Visible: true}}},
	}

	candidates := QQMiniappCandidates(snapshots)
	if len(candidates) != 1 || candidates[0].PID != 20 || candidates[0].ParentPID != 10 {
		t.Fatalf("strict QQ candidates = %#v, want PID 20", candidates)
	}
}

func TestQQMiniappCandidatesNeverReturnMainOrProcessOnlyQQ(t *testing.T) {
	snapshots := []HostProcessSnapshot{
		{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", Windows: []HostWindowSnapshot{{PID: 10, HWND: 100, Title: "QQ", Visible: true}}},
		{PID: 20, ParentPID: 10, ProcessName: "QQ.exe"},
		{PID: 21, ParentPID: 20, ProcessName: "QQ.exe"},
	}
	if candidates := QQMiniappCandidates(snapshots); len(candidates) != 0 {
		t.Fatalf("strict QQ candidates = %#v, want none", candidates)
	}
}
```

- [ ] **Step 2: Run strict selection tests and verify RED**

Run: `go test ./internal/runtime/guard -run '^TestQQMiniappCandidates' -count=1`

Expected: build failure with `undefined: QQMiniappCandidates`.

- [ ] **Step 3: Implement strict selection and tree-exit helpers**

Create `qq_restart.go` with:

```go
package guard

import (
	"sort"
	"strings"
)

const DefaultQQMiniappCloseTimeout = 5 * time.Second

func QQMiniappCandidates(snapshots []HostProcessSnapshot) []HostProcessCandidate {
	byPID := make(map[int]HostProcessSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		byPID[snapshot.PID] = snapshot
	}
	result := make([]HostProcessCandidate, 0, 1)
	for _, snapshot := range snapshots {
		if !strings.EqualFold(strings.TrimSpace(snapshot.ProcessName), "QQ.exe") {
			continue
		}
		parent, ok := byPID[snapshot.ParentPID]
		if !ok || !strings.EqualFold(strings.TrimSpace(parent.ProcessName), "QQ.exe") {
			continue
		}
		candidate := candidateFromSnapshot(snapshot)
		if len(candidate.HWNDs) == 0 || !hasMiniappWindowTitle(candidate.WindowTitles) {
			continue
		}
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].PID < result[j].PID })
	return result
}

func QQMiniappTreeExited(rootPID int, snapshots []HostProcessSnapshot) bool {
	if rootPID <= 0 {
		return true
	}
	for _, snapshot := range snapshots {
		if snapshot.PID == rootPID || processDescendsFrom(snapshot.PID, rootPID, snapshots) {
			return false
		}
	}
	return true
}

func processDescendsFrom(pid int, rootPID int, snapshots []HostProcessSnapshot) bool {
	parents := make(map[int]int, len(snapshots))
	for _, snapshot := range snapshots {
		parents[snapshot.PID] = snapshot.ParentPID
	}
	seen := map[int]bool{}
	for current := pid; current > 0 && !seen[current]; current = parents[current] {
		seen[current] = true
		parent, ok := parents[current]
		if !ok {
			return false
		}
		if parent == rootPID {
			return true
		}
	}
	return false
}
```

Task 2 will add `errors`, `fmt`, and `time` when the restart orchestrator first uses them.

- [ ] **Step 4: Add failing tests for descendants after root exit**

Add:

```go
func TestQQMiniappTreeExitedDetectsRootAndOrphanedDescendants(t *testing.T) {
	withRoot := []HostProcessSnapshot{
		{PID: 20, ParentPID: 10, ProcessName: "QQ.exe"},
		{PID: 21, ParentPID: 20, ProcessName: "QQ.exe"},
		{PID: 22, ParentPID: 21, ProcessName: "QQ.exe"},
	}
	if QQMiniappTreeExited(20, withRoot) {
		t.Fatal("tree reported exited while root was present")
	}
	if QQMiniappTreeExited(20, withRoot[1:]) {
		t.Fatal("tree reported exited while descendants still referenced the old root")
	}
	if !QQMiniappTreeExited(20, []HostProcessSnapshot{{PID: 10, ProcessName: "QQ.exe"}}) {
		t.Fatal("tree did not report exited after root and descendants disappeared")
	}
}
```

- [ ] **Step 5: Run focused and package tests and verify GREEN**

Run: `go test ./internal/runtime/guard -run '^(TestQQMiniappCandidates|TestQQMiniappTreeExited)' -count=1`

Expected: PASS.

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS, including legacy `AutoBind` and WeChat restart tests.

- [ ] **Step 6: Commit strict QQ identity**

```powershell
git add internal/runtime/guard/qq_restart.go internal/runtime/guard/qq_restart_test.go
git commit -m "feat: identify QQ miniapp process trees"
```

### Task 2: Implement Graceful QQ Restart And Launch-Only Recovery

**Files:**
- Modify: `internal/runtime/guard/qq_restart.go`
- Modify: `internal/runtime/guard/qq_restart_test.go`

- [ ] **Step 1: Write the successful graceful-restart failing test**

Replace the test-file import with the imports used by the successful flow:

```go
import (
	"fmt"
	"reflect"
	"testing"
	"time"
)
```

The test compiles against the following intended API. Do not define these types in the test file; Step 3 adds them to `qq_restart.go`:

```go
type QQCloseObservation struct {
	Disconnected bool
	TreeExited   bool
	Snapshots    []HostProcessSnapshot
}

type QQReadyObservation struct {
	Accepted   bool
	Target     string
	Connected  bool
	Ready      bool
	InstanceID string
}

type QQRestartRequest struct {
	Registry         *HostBindingRegistry
	Owner            string
	Snapshots        []HostProcessSnapshot
	RuntimeConnected bool
	RuntimeInstanceID string
	CloseWindows     func([]HostWindowSnapshot) error
	WaitForClosed    func(int, time.Duration) (QQCloseObservation, error)
	StopPID          func(int) error
	Launch           func(LaunchRequest) error
	WaitForReady     func(string, time.Duration) QQReadyObservation
	RefreshSnapshots func() ([]HostProcessSnapshot, error)
	CloseTimeout     time.Duration
	ReconnectTimeout time.Duration
}
```

Then add:

```go
func TestRestartQQMiniappClosesWaitsLaunchesAndBindsReplacement(t *testing.T) {
	initial := qqSnapshots(20, 200)
	registry := NewHostBindingRegistry()
	calls := []string{}
	request := QQRestartRequest{
		Registry: registry, Owner: "main", Snapshots: initial,
		RuntimeConnected: true, RuntimeInstanceID: "qq-1",
		CloseWindows: func(windows []HostWindowSnapshot) error {
			calls = append(calls, fmt.Sprintf("close:%d", windows[0].HWND))
			return nil
		},
		WaitForClosed: func(rootPID int, timeout time.Duration) (QQCloseObservation, error) {
			calls = append(calls, fmt.Sprintf("closed:%d:%s", rootPID, timeout))
			return QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: qqMainOnlySnapshots()}, nil
		},
		StopPID: func(pid int) error { t.Fatalf("unexpected forced stop: %d", pid); return nil },
		Launch: func(launch LaunchRequest) error {
			calls = append(calls, "launch")
			if launch.Mode != "protocol" || launch.Protocol != qqFarmLaunchProtocol {
				t.Fatalf("launch = %#v", launch)
			}
			return nil
		},
		WaitForReady: func(previous string, timeout time.Duration) QQReadyObservation {
			calls = append(calls, "ready:"+previous+":"+timeout.String())
			return QQReadyObservation{Accepted: true, Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-2"}
		},
		RefreshSnapshots: func() ([]HostProcessSnapshot, error) {
			calls = append(calls, "refresh")
			return qqSnapshots(30, 300), nil
		},
		CloseTimeout: 5 * time.Second, ReconnectTimeout: 45 * time.Second,
	}

	result, err := RestartQQMiniapp(request)
	if err != nil {
		t.Fatalf("restart QQ miniapp: %v", err)
	}
	if result.Status != RestartStatusReconnected || result.OldPID != 20 || result.Stopped || !result.LaunchDispatched {
		t.Fatalf("result = %#v", result)
	}
	if result.Binding == nil || result.Binding.PID != 30 || !reflect.DeepEqual(result.Binding.HWNDs, []uint64{300}) {
		t.Fatalf("replacement binding = %#v", result.Binding)
	}
	want := []string{"close:200", "closed:20:5s", "launch", "ready:qq-1:45s", "refresh"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func qqSnapshots(rootPID int, hwnd uint64) []HostProcessSnapshot {
	return []HostProcessSnapshot{
		{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []HostWindowSnapshot{{PID: 10, HWND: 100, Title: "QQ", Visible: true}}},
		{PID: rootPID, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []HostWindowSnapshot{{PID: rootPID, HWND: hwnd, Title: "QQ经典农场", Visible: true}}},
		{PID: rootPID + 1, ParentPID: rootPID, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
	}
}

func qqMainOnlySnapshots() []HostProcessSnapshot {
	return []HostProcessSnapshot{{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`}}
}
```

Add `fmt` to the test imports.

- [ ] **Step 2: Run the successful flow and verify RED**

Run: `go test ./internal/runtime/guard -run '^TestRestartQQMiniappClosesWaitsLaunchesAndBindsReplacement$' -count=1`

Expected: build failure because `QQRestartRequest`, observation types, and `RestartQQMiniapp` do not exist.

- [ ] **Step 3: Implement the request types, strict selection, launch, and final binding**

Expand the `qq_restart.go` import block to:

```go
import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)
```

Append to `qq_restart.go`:

```go
type QQCloseObservation struct {
	Disconnected bool
	TreeExited   bool
	Snapshots    []HostProcessSnapshot
}

type QQReadyObservation struct {
	Accepted   bool
	Target     string
	Connected  bool
	Ready      bool
	InstanceID string
}

type QQRestartRequest struct {
	Registry          *HostBindingRegistry
	Owner             string
	Snapshots         []HostProcessSnapshot
	RuntimeConnected  bool
	RuntimeInstanceID string
	CloseWindows      func([]HostWindowSnapshot) error
	WaitForClosed     func(int, time.Duration) (QQCloseObservation, error)
	StopPID           func(int) error
	Launch            func(LaunchRequest) error
	WaitForReady      func(string, time.Duration) QQReadyObservation
	RefreshSnapshots  func() ([]HostProcessSnapshot, error)
	CloseTimeout      time.Duration
	ReconnectTimeout  time.Duration
}

func RestartQQMiniapp(request QQRestartRequest) (RestartResult, error) {
	if request.Registry == nil {
		return RestartResult{}, errors.New("host binding registry is required")
	}
	owner := normalizeOwner(request.Owner)
	result := RestartResult{Owner: owner, Candidates: QQMiniappCandidates(request.Snapshots)}
	fail := func(stage string, err error) (RestartResult, error) {
		wrapped := fmt.Errorf("%s: %w", stage, err)
		result.Status = "restart_failed"
		result.Reason = wrapped.Error()
		return result, wrapped
	}

	if len(result.Candidates) > 1 {
		return fail("qq_miniapp_unverified", errors.New("multiple strict QQ miniapp roots found"))
	}
	if len(result.Candidates) == 0 {
		if request.RuntimeConnected {
			return fail("qq_miniapp_unverified", errors.New("runtime is connected without a strict QQ miniapp root"))
		}
		request.Registry.Clear(owner)
		return launchAndBindQQMiniapp(request, result, 0, 0)
	}

	current := result.Candidates[0]
	binding, err := request.Registry.Bind(owner, current)
	if err != nil {
		return fail("binding_refresh_failed", err)
	}
	result.OldPID = binding.PID
	result.Binding = &binding
	currentSnapshot, ok := findSnapshotByPID(request.Snapshots, binding.PID)
	if !ok {
		return fail("qq_miniapp_unverified", errors.New("selected QQ miniapp root disappeared"))
	}
	if request.CloseWindows == nil {
		return fail("close_failed", errors.New("close windows callback is required"))
	}
	if err := request.CloseWindows(currentSnapshot.Windows); err != nil {
		return fail("close_failed", err)
	}
	if request.WaitForClosed == nil {
		return fail("disconnect_timeout", errors.New("close observer is required"))
	}
	observation, err := request.WaitForClosed(binding.PID, request.CloseTimeout)
	if err != nil {
		return fail("disconnect_timeout", err)
	}
	if !observation.TreeExited {
		return fail("old_tree_exit_timeout", errors.New("old QQ miniapp subtree is still running"))
	}
	if !observation.Disconnected {
		return fail("disconnect_timeout", errors.New("old QQ WS session is still connected"))
	}
	request.Registry.Clear(owner)
	return launchAndBindQQMiniapp(request, result, binding.PID, binding.ParentPID)
}

func launchAndBindQQMiniapp(request QQRestartRequest, result RestartResult, oldPID int, expectedParentPID int) (RestartResult, error) {
	fail := func(stage string, err error) (RestartResult, error) {
		wrapped := fmt.Errorf("%s: %w", stage, err)
		result.Status = "restart_failed"
		result.Reason = wrapped.Error()
		return result, wrapped
	}
	if request.Launch == nil {
		return fail("launch_failed", errors.New("launch callback is required"))
	}
	launchRequest, err := launchRequestForPlatform("qq", result.Binding)
	if err != nil {
		return fail("launch_failed", err)
	}
	if err := request.Launch(launchRequest); err != nil {
		return fail("launch_failed", err)
	}
	result.LaunchDispatched = true
	if request.WaitForReady == nil {
		return fail("reconnect_timeout", errors.New("ready observer is required"))
	}
	ready := request.WaitForReady(request.RuntimeInstanceID, request.ReconnectTimeout)
	if !ready.Accepted || strings.TrimSpace(ready.Target) != "qq_ws" {
		return fail("reconnect_timeout", errors.New("replacement QQ WS readiness was not accepted"))
	}
	if !ready.Connected || !ready.Ready {
		return fail("reconnect_timeout", errors.New("replacement QQ WS session did not become ready"))
	}
	if strings.TrimSpace(ready.InstanceID) == "" || (request.RuntimeInstanceID != "" && ready.InstanceID == request.RuntimeInstanceID) {
		return fail("new_instance_not_observed", errors.New("replacement QQ WS instance ID was not observed"))
	}
	if request.RefreshSnapshots == nil {
		return fail("new_process_not_found", errors.New("snapshot refresh callback is required"))
	}
	refreshed, err := request.RefreshSnapshots()
	if err != nil {
		return fail("new_process_not_found", err)
	}
	candidates := QQMiniappCandidates(refreshed)
	filtered := candidates[:0]
	for _, candidate := range candidates {
		if candidate.PID == oldPID || (expectedParentPID > 0 && candidate.ParentPID != expectedParentPID) {
			continue
		}
		filtered = append(filtered, candidate)
	}
	result.Candidates = filtered
	if len(filtered) != 1 {
		return fail("new_process_not_found", fmt.Errorf("strict replacement candidates: %d", len(filtered)))
	}
	binding, err := request.Registry.Bind(result.Owner, filtered[0])
	if err != nil {
		return fail("binding_refresh_failed", err)
	}
	result.Status = RestartStatusReconnected
	result.Reason = "QQ WS reconnected and became ready"
	result.Binding = &binding
	return result, nil
}
```

This first implementation deliberately returns `old_tree_exit_timeout` without force cleanup; Task 3 adds the verified timeout recovery after its safety tests exist.

- [ ] **Step 4: Add failing tests for launch-only and final-state failures**

Before the failure tests, add this complete reusable fixture:

```go
func newQQRestartFixture(t *testing.T) (*HostBindingRegistry, QQRestartRequest, *[]string) {
	t.Helper()
	registry := NewHostBindingRegistry()
	calls := []string{}
	request := QQRestartRequest{
		Registry: registry, Owner: "main", Snapshots: qqSnapshots(20, 200),
		RuntimeConnected: true, RuntimeInstanceID: "qq-1",
		CloseWindows: func(windows []HostWindowSnapshot) error {
			calls = append(calls, fmt.Sprintf("close:%d", windows[0].HWND))
			return nil
		},
		WaitForClosed: func(rootPID int, timeout time.Duration) (QQCloseObservation, error) {
			calls = append(calls, fmt.Sprintf("closed:%d:%s", rootPID, timeout))
			return QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: qqMainOnlySnapshots()}, nil
		},
		StopPID: func(pid int) error {
			t.Fatalf("unexpected forced stop: %d", pid)
			return nil
		},
		Launch: func(launch LaunchRequest) error {
			calls = append(calls, "launch")
			return nil
		},
		WaitForReady: func(previous string, timeout time.Duration) QQReadyObservation {
			calls = append(calls, "ready:"+previous+":"+timeout.String())
			return QQReadyObservation{Accepted: true, Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-2"}
		},
		RefreshSnapshots: func() ([]HostProcessSnapshot, error) {
			calls = append(calls, "refresh")
			return qqSnapshots(30, 300), nil
		},
		CloseTimeout: 5 * time.Second, ReconnectTimeout: 45 * time.Second,
	}
	return registry, request, &calls
}
```

Add `errors` and `strings` to the test imports, then add tests with these exact names and assertions:

```text
TestRestartQQMiniappLaunchOnlyWhenDisconnected
  snapshots contain only QQ main; RuntimeConnected is false; no close or stop callback runs; launch, ready, refresh run; PID 30 is bound; status is reconnected
TestRestartQQMiniappRejectsConnectedRuntimeWithoutRoot
  snapshots contain only QQ main; RuntimeConnected is true; error contains qq_miniapp_unverified; no close, stop, launch, or ready callback runs
TestRestartQQMiniappRejectsAmbiguousRoots
  two titled roots exist under the QQ main; error contains qq_miniapp_unverified; no process action runs
TestRestartQQMiniappCloseFailureDoesNotStopOrLaunch
  CloseWindows returns a sentinel error; result is restart_failed; StopPID and Launch remain uncalled
TestRestartQQMiniappRequiresDisconnectBeforeLaunch
  WaitForClosed returns TreeExited true and Disconnected false; error contains disconnect_timeout; Launch remains uncalled
TestRestartQQMiniappReadyTimeoutIsFinalFailure
  WaitForReady returns a zero observation (Accepted false and empty Target); result.LaunchDispatched is true; error contains reconnect_timeout; RefreshSnapshots remains uncalled
TestRestartQQMiniappRequiresAcceptedQQWSReadyObservation
  table cases return healthy Connected/Ready fields with Accepted false or a non-qq_ws Target; error contains reconnect_timeout; RefreshSnapshots remains uncalled
TestRestartQQMiniappLaunchFailureDoesNotWaitForReady
  Launch returns a sentinel error; error contains launch_failed; WaitForReady and RefreshSnapshots remain uncalled
TestRestartQQMiniappRejectsOldOrEmptyInstanceID
  table cases return old ID qq-1 or empty ID while Accepted is true, Target is qq_ws, and Connected and Ready are true; error contains new_instance_not_observed
TestRestartQQMiniappRejectsMissingOrAmbiguousReplacement
  table cases refresh zero or two strict roots; error contains new_process_not_found; registry has no replacement binding
TestRestartQQMiniappReportsBindingRefreshFailure
  pre-bind replacement PID 30 to another owner before restart; error contains binding_refresh_failed; the other owner binding remains unchanged
```

Each test builds its complete request explicitly or through a `newQQRestartFixture(t)` helper that returns a request with the same concrete callbacks and snapshots as the successful test. Every callback not expected in a branch must call `t.Fatalf` if invoked.

- [ ] **Step 5: Run focused tests, complete missing branches, and verify GREEN**

Run: `go test ./internal/runtime/guard -run '^TestRestartQQMiniapp' -count=1`

Expected after completing the branches above: PASS.

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS.

- [ ] **Step 6: Commit graceful and launch-only QQ restart**

```powershell
git add internal/runtime/guard/qq_restart.go internal/runtime/guard/qq_restart_test.go
git commit -m "feat: restart QQ miniapp through final readiness"
```

### Task 3: Add Revalidated Timeout Cleanup Without Crossing The QQ Root

**Files:**
- Modify: `internal/runtime/guard/qq_restart.go`
- Modify: `internal/runtime/guard/qq_restart_test.go`

- [ ] **Step 1: Write the deepest-first cleanup failing test**

Add:

```go
func TestRestartQQMiniappTimeoutStopsVerifiedTreeDeepestFirst(t *testing.T) {
	_, request, calls := newQQRestartFixture(t)
	timedOut := []HostProcessSnapshot{
		{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
		{PID: 20, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []HostWindowSnapshot{{PID: 20, HWND: 200, Title: "QQ经典农场", Visible: true}}},
		{PID: 21, ParentPID: 20, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
		{PID: 22, ParentPID: 21, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
	}
	request.Snapshots = timedOut
	waits := 0
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		waits++
		if waits == 1 {
			return QQCloseObservation{Disconnected: false, TreeExited: false, Snapshots: timedOut}, nil
		}
		return QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: qqMainOnlySnapshots()}, nil
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		if len(timedOut) == 1 {
			return qqSnapshots(30, 300), nil
		}
		return append([]HostProcessSnapshot(nil), timedOut...), nil
	}
	request.StopPID = func(pid int) error {
		*calls = append(*calls, fmt.Sprintf("stop:%d", pid))
		for i := range timedOut {
			if timedOut[i].PID == pid {
				timedOut = append(timedOut[:i], timedOut[i+1:]...)
				break
			}
		}
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err != nil {
		t.Fatalf("restart with timeout cleanup: %v", err)
	}
	if !result.Stopped || result.Status != RestartStatusReconnected {
		t.Fatalf("result = %#v", result)
	}
	if got, want := filterCalls(*calls, "stop:"), []string{"stop:22", "stop:21", "stop:20"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("stops = %#v, want %#v", got, want)
	}
	for _, call := range filterCalls(*calls, "stop:") {
		if call == "stop:10" {
			t.Fatal("QQ main PID was stopped")
		}
	}
}
```

Add `filterCalls` as a complete helper:

```go
func filterCalls(calls []string, prefix string) []string {
	result := []string{}
	for _, call := range calls {
		if strings.HasPrefix(call, prefix) {
			result = append(result, call)
		}
	}
	return result
}
```

- [ ] **Step 2: Run the cleanup test and verify RED**

Run: `go test ./internal/runtime/guard -run '^TestRestartQQMiniappTimeoutStopsVerifiedTreeDeepestFirst$' -count=1`

Expected: FAIL because the first close observation returns `old_tree_exit_timeout` without calling `StopPID`.

- [ ] **Step 3: Implement stable fingerprints and deepest-first ordering**

Add:

```go
type qqProcessFingerprint struct {
	PID            int
	ParentPID      int
	ProcessName    string
	ExecutablePath string
	Depth          int
}

func qqTreeFingerprints(rootPID int, snapshots []HostProcessSnapshot) []qqProcessFingerprint {
	result := []qqProcessFingerprint{}
	for _, snapshot := range snapshots {
		depth, ok := qqTreeDepth(snapshot.PID, rootPID, snapshots)
		if !ok {
			continue
		}
		result = append(result, qqProcessFingerprint{
			PID: snapshot.PID, ParentPID: snapshot.ParentPID,
			ProcessName: snapshot.ProcessName, ExecutablePath: snapshot.ExecutablePath, Depth: depth,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Depth == result[j].Depth {
			return result[i].PID < result[j].PID
		}
		return result[i].Depth > result[j].Depth
	})
	return result
}

func qqTreeDepth(pid int, rootPID int, snapshots []HostProcessSnapshot) (int, bool) {
	if pid == rootPID {
		return 0, true
	}
	parents := make(map[int]int, len(snapshots))
	for _, snapshot := range snapshots {
		parents[snapshot.PID] = snapshot.ParentPID
	}
	seen := map[int]bool{}
	depth := 0
	for current := pid; current > 0 && !seen[current]; current = parents[current] {
		seen[current] = true
		parent, ok := parents[current]
		if !ok {
			return 0, false
		}
		depth++
		if parent == rootPID {
			return depth, true
		}
	}
	return 0, false
}

func qqFingerprintMatches(fingerprint qqProcessFingerprint, snapshot HostProcessSnapshot) bool {
	return fingerprint.PID == snapshot.PID &&
		fingerprint.ParentPID == snapshot.ParentPID &&
		strings.EqualFold(strings.TrimSpace(fingerprint.ProcessName), strings.TrimSpace(snapshot.ProcessName)) &&
		strings.EqualFold(strings.TrimSpace(fingerprint.ExecutablePath), strings.TrimSpace(snapshot.ExecutablePath))
}
```

Before `CloseWindows` runs in `RestartQQMiniapp`, capture the authorized process set from the original strict snapshot:

```go
authorizedTree := qqTreeFingerprints(binding.PID, request.Snapshots)
if len(authorizedTree) == 0 || authorizedTree[len(authorizedTree)-1].PID != binding.PID {
	return fail("qq_miniapp_unverified", errors.New("authorized QQ miniapp tree fingerprint was not found"))
}
```

This original set is the maximum set of PIDs that timeout cleanup may stop. A process created after `WM_CLOSE` is not added to the authorization set.

- [ ] **Step 4: Implement per-PID revalidation and timeout cleanup**

Add:

```go
func stopVerifiedQQTree(request QQRestartRequest, rootPID int, authorized []qqProcessFingerprint, snapshots []HostProcessSnapshot) error {
	if request.StopPID == nil || request.RefreshSnapshots == nil {
		return errors.New("verified process cleanup callbacks are required")
	}
	if len(authorized) == 0 || authorized[len(authorized)-1].PID != rootPID {
		return errors.New("authorized QQ miniapp root fingerprint was not found")
	}
	timedOutRoot, ok := findSnapshotByPID(snapshots, rootPID)
	if !ok || !qqFingerprintMatches(authorized[len(authorized)-1], timedOutRoot) {
		return errors.New("verified QQ miniapp root identity changed before cleanup")
	}
	for _, fingerprint := range authorized {
		currentSnapshots, err := request.RefreshSnapshots()
		if err != nil {
			return err
		}
		current, ok := findSnapshotByPID(currentSnapshots, fingerprint.PID)
		if !ok {
			continue
		}
		if !qqFingerprintMatches(fingerprint, current) {
			return fmt.Errorf("PID %d identity changed before cleanup", fingerprint.PID)
		}
		if fingerprint.PID != rootPID && !processDescendsFrom(fingerprint.PID, rootPID, currentSnapshots) {
			return fmt.Errorf("PID %d left the verified QQ miniapp tree", fingerprint.PID)
		}
		if err := request.StopPID(fingerprint.PID); err != nil {
			return fmt.Errorf("stop PID %d: %w", fingerprint.PID, err)
		}
	}
	return nil
}
```

Replace the first `!observation.TreeExited` return inside `RestartQQMiniapp` with:

```go
if !observation.TreeExited {
	if err := stopVerifiedQQTree(request, binding.PID, authorizedTree, observation.Snapshots); err != nil {
		return fail("old_tree_cleanup_failed", err)
	}
	result.Stopped = true
	observation, err = request.WaitForClosed(binding.PID, request.CloseTimeout)
	if err != nil {
		return fail("old_tree_exit_timeout", err)
	}
}
if !observation.TreeExited {
	return fail("old_tree_exit_timeout", errors.New("old QQ miniapp subtree is still running"))
}
if !observation.Disconnected {
	return fail("disconnect_timeout", errors.New("old QQ WS session is still connected"))
}
```

- [ ] **Step 5: Add failing safety tests before completing cleanup behavior**

Add these complete scenarios with callbacks that fail the test if any unsafe PID is stopped:

```text
TestRestartQQMiniappCleanupRejectsChangedRootParent
  first observation has root PID 20 under parent 10; RefreshSnapshots changes root parent to 99; error contains old_tree_cleanup_failed; no PID is stopped
TestRestartQQMiniappCleanupRejectsChangedExecutable
  RefreshSnapshots changes root executable path; error contains old_tree_cleanup_failed; root and parent remain running
TestRestartQQMiniappCleanupSkipsAlreadyExitedChildren
  timeout snapshot contains 22 -> 21 -> 20; first refresh omits 22; stops are 21 then 20; operation continues
TestRestartQQMiniappCleanupNeverStopsParent
  timeout snapshot includes parent PID 10 and subtree; record every StopPID call; assert PID 10 is absent
TestRestartQQMiniappCleanupStopFailureDoesNotLaunch
  StopPID for a child returns a sentinel error; error contains old_tree_cleanup_failed; Launch remains uncalled
TestRestartQQMiniappCleanupStillRunningDoesNotLaunch
  second WaitForClosed remains TreeExited false; error contains old_tree_exit_timeout; Launch remains uncalled
TestRestartQQMiniappCleanupStillConnectedDoesNotLaunch
  second WaitForClosed is TreeExited true and Disconnected false; error contains disconnect_timeout; Launch remains uncalled
```

- [ ] **Step 6: Run focused cleanup tests and verify GREEN**

Run: `go test ./internal/runtime/guard -run '^TestRestartQQMiniapp(Cleanup|Timeout)' -count=1`

Expected: PASS.

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS.

- [ ] **Step 7: Commit verified QQEX cleanup**

```powershell
git add internal/runtime/guard/qq_restart.go internal/runtime/guard/qq_restart_test.go
git commit -m "fix: bound QQ restart cleanup to verified subtree"
```

### Task 4: Route `qq_ws` Through The Dedicated Orchestrator

**Files:**
- Modify: `app.go:79-93,298-310,3203-3276`
- Modify: `app_test.go:1785-1840`
- Modify: `app_guardian_minimize_test.go`

- [ ] **Step 1: Replace the legacy QQ stop-delay-launch test with a failing final-result test**

Replace `TestAppQQRestartKeepsStopDelayLaunchFlow` with:

```go
func TestAppQQRestartClosesAndWaitsForReplacementReady(t *testing.T) {
	app := newAuthorizedTestApp(t)
	app.cfg.Runtime.CurrentTarget = "qq_ws"
	app.manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-1"})
	initial := qqAppSnapshots(20, 200)
	refreshed := qqAppSnapshots(30, 300)
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) { return initial, nil }
	calls := []string{}
	app.closeHostWindows = func(windows []guard.HostWindowSnapshot) error {
		calls = append(calls, "close:"+itoa(int(windows[0].HWND)))
		return nil
	}
	app.waitForQQMiniappClosed = func(rootPID int, timeout time.Duration) (guard.QQCloseObservation, error) {
		calls = append(calls, "closed:"+itoa(rootPID)+":"+timeout.String())
		return guard.QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: qqAppMainOnlySnapshots()}, nil
	}
	app.stopHostPID = func(pid int) error { t.Fatalf("unexpected stop PID %d", pid); return nil }
	app.launchHost = func(guard.LaunchRequest) error { calls = append(calls, "launch"); return nil }
	app.waitForRuntimeStatus = func(timeout time.Duration, accept func(farmruntime.Status) bool) bool {
		calls = append(calls, "ready:"+timeout.String())
		app.manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-2"})
		return accept(app.manager.Status())
	}
	refreshCalls := 0
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
		refreshCalls++
		if refreshCalls == 1 {
			return initial, nil
		}
		calls = append(calls, "refresh")
		return refreshed, nil
	}

	result := app.RestartHostProcess()
	if result.Status != guard.RestartStatusReconnected || result.Binding == nil || result.Binding.PID != 30 {
		t.Fatalf("restart = %#v", result)
	}
	want := []string{"close:200", "closed:20:5s", "launch", "ready:45s", "refresh"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func qqAppSnapshots(rootPID int, hwnd uint64) []guard.HostProcessSnapshot {
	return []guard.HostProcessSnapshot{
		{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []guard.HostWindowSnapshot{{PID: 10, HWND: 100, Title: "QQ", Visible: true}}},
		{PID: rootPID, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []guard.HostWindowSnapshot{{PID: rootPID, HWND: hwnd, Title: "QQ经典农场", Visible: true}}},
	}
}

func qqAppMainOnlySnapshots() []guard.HostProcessSnapshot {
	return []guard.HostProcessSnapshot{{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`}}
}
```

- [ ] **Step 2: Run the app test and verify RED**

Run: `go test . -run '^TestAppQQRestartClosesAndWaitsForReplacementReady$' -count=1`

Expected: build failure because `App.waitForQQMiniappClosed` does not exist.

- [ ] **Step 3: Add the App callback and production close observer**

Add this field beside `waitForRuntimeStatus`:

```go
waitForQQMiniappClosed func(int, time.Duration) (guard.QQCloseObservation, error)
```

After `app` is constructed in `NewApp`, initialize it:

```go
app.waitForQQMiniappClosed = func(rootPID int, timeout time.Duration) (guard.QQCloseObservation, error) {
	return waitForQQMiniappClosed(app.manager, app.listHostSnapshots, rootPID, timeout)
}
```

Add the production helper beside `waitForRuntimeStatus`:

```go
func waitForQQMiniappClosed(
	manager *farmruntime.Manager,
	listSnapshots func() ([]guard.HostProcessSnapshot, error),
	rootPID int,
	timeout time.Duration,
) (guard.QQCloseObservation, error) {
	observe := func() (guard.QQCloseObservation, error) {
		if manager == nil || listSnapshots == nil {
			return guard.QQCloseObservation{}, errors.New("QQ close observer dependencies are required")
		}
		snapshots, err := listSnapshots()
		if err != nil {
			return guard.QQCloseObservation{}, err
		}
		status := manager.Status()
		return guard.QQCloseObservation{
			Disconnected: status.Target == string(farmruntime.RuntimeTargetQQWS) && !status.Connected,
			TreeExited: guard.QQMiniappTreeExited(rootPID, snapshots),
			Snapshots: snapshots,
		}, nil
	}
	observation, err := observe()
	if err != nil || (observation.Disconnected && observation.TreeExited) || timeout <= 0 {
		return observation, err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			observation, err = observe()
			if err != nil || (observation.Disconnected && observation.TreeExited) {
				return observation, err
			}
		case <-timer.C:
			return observation, nil
		}
	}
}
```

`app.go` already imports `errors`; if the import is absent when executing, add it.

- [ ] **Step 4: Add focused close-observer tests and verify them**

Add tests for immediate completion, delayed process/runtime transition, timeout returning the last partial observation, and snapshot errors:

```text
TestWaitForQQMiniappClosedImmediateSuccess
TestWaitForQQMiniappClosedWaitsForBothSignals
TestWaitForQQMiniappClosedTimeoutReturnsPartialState
TestWaitForQQMiniappClosedReturnsSnapshotError
```

Use a real `farmruntime.Manager`, deterministic snapshot call counters, and timeouts bounded below 200 milliseconds. Assert the observer never reports success when only one of `Disconnected` or `TreeExited` is true.

Run: `go test . -run '^TestWaitForQQMiniappClosed' -count=1`

Expected: PASS.

- [ ] **Step 5: Route QQ before generic auto-binding**

Change the start of `restartHostForRuntimeTarget` so QQ snapshots are passed directly to the strict orchestrator and generic `AutoBindHostProcess` is never called for QQ:

```go
func (a *App) restartHostForRuntimeTarget(runtimeTarget string) (guard.RestartResult, error) {
	target := farmruntime.RuntimeTarget(runtimeTarget)
	if target == farmruntime.RuntimeTargetQQWS {
		snapshots, err := a.listHostSnapshots()
		if err != nil {
			return guard.RestartResult{}, err
		}
		settings := a.guard.Settings()
		current := a.manager.Status()
		result, restartErr := guard.RestartQQMiniapp(guard.QQRestartRequest{
			Registry: a.hosts, Owner: "main", Snapshots: snapshots,
			RuntimeConnected: current.Target == string(farmruntime.RuntimeTargetQQWS) && current.Connected,
			RuntimeInstanceID: current.InstanceID,
			CloseWindows: a.closeHostWindows,
			WaitForClosed: a.waitForQQMiniappClosed,
			StopPID: a.stopHostPID,
			Launch: a.launchHost,
			WaitForReady: func(previous string, timeout time.Duration) guard.QQReadyObservation {
				accepted := a.waitForRuntimeStatus(timeout, func(status farmruntime.Status) bool {
					return status.Target == string(farmruntime.RuntimeTargetQQWS) &&
						status.Connected && status.Ready && status.InstanceID != "" && status.InstanceID != previous
				})
				status := a.manager.Status()
				return guard.QQReadyObservation{
					Accepted: accepted, Target: status.Target,
					Connected: status.Connected, Ready: status.Ready, InstanceID: status.InstanceID,
				}
			},
			RefreshSnapshots: a.listHostSnapshots,
			CloseTimeout: guard.DefaultQQMiniappCloseTimeout,
			ReconnectTimeout: time.Duration(settings.RestartReconnectGraceSec) * time.Second,
		})
		if restartErr == nil && result.Status == guard.RestartStatusReconnected && settings.AutoMinimizeAfterRestart {
			a.queuePendingHostMinimize(runtimeTarget)
			a.consumePendingHostMinimize(runtimeTarget)
		}
		return result, restartErr
	}

	a.AutoBindHostProcess()
	snapshots, err := a.listHostSnapshots()
	if err != nil {
		return guard.RestartResult{}, err
	}
	// Keep the existing WeChat branch and generic YYB RestartBoundHost call below.
```

Remove QQ from the old delayed-minimize condition by leaving that condition reachable only for the generic YYB result. Do not change `RunGuardianAction`; it already accepts `RestartStatusReconnected`.

- [ ] **Step 6: Add launch-only and unchanged-platform integration tests**

Add:

```text
TestAppQQRestartLaunchesWithoutStoppingWhenAlreadyClosed
  initial snapshots contain only QQ main; manager is disconnected; no close or StopPID runs; final PID 30 is bound and result is reconnected
TestAppQQRestartNeverAutoBindsQQMain
  manager is connected and snapshots contain only main PID 10; result is restart_failed with qq_miniapp_unverified; StopPID and Launch remain uncalled
TestAppWeChatRestartStillPreservesHostAndWaitsForReconnect
  retain the existing same-PID close/disconnect/ready/refresh assertions
TestAppYYBRestartStillUsesBoundStopDelayLaunch
  configure YYB candidate and assert stop, two-second sleep, and launch_dispatched behavior remains unchanged
TestAppQQManualRestartRecordsFinalRestartEvent
  invoke RestartHostProcess through the manager; assert one manual qq_ws event whose result status is reconnected
```

Run: `go test . -run 'Test(AppQQRestart|AppWeChatRestart|AppYYBRestart|AppQQManualRestart)' -count=1`

Expected: PASS.

- [ ] **Step 7: Update QQ auto-minimize tests**

Add this complete package-level test fixture in `app_test.go` so `app_guardian_minimize_test.go` can reuse it:

```go
func configureQQFinalRestartTest(t *testing.T, app *App) {
	t.Helper()
	app.cfg.Runtime.CurrentTarget = "qq_ws"
	app.manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-1"})
	listCalls := 0
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
		listCalls++
		if listCalls == 1 {
			return qqAppSnapshots(20, 200), nil
		}
		return qqAppSnapshots(30, 300), nil
	}
	app.closeHostWindows = func([]guard.HostWindowSnapshot) error { return nil }
	app.waitForQQMiniappClosed = func(int, time.Duration) (guard.QQCloseObservation, error) {
		return guard.QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: qqAppMainOnlySnapshots()}, nil
	}
	app.stopHostPID = func(pid int) error {
		t.Fatalf("unexpected forced stop PID %d", pid)
		return nil
	}
	app.launchHost = func(guard.LaunchRequest) error { return nil }
	app.waitForRuntimeStatus = func(_ time.Duration, accept func(farmruntime.Status) bool) bool {
		app.manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-2"})
		return accept(app.manager.Status())
	}
}
```

Replace the old delayed QQ minimize expectation with a final binding expectation:

```go
func TestGuardianQQRestartMinimizesReplacementWindowAfterReady(t *testing.T) {
	app := newAuthorizedTestApp(t)
	configureQQFinalRestartTest(t, app)
	if _, err := app.SaveGuardianSettings(map[string]any{"autoMinimizeAfterRestart": true}); err != nil {
		t.Fatalf("enable auto minimize: %v", err)
	}
	var minimized []guard.HostWindowSnapshot
	app.minimizeHostWindows = func(windows []guard.HostWindowSnapshot) error {
		minimized = append(minimized, windows...)
		return nil
	}
	result, err := app.restartHostForRuntimeTarget(string(farmruntime.RuntimeTargetQQWS))
	if err != nil || result.Status != guard.RestartStatusReconnected {
		t.Fatalf("restart = %#v, err = %v", result, err)
	}
	if len(minimized) != 1 || minimized[0].PID != 30 || minimized[0].HWND != 300 {
		t.Fatalf("minimized = %#v, want replacement PID 30 HWND 300", minimized)
	}
}
```

Retain table coverage proving disabled auto-minimize and launch/reconnect failures never minimize. Update those fixtures to use strict parent/root snapshots and final QQ callbacks rather than legacy `stopHostPID` success.

- [ ] **Step 8: Run root package and guard package tests and verify GREEN**

Run: `go test . -count=1`

Expected: PASS.

Run: `go test ./internal/runtime/guard -count=1`

Expected: PASS.

- [ ] **Step 9: Commit App routing and behavior**

```powershell
git add app.go app_test.go app_guardian_minimize_test.go
git commit -m "feat: route QQ restart through verified final reconnect"
```

### Task 5: Full Regression And Live Process Verification

**Files:**
- Modify only if verification exposes a defect in `app.go`, `app_test.go`, `app_guardian_minimize_test.go`, `internal/runtime/guard/qq_restart.go`, or `internal/runtime/guard/qq_restart_test.go`.

- [ ] **Step 1: Format every planned Go file**

Run:

```powershell
gofmt -w app.go app_test.go app_guardian_minimize_test.go internal/runtime/guard/qq_restart.go internal/runtime/guard/qq_restart_test.go
```

Expected: exit code 0.

- [ ] **Step 2: Run focused QQ restart tests**

Run: `go test ./internal/runtime/guard -run '^(TestQQMiniapp|TestRestartQQMiniapp)' -count=1`

Expected: PASS.

Run: `go test . -run 'Test(AppQQ|GuardianQQ|WaitForQQ)' -count=1`

Expected: PASS.

- [ ] **Step 3: Run the complete backend suite**

Run: `go test ./... -count=1`

Expected: PASS with zero failed packages.

- [ ] **Step 4: Run race-sensitive runtime packages**

Run: `go test -race ./internal/runtime/guard ./internal/runtime/qqws ./internal/runtime/qqlink -count=1`

Expected: PASS with no race reports.

- [ ] **Step 5: Run frontend regression tests**

Run from `frontend`: `npm test -- --run`

Expected: PASS. No generated Wails model change is expected because `RestartResult` fields remain unchanged.

- [ ] **Step 6: Run build and diff verification**

Run: `go build ./...`

Expected: exit code 0.

Run: `git diff --check`

Expected: no output.

Run: `git status --short`

Expected: only corrections to the planned files are present; WeChat, YYB, QQ WS framing, and patch injection files are unchanged.

- [ ] **Step 7: Capture the live baseline before clicking restart**

With QQ Farm connected, run:

```powershell
$all = Get-CimInstance Win32_Process
$root = $all | Where-Object { $_.Name -eq 'QQ.exe' -and $_.CommandLine -match 'QQEXMiniProgram' } | Select-Object -First 1
$main = $all | Where-Object { $_.ProcessId -eq $root.ParentProcessId } | Select-Object -First 1
[pscustomobject]@{
  QQMainPID = $main.ProcessId
  MiniappRootPID = $root.ProcessId
  MiniappChildren = @( $all | Where-Object ParentProcessId -eq $root.ProcessId ).Count
  FarmPID = @(Get-NetTCPConnection -State Listen -LocalPort 8787).OwningProcess
  WSConnections = @(Get-NetTCPConnection -State Established | Where-Object { $_.LocalPort -eq 8787 -or $_.RemotePort -eq 8787 }).Count
} | Format-List
```

Expected: one miniapp root, seven observed descendants on the current QQ build, Farm_Go listening on `8787`, and two established TCP endpoints for one loopback WS connection.

- [ ] **Step 8: Trigger one restart and capture the final state**

Click the existing Farm_Go restart action once. Do not close QQ or the miniapp manually during the operation. After the action resolves, rerun the baseline command.

Verify all of these conditions:

```text
QQ main PID                         unchanged
Farm_Go PID and 8787 listener       unchanged
old QQEX root PID                   absent
old QQEX descendants                absent
replacement QQEX root PID           present and different
replacement root parent PID         equals unchanged QQ main PID
visible QQ Farm window              owned by replacement root
old WS connection                   absent
new WS connection                   established
runtime InstanceID                  nonempty and different
runtime Connected and Ready         true
restart result                      reconnected
refreshed binding PID and HWND       match replacement window
```

If the graceful close exceeds five seconds, also verify recorded `StopPID` actions never include the QQ main PID or any process outside the old QQEX descendant chain.

- [ ] **Step 9: Commit verification-only corrections if required**

If verification finds a defect, first add a failing automated test for the observed stage, run it to confirm RED, apply the smallest correction in the planned files, rerun Steps 2 through 6, and commit only that correction:

```powershell
git add app.go app_test.go app_guardian_minimize_test.go internal/runtime/guard/qq_restart.go internal/runtime/guard/qq_restart_test.go
git commit -m "fix: harden QQ miniapp restart verification"
```

If verification requires no correction, do not create an empty commit.
