package wmpf

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	farmruntime "Farm_Go/internal/runtime"
)

func TestValidateFridaResources(t *testing.T) {
	root := testFridaRoot()

	if err := ValidateFridaResources(root); err != nil {
		t.Fatalf("validate frida resources: %v", err)
	}
}

func TestBuildFridaHookScriptInjectsConfigAndRuntimeTarget(t *testing.T) {
	script, err := BuildFridaHookScript(FridaHookOptions{
		Root:              testFridaRoot(),
		Target:            farmruntime.RuntimeTargetYYBCDP,
		Version:           "5.10.2700.327",
		DebugWebSocketURL: "ws://127.0.0.1:9420/",
	})
	if err != nil {
		t.Fatalf("build hook: %v", err)
	}

	if strings.Contains(script, "@@CONFIG@@") {
		t.Fatal("hook config placeholder was not replaced")
	}
	if !strings.Contains(script, `"RuntimeTarget":"yyb"`) {
		t.Fatalf("hook script did not include yyb runtime target")
	}
	if !strings.Contains(script, `"DebugWebSocketURL":"ws://127.0.0.1:9420/"`) {
		t.Fatalf("hook script did not include debug websocket url")
	}
	if !strings.Contains(script, `"CDPFilterHookOffset"`) {
		t.Fatalf("hook script did not include address config")
	}
}

func TestBuildFridaHookScriptSupportsWMPF20079(t *testing.T) {
	script, err := BuildFridaHookScript(FridaHookOptions{
		Root:    testFridaRoot(),
		Target:  farmruntime.RuntimeTargetWeChatCDP,
		Version: "20079",
	})
	if err != nil {
		t.Fatalf("build 20079 hook: %v", err)
	}
	for _, item := range []string{
		`"Version":20079`,
		`"LoadStartHookOffset":"0x25DFB90"`,
		`"CDPFilterHookOffset":"0x2D954D0"`,
		`"SceneOffsets":[64,1480,8,1416,16,456]`,
		`"RuntimeTarget":"cdp"`,
	} {
		if !strings.Contains(script, item) {
			t.Fatalf("20079 hook script missing %q", item)
		}
	}
}

func TestBuildFridaHookScriptSupportsWMPF20089(t *testing.T) {
	script, err := BuildFridaHookScript(FridaHookOptions{
		Root:    testFridaRoot(),
		Target:  farmruntime.RuntimeTargetWeChatCDP,
		Version: "20089",
	})
	if err != nil {
		t.Fatalf("build 20089 hook: %v", err)
	}
	for _, item := range []string{
		`"Version":20089`,
		`"LoadStartHookOffset":"0x25E0170"`,
		`"CDPFilterHookOffset":"0x2D95AB0"`,
		`"SceneOffsets":[64,1480,8,1416,16,456]`,
		`"RuntimeTarget":"cdp"`,
	} {
		if !strings.Contains(script, item) {
			t.Fatalf("20089 hook script missing %q", item)
		}
	}
}

func TestBuildFridaHookScriptSupportsWMPF25297(t *testing.T) {
	script, err := BuildFridaHookScript(FridaHookOptions{
		Root:    testFridaRoot(),
		Target:  farmruntime.RuntimeTargetWeChatCDP,
		Version: "25297",
	})
	if err != nil {
		t.Fatalf("build 25297 hook: %v", err)
	}
	for _, item := range []string{
		`"Version":25297`,
		`"LoadStartHookOffset":"0x2A5D800"`,
		`"CDPFilterHookOffset":"0x3716230"`,
		`"SceneOffsets":[64,640]`,
		`"RuntimeTarget":"cdp"`,
	} {
		if !strings.Contains(script, item) {
			t.Fatalf("25297 hook script missing %q", item)
		}
	}
	if !strings.Contains(script, "config.SceneOffsets.length === 2") {
		t.Fatal("25297 hook script does not preserve its direct two-offset scene path")
	}
}

func TestBuildFridaHookScriptReadsEmbeddedResources(t *testing.T) {
	resources := fstest.MapFS{
		"frida/hook.js": {
			Data: []byte("const config = @@CONFIG@@;"),
		},
		"frida/config/addresses.20005.json": {
			Data: []byte(`{"LoadStartHookOffset":"0x1","CDPFilterHookOffset":"0x2","SceneOffsets":[3]}`),
		},
	}

	script, err := BuildFridaHookScript(FridaHookOptions{
		Root:      "frida",
		Resources: resources,
		Target:    farmruntime.RuntimeTargetWeChatCDP,
		Version:   "20005",
	})
	if err != nil {
		t.Fatalf("build hook from embedded resources: %v", err)
	}
	if strings.Contains(script, "@@CONFIG@@") {
		t.Fatal("embedded hook config placeholder was not replaced")
	}
	if !strings.Contains(script, `"RuntimeTarget":"cdp"`) {
		t.Fatalf("embedded hook script did not include runtime target")
	}
}

