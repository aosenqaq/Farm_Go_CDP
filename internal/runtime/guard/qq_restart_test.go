package guard

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

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

func TestQQTreeFingerprintsRejectDuplicatePID(t *testing.T) {
	snapshots := append(qqSnapshots(20, 200), HostProcessSnapshot{
		PID: 21, ParentPID: 20, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`,
	})
	if _, err := qqTreeFingerprints(20, 10, snapshots); err == nil {
		t.Fatal("duplicate PID was accepted for cleanup authorization")
	}
}

func TestQQTreeFingerprintsExcludeRootParentAndPathsThroughParent(t *testing.T) {
	snapshots := []HostProcessSnapshot{
		{PID: 10, ParentPID: 20, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
		{PID: 20, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
		{PID: 21, ParentPID: 20, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
		{PID: 9, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
	}
	authorized, err := qqTreeFingerprints(20, 10, snapshots)
	if err != nil {
		t.Fatalf("build authorized tree: %v", err)
	}
	got := make([]int, 0, len(authorized))
	for _, fingerprint := range authorized {
		got = append(got, fingerprint.PID)
	}
	if want := []int{21, 20}; !reflect.DeepEqual(got, want) {
		t.Fatalf("authorized PIDs = %#v, want %#v", got, want)
	}
}

func TestRestartQQMiniappRejectsDuplicateInitialPIDWithoutStoppingOrLaunching(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	request.Snapshots = append(request.Snapshots, HostProcessSnapshot{
		PID: 21, ParentPID: 20, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`,
	})
	request.CloseWindows = func([]HostWindowSnapshot) error {
		t.Fatal("unexpected close with duplicate PID")
		return nil
	}
	request.StopPID = func(pid int) error {
		t.Fatalf("unexpected stop with duplicate PID %d", pid)
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch with duplicate PID")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "qq_miniapp_unverified") {
		t.Fatalf("duplicate initial PID = %#v, err = %v", result, err)
	}
	if result.Stopped || result.LaunchDispatched {
		t.Fatalf("duplicate initial PID result = %#v", result)
	}
}

