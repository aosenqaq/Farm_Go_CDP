# WMPF 25297 Static Address Compatibility Design

## Goal

Add exact static address support for the installed WeChat WMPF 25297 runtime so Farm_Go can load its Frida hook and complete the CDP debug WebSocket handshake.

The change ends at connection readiness. It does not invoke farm, warehouse, friend, reward, or any other business operation.

## Current Failure

Farm_Go extracts the WMPF version from the running `WeChatAppEx.exe` path and uses it as an exact resource name. For the installed plugin path ending in `RadiumWMPF/25297/extracted/runtime`, `loadFridaAddressConfig` reads `resources/wmpf/frida/config/addresses.25297.json`.

That file is absent. Resource loading fails before Frida attaches, which produces the visible `file does not exist` error. The resource tree currently ends at `addresses.20089.json`.

## Evidence And Address Selection

The local `25297/extracted/runtime/flue.dll` has SHA-256 `AF2BE53A10C20A96EA801C255986B4B93AADC4ADD8B52032A7215556041BA2AF`.

The CDP filter candidate is the function at RVA `0x3716230`. It is the unique function that references both `devtools_message_filter_applet_webview.cc` and `SendToClientFilter`, is contained in the executable `.text` section, and has a PE function-table entry beginning at that address.

Before adding the configuration, identify the exact `AppletIndexContainer::OnLoadStart(bool, const std::string&)` function from the same DLL. The selected RVA must satisfy all of these conditions:

1. It is an executable-section function entry recorded by the PE exception table.
2. Its class/source references identify `applet_index_container.cc`, rather than another `OnLoadStart` implementation such as a music, worker, OSR, game-center, sidebar, or picture-in-picture window.
3. Its x64 parameter handling matches the hook contract: `RCX` is the object pointer and the low byte of `RDX` is the load-start flag that Farm_Go changes to `1`.
4. The original six-step scene path `[64, 1480, 8, 1416, 16, 456]` is not valid for the selected object layout: its second member at `+0x5C8` is null. Live pointer tracing instead resolves the scene directly through `[64, 640]`, producing the observed `1023` and `1000` values.

No `20089` or earlier offset may be reused as a fallback or inferred solely from an RVA delta.

## Design

1. Preserve the current exact-version lookup. Do not add nearest-version fallback, dynamic signature scanning, or changes to process selection.
2. Add a focused hook-builder regression test for version `25297`. Before the address file is added, it must fail because the exact resource is absent. Once static verification provides both RVAs, the test must assert the version, both exact offsets, scene path, and `RuntimeTarget: cdp` in the generated script.
3. Add `resources/wmpf/frida/config/addresses.25297.json` containing the verified version, both RVAs, and the direct two-offset scene path `[64, 640]`.
4. Keep the existing endpoint behavior for pointer-based paths, but make `hook.js` skip endpoint-pointer resolution when a direct scene field has no pass-args object. The CDP client and all farm automation code remain unchanged.

## Validation

Run the focused regression test, the WMPF package tests, the full Go test suite, and `go build ./...`.

For live validation, start Farm_Go against WMPF 25297 and open the mini-program normally. Confirm that the Frida helper reports `FRIDA_LOADED` and that the WeChat CDP runtime reaches the ready/connected state. Do not send business-level `gameCtl` calls.

## Failure Handling

If either hook causes an access violation, process exit, failed Frida load, or missing CDP handshake, stop the live attempt. Remove no safeguards and do not add a fallback; return to the local binary evidence to re-evaluate the corresponding function entry or scene path.

## Acceptance Criteria

- `addresses.25297.json` exists and is selected by the exact-version loader.
- Its two RVAs are independently verified against the local 25297 DLL hash recorded above.
- The focused 25297 hook-builder test fails before the config is added and passes afterward.
- WMPF tests, full Go tests, and the Go build pass.
- The live runtime reports Frida loaded and reaches CDP ready without farm-operation traffic.
