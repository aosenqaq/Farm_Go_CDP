# Runtime Links Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add switchable runtime links for QQ WS, WeChat CDP, and YingYongBao CDP, then port the reference project's WeChat WMPF/CDP connection path from Node to Go while keeping required Frida hook/config resources.

**Architecture:** Introduce a `RuntimeLink` interface and a supervisor that owns the active link. QQ WS becomes one link implementation; WeChat CDP and YYB CDP share a WMPF/CDP bridge with different process/config profiles. Settings persist the default startup link and allow immediate switching from the UI.

**Tech Stack:** Go, Wails v2, React/TypeScript, SQLite, Gorilla WebSocket, Frida Go bindings or a minimal Frida sidecar fallback if direct Go binding packaging blocks release.

---

## Scope

This stage builds connection infrastructure only. It must not migrate farm automation task logic yet. The acceptance target is: the app can switch links, persist the default link, keep QQ WS working, and connect to WeChat/YYB miniapp CDP far enough for diagnostics to call `host.describe` and `gameCtl.probe` through a unified runtime caller.

Reference project files to read during implementation:

- `E:\desktop\farm-tauri\core\src\config.js`
- `E:\desktop\farm-tauri\core\run.cjs`
- `E:\desktop\farm-tauri\core\apply-cli-overrides.cjs`
- `E:\desktop\farm-tauri\core\src\cdp-session.js`
- `E:\desktop\farm-tauri\core\src\cdp-wmpf-session.js`
- `E:\desktop\farm-tauri\core\wmpf\src\index.js`
- `E:\desktop\farm-tauri\core\wmpf\src\cdp_automation.js`
- `E:\desktop\farm-tauri\core\wmpf\frida\hook.js`
- `E:\desktop\farm-tauri\core\wmpf\frida\config\*.json`
- `E:\desktop\farm-tauri\core\wmpf\frida\config\yyb\*.json`

Do not modify `E:\desktop\farm-tauri`.

## File Structure

- Modify `internal/config/config.go`: add runtime target, CDP, WMPF, and startup settings.
- Modify `internal/storage/settings.go`: implement typed settings read/write on top of the existing `settings` table.
- Create `internal/runtime/link.go`: define `RuntimeLink`, `RuntimeTarget`, and shared status/event types.
- Create `internal/runtime/supervisor.go`: start/stop/switch active runtime links.
- Create `internal/runtime/qqlink/link.go`: wrap existing `qqws.Adapter` as a link.
- Create `internal/runtime/cdp/client.go`: CDP WebSocket command client and `Runtime.evaluate`.
- Create `internal/runtime/cdp/context.go`: context scoring, context selection, runtime readiness checks.
- Create `internal/runtime/wmpf/process.go`: WeChat/YYB WMPF process discovery and version extraction.
- Create `internal/runtime/wmpf/debug_server.go`: miniapp debug WebSocket server and CDP proxy server.
- Create `internal/runtime/wmpf/protocol.go`: WARemoteDebug protobuf wrapping/unwrapping equivalent.
- Create `internal/runtime/wmpf/frida.go`: attach and load Frida hook resources.
- Create `internal/runtime/wmpf/link.go`: shared WeChat/YYB CDP link.
- Copy only necessary resources into `resources/wmpf/`: `hook.js`, protobuf JS reference only if needed during porting, and address config JSON files.
- Modify `app.go`: initialize settings, supervisor, link switching APIs.
- Modify `internal/app/app.go`: expose connection info for the active runtime link.
- Modify `internal/diagnostics/service.go`: keep using the `RuntimeCaller` interface, but point it at the supervisor.
- Modify `frontend/src/views/SettingsView.tsx`: add link selector, default selector, and CDP/WMPF fields.
- Modify `frontend/src/views/ConnectionView.tsx`: show active link details instead of QQ-only text.
- Modify `frontend/src/views/DiagnosticsView.tsx`: remove QQ-specific copy and show active link context.
- Regenerate Wails bindings after backend API changes.