func TestRestartQQMiniappCleanupCycleNeverStopsRootParent(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	running := []HostProcessSnapshot{
		{PID: 10, ParentPID: 20, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
		{PID: 20, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []HostWindowSnapshot{{PID: 20, HWND: 200, Title: "QQ经典农场", Visible: true}}},
		{PID: 21, ParentPID: 20, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
		{PID: 9, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
	}
	request.Snapshots = append([]HostProcessSnapshot(nil), running...)
	waits := 0
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		waits++
		if waits == 1 {
			return QQCloseObservation{TreeExited: false, Snapshots: request.Snapshots}, nil
		}
		return QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: running}, nil
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		if waits > 1 {
			return qqSnapshots(30, 300), nil
		}
		return append([]HostProcessSnapshot(nil), running...), nil
	}
	stopped := []int{}
	request.StopPID = func(pid int) error {
		stopped = append(stopped, pid)
		for i := range running {
			if running[i].PID == pid {
				running = append(running[:i], running[i+1:]...)
				break
			}
		}
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err != nil || result.Status != RestartStatusReconnected {
		t.Fatalf("cleanup with parent cycle = %#v, err = %v", result, err)
	}
	if want := []int{21, 20}; !reflect.DeepEqual(stopped, want) {
		t.Fatalf("stopped = %#v, want %#v", stopped, want)
	}
}

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

func TestRestartQQMiniappCleanupRejectsProcessedPIDReappearing(t *testing.T) {
	for name, change := range map[string]func(*HostProcessSnapshot){
		"same_fingerprint": func(*HostProcessSnapshot) {},
		"changed_identity": func(snapshot *HostProcessSnapshot) {
			snapshot.ProcessName = "Other.exe"
			snapshot.ExecutablePath = `D:\Other\Other.exe`
		},
	} {
		change := change
		t.Run(name, func(t *testing.T) {
			_, request, _ := newQQRestartFixture(t)
			initial := []HostProcessSnapshot{
				{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
				{PID: 20, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []HostWindowSnapshot{{PID: 20, HWND: 200, Title: "QQ经典农场", Visible: true}}},
				{PID: 21, ParentPID: 20, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
				{PID: 22, ParentPID: 21, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
			}
			request.Snapshots = initial
			running := append([]HostProcessSnapshot(nil), initial...)
			request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
				return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
			}
			refreshes := 0
			request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
				refreshes++
				current := append([]HostProcessSnapshot(nil), running...)
				if refreshes >= 3 {
					reused := initial[3]
					change(&reused)
					current = append(current, reused)
				}
				return current, nil
			}
			stopped := []int{}
			request.StopPID = func(pid int) error {
				stopped = append(stopped, pid)
				for i := range running {
					if running[i].PID == pid {
						running = append(running[:i], running[i+1:]...)
						break
					}
				}
				return nil
			}
			request.Launch = func(LaunchRequest) error {
				t.Fatal("unexpected launch after processed PID reappeared")
				return nil
			}

			result, err := RestartQQMiniapp(request)
			if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
				t.Fatalf("processed PID reuse = %#v, err = %v", result, err)
			}
			if want := []int{22}; !reflect.DeepEqual(stopped, want) {
				t.Fatalf("stopped = %#v, want %#v", stopped, want)
			}
			if !result.Stopped || result.LaunchDispatched {
				t.Fatalf("processed PID reuse result = %#v", result)
			}
		})
	}
}

func TestRestartQQMiniappCleanupRejectsPIDMissingThenReappearingBeforeTurn(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	initial := []HostProcessSnapshot{
		{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
		{PID: 20, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []HostWindowSnapshot{{PID: 20, HWND: 200, Title: "QQ经典农场", Visible: true}}},
		{PID: 21, ParentPID: 20, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
		{PID: 22, ParentPID: 21, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
	}
	request.Snapshots = initial
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
	}
	refreshes := 0
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		refreshes++
		if refreshes == 1 {
			return append([]HostProcessSnapshot(nil), initial[:3]...), nil
		}
		return append([]HostProcessSnapshot(nil), initial...), nil
	}
	stopped := []int{}
	request.StopPID = func(pid int) error {
		stopped = append(stopped, pid)
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch after missing PID reappeared")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
		t.Fatalf("missing PID reuse = %#v, err = %v", result, err)
	}
	if len(stopped) != 0 || result.Stopped || result.LaunchDispatched {
		t.Fatalf("missing PID reuse stopped = %#v, result = %#v", stopped, result)
	}
}

func TestRestartQQMiniappCleanupRejectsChangedRootParent(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
	}
	changed := append([]HostProcessSnapshot(nil), initial...)
	for i := range changed {
		if changed[i].PID == 20 {
			changed[i].ParentPID = 99
		}
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) { return changed, nil }
	stopped := []int{}
	request.StopPID = func(pid int) error {
		stopped = append(stopped, pid)
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
		t.Fatalf("changed root parent = %#v, err = %v", result, err)
	}
	if len(stopped) != 0 {
		t.Fatalf("stopped PIDs after root parent changed: %#v", stopped)
	}
}

func TestRestartQQMiniappCleanupRejectsChangedExecutable(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
	}
	changed := append([]HostProcessSnapshot(nil), initial...)
	for i := range changed {
		if changed[i].PID == 20 {
			changed[i].ExecutablePath = `D:\Other\QQ.exe`
		}
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) { return changed, nil }
	stopped := []int{}
	request.StopPID = func(pid int) error {
		stopped = append(stopped, pid)
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
		t.Fatalf("changed root executable = %#v, err = %v", result, err)
	}
	if len(stopped) != 0 {
		t.Fatalf("stopped PIDs after root executable changed: %#v", stopped)
	}
}

func TestRestartQQMiniappCleanupRejectsReusedAuthorizedPID(t *testing.T) {
	for name, change := range map[string]func(*HostProcessSnapshot){
		"process_name":    func(snapshot *HostProcessSnapshot) { snapshot.ProcessName = "Other.exe" },
		"executable_path": func(snapshot *HostProcessSnapshot) { snapshot.ExecutablePath = `D:\Other\QQ.exe` },
		"parent_pid":      func(snapshot *HostProcessSnapshot) { snapshot.ParentPID = 99 },
	} {
		change := change
		t.Run(name, func(t *testing.T) {
			_, request, _ := newQQRestartFixture(t)
			initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
			request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
				return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
			}
			changed := append([]HostProcessSnapshot(nil), initial...)
			for i := range changed {
				if changed[i].PID == 21 {
					change(&changed[i])
				}
			}
			request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) { return changed, nil }
			stopped := []int{}
			request.StopPID = func(pid int) error {
				stopped = append(stopped, pid)
				return nil
			}
			request.Launch = func(LaunchRequest) error {
				t.Fatal("unexpected launch")
				return nil
			}

			result, err := RestartQQMiniapp(request)
			if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
				t.Fatalf("reused authorized PID = %#v, err = %v", result, err)
			}
			if len(stopped) != 0 {
				t.Fatalf("stopped reused authorized PID: %#v", stopped)
			}
		})
	}
}

func TestRestartQQMiniappCleanupRejectsEmptyExecutablePathBeforeStopping(t *testing.T) {
	for name, emptyPID := range map[string]int{
		"root_empty":  20,
		"child_empty": 21,
	} {
		emptyPID := emptyPID
		t.Run(name, func(t *testing.T) {
			_, request, _ := newQQRestartFixture(t)
			initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
			for i := range initial {
				if initial[i].PID == emptyPID {
					initial[i].ExecutablePath = ""
				}
			}
			request.Snapshots = initial
			request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
				return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
			}
			request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) { return initial, nil }
			stopped := []int{}
			request.StopPID = func(pid int) error {
				stopped = append(stopped, pid)
				return nil
			}
			request.Launch = func(LaunchRequest) error {
				t.Fatal("unexpected launch with uncertain executable path")
				return nil
			}

			result, err := RestartQQMiniapp(request)
			if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
				t.Fatalf("empty executable path = %#v, err = %v", result, err)
			}
			if len(stopped) != 0 || result.Stopped || result.LaunchDispatched {
				t.Fatalf("empty executable path stopped = %#v, result = %#v", stopped, result)
			}
		})
	}
}

