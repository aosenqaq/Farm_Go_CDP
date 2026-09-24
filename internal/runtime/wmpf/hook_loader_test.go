package wmpf

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	farmruntime "Farm_Go/internal/runtime"
)

func TestFridaHookLoaderAttachesMainWeChatProcess(t *testing.T) {
	finder := fakeProcessFinder{processes: []RuntimeProcess{
		{
			Name:        "WeChatAppEx.exe",
			PID:         101,
			ParentPID:   202,
			Path:        `C:\Users\tester\AppData\Roaming\Tencent\xwechat\xplugin\plugins\RadiumWMPF\20005\extracted\runtime\WeChatAppEx.exe`,
			CommandLine: `"WeChatAppEx.exe" --type=renderer`,
			Match:       ProcessMatch{Version: "20005"},
		},
		{
			Name:        "WeChatAppEx.exe",
			PID:         202,
			ParentPID:   88,
			Path:        `C:\Users\tester\AppData\Roaming\Tencent\xwechat\xplugin\plugins\RadiumWMPF\20005\extracted\runtime\WeChatAppEx.exe`,
			CommandLine: `"WeChatAppEx.exe" --enable-applet-v3`,
			Match:       ProcessMatch{Version: "20005"},
		},
		{
			Name:        "WeChatAppEx.exe",
			PID:         303,
			ParentPID:   202,
			Path:        `C:\Users\tester\AppData\Roaming\Tencent\xwechat\xplugin\plugins\RadiumWMPF\20005\extracted\runtime\WeChatAppEx.exe`,
			CommandLine: `"WeChatAppEx.exe" --type=utility --utility-sub-type=flue.mojom.ILinkServiceHost`,
			Match:       ProcessMatch{Version: "20005"},
		},
	}}
	runner := &fakeFridaHookRunner{}
	loader := NewFridaHookLoader(FridaHookLoaderOptions{
		Profile: ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		Root:    testFridaRoot(),
		Finder:  finder,
		Runner:  runner,
	})

	if err := loader.Start(context.Background()); err != nil {
		t.Fatalf("start hook loader: %v", err)
	}

	if len(runner.pids) != 1 || runner.pids[0] != 202 {
		t.Fatalf("expected the parent WMPF host process only, got %#v", runner.pids)
	}
	if runner.hooks[0].Version != "20005" || runner.hooks[0].Target != farmruntime.RuntimeTargetWeChatCDP {
		t.Fatalf("unexpected hook options %#v", runner.hooks[0])
	}
}

func TestFridaHookLoaderUsesChildVersionWhenAttachingWeChatParent(t *testing.T) {
	finder := fakeProcessFinder{processes: []RuntimeProcess{
		{
			Name:        "WeChatAppEx.exe",
			PID:         900,
			ParentPID:   77,
			Path:        `C:\Windows\System32\WeChatAppEx.exe`,
			CommandLine: `"WeChatAppEx.exe" --enable-applet-v3`,
		},
		{
			Name:        "WeChatAppEx.exe",
			PID:         901,
			ParentPID:   900,
			Path:        `C:\Users\tester\AppData\Roaming\Tencent\xwechat\xplugin\plugins\RadiumWMPF\20005\extracted\runtime\WeChatAppEx.exe`,
			CommandLine: `"WeChatAppEx.exe" --type=utility --utility-sub-type=flue.mojom.ILinkServiceHost`,
			Match:       ProcessMatch{Version: "20005"},
		},
	}}
	runner := &fakeFridaHookRunner{}
	loader := NewFridaHookLoader(FridaHookLoaderOptions{
		Profile: ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		Root:    testFridaRoot(),
		Finder:  finder,
		Runner:  runner,
	})

	if err := loader.Start(context.Background()); err != nil {
		t.Fatalf("start hook loader: %v", err)
	}

	if len(runner.pids) != 1 || runner.pids[0] != 900 {
		t.Fatalf("expected parent pid 900, got %#v", runner.pids)
	}
	if runner.hooks[0].Version != "20005" {
		t.Fatalf("expected version from child process, got %#v", runner.hooks[0])
	}
}

