package guard

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRestartWeChatMiniappPreservesHostAndWaitsForReady(t *testing.T) {
	registry, request, calls := newWeChatRestartFixture(t)

	result, err := RestartWeChatMiniapp(request)
	if err != nil {
		t.Fatalf("restart WeChat miniapp: %v", err)
	}
	if result.Status != RestartStatusReconnected || result.Stopped || !result.LaunchDispatched {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.OldPID != 42 || result.Binding == nil || result.Binding.PID != 42 || !reflect.DeepEqual(result.Binding.HWNDs, []uint64{421}) {
		t.Fatalf("binding was not refreshed on the same host: %#v", result)
	}
	if got, want := *calls, []string{"close:420", "wait:disconnected", "launch", "wait:ready", "refresh"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
	binding, ok := registry.Binding("main")
	if !ok || binding.PID != 42 || !reflect.DeepEqual(binding.HWNDs, []uint64{421}) {
		t.Fatalf("registry binding was not refreshed: %#v, ok=%v", binding, ok)
	}
}

func TestRestartWeChatMiniappCloseFailureDoesNotLaunch(t *testing.T) {
	_, request, calls := newWeChatRestartFixture(t)
	wantErr := errors.New("close failed")
	request.CloseWindows = func([]HostWindowSnapshot) error {
		*calls = append(*calls, "close")
		return wantErr
	}

	result, err := RestartWeChatMiniapp(request)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if result.LaunchDispatched || !reflect.DeepEqual(*calls, []string{"close"}) {
		t.Fatalf("unexpected result/calls: %#v %#v", result, *calls)
	}
}

func TestRestartWeChatMiniappDisconnectTimeoutDoesNotLaunch(t *testing.T) {
	_, request, calls := newWeChatRestartFixture(t)
	request.WaitForDisconnected = func(timeout time.Duration) bool {
		*calls = append(*calls, "wait:disconnected")
		return false
	}

	result, err := RestartWeChatMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "disconnect") {
		t.Fatalf("error = %v, want disconnect timeout", err)
	}
	if result.LaunchDispatched || !reflect.DeepEqual(*calls, []string{"close:420", "wait:disconnected"}) {
		t.Fatalf("unexpected result/calls: %#v %#v", result, *calls)
	}
}

func TestRestartWeChatMiniappLaunchFailureDoesNotWaitForReady(t *testing.T) {
	_, request, calls := newWeChatRestartFixture(t)
	wantErr := errors.New("launch failed")
	request.Launch = func(LaunchRequest) error {
		*calls = append(*calls, "launch")
		return wantErr
	}

	result, err := RestartWeChatMiniapp(request)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if result.LaunchDispatched || !reflect.DeepEqual(*calls, []string{"close:420", "wait:disconnected", "launch"}) {
		t.Fatalf("unexpected result/calls: %#v %#v", result, *calls)
	}
}

func TestRestartWeChatMiniappReadyTimeoutIsFinalFailure(t *testing.T) {
	_, request, calls := newWeChatRestartFixture(t)
	request.WaitForReady = func(timeout time.Duration) bool {
		*calls = append(*calls, "wait:ready")
		return false
	}

	result, err := RestartWeChatMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "readiness") {
		t.Fatalf("error = %v, want readiness timeout", err)
	}
	if !result.LaunchDispatched || !reflect.DeepEqual(*calls, []string{"close:420", "wait:disconnected", "launch", "wait:ready"}) {
		t.Fatalf("unexpected result/calls: %#v %#v", result, *calls)
	}
}

func TestRestartWeChatMiniappRejectsReplacementHostPID(t *testing.T) {
	registry, request, calls := newWeChatRestartFixture(t)
	request.RefreshSnapshots = func() ([]HostProcessSnapshot, error) {
		*calls = append(*calls, "refresh")
		return []HostProcessSnapshot{wechatHostSnapshot(99, 990)}, nil
	}

	_, err := RestartWeChatMiniapp(request)
	if err == nil || !strings.Contains(err.Error(), "PID disappeared") {
		t.Fatalf("error = %v, want preserved PID failure", err)
	}
	binding, ok := registry.Binding("main")
	if !ok || binding.PID != 42 || !reflect.DeepEqual(binding.HWNDs, []uint64{420}) {
		t.Fatalf("original binding changed: %#v, ok=%v", binding, ok)
	}
}

func newWeChatRestartFixture(t *testing.T) (*HostBindingRegistry, WeChatRestartRequest, *[]string) {
	t.Helper()
	initial := wechatHostSnapshot(42, 420)
	registry := NewHostBindingRegistry()
	if _, err := registry.Bind("main", candidateFromSnapshot(initial)); err != nil {
		t.Fatalf("bind initial host: %v", err)
	}
	calls := []string{}
	request := WeChatRestartRequest{
		Registry:  registry,
		Owner:     "main",
		Snapshots: []HostProcessSnapshot{initial},
		CloseWindows: func(windows []HostWindowSnapshot) error {
			if len(windows) != 1 {
				t.Fatalf("close windows = %#v", windows)
			}
			calls = append(calls, "close:"+strconv.Itoa(int(windows[0].HWND)))
			return nil
		},
		WaitForDisconnected: func(timeout time.Duration) bool {
			if timeout != 2*time.Second {
				t.Fatalf("close timeout = %s", timeout)
			}
			calls = append(calls, "wait:disconnected")
			return true
		},
		Launch: func(request LaunchRequest) error {
			if request.Mode != "protocol" || request.Protocol != wxFarmLaunchProtocol {
				t.Fatalf("launch request = %#v", request)
			}
			calls = append(calls, "launch")
			return nil
		},
		WaitForReady: func(timeout time.Duration) bool {
			if timeout != 45*time.Second {
				t.Fatalf("reconnect timeout = %s", timeout)
			}
			calls = append(calls, "wait:ready")
			return true
		},
		RefreshSnapshots: func() ([]HostProcessSnapshot, error) {
			calls = append(calls, "refresh")
			return []HostProcessSnapshot{wechatHostSnapshot(42, 421)}, nil
		},
		CloseTimeout:     2 * time.Second,
		ReconnectTimeout: 45 * time.Second,
	}
	return registry, request, &calls
}

func wechatHostSnapshot(pid int, hwnd uint64) HostProcessSnapshot {
	return HostProcessSnapshot{
		PID:         pid,
		ParentPID:   40756,
		ProcessName: "WeChatAppEx.exe",
		Windows: []HostWindowSnapshot{{
			HWND:    hwnd,
			PID:     pid,
			Title:   "QQ经典农场",
			Visible: true,
		}},
	}
}
