# WMPF 25297 Static Address Compatibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore Frida injection and WeChat CDP readiness for the exact WMPF 25297 runtime without invoking farm business operations.

**Architecture:** Keep the existing exact-version resource lookup. Add one 25297 Frida address resource after a focused regression test proves the missing-file failure, using RVAs validated against the local `flue.dll` hash `AF2BE53A10C20A96EA801C255986B4B93AADC4ADD8B52032A7215556041BA2AF`: `AppletIndexContainer::OnLoadStart` at `0x2A5D800` and `SendToClientFilter` at `0x3716230`. Live pointer tracing rejects the inherited six-step path because `this + 0x40 + 0x5C8` is null; the verified direct scene field is `[64, 640]`. The hook must not derive a WebSocket pass-args pointer from this direct field.

**Tech Stack:** Go tests, JSON Frida resources, local PE/COFF static analysis, Frida, WeChat CDP.

---

### Task 1: Capture The Exact-Version Regression

**Files:**
- Modify: `internal/runtime/wmpf/frida_test.go`

- [ ] **Step 1: Write the failing 25297 hook-builder test**

Add this test after `TestBuildFridaHookScriptSupportsWMPF20089`:

```go
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
}
```

- [ ] **Step 2: Run the focused test and verify the expected red state**

Run:

```powershell
go test ./internal/runtime/wmpf -run '^TestBuildFridaHookScriptSupportsWMPF25297$' -count=1 -v
```

Expected: the test fails with an error naming `resources/wmpf/frida/config/addresses.25297.json` and `file does not exist`.

- [ ] **Step 3: Commit the regression test**

```powershell
git add internal/runtime/wmpf/frida_test.go
git commit -m "test: cover WMPF 25297 Frida configuration"
```

### Task 2: Add The Verified 25297 Address Resource

**Files:**
- Create: `resources/wmpf/frida/config/addresses.25297.json`
- Verify: `internal/runtime/wmpf/frida_test.go`

- [ ] **Step 1: Add the exact static configuration**

Create `resources/wmpf/frida/config/addresses.25297.json` with exactly:

```json
{
    "Version": 25297,
    "LoadStartHookOffset": "0x2A5D800",
    "CDPFilterHookOffset": "0x3716230",
    "SceneOffsets": [64, 640]
}
```

The file is a WeChat CDP resource, so it belongs directly in `config/`, not `config/yyb/`.

- [ ] **Step 2: Re-run the focused test and verify the green state**

Run:

```powershell
go test ./internal/runtime/wmpf -run '^TestBuildFridaHookScriptSupportsWMPF25297$' -count=1 -v
```

Expected: `PASS`, with the generated script including both exact RVAs, the six-step scene path, and `RuntimeTarget` `cdp`.

- [ ] **Step 3: Validate all Frida resources**

Run:

```powershell
go test ./internal/runtime/wmpf -run '^TestValidateFridaResources$' -count=1 -v
```

Expected: `PASS`; the new JSON conforms to the existing resource schema.

- [ ] **Step 4: Commit the resource and passing regression test**

```powershell
git add resources/wmpf/frida/config/addresses.25297.json internal/runtime/wmpf/frida_test.go
git commit -m "feat: support WeChat WMPF 25297 Frida hook"
```

### Task 3: Verify Runtime Integration Boundaries

**Files:**
- Verify: `internal/runtime/wmpf/frida_test.go`
- Verify: `resources/wmpf/frida/config/addresses.25297.json`

- [ ] **Step 1: Run the complete WMPF package suite**

Run:

```powershell
go test ./internal/runtime/wmpf -count=1
```

Expected: `ok   Farm_Go/internal/runtime/wmpf`.

- [ ] **Step 2: Run full Go verification and build**

Run:

```powershell
go test ./... -count=1
go build ./...
git diff --check
```

Expected: both Go commands exit `0`; `git diff --check` prints no whitespace errors.

- [ ] **Step 3: Validate the live connection without business traffic**

Start Farm_Go, open the WeChat mini-program normally, and inspect the Frida diagnostic log for the selected WMPF process.

Expected evidence:

```text
FRIDA_LOADED <pid>
```

Then confirm the WeChat CDP runtime reports its connected or ready state. Stop validation there; do not invoke `gameCtl` methods or any farm, warehouse, friend, reward, or automation action.

- [ ] **Step 4: Confirm verification introduced no unreviewed changes**

```powershell
git status --short
```

Expected: no files are listed. The test and resource changes were already committed in Tasks 1 and 2; live validation must not alter source files.