## Task 1: Runtime Target Config And Persistence

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/storage/settings.go`
- Test: `internal/config/config_test.go`
- Test: `internal/storage/storage_test.go`

- [ ] **Step 1: Write failing config tests**

Add tests that expect defaults:

```go
func TestDefaultRuntimeTargetIsQQWS(t *testing.T) {
	cfg := Default()
	if cfg.Runtime.DefaultTarget != "qq_ws" {
		t.Fatalf("expected qq_ws, got %q", cfg.Runtime.DefaultTarget)
	}
	if cfg.CDP.Port != 62000 {
		t.Fatalf("expected cdp port 62000, got %d", cfg.CDP.Port)
	}
	if cfg.WMPF.DebugPort != 9420 {
		t.Fatalf("expected wmpf debug port 9420, got %d", cfg.WMPF.DebugPort)
	}
}
```

- [ ] **Step 2: Run the failing tests**

Run: `go test ./internal/config`

Expected: fail because `Runtime`, `CDP`, and `WMPF` fields do not exist.

- [ ] **Step 3: Add config fields**

Implement:

```go
type RuntimeConfig struct {
	DefaultTarget string `json:"defaultTarget"`
	CurrentTarget string `json:"currentTarget"`
	AutoStart     bool   `json:"autoStart"`
}

type CDPConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	TimeoutMS int    `json:"timeoutMs"`
	ContextName string `json:"contextName"`
}