func TestBuildFridaHookScriptPreservesYYBDebugScenes(t *testing.T) {
	script, err := BuildFridaHookScript(FridaHookOptions{
		Root:    testFridaRoot(),
		Target:  farmruntime.RuntimeTargetYYBCDP,
		Version: "5.10.2700.327",
	})
	if err != nil {
		t.Fatalf("build hook: %v", err)
	}

	if !strings.Contains(script, `"DebugScenes":[1168]`) {
		t.Fatalf("hook script did not preserve yyb debug scenes")
	}
}

func TestBuildFridaHookScriptKeepsBlockedWeChatScenesDisabled(t *testing.T) {
	script, err := BuildFridaHookScript(FridaHookOptions{
		Root:    testFridaRoot(),
		Target:  farmruntime.RuntimeTargetWeChatCDP,
		Version: "20005",
	})
	if err != nil {
		t.Fatalf("build hook: %v", err)
	}

	if strings.Contains(script, `"AllowBlockedScenes":true`) {
		t.Fatalf("hook script enabled scene 1000 even though the reference hook blocks it as crash-prone")
	}
}

func TestBuildFridaHookScriptPatchesConfiguredWebSocketEndpoint(t *testing.T) {
	script, err := BuildFridaHookScript(FridaHookOptions{
		Root:              testFridaRoot(),
		Target:            farmruntime.RuntimeTargetWeChatCDP,
		Version:           "20005",
		DebugWebSocketURL: "ws://127.0.0.1:9420/",
	})
	if err != nil {
		t.Fatalf("build hook: %v", err)
	}

	for _, want := range []string{
		"const patchDebugWebSocketURL =",
		"const resolveLoadStartPassArgs =",
		"config.DebugWebSocketURL",
		"writeUtf8String(url)",
		"hookOnLoadScene(this.context.rcx, config)",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("hook script did not include websocket pointer patch %q", want)
		}
	}
}

func TestBuildFridaHookScriptKeepsSafeEndpointHandling(t *testing.T) {
	script, err := BuildFridaHookScript(FridaHookOptions{
		Root:    testFridaRoot(),
		Target:  farmruntime.RuntimeTargetWeChatCDP,
		Version: "20005",
	})
	if err != nil {
		t.Fatalf("build hook: %v", err)
	}

	for _, want := range []string{
		"BLOCKED_PATCH_SCENE_NUMBERS.indexOf(scene) === -1",
		"sceneOffsets.length < 3",
		"passArgs.isNull()",
		"readPointerIfAccessible(passArgs.add(8))",
		"url.length > original.length",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("hook script did not include %q", want)
		}
	}
}

func TestBuildFridaHookScriptSafelySkipsUnreadableScenePointers(t *testing.T) {
	script, err := BuildFridaHookScript(FridaHookOptions{
		Root:    testFridaRoot(),
		Target:  farmruntime.RuntimeTargetWeChatCDP,
		Version: "25297",
	})
	if err != nil {
		t.Fatalf("build hook: %v", err)
	}

	for _, want := range []string{
		"const isMemoryRangeAccessible = (address, size, access) =>",
		"const readPointerIfAccessible = (address) =>",
		"const readIntIfAccessible = (address) =>",
		"const scene = readIntIfAccessible(miniappScenePtr);",
		"if (!isMemoryRangeAccessible(miniappScenePtr, 4, \"w\"))",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("hook script did not include scene pointer protection %q", want)
		}
	}
	if strings.Contains(script, "miniappScenePtr.readInt()") {
		t.Fatal("hook script still reads the scene pointer without checking its mapping")
	}
}

func TestFridaAttachLoadsBuiltHookScript(t *testing.T) {
	device := &fakeFridaDevice{}
	err := AttachAndLoadFridaHook(context.Background(), device, FridaAttachOptions{
		PID: 1234,
		Hook: FridaHookOptions{
			Root:    testFridaRoot(),
			Target:  farmruntime.RuntimeTargetYYBCDP,
			Version: "5.10.2700.327",
		},
	})
	if err != nil {
		t.Fatalf("attach and load: %v", err)
	}
	if device.attachedPID != 1234 {
		t.Fatalf("unexpected attached pid %d", device.attachedPID)
	}
	if !strings.Contains(device.scriptSource, `"RuntimeTarget":"yyb"`) {
		t.Fatalf("unexpected loaded script")
	}
	if !device.loaded {
		t.Fatal("script was not loaded")
	}
}

type fakeFridaDevice struct {
	attachedPID  int
	scriptSource string
	loaded       bool
}

func (f *fakeFridaDevice) Attach(ctx context.Context, pid int) (FridaSession, error) {
	f.attachedPID = pid
	return fakeFridaSession{device: f}, nil
}

type fakeFridaSession struct {
	device *fakeFridaDevice
}

func (f fakeFridaSession) CreateScript(ctx context.Context, source string) (FridaScript, error) {
	f.device.scriptSource = source
	return fakeFridaScript{device: f.device}, nil
}

type fakeFridaScript struct {
	device *fakeFridaDevice
}

func (f fakeFridaScript) Load(ctx context.Context) error {
	f.device.loaded = true
	return nil
}

func testFridaRoot() string {
	return filepath.Join("..", "..", "..", "resources", "wmpf", "frida")
}