func TestRestartQQMiniappCleanupSkipsAlreadyExitedChildren(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	initial := []HostProcessSnapshot{
		{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
		{PID: 20, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []HostWindowSnapshot{{PID: 20, HWND: 200, Title: "QQ经典农场", Visible: true}}},
		{PID: 21, ParentPID: 20, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
		{PID: 22, ParentPID: 21, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
	}
	request.Snapshots = initial
	running := append([]HostProcessSnapshot(nil), initial[:3]...)
	waits := 0
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		waits++
		if waits == 1 {
			return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
		}
		return QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: qqMainOnlySnapshots()}, nil
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		if len(running) == 1 {
			return qqSnapshots(30, 300), nil
		}
		return append([]HostProcessSnapshot(nil), running...), nil
	}
	stopped := []int{}
	request.StopPID = func(pid int) error {
		stopped = append(stopped, pid)
		for i := range running {
			if running[i].PID == pid {
				running = append(running[:i], running[i+1:]...)
				break
			}
		}
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err != nil || result.Status != RestartStatusReconnected {
		t.Fatalf("cleanup with exited child = %#v, err = %v", result, err)
	}
	if want := []int{21, 20}; !reflect.DeepEqual(stopped, want) {
		t.Fatalf("stopped = %#v, want %#v", stopped, want)
	}
}

func TestRestartQQMiniappCleanupSkipsTreeThatExitedBeforeRefresh(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
	waits := 0
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		waits++
		if waits == 1 {
			return QQCloseObservation{Disconnected: false, TreeExited: false, Snapshots: initial}, nil
		}
		return QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: qqMainOnlySnapshots()}, nil
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		if waits == 1 {
			return qqMainOnlySnapshots(), nil
		}
		return qqSnapshots(30, 300), nil
	}
	request.StopPID = func(pid int) error {
		t.Fatalf("unexpected stop for already exited PID %d", pid)
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err != nil {
		t.Fatalf("cleanup after whole tree exited: %v", err)
	}
	if waits != 2 || result.Status != RestartStatusReconnected || result.Stopped {
		t.Fatalf("result = %#v, waits = %d", result, waits)
	}
	if result.Binding == nil || result.Binding.PID != 30 {
		t.Fatalf("replacement binding = %#v", result.Binding)
	}
}

func TestRestartQQMiniappCleanupRejectsLiveChildAfterRootExited(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		return QQCloseObservation{Disconnected: false, TreeExited: false, Snapshots: initial}, nil
	}
	orphaned := []HostProcessSnapshot{
		{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
		{PID: 21, ParentPID: 20, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) { return orphaned, nil }
	request.StopPID = func(pid int) error {
		t.Fatalf("unexpected stop for orphaned live child PID %d", pid)
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
		t.Fatalf("orphaned live child = %#v, err = %v", result, err)
	}
	if result.LaunchDispatched {
		t.Fatalf("orphaned live child launched: %#v", result)
	}
}

func TestRestartQQMiniappCleanupReportsPartialStopBeforeRootStopFailure(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
	running := append([]HostProcessSnapshot(nil), initial...)
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		return append([]HostProcessSnapshot(nil), running...), nil
	}
	sentinel := errors.New("root stop denied")
	successfulStops := []int{}
	request.StopPID = func(pid int) error {
		if pid == 20 {
			return sentinel
		}
		successfulStops = append(successfulStops, pid)
		for i := range running {
			if running[i].PID == pid {
				running = append(running[:i], running[i+1:]...)
				break
			}
		}
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch after partial cleanup failure")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
		t.Fatalf("partial cleanup failure = %#v, err = %v", result, err)
	}
	if want := []int{21}; !reflect.DeepEqual(successfulStops, want) {
		t.Fatalf("successful stops = %#v, want %#v", successfulStops, want)
	}
	if !result.Stopped || result.LaunchDispatched {
		t.Fatalf("partial cleanup result = %#v", result)
	}
}

func TestRestartQQMiniappCleanupNeverStopsParent(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	running := append([]HostProcessSnapshot(nil), request.Snapshots...)
	waits := 0
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		waits++
		if waits == 1 {
			return QQCloseObservation{TreeExited: false, Snapshots: running}, nil
		}
		return QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: qqMainOnlySnapshots()}, nil
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		if len(running) == 1 {
			return qqSnapshots(30, 300), nil
		}
		return append([]HostProcessSnapshot(nil), running...), nil
	}
	stopped := []int{}
	request.StopPID = func(pid int) error {
		stopped = append(stopped, pid)
		for i := range running {
			if running[i].PID == pid {
				running = append(running[:i], running[i+1:]...)
				break
			}
		}
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err != nil || result.Status != RestartStatusReconnected {
		t.Fatalf("cleanup preserving parent = %#v, err = %v", result, err)
	}
	for _, pid := range stopped {
		if pid == 10 {
			t.Fatalf("QQ parent was stopped: %#v", stopped)
		}
	}
}

func TestRestartQQMiniappCleanupStopFailureDoesNotLaunch(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) { return initial, nil }
	sentinel := errors.New("stop denied")
	request.StopPID = func(int) error { return sentinel }
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
		t.Fatalf("cleanup stop failure = %#v, err = %v", result, err)
	}
	if result.LaunchDispatched {
		t.Fatalf("cleanup stop failure launched: %#v", result)
	}
}