type WMPFConfig struct {
	DebugPort      int    `json:"debugPort"`
	LegacyDebugPort int  `json:"legacyDebugPort"`
	ConfigDir      string `json:"configDir"`
	YYBConfigDir   string `json:"yybConfigDir"`
}
```

Update `Config` and `Default()` with `qq_ws`, `127.0.0.1`, `62000`, `9420`, `9421`, `8000`, and `gameContext`.

- [ ] **Step 4: Add storage tests**

Create tests for saving and loading runtime settings:

```go
func TestStoreRuntimeSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	input := RuntimeSettings{
		DefaultTarget: "wechat_cdp",
		CurrentTarget: "yyb_cdp",
		AutoStart: true,
		CDPPort: 62000,
		WMPFDebugPort: 9420,
	}
	if err := store.SaveRuntimeSettings(ctx, input); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	got, err := store.LoadRuntimeSettings(ctx)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if got.DefaultTarget != input.DefaultTarget || got.CurrentTarget != input.CurrentTarget {
		t.Fatalf("settings mismatch: %#v", got)
	}
}
```

- [ ] **Step 5: Implement typed settings storage**

Add `RuntimeSettings`, `LoadRuntimeSettings`, and `SaveRuntimeSettings`. Store each field under deterministic keys such as `runtime.defaultTarget`, `runtime.currentTarget`, `runtime.autoStart`, `cdp.port`, and `wmpf.debugPort`.

- [ ] **Step 6: Verify**

Run: `go test ./internal/config ./internal/storage`

Expected: all tests pass.

## Task 2: RuntimeLink Interface And Supervisor

**Files:**
- Create: `internal/runtime/link.go`
- Create: `internal/runtime/supervisor.go`
- Test: `internal/runtime/supervisor_test.go`

- [ ] **Step 1: Write supervisor tests**

Test that switching stops the old link and starts the new link:

```go
func TestSupervisorSwitchesRuntimeLinks(t *testing.T) {
	manager := NewManager()
	qq := newFakeLink("qq_ws")
	wx := newFakeLink("wechat_cdp")
	s := NewSupervisor(manager, map[RuntimeTarget]RuntimeLink{
		RuntimeTargetQQWS: qq,
		RuntimeTargetWeChatCDP: wx,
	})

	if err := s.Switch(context.Background(), RuntimeTargetQQWS); err != nil {
		t.Fatalf("switch qq: %v", err)
	}
	if err := s.Switch(context.Background(), RuntimeTargetWeChatCDP); err != nil {
		t.Fatalf("switch wx: %v", err)
	}
	if qq.stopCount != 1 || wx.startCount != 1 {
		t.Fatalf("unexpected counts qq=%#v wx=%#v", qq, wx)
	}
	if manager.Status().Target != "wechat_cdp" {
		t.Fatalf("expected active target in status, got %#v", manager.Status())
	}
}
```

- [ ] **Step 2: Define runtime targets**

Use stable values:

```go
const (
	RuntimeTargetQQWS      RuntimeTarget = "qq_ws"
	RuntimeTargetWeChatCDP RuntimeTarget = "wechat_cdp"
	RuntimeTargetYYBCDP    RuntimeTarget = "yyb_cdp"
)
```

- [ ] **Step 3: Define `RuntimeLink`**

```go
type RuntimeLink interface {
	Target() RuntimeTarget
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Status() Status
	Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error)
}
```

- [ ] **Step 4: Implement `Supervisor`**

It must serialize switches with a mutex, keep the active target, return `"runtime link is not connected"` if no active link is available, and update `runtime.Manager` on every switch.

- [ ] **Step 5: Verify**

Run: `go test ./internal/runtime`

Expected: all tests pass.

## Task 3: Wrap QQ WS As A Runtime Link

**Files:**
- Create: `internal/runtime/qqlink/link.go`
- Modify: `app.go`
- Test: `internal/runtime/qqlink/link_test.go`
- Test: `app_test.go`

- [ ] **Step 1: Write QQ link tests**

Test that the wrapper reports `qq_ws` and forwards `Call` to the adapter.

- [ ] **Step 2: Implement `qqlink.Link`**

The wrapper owns the existing `qqws.Adapter` and calls `adapter.Start(ctx)` in `Start`.

- [ ] **Step 3: Replace direct adapter use in `App`**

`NewApp()` should create a supervisor and register QQ link. Diagnostics should receive the supervisor as the `RuntimeCaller`.

- [ ] **Step 4: Keep automatic QQ patch conditional**

Run QQ patch only when selected startup target is `qq_ws`.

- [ ] **Step 5: Verify no QQ regression**

Run:

```powershell
go test ./...
cd frontend; npm test
powershell -ExecutionPolicy Bypass -File scripts\build.ps1
```

Expected: all pass; `Farm_Go.exe` still listens on `127.0.0.1:8787`.

## Task 4: Settings UI For Link Switching

**Files:**
- Modify: `app.go`
- Modify: `frontend/src/views/SettingsView.tsx`
- Modify: `frontend/src/views/ConnectionView.tsx`
- Modify: `frontend/src/style.css`
- Generated: `frontend/wailsjs/go/main/App.js`
- Generated: `frontend/wailsjs/go/main/App.d.ts`
- Generated: `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Add backend API tests**

Add tests for:

- `RuntimeSettings()` returns defaults.
- `SaveRuntimeSettings()` persists default target.
- `SwitchRuntimeTarget("wechat_cdp")` changes active target or returns a structured startup error.

- [ ] **Step 2: Add Wails methods**

Expose:

```go
func (a *App) RuntimeSettings() storage.RuntimeSettings
func (a *App) SaveRuntimeSettings(settings storage.RuntimeSettings) storage.RuntimeSettings
func (a *App) SwitchRuntimeTarget(target string) farmruntime.Status
func (a *App) RuntimeLinkStatus() farmruntime.Status
```

- [ ] **Step 3: Update settings page**

Add a segmented selector or select controls:

- Current link: QQ WS / 微信 CDP / 应用宝 CDP
- Default startup link: QQ WS / 微信 CDP / 应用宝 CDP
- CDP port input
- WMPF debug port input
- Save button
- Switch now button

- [ ] **Step 4: Update copy**

Connection and diagnostics pages must say “当前链路” instead of “QQ WS” unless the selected link is specifically QQ WS.

- [ ] **Step 5: Verify UI**

Run:

```powershell
cd frontend; npm test
cd frontend; npm run build
```

