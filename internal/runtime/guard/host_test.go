package guard

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWeChatCandidatesExcludeMainWeChatProcess(t *testing.T) {
	candidates := CandidatesForPlatform("wechat_cdp", []HostProcessSnapshot{
		{PID: 10, ProcessName: "WeChat.exe", Windows: []HostWindowSnapshot{{PID: 10, Title: "微信", Visible: true}}},
		{PID: 11, ProcessName: "WeChatAppEx.exe", Windows: []HostWindowSnapshot{{PID: 11, Title: "QQ经典农场", Visible: true}}},
	})

	if len(candidates) != 1 || candidates[0].PID != 11 {
		t.Fatalf("expected only the miniapp process candidate, got %#v", candidates)
	}
}

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

func TestAutoBindFallsBackToSingleNonWeChatProcessCandidate(t *testing.T) {
	registry := NewHostBindingRegistry()
	candidates := []HostProcessCandidate{
		{PID: 20, ProcessName: "QQ.exe", Available: true},
	}

	result, err := registry.AutoBind("main", candidates)
	if err != nil {
		t.Fatalf("auto bind: %v", err)
	}
	if result.Status != "bound" || result.Binding == nil || result.Binding.PID != 20 {
		t.Fatalf("expected process-only QQ candidate to bind, got %#v", result)
	}
}

func TestAutoBindDoesNotFallbackToGenericWeChatAppEx(t *testing.T) {
	registry := NewHostBindingRegistry()
	candidates := []HostProcessCandidate{
		{PID: 20, ProcessName: "WeChatAppEx.exe", Available: true},
	}

	result, err := registry.AutoBind("main", candidates)
	if err != nil {
		t.Fatalf("auto bind: %v", err)
	}
	if result.Status != "not_found" || result.Binding != nil {
		t.Fatalf("expected generic WeChatAppEx to stay unbound, got %#v", result)
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
		Registry:  registry,
		Owner:     "main",
		Platform:  "qq",
		Snapshots: []HostProcessSnapshot{{PID: 20, ProcessName: "QQ.exe", Windows: []HostWindowSnapshot{{PID: 20, Title: "QQ经典农场", Visible: true}}}},
		StopPID:   func(pid int) error { stopped = append(stopped, pid); return nil },
		Launch:    func(LaunchRequest) error { return nil },
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

func TestLaunchRequestForPlatform(t *testing.T) {
	cases := map[string]string{
		"qq":         "protocol",
		"wechat_cdp": "protocol",
		"yyb_cdp":    "yyb_shortcut",
	}
	for platform, want := range cases {
		got := LaunchRequestForPlatform(platform)
		if got.Mode != want {
			t.Fatalf("%s mode = %s, want %s", platform, got.Mode, want)
		}
	}
}

func TestResolveLaunchRequestForPlatformFindsYYBLauncherFromRunningWMPFProcess(t *testing.T) {
	launcher := filepath.Clean(`E:\Apps\Tencent\Androws\Application\AndrowsLauncher.exe`)
	request, err := resolveLaunchRequestForPlatform(
		"yyb_cdp",
		[]HostProcessSnapshot{{
			ProcessName:    "WeChatAppEx.exe",
			ExecutablePath: `E:\Apps\Tencent\Androws\WmpfRuntime\5.10.2700.327\runtime\WeChatAppEx.exe`,
		}},
		func(string) string { return "" },
		func(path string) bool { return filepath.Clean(path) == launcher },
	)
	if err != nil {
		t.Fatal(err)
	}
	if request.TargetDisplayName != launcher {
		t.Fatalf("target = %q, want %q", request.TargetDisplayName, launcher)
	}
	if request.WorkingDirectory != filepath.Dir(launcher) {
		t.Fatalf("working directory = %q", request.WorkingDirectory)
	}
	if !strings.Contains(request.Parameters, "launchWithShortcut?pkgname=wx5306c5978fdb76e4") {
		t.Fatalf("parameters do not open QQ Classic Farm: %q", request.Parameters)
	}
}

func TestResolveLaunchRequestForPlatformFindsRootLevelYYBInstall(t *testing.T) {
	launcher := filepath.Clean(`D:\Androws\Application\AndrowsLauncher.exe`)
	request, err := resolveLaunchRequestForPlatform(
		"yyb",
		[]HostProcessSnapshot{{
			ProcessName:    "WeChatAppEx.exe",
			ExecutablePath: `D:\Androws\WmpfRuntime\5.10.2700.327\runtime\WeChatAppEx.exe`,
		}},
		func(string) string { return "" },
		func(path string) bool { return filepath.Clean(path) == launcher },
	)
	if err != nil || request.TargetDisplayName != launcher {
		t.Fatalf("request = %#v, err = %v", request, err)
	}
}

func TestResolveLaunchRequestForPlatformFallsBackToCommonYYBDirectory(t *testing.T) {
	launcher := filepath.Clean(`C:\CustomProgramFiles\Tencent\Androws\Application\AndrowsLauncher.exe`)
	request, err := resolveLaunchRequestForPlatform(
		"yyb",
		nil,
		func(name string) string {
			if name == "ProgramFiles" {
				return `C:\CustomProgramFiles`
			}
			return ""
		},
		func(path string) bool { return filepath.Clean(path) == launcher },
	)
	if err != nil || request.TargetDisplayName != launcher {
		t.Fatalf("request = %#v, err = %v", request, err)
	}
}

func TestResolveLaunchRequestForPlatformRejectsMissingYYBLauncher(t *testing.T) {
	request, err := resolveLaunchRequestForPlatform(
		"yyb",
		nil,
		func(string) string { return "" },
		func(string) bool { return false },
	)
	if err == nil || !strings.Contains(err.Error(), "未找到应用宝安装目录") {
		t.Fatalf("request = %#v, err = %v", request, err)
	}
}

func TestResolveLaunchRequestForPlatformKeepsProtocolLaunches(t *testing.T) {
	for _, target := range []string{"qq_ws", "wechat_cdp"} {
		request, err := resolveLaunchRequestForPlatform(
			target,
			nil,
			func(string) string { return "" },
			func(string) bool { return false },
		)
		if err != nil || request.Mode != "protocol" || request.Protocol == "" {
			t.Fatalf("%s request = %#v, err = %v", target, request, err)
		}
	}
}
