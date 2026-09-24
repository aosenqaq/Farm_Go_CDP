# WMPF 20089 Static Address Compatibility Implementation Plan

> **For agentic workers:** Execute each task in order with tests before production changes.

**Goal:** Add verified static hook addresses for WeChat WMPF 20089.

**Architecture:** Preserve exact-version Frida resource lookup. Add one versioned configuration after a failing hook-builder test proves the missing-resource symptom, then verify the existing WMPF package and application build.

**Tech Stack:** Go tests, Frida JavaScript resources, PE/COFF inspection of the installed WMPF `flue.dll`.

---

### Task 1: Capture the missing-resource regression

**Files:**
- Modify: `internal/runtime/wmpf/frida_test.go`

- [ ] Add `TestBuildFridaHookScriptSupportsWMPF20089`, calling `BuildFridaHookScript` with WeChat CDP target and version `20089`.
- [ ] Assert the built script contains version `20089`, `LoadStartHookOffset` `0x25E0170`, `CDPFilterHookOffset` `0x2D95AB0`, the established scene path, and `RuntimeTarget` `cdp`.
- [ ] Run `go test ./internal/runtime/wmpf -run '^TestBuildFridaHookScriptSupportsWMPF20089$' -count=1 -v` and confirm it fails because `addresses.20089.json` does not exist.

### Task 2: Add the exact address configuration

**Files:**
- Create: `resources/wmpf/frida/config/addresses.20089.json`

- [ ] Create the exact version configuration using RVAs independently validated from `20089/extracted/runtime/flue.dll` SHA-256 `F2FE383D44309C28A8F77101DD71A6DAF7A83835B9DD93B0B09313E0AB52AA2F`.
- [ ] Re-run the focused test and confirm it passes.

### Task 3: Verify integration

**Files:**
- Verify: `internal/runtime/wmpf/frida_test.go`
- Verify: `resources/wmpf/frida/config/addresses.20089.json`

- [ ] Run `go test ./internal/runtime/wmpf -count=1`.
- [ ] Run `go test ./... -count=1` and `go build ./...`.
- [ ] Run `git diff --check`.
- [ ] Restart the current Farm_Go application and confirm the mini-program connects through Frida/CDP without invoking farm mutations.