Expected: TypeScript build succeeds and settings page renders with generated Wails bindings.

## Task 5: Go CDP Client

**Files:**
- Create: `internal/runtime/cdp/client.go`
- Create: `internal/runtime/cdp/client_test.go`
- Create: `internal/runtime/cdp/context.go`
- Create: `internal/runtime/cdp/context_test.go`

- [ ] **Step 1: Test command response matching**

Use `httptest` plus Gorilla WebSocket to assert `Runtime.enable` and `Runtime.evaluate` responses resolve by command id.

- [ ] **Step 2: Implement CDP client**

Support:

- `Connect(ctx, url)`
- `Send(ctx, method, params, timeout)`
- `Evaluate(ctx, expression, contextID, timeout)`
- CDP event callbacks for `Runtime.executionContextCreated`, `Runtime.executionContextDestroyed`, and `Runtime.executionContextsCleared`

- [ ] **Step 3: Test context scoring**

Use sample probes to ensure a context with `cc`, `GameGlobal`, and scene wins over ordinary browser contexts.

- [ ] **Step 4: Implement context selection**

Port the reference scoring behavior:

- `hasCc` adds the strongest score.
- scene name adds strong score.
- `GameGlobal`, canvas, document, wx add smaller scores.
- Choose the highest score above `100`.

- [ ] **Step 5: Verify**

Run: `go test ./internal/runtime/cdp`

Expected: all tests pass.

## Task 6: WMPF Process Discovery

**Files:**
- Create: `internal/runtime/wmpf/process.go`
- Create: `internal/runtime/wmpf/process_test.go`

- [ ] **Step 1: Test WeChat path detection**

Use paths containing `Tencent\xwechat\xplugin\plugins\RadiumWMPF`.

- [ ] **Step 2: Test YYB path detection**

Use paths matching `Tencent\Androws\WmpfRuntime\5.10.2700.327\runtime`.

- [ ] **Step 3: Implement process profile logic**

Profiles:

- `wechat_cdp`: accept WeChat WMPF paths and reject YYB paths.
- `yyb_cdp`: accept YYB paths and use the YYB runtime version segment.

- [ ] **Step 4: Verify**

Run: `go test ./internal/runtime/wmpf`

Expected: all tests pass.

## Task 7: WMPF Debug Bridge And CDP Proxy

**Files:**
- Create: `internal/runtime/wmpf/protocol.go`
- Create: `internal/runtime/wmpf/debug_server.go`
- Create: `internal/runtime/wmpf/debug_server_test.go`

- [ ] **Step 1: Port protocol handling**

Implement the equivalent of `WARemoteDebug_DebugMessage` decode/encode and `RemoteDebugCodex` wrap/unwrap. If a pure Go protobuf schema is unavailable, generate Go protobuf types from the reference protocol once and commit the generated Go source.

- [ ] **Step 2: Test miniapp connection lifecycle**

Assert server status changes:

- `miniappConnected=false` initially
- true after miniapp websocket connects
- false after close

- [ ] **Step 3: Test CDP proxy forwarding**

When CDP client sends JSON, debug bridge should wrap and emit a `chromeDevtools` miniapp message. When miniapp sends `chromeDevtoolsResult`, CDP client should receive the raw JSON.

- [ ] **Step 4: Implement listeners**

Start:

- miniapp debug server on `9420`
- legacy miniapp debug server on `9421`
- CDP proxy on `62000`

- [ ] **Step 5: Verify**

Run: `go test ./internal/runtime/wmpf`

Expected: protocol and bridge tests pass.

## Task 8: Frida Hook Resource Integration

**Files:**
- Create: `resources/wmpf/frida/hook.js`
- Create: `resources/wmpf/frida/config/*.json`
- Create: `resources/wmpf/frida/config/yyb/*.json`
- Create: `internal/runtime/wmpf/frida.go`
- Create: `internal/runtime/wmpf/frida_test.go`

- [ ] **Step 1: Copy only necessary resources**