func TestRestartQQMiniappCleanupDoesNotAuthorizePostCloseChild(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
	running := append(append([]HostProcessSnapshot(nil), initial...), HostProcessSnapshot{
		PID: 22, ParentPID: 21, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`,
	})
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		return QQCloseObservation{TreeExited: false, Snapshots: running}, nil
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		return append([]HostProcessSnapshot(nil), running...), nil
	}
	stopped := []int{}
	request.StopPID = func(pid int) error {
		stopped = append(stopped, pid)
		for i := range running {
			if running[i].PID == pid {
				running = append(running[:i], running[i+1:]...)
				break
			}
		}
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch with post-close child")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
		t.Fatalf("cleanup with post-close child = %#v, err = %v", result, err)
	}
	if len(stopped) != 0 || result.LaunchDispatched {
		t.Fatalf("post-close child stopped = %#v, result = %#v", stopped, result)
	}
}

func TestRestartQQMiniappCleanupRejectsNewChildAppearingBeforeFirstStop(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
	}
	refreshes := 0
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		refreshes++
		current := append([]HostProcessSnapshot(nil), initial...)
		if refreshes >= 2 {
			current = append(current, HostProcessSnapshot{
				PID: 22, ParentPID: 21, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`,
			})
		}
		return current, nil
	}
	stopped := []int{}
	request.StopPID = func(pid int) error {
		stopped = append(stopped, pid)
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch with new child")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
		t.Fatalf("new child before first stop = %#v, err = %v", result, err)
	}
	if len(stopped) != 0 || result.LaunchDispatched {
		t.Fatalf("new child before first stop stopped = %#v, result = %#v", stopped, result)
	}
}

func TestRestartQQMiniappCleanupFindsNewChildAfterAuthorizedParentExited(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
	running := append([]HostProcessSnapshot(nil), initial...)
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
	}
	refreshes := 0
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		refreshes++
		current := append([]HostProcessSnapshot(nil), running...)
		if refreshes >= 3 {
			current = append(current, HostProcessSnapshot{
				PID: 22, ParentPID: 21, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`,
			})
		}
		return current, nil
	}
	stopped := []int{}
	request.StopPID = func(pid int) error {
		stopped = append(stopped, pid)
		for i := range running {
			if running[i].PID == pid {
				running = append(running[:i], running[i+1:]...)
				break
			}
		}
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch with fallback-linked child")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
		t.Fatalf("fallback-linked child = %#v, err = %v", result, err)
	}
	if want := []int{21}; !reflect.DeepEqual(stopped, want) {
		t.Fatalf("stopped = %#v, want %#v", stopped, want)
	}
	if result.LaunchDispatched {
		t.Fatalf("fallback-linked child launched: %#v", result)
	}
}

func TestRestartQQMiniappCleanupFinalRefreshRejectsNewChild(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
	running := append([]HostProcessSnapshot(nil), initial...)
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
	}
	refreshes := 0
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		refreshes++
		current := append([]HostProcessSnapshot(nil), running...)
		if refreshes >= 4 {
			current = append(current, HostProcessSnapshot{
				PID: 22, ParentPID: 21, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`,
			})
		}
		return current, nil
	}
	stopped := []int{}
	request.StopPID = func(pid int) error {
		stopped = append(stopped, pid)
		for i := range running {
			if running[i].PID == pid {
				running = append(running[:i], running[i+1:]...)
				break
			}
		}
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch after final refresh found new child")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
		t.Fatalf("final refresh new child = %#v, err = %v", result, err)
	}
	if want := []int{21, 20}; !reflect.DeepEqual(stopped, want) {
		t.Fatalf("stopped = %#v, want %#v", stopped, want)
	}
	if !result.Stopped || result.LaunchDispatched {
		t.Fatalf("final refresh new child result = %#v", result)
	}
}

func TestRestartQQMiniappCleanupSecondObservationFailureDoesNotLaunch(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	running := append([]HostProcessSnapshot(nil), request.Snapshots...)
	waits := 0
	sentinel := errors.New("second close observation failed")
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		waits++
		if waits == 1 {
			return QQCloseObservation{TreeExited: false, Snapshots: running}, nil
		}
		return QQCloseObservation{}, sentinel
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		return append([]HostProcessSnapshot(nil), running...), nil
	}
	request.StopPID = func(pid int) error {
		for i := range running {
			if running[i].PID == pid {
				running = append(running[:i], running[i+1:]...)
				break
			}
		}
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "old_tree_exit_timeout") {
		t.Fatalf("second close observation = %#v, err = %v", result, err)
	}
	if result.LaunchDispatched {
		t.Fatalf("second close observation failure launched: %#v", result)
	}
}

func TestRestartQQMiniappCleanupValidatesSecondObservationBeforeLaunch(t *testing.T) {
	for name, suspicious := range map[string]HostProcessSnapshot{
		"processed_authorized_pid": {
			PID: 20, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`,
		},
		"fallback_linked_new_pid": {
			PID: 22, ParentPID: 21, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`,
		},
	} {
		suspicious := suspicious
		t.Run(name, func(t *testing.T) {
			_, request, _ := newQQRestartFixture(t)
			initial := append([]HostProcessSnapshot(nil), request.Snapshots...)
			running := append([]HostProcessSnapshot(nil), initial...)
			waits := 0
			request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
				waits++
				if waits == 1 {
					return QQCloseObservation{TreeExited: false, Snapshots: initial}, nil
				}
				observed := append(qqMainOnlySnapshots(), suspicious)
				return QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: observed}, nil
			}
			request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
				if waits > 1 {
					return qqSnapshots(30, 300), nil
				}
				return append([]HostProcessSnapshot(nil), running...), nil
			}
			request.StopPID = func(pid int) error {
				for i := range running {
					if running[i].PID == pid {
						running = append(running[:i], running[i+1:]...)
						break
					}
				}
				return nil
			}
			launchCalled := false
			request.Launch = func(LaunchRequest) error {
				launchCalled = true
				return nil
			}

			result, err := RestartQQMiniapp(request)
			if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
				t.Fatalf("second observation validation = %#v, err = %v", result, err)
			}
			if launchCalled || result.LaunchDispatched || !result.Stopped {
				t.Fatalf("second observation validation result = %#v, launchCalled = %v", result, launchCalled)
			}
		})
	}
}

