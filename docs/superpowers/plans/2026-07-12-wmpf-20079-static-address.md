# WMPF 20079 Static Address Compatibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add verified static hook addresses for WeChat WMPF 20079 and prove the current runtime completes its CDP debug WebSocket handshake.

**Architecture:** Keep the existing exact-version resource lookup unchanged. Derive and validate the two 20079 `flue.dll` RVAs offline, add one versioned JSON resource, and cover it through the existing Frida hook builder before performing a narrow live connection check.

**Tech Stack:** Go 1.24 tests, Frida JavaScript hook resources, PE/COFF analysis with Python `pefile` and `capstone`, Windows WMPF runtime.

---

### Task 1: Identify and Validate the 20079 Hook RVAs

**Files:**
- Read: `resources/wmpf/frida/config/addresses.20005.json`
- Read: `resources/wmpf/frida/hook.js`
- Read: `%APPDATA%/Tencent/xwechat/xplugin/plugins/RadiumWMPF/20079/extracted/runtime/flue.dll`

- [ ] **Step 1: Confirm the installed image and PE executable sections**

Run a read-only `pefile` inspection against the installed `flue.dll`, printing its SHA-256, image base, image size, and executable section RVA ranges.

Expected: the installed image resolves under `RadiumWMPF/20079`, and both final candidates can be constrained to executable sections.

- [ ] **Step 2: Locate the two function candidates**

Use strings, cross-references, instruction patterns, and function-boundary decoding to locate the 20079 equivalents of:

```text
AppletIndexContainer::OnLoadStart
SendToClientFilter / devtools_message_filter_applet_webview
```

Expected: one defensible function entry RVA for each hook role. Do not use the 20005 RVAs as 20079 values without independent instruction evidence.

- [ ] **Step 3: Validate instruction boundaries and roles**

Decode each candidate with Capstone from the function entry and verify that the RVA is inside an executable section, starts on an instruction boundary, and has surrounding control/data flow consistent with the hook's argument use.

Expected: two validated RVAs. If either remains ambiguous, stop before writing configuration or attaching Frida.

### Task 2: Add the 20079 Configuration with TDD

**Files:**
- Modify: `internal/runtime/wmpf/frida_test.go`
- Create: `resources/wmpf/frida/config/addresses.20079.json`

- [ ] **Step 1: Write the failing version-specific test**

Add this test beside the existing hook-builder tests, using the repository resource directory:

```go
func TestBuildFridaHookScriptSupportsWMPF20079(t *testing.T) {
	script, err := BuildFridaHookScript(FridaHookOptions{
		Root:    testFridaRoot(t),
		Target:  farmruntime.RuntimeTargetWeChatCDP,
		Version: "20079",
	})
	if err != nil {
		t.Fatalf("build 20079 hook: %v", err)
	}
	for _, item := range []string{`"Version":20079`, `"RuntimeTarget":"cdp"`} {
		if !strings.Contains(script, item) {
			t.Fatalf("20079 hook script missing %q", item)
		}
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
go test ./internal/runtime/wmpf -run TestBuildFridaHookScriptSupportsWMPF20079 -count=1
```

Expected: FAIL because `config/addresses.20079.json` does not exist.

- [ ] **Step 3: Add the minimal exact-version resource**

Create `resources/wmpf/frida/config/addresses.20079.json` with `Version` set to `20079`, the two hexadecimal RVAs recorded in Task 1, and this established scene path:

```json
"SceneOffsets": [64, 1480, 8, 1416, 16, 456]
```

The file must contain only the same five fields as `addresses.20005.json`; no fallback or scanner configuration is added.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run:

```powershell
go test ./internal/runtime/wmpf -run TestBuildFridaHookScriptSupportsWMPF20079 -count=1
```

Expected: PASS.

- [ ] **Step 5: Run adjacent WMPF tests**

Run:

```powershell
go test ./internal/runtime/wmpf -count=1
```

Expected: PASS with no test failures.

### Task 3: Validate the Live 20079 Handshake

**Files:**
- Verify: `resources/wmpf/frida/config/addresses.20079.json`
- Observe: `%TEMP%/farm_go_frida_*.log`

- [ ] **Step 1: Build the application with embedded resources**

Run:

```powershell
go build ./...
```

Expected: all packages compile successfully and the new JSON is accepted by the existing embedded resource tree.

- [ ] **Step 2: Start the normal Farm_Go connection flow for WMPF 20079**

Use the existing application/runtime supervisor flow. Do not invoke planting, warehouse, friend, reward, or other farm operations.

Expected: the loader identifies version `20079` from the running process path and starts the Frida helper.

- [ ] **Step 3: Prove Frida load and WebSocket handshake**

Inspect the newest `%TEMP%/farm_go_frida_*.log` and application runtime status.

Expected: the log contains `FRIDA_LOADED` for the selected PID, and application status reaches connected/ready without an attach error, access violation, or unexpected WMPF process exit. Stop after connection readiness.

- [ ] **Step 4: Run final regression checks**

Run:

```powershell
go test ./internal/runtime/wmpf -count=1
go test ./... -count=1
git diff --check
```

Expected: both test commands pass and `git diff --check` reports no whitespace errors.

- [ ] **Step 5: Commit the implementation**

```powershell
git add internal/runtime/wmpf/frida_test.go resources/wmpf/frida/config/addresses.20079.json
git commit -m "fix: support WMPF 20079 handshake"
```