Copy:

- `E:\desktop\farm-tauri\core\wmpf\frida\hook.js`
- `E:\desktop\farm-tauri\core\wmpf\frida\config\addresses.*.json`
- `E:\desktop\farm-tauri\core\wmpf\frida\config\yyb\addresses.*.json`

- [ ] **Step 2: Add resource validation tests**

Test every JSON config has:

- `LoadStartHookOffset`
- `CDPFilterHookOffset`
- non-empty `SceneOffsets`

- [ ] **Step 3: Implement hook loading**

Read embedded hook and replace `@@CONFIG@@` with selected config JSON plus `RuntimeTarget`.

- [ ] **Step 4: Implement attach abstraction**

Use a small interface around Frida calls so tests can fake:

- process enumerate
- attach
- create script
- load script

- [ ] **Step 5: Verify**

Run: `go test ./internal/runtime/wmpf`

Expected: resources validate and fake Frida attach loads the expected script.

## Task 9: WeChat And YYB CDP Runtime Links

**Files:**
- Create: `internal/runtime/wmpf/link.go`
- Create: `internal/runtime/wmpf/link_test.go`
- Modify: `app.go`

- [ ] **Step 1: Test link startup sequence**

With fake components, assert:

- debug bridge starts first
- CDP proxy starts
- Frida attach loop starts
- status becomes `listening`

- [ ] **Step 2: Test context ready transition**

Simulate miniapp connected, CDP Runtime context event, and successful runtime probe. Status must become `ready`.

- [ ] **Step 3: Implement shared link**

`NewCDPLink(profile, config, manager)` should support both `wechat_cdp` and `yyb_cdp`.

- [ ] **Step 4: Implement `Call`**

For first diagnostics:

- `host.describe`: return link status, transport status, selected context, WMPF version, PID, and profile.
- `gameCtl.probe`: evaluate a small expression checking `globalThis.gameCtl`, available method count, scene, and farm root if available.

- [ ] **Step 5: Register links**

Register `qq_ws`, `wechat_cdp`, and `yyb_cdp` in `App`.

- [ ] **Step 6: Verify**

Run: `go test ./...`

Expected: all tests pass.

## Task 10: Manual Live Verification

**Files:**
- No source changes unless verification finds a bug.

- [ ] **Step 1: Build exe**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\build.ps1
```

Expected: `build\bin\Farm_Go.exe` is produced.

- [ ] **Step 2: Verify QQ regression**

Start QQ WS link and check:

```powershell
Get-NetTCPConnection -LocalPort 8787
```

Expected: listener exists, and QQ miniapp shows `Established` after reopening the patched QQ farm miniapp.

- [ ] **Step 3: Verify WeChat CDP**

Switch current link to `wechat_cdp`, open WeChat farm miniapp, then check:

```powershell
Get-NetTCPConnection -LocalPort 9420
Get-NetTCPConnection -LocalPort 62000
```

Expected: debug server listens, CDP proxy listens, miniapp bridge connects after miniapp reload, and diagnostics returns `ok:true` for `host.describe`.

- [ ] **Step 4: Verify YYB CDP**

Switch current link to `yyb_cdp`, open YYB miniapp, then confirm status includes:

- runtime target `yyb_cdp`
- YYB WMPF process path
- WMPF version
- config path under `resources/wmpf/frida/config/yyb`

- [ ] **Step 5: Package final exe**

Run the build command again after live fixes and record exe path and timestamp in the completion note.

## Risks And Decisions

- Frida packaging is the main release risk. The implementation should first try direct Go integration; if that blocks the Windows exe, use a minimal embedded helper process only for Frida attach/load while keeping CDP/session logic in Go.
- The WARemoteDebug protobuf port must be verified with captured binary frames or live miniapp traffic; JSON-only tests are not enough.
- WeChat and YYB share most CDP logic; avoid duplicating link implementations.
- QQ WS must remain the default startup link until WeChat CDP has live verification.

