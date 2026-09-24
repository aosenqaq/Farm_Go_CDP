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
	if err == nil || !strings.Contains(err.Error(), "PID disappeared") {
		t.Fatalf("error = %v, want preserved PID failure", err)
	}
	binding, ok := registry.Binding("main")
	if !ok || binding.PID != 42 || !reflect.DeepEqual(binding.HWNDs, []uint64{420}) {
		t.Fatalf("original binding changed: %#v, ok=%v", binding, ok)
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
		Registry:  registry,
		Owner:     "main",
		Snapshots: []HostProcessSnapshot{initial},
		CloseWindows: func(windows []HostWindowSnapshot) error {
			if len(windows) != 1 || windows[0].HWND != 420 {
				t.Fatalf("close windows = %#v", windows)
			}
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
		CloseTimeout:     2 * time.Second,
		ReconnectTimeout: 45 * time.Second,
	}
	return registry, request, &calls
}

func yybHostSnapshot(pid int, hwnd uint64) HostProcessSnapshot {
	return HostProcessSnapshot{
		PID:            pid,
		ParentPID:      7176,
		ProcessName:    "WeChatAppEx.exe",
		ExecutablePath: `E:\Program Files\Tencent\Androws\WmpfRuntime\5.10.2700.327\runtime\WeChatAppEx.exe`,
		Windows: []HostWindowSnapshot{{
			HWND:    hwnd,
			PID:     pid,
			Title:   "QQ经典农场",
			Visible: true,
		}},
	}
}