func TestRestartQQMiniappValidatesFirstClosedObservationBeforeLaunch(t *testing.T) {
	for name, suspicious := range map[string]HostProcessSnapshot{
		"authorized_pid_still_present": {
			PID: 20, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`,
		},
		"fallback_linked_new_pid": {
			PID: 22, ParentPID: 21, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`,
		},
	} {
		suspicious := suspicious
		t.Run(name, func(t *testing.T) {
			_, request, _ := newQQRestartFixture(t)
			request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
				observed := append(qqMainOnlySnapshots(), suspicious)
				return QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: observed}, nil
			}
			request.StopPID = func(pid int) error {
				t.Fatalf("unexpected stop after closed observation for PID %d", pid)
				return nil
			}
			launchCalled := false
			request.Launch = func(LaunchRequest) error {
				launchCalled = true
				return nil
			}

			result, err := RestartQQMiniapp(request)
			if err == nil || !strings.Contains(err.Error(), "old_tree_cleanup_failed") {
				t.Fatalf("first closed observation = %#v, err = %v", result, err)
			}
			if launchCalled || result.LaunchDispatched || result.Stopped {
				t.Fatalf("first closed observation result = %#v, launchCalled = %v", result, launchCalled)
			}
		})
	}
}

func filterCalls(calls []string, prefix string) []string {
	result := []string{}
	for _, call := range calls {
		if strings.HasPrefix(call, prefix) {
			result = append(result, call)
		}
	}
	return result
}

func TestRestartQQMiniappLaunchOnlyWhenDisconnected(t *testing.T) {
	registry, request, calls := newQQRestartFixture(t)
	request.Snapshots = qqMainOnlySnapshots()
	request.RuntimeConnected = false
	request.CloseWindows = func([]HostWindowSnapshot) error {
		t.Fatal("unexpected close")
		return nil
	}
	request.StopPID = func(int) error {
		t.Fatal("unexpected stop")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err != nil || result.Status != RestartStatusReconnected {
		t.Fatalf("launch-only restart = %#v, err = %v", result, err)
	}
	if result.OldPID != 0 || result.Stopped || result.Binding == nil || result.Binding.PID != 30 {
		t.Fatalf("launch-only result = %#v", result)
	}
	want := []string{"launch", "ready:qq-1:45s", "refresh"}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("calls = %#v, want %#v", *calls, want)
	}
	if binding, ok := registry.Binding("main"); !ok || binding.PID != 30 {
		t.Fatalf("replacement registry binding = %#v, ok = %v", binding, ok)
	}
}

func TestRestartQQMiniappRejectsConnectedRuntimeWithoutRoot(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	request.Snapshots = qqMainOnlySnapshots()
	request.RuntimeConnected = true
	request.CloseWindows = func([]HostWindowSnapshot) error {
		t.Fatal("unexpected close")
		return nil
	}
	request.StopPID = func(int) error {
		t.Fatal("unexpected stop")
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch")
		return nil
	}
	request.WaitForReady = func(string, time.Duration) QQReadyObservation {
		t.Fatal("unexpected ready wait")
		return QQReadyObservation{}
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "qq_miniapp_unverified") {
		t.Fatalf("connected rootless restart = %#v, err = %v", result, err)
	}
	if result.Status != "restart_failed" || result.LaunchDispatched {
		t.Fatalf("connected rootless result = %#v", result)
	}
}

func TestRestartQQMiniappRejectsAmbiguousRoots(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	request.Snapshots = append(request.Snapshots,
		HostProcessSnapshot{PID: 40, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []HostWindowSnapshot{{PID: 40, HWND: 400, Title: "QQ经典农场", Visible: true}}},
	)
	request.CloseWindows = func([]HostWindowSnapshot) error {
		t.Fatal("unexpected close")
		return nil
	}
	request.StopPID = func(int) error {
		t.Fatal("unexpected stop")
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "qq_miniapp_unverified") {
		t.Fatalf("ambiguous restart = %#v, err = %v", result, err)
	}
	if result.Status != "restart_failed" || len(result.Candidates) != 2 {
		t.Fatalf("ambiguous result = %#v", result)
	}
}

func TestRestartQQMiniappCloseFailureDoesNotStopOrLaunch(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	sentinel := errors.New("close refused")
	request.CloseWindows = func([]HostWindowSnapshot) error { return sentinel }
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		t.Fatal("unexpected close wait")
		return QQCloseObservation{}, nil
	}
	request.StopPID = func(int) error {
		t.Fatal("unexpected stop")
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "close_failed") {
		t.Fatalf("close failure = %#v, err = %v", result, err)
	}
	if result.Status != "restart_failed" || result.Stopped || result.LaunchDispatched {
		t.Fatalf("close failure result = %#v", result)
	}
}

func TestRestartQQMiniappRequiresDisconnectBeforeLaunch(t *testing.T) {
	registry, request, _ := newQQRestartFixture(t)
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		return QQCloseObservation{TreeExited: true, Disconnected: false, Snapshots: qqMainOnlySnapshots()}, nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "disconnect_timeout") {
		t.Fatalf("disconnect failure = %#v, err = %v", result, err)
	}
	if result.Status != "restart_failed" || result.LaunchDispatched {
		t.Fatalf("disconnect failure result = %#v", result)
	}
	if binding, ok := registry.Binding("main"); ok {
		t.Fatalf("stale binding retained after tree exit: %#v", binding)
	}
}