func TestFridaHookLoaderPassesDebugWebSocketURL(t *testing.T) {
	finder := fakeProcessFinder{processes: []RuntimeProcess{
		{
			Name:      "WeChatAppEx.exe",
			PID:       202,
			ParentPID: 88,
			Path:      `C:\Users\tester\AppData\Roaming\Tencent\xwechat\xplugin\plugins\RadiumWMPF\20005\extracted\runtime\WeChatAppEx.exe`,
			Match:     ProcessMatch{Version: "20005"},
		},
		{
			Name:      "WeChatAppEx.exe",
			PID:       303,
			ParentPID: 202,
			Path:      `C:\Users\tester\AppData\Roaming\Tencent\xwechat\xplugin\plugins\RadiumWMPF\20005\extracted\runtime\WeChatAppEx.exe`,
			Match:     ProcessMatch{Version: "20005"},
		},
	}}
	runner := &fakeFridaHookRunner{}
	loader := NewFridaHookLoader(FridaHookLoaderOptions{
		Profile:           ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		Root:              testFridaRoot(),
		DebugWebSocketURL: "ws://127.0.0.1:9420/",
		Finder:            finder,
		Runner:            runner,
	})

	if err := loader.Start(context.Background()); err != nil {
		t.Fatalf("start hook loader: %v", err)
	}

	if len(runner.hooks) != 1 || runner.hooks[0].DebugWebSocketURL != "ws://127.0.0.1:9420/" {
		t.Fatalf("expected debug websocket url to reach hook options, got %#v", runner.hooks)
	}
}

func TestFridaHookLoaderSelectsYYBParentProcessWhenPresent(t *testing.T) {
	yybPath := `D:\Androws\WmpfRuntime\5.10.2700.327\runtime\WeChatAppEx.exe`
	finder := fakeProcessFinder{processes: []RuntimeProcess{
		{Name: "WeChatAppEx.exe", PID: 500, ParentPID: 10, Path: yybPath, CommandLine: `"WeChatAppEx.exe"`, Match: ProcessMatch{Version: "5.10.2700.327"}},
		{Name: "WeChatAppEx.exe", PID: 501, ParentPID: 500, Path: yybPath, CommandLine: `"WeChatAppEx.exe" --type=renderer`, Match: ProcessMatch{Version: "5.10.2700.327"}},
		{Name: "WeChatAppEx.exe", PID: 502, ParentPID: 500, Path: yybPath, CommandLine: `"WeChatAppEx.exe" --type=utility`, Match: ProcessMatch{Version: "5.10.2700.327"}},
	}}
	runner := &fakeFridaHookRunner{}
	loader := NewFridaHookLoader(FridaHookLoaderOptions{
		Profile: ProfileForTarget(farmruntime.RuntimeTargetYYBCDP),
		Root:    testFridaRoot(),
		Finder:  finder,
		Runner:  runner,
	})

	if err := loader.Start(context.Background()); err != nil {
		t.Fatalf("start hook loader: %v", err)
	}

	if len(runner.pids) != 1 || runner.pids[0] != 500 {
		t.Fatalf("expected yyb parent process, got %#v", runner.pids)
	}
	if runner.hooks[0].Version != "5.10.2700.327" || runner.hooks[0].Target != farmruntime.RuntimeTargetYYBCDP {
		t.Fatalf("unexpected yyb hook options %#v", runner.hooks[0])
	}
}

func TestFridaHookLoaderFallsBackToYYBProcessWithMostChildren(t *testing.T) {
	yybPath := `D:\Androws\WmpfRuntime\5.10.2700.327\runtime\WeChatAppEx.exe`
	finder := fakeProcessFinder{processes: []RuntimeProcess{
		{Name: "WeChatAppEx.exe", PID: 600, ParentPID: 900, Path: yybPath, CommandLine: `"WeChatAppEx.exe" --type=renderer`, Match: ProcessMatch{Version: "5.10.2700.327"}},
		{Name: "WeChatAppEx.exe", PID: 601, ParentPID: 600, Path: yybPath, CommandLine: `"WeChatAppEx.exe" --type=renderer`, Match: ProcessMatch{Version: "5.10.2700.327"}},
		{Name: "WeChatAppEx.exe", PID: 602, ParentPID: 600, Path: yybPath, CommandLine: `"WeChatAppEx.exe" --type=utility`, Match: ProcessMatch{Version: "5.10.2700.327"}},
		{Name: "WeChatAppEx.exe", PID: 603, ParentPID: 777, Path: yybPath, CommandLine: `"WeChatAppEx.exe" --type=utility`, Match: ProcessMatch{Version: "5.10.2700.327"}},
		{Name: "WMPFRuntime.exe", PID: 777, ParentPID: 1, Path: `D:\Androws\WmpfRuntime\5.10.2700.327\runtime\WMPFRuntime.exe`, CommandLine: `"WMPFRuntime.exe"`, Match: ProcessMatch{Version: "5.10.2700.327"}},
	}}
	runner := &fakeFridaHookRunner{}
	loader := NewFridaHookLoader(FridaHookLoaderOptions{
		Profile: ProfileForTarget(farmruntime.RuntimeTargetYYBCDP),
		Root:    testFridaRoot(),
		Finder:  finder,
		Runner:  runner,
	})

	if err := loader.Start(context.Background()); err != nil {
		t.Fatalf("start hook loader: %v", err)
	}

	if len(runner.pids) != 1 || runner.pids[0] != 600 {
		t.Fatalf("expected yyb process with most children, got %#v", runner.pids)
	}
}

func TestFridaHookLoaderWaitsWhenNoProcessExists(t *testing.T) {
	runner := &fakeFridaHookRunner{}
	loader := NewFridaHookLoader(FridaHookLoaderOptions{
		Profile: ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		Root:    testFridaRoot(),
		Finder:  fakeProcessFinder{},
		Runner:  runner,
	})

	if err := loader.Start(context.Background()); err != nil {
		t.Fatalf("start hook loader: %v", err)
	}
	if len(runner.pids) != 0 {
		t.Fatalf("expected no attach without process, got %#v", runner.pids)
	}
}

func TestFridaHookLoaderUsesConfiguredPythonRuntime(t *testing.T) {
	loader := NewFridaHookLoader(FridaHookLoaderOptions{
		Profile: ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		Root:    testFridaRoot(),
		Python:  `C:\FarmGo\runtime\frida-python\python.exe`,
	})

	runner, ok := loader.runner.(PythonFridaHookRunner)
	if !ok {
		t.Fatalf("expected default Python Frida runner, got %T", loader.runner)
	}
	if runner.Python != `C:\FarmGo\runtime\frida-python\python.exe` {
		t.Fatalf("expected configured Python path, got %q", runner.Python)
	}
}

func TestFridaLogPathIncludesTargetPID(t *testing.T) {
	path := fridaLogPath(24804)

	if !strings.Contains(path, "farm_go_frida_24804.log") {
		t.Fatalf("unexpected log path %q", path)
	}
}

func TestPythonFridaHookCommandCarriesFarmGoParentPID(t *testing.T) {
	cmd := newPythonFridaCommand(context.Background(), "python", 24804, "hook-source")

	expected := fmt.Sprintf("FARM_GO_FRIDA_PARENT_PID=%d", os.Getpid())
	if !hasEnv(cmd.Env, expected) {
		t.Fatalf("expected helper env %q, got %#v", expected, cmd.Env)
	}
}

func TestPythonFridaHelperExitsWhenFarmGoParentExits(t *testing.T) {
	required := []string{
		"FARM_GO_FRIDA_PARENT_PID",
		"WaitForSingleObject",
		"FRIDA_PARENT_EXITED",
	}
	for _, item := range required {
		if !strings.Contains(pythonFridaHelper, item) {
			t.Fatalf("python helper must contain %q to avoid orphan frida sessions", item)
		}
	}
}

func hasEnv(env []string, expected string) bool {
	for _, item := range env {
		if item == expected {
			return true
		}
	}
	return false
}

type fakeProcessFinder struct {
	processes []RuntimeProcess
	err       error
}

func (f fakeProcessFinder) List(profile Profile) ([]RuntimeProcess, error) {
	return f.processes, f.err
}

type fakeFridaHookRunner struct {
	pids  []int
	hooks []FridaHookOptions
	err   error
}

func (f *fakeFridaHookRunner) Start(ctx context.Context, pid int, hook FridaHookOptions) error {
	f.pids = append(f.pids, pid)
	f.hooks = append(f.hooks, hook)
	return f.err
}