func TestRestartQQMiniappCleanupStillRunningDoesNotLaunch(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	running := append([]HostProcessSnapshot(nil), request.Snapshots...)
	waits := 0
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		waits++
		return QQCloseObservation{TreeExited: false, Disconnected: true, Snapshots: running}, nil
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		return append([]HostProcessSnapshot(nil), running...), nil
	}
	request.StopPID = func(pid int) error {
		for i := range running {
			if running[i].PID == pid {
				running = append(running[:i], running[i+1:]...)
				break
			}
		}
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "old_tree_exit_timeout") {
		t.Fatalf("tree timeout = %#v, err = %v", result, err)
	}
	if waits != 2 || result.Status != "restart_failed" || !result.Stopped || result.LaunchDispatched {
		t.Fatalf("tree timeout result = %#v", result)
	}
}

func TestRestartQQMiniappCleanupStillConnectedDoesNotLaunch(t *testing.T) {
	registry, request, _ := newQQRestartFixture(t)
	running := append([]HostProcessSnapshot(nil), request.Snapshots...)
	waits := 0
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		waits++
		if waits == 1 {
			return QQCloseObservation{TreeExited: false, Disconnected: false, Snapshots: running}, nil
		}
		return QQCloseObservation{TreeExited: true, Disconnected: false, Snapshots: qqMainOnlySnapshots()}, nil
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		return append([]HostProcessSnapshot(nil), running...), nil
	}
	request.StopPID = func(pid int) error {
		for i := range running {
			if running[i].PID == pid {
				running = append(running[:i], running[i+1:]...)
				break
			}
		}
		return nil
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "disconnect_timeout") {
		t.Fatalf("disconnect timeout = %#v, err = %v", result, err)
	}
	if waits != 2 || result.Status != "restart_failed" || !result.Stopped || result.LaunchDispatched {
		t.Fatalf("disconnect timeout result = %#v", result)
	}
	if binding, ok := registry.Binding("main"); ok {
		t.Fatalf("stale binding retained after tree exit: %#v", binding)
	}
}

func TestRestartQQMiniappReadyTimeoutIsFinalFailure(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	request.WaitForReady = func(string, time.Duration) QQReadyObservation {
		return QQReadyObservation{}
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		t.Fatal("unexpected snapshot refresh")
		return nil, nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "reconnect_timeout") {
		t.Fatalf("ready timeout = %#v, err = %v", result, err)
	}
	if result.Status != "restart_failed" || !result.LaunchDispatched {
		t.Fatalf("ready timeout result = %#v", result)
	}
}

func TestRestartQQMiniappRequiresAcceptedQQWSReadyObservation(t *testing.T) {
	for name, observation := range map[string]QQReadyObservation{
		"wait_not_accepted": {Accepted: false, Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-2"},
		"wrong_target":      {Accepted: true, Target: "wechat_cdp", Connected: true, Ready: true, InstanceID: "qq-2"},
	} {
		name, observation := name, observation
		t.Run(name, func(t *testing.T) {
			_, request, _ := newQQRestartFixture(t)
			request.WaitForReady = func(string, time.Duration) QQReadyObservation { return observation }
			request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
				t.Fatal("unexpected snapshot refresh")
				return nil, nil
			}

			result, err := RestartQQMiniapp(request)
			if err == nil || !strings.Contains(err.Error(), "reconnect_timeout") {
				t.Fatalf("ready observation = %#v, result = %#v, err = %v", observation, result, err)
			}
			if result.Status != "restart_failed" || !result.LaunchDispatched {
				t.Fatalf("ready observation result = %#v", result)
			}
		})
	}
}

func TestRestartQQMiniappRejectsPartialReadyState(t *testing.T) {
	for name, observation := range map[string]QQReadyObservation{
		"connected_not_ready": {Accepted: true, Target: "qq_ws", Connected: true, Ready: false, InstanceID: "qq-2"},
		"ready_not_connected": {Accepted: true, Target: "qq_ws", Connected: false, Ready: true, InstanceID: "qq-2"},
	} {
		name, observation := name, observation
		t.Run(name, func(t *testing.T) {
			_, request, _ := newQQRestartFixture(t)
			request.WaitForReady = func(string, time.Duration) QQReadyObservation { return observation }
			request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
				t.Fatal("unexpected snapshot refresh")
				return nil, nil
			}

			result, err := RestartQQMiniapp(request)
			if err == nil || !strings.Contains(err.Error(), "reconnect_timeout") {
				t.Fatalf("partial ready = %#v, result = %#v, err = %v", observation, result, err)
			}
			if result.Status != "restart_failed" || !result.LaunchDispatched {
				t.Fatalf("partial ready result = %#v", result)
			}
		})
	}
}

func TestRestartQQMiniappLaunchFailureDoesNotWaitForReady(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	sentinel := errors.New("protocol launch failed")
	request.Launch = func(LaunchRequest) error { return sentinel }
	request.WaitForReady = func(string, time.Duration) QQReadyObservation {
		t.Fatal("unexpected ready wait")
		return QQReadyObservation{}
	}
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		t.Fatal("unexpected snapshot refresh")
		return nil, nil
	}

	result, err := RestartQQMiniapp(request)
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "launch_failed") {
		t.Fatalf("launch failure = %#v, err = %v", result, err)
	}
	if result.Status != "restart_failed" || result.LaunchDispatched {
		t.Fatalf("launch failure result = %#v", result)
	}
}

func TestRestartQQMiniappRejectsOldOrEmptyInstanceID(t *testing.T) {
	for _, instanceID := range []string{"qq-1", ""} {
		instanceID := instanceID
		t.Run(fmt.Sprintf("instance_%q", instanceID), func(t *testing.T) {
			_, request, _ := newQQRestartFixture(t)
			request.WaitForReady = func(string, time.Duration) QQReadyObservation {
				return QQReadyObservation{Accepted: true, Target: "qq_ws", Connected: true, Ready: true, InstanceID: instanceID}
			}
			request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
				t.Fatal("unexpected snapshot refresh")
				return nil, nil
			}

			result, err := RestartQQMiniapp(request)
			if err == nil || !strings.Contains(err.Error(), "new_instance_not_observed") {
				t.Fatalf("instance rejection = %#v, err = %v", result, err)
			}
			if result.Status != "restart_failed" || !result.LaunchDispatched {
				t.Fatalf("instance rejection result = %#v", result)
			}
		})
	}
}

func TestRestartQQMiniappRejectsMissingOrAmbiguousReplacement(t *testing.T) {
	ambiguous := append(qqSnapshots(30, 300), HostProcessSnapshot{
		PID: 40, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`,
		Windows: []HostWindowSnapshot{{PID: 40, HWND: 400, Title: "QQ经典农场", Visible: true}},
	})
	for name, refreshed := range map[string][]HostProcessSnapshot{
		"missing":   qqMainOnlySnapshots(),
		"ambiguous": ambiguous,
	} {
		name, refreshed := name, refreshed
		t.Run(name, func(t *testing.T) {
			registry, request, _ := newQQRestartFixture(t)
			request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) { return refreshed, nil }

			result, err := RestartQQMiniapp(request)
			if err == nil || !strings.Contains(err.Error(), "new_process_not_found") {
				t.Fatalf("replacement rejection = %#v, err = %v", result, err)
			}
			if result.Status != "restart_failed" || !result.LaunchDispatched {
				t.Fatalf("replacement rejection result = %#v", result)
			}
			if binding, ok := registry.Binding("main"); ok {
				t.Fatalf("unexpected replacement binding = %#v", binding)
			}
		})
	}
}

func TestRestartQQMiniappRejectsReplacementSnapshotContainingOldAndNewRoots(t *testing.T) {
	registry, request, _ := newQQRestartFixture(t)
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		return append(qqSnapshots(20, 200), HostProcessSnapshot{
			PID: 30, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`,
			Windows: []HostWindowSnapshot{{PID: 30, HWND: 300, Title: "QQ经典农场", Visible: true}},
		}), nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "new_process_not_found") {
		t.Fatalf("old and new replacement = %#v, err = %v", result, err)
	}
	if result.Status != "restart_failed" || !result.LaunchDispatched {
		t.Fatalf("old and new replacement result = %#v", result)
	}
	if binding, ok := registry.Binding("main"); ok {
		t.Fatalf("unexpected replacement binding = %#v", binding)
	}
}

func TestRestartQQMiniappRejectsReplacementContainingOnlyOldRoot(t *testing.T) {
	registry, request, _ := newQQRestartFixture(t)
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) { return qqSnapshots(20, 200), nil }

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "new_process_not_found") {
		t.Fatalf("old-only replacement = %#v, err = %v", result, err)
	}
	if binding, ok := registry.Binding("main"); ok {
		t.Fatalf("unexpected old-root binding = %#v", binding)
	}
}

func TestRestartQQMiniappRejectsReplacementWithWrongParent(t *testing.T) {
	registry, request, _ := newQQRestartFixture(t)
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		return []HostProcessSnapshot{
			{PID: 11, ParentPID: 1, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`},
			{PID: 30, ParentPID: 11, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []HostWindowSnapshot{{PID: 30, HWND: 300, Title: "QQ经典农场", Visible: true}}},
		}, nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "new_process_not_found") {
		t.Fatalf("wrong-parent replacement = %#v, err = %v", result, err)
	}
	if binding, ok := registry.Binding("main"); ok {
		t.Fatalf("unexpected wrong-parent binding = %#v", binding)
	}
}

func TestRestartQQMiniappReportsBindingRefreshFailure(t *testing.T) {
	registry, request, _ := newQQRestartFixture(t)
	replacement := QQMiniappCandidates(qqSnapshots(30, 300))[0]
	other, err := registry.Bind("other", replacement)
	if err != nil {
		t.Fatalf("pre-bind replacement: %v", err)
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "binding_refresh_failed") {
		t.Fatalf("binding refresh failure = %#v, err = %v", result, err)
	}
	if result.Status != "restart_failed" || !result.LaunchDispatched {
		t.Fatalf("binding refresh failure result = %#v", result)
	}
	if binding, ok := registry.Binding("other"); !ok || binding.PID != other.PID || binding.HWNDs[0] != other.HWNDs[0] {
		t.Fatalf("other owner binding changed: %#v, ok = %v", binding, ok)
	}
	if binding, ok := registry.Binding("main"); ok {
		t.Fatalf("unexpected main replacement binding = %#v", binding)
	}
}

func TestRestartQQMiniappRejectsInitiallyOccupiedRoot(t *testing.T) {
	registry, request, _ := newQQRestartFixture(t)
	current := QQMiniappCandidates(request.Snapshots)[0]
	if _, err := registry.Bind("other", current); err != nil {
		t.Fatalf("pre-bind current root: %v", err)
	}
	request.CloseWindows = func([]HostWindowSnapshot) error {
		t.Fatal("unexpected close")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "binding_refresh_failed") {
		t.Fatalf("initial binding failure = %#v, err = %v", result, err)
	}
	if result.Status != "restart_failed" || result.OldPID != 0 {
		t.Fatalf("initial binding failure result = %#v", result)
	}
}

func TestRestartQQMiniappCloseObservationFailureDoesNotLaunch(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	sentinel := errors.New("snapshot observation failed")
	request.WaitForClosed = func(int, time.Duration) (QQCloseObservation, error) {
		return QQCloseObservation{}, sentinel
	}
	request.Launch = func(LaunchRequest) error {
		t.Fatal("unexpected launch")
		return nil
	}

	result, err := RestartQQMiniapp(request)
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "disconnect_timeout") {
		t.Fatalf("close observation failure = %#v, err = %v", result, err)
	}
	if result.Status != "restart_failed" || result.LaunchDispatched {
		t.Fatalf("close observation failure result = %#v", result)
	}
}

func TestRestartQQMiniappSnapshotRefreshFailureIsFinal(t *testing.T) {
	_, request, _ := newQQRestartFixture(t)
	sentinel := errors.New("replacement snapshot failed")
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) { return nil, sentinel }

	result, err := RestartQQMiniapp(request)
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "new_process_not_found") {
		t.Fatalf("snapshot refresh failure = %#v, err = %v", result, err)
	}
	if result.Status != "restart_failed" || !result.LaunchDispatched {
		t.Fatalf("snapshot refresh failure result = %#v", result)
	}
}

func TestRestartQQMiniappRejectsMissingCallbacksWithoutPanicking(t *testing.T) {
	tests := []struct {
		name  string
		stage string
		clear func(*QQRestartRequest)
	}{
		{name: "close", stage: "close_failed", clear: func(request *QQRestartRequest) { request.CloseWindows = nil }},
		{name: "closed", stage: "disconnect_timeout", clear: func(request *QQRestartRequest) { request.WaitForClosed = nil }},
		{name: "launch", stage: "launch_failed", clear: func(request *QQRestartRequest) { request.Launch = nil }},
		{name: "ready", stage: "reconnect_timeout", clear: func(request *QQRestartRequest) { request.WaitForReady = nil }},
		{name: "refresh", stage: "new_process_not_found", clear: func(request *QQRestartRequest) { request.RefreshSnapshots = nil }},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			_, request, _ := newQQRestartFixture(t)
			testCase.clear(&request)
			result, err, panicked := restartQQWithoutPanic(request)
			if panicked != nil {
				t.Fatalf("RestartQQMiniapp panicked: %v", panicked)
			}
			if err == nil || !strings.Contains(err.Error(), testCase.stage) {
				t.Fatalf("missing %s callback = %#v, err = %v", testCase.name, result, err)
			}
			if result.Status != "restart_failed" {
				t.Fatalf("missing %s callback result = %#v", testCase.name, result)
			}
		})
	}
}

func restartQQWithoutPanic(request QQRestartRequest) (result RestartResult, err error, panicked any) {
	defer func() { panicked = recover() }()
	result, err = RestartQQMiniapp(request)
	return result, err, nil
}

func TestDefaultQQMiniappCloseTimeout(t *testing.T) {
	if DefaultQQMiniappCloseTimeout != 5*time.Second {
		t.Fatalf("default close timeout = %s, want 5s", DefaultQQMiniappCloseTimeout)
	}
}

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

func TestQQMiniappCandidatesRequireNonzeroWindowHandle(t *testing.T) {
	snapshots := []HostProcessSnapshot{
		{PID: 10, ParentPID: 1, ProcessName: "QQ.exe"},
		{PID: 20, ParentPID: 10, ProcessName: "QQ.exe", Windows: []HostWindowSnapshot{{PID: 20, Title: "QQ经典农场", Visible: true}}},
	}
	if candidates := QQMiniappCandidates(snapshots); len(candidates) != 0 {
		t.Fatalf("strict QQ candidates = %#v, want none", candidates)
	}
}

func TestQQMiniappCandidatesRequireTitleAndNonzeroHandleOnSameWindow(t *testing.T) {
	snapshots := []HostProcessSnapshot{
		{PID: 10, ParentPID: 1, ProcessName: "QQ.exe"},
		{PID: 20, ParentPID: 10, ProcessName: "QQ.exe", Windows: []HostWindowSnapshot{
			{PID: 20, Title: "QQ经典农场", Visible: true},
			{PID: 20, HWND: 200, Title: "QQ", Visible: true},
		}},
	}
	if candidates := QQMiniappCandidates(snapshots); len(candidates) != 0 {
		t.Fatalf("strict QQ candidates = %#v, want none", candidates)
	}
}

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

func TestQQMiniappTreeExitedIgnoresUnrelatedParentCycle(t *testing.T) {
	snapshots := []HostProcessSnapshot{
		{PID: 21, ParentPID: 22, ProcessName: "QQ.exe"},
		{PID: 22, ParentPID: 21, ProcessName: "QQ.exe"},
	}
	if !QQMiniappTreeExited(20, snapshots) {
		t.Fatal("tree did not report exited for an unrelated parent cycle")
	}
}

func TestQQMiniappTreeExitedIgnoresBrokenParentChain(t *testing.T) {
	snapshots := []HostProcessSnapshot{
		{PID: 21, ParentPID: 99, ProcessName: "QQ.exe"},
	}
	if !QQMiniappTreeExited(20, snapshots) {
		t.Fatal("tree did not report exited for a broken parent chain")
	}
}
