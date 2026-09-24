# CDP Embedded Button Script Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make WeChat and YYB CDP runtime injection use the `button.js` source embedded in the executable so protected single-file releases become ready without an adjacent `resources` directory.

**Architecture:** Extend `wmpf.CDPLinkConfig` with an in-memory script value and centralize source selection in a small loader that prefers memory and falls back to the existing disk path. Centralize application CDP configuration so both profiles, during initial construction and runtime rebuild, always receive the embedded script.

**Tech Stack:** Go 1.24, Wails, `go:embed`, Go table-driven tests.

---

### Task 1: Load the CDP button script from memory

**Files:**
- Modify: `internal/runtime/wmpf/link.go:26-38`
- Modify: `internal/runtime/wmpf/link.go:1241-1295`
- Test: `internal/runtime/wmpf/link_test.go`

- [ ] **Step 1: Write failing source-selection tests**

Add these tests near the existing CDP profile tests in
`internal/runtime/wmpf/link_test.go`:

```go
func TestCDPLinkButtonScriptSourcePrefersEmbeddedSourceForAllProfiles(t *testing.T) {
	for _, target := range []farmruntime.RuntimeTarget{
		farmruntime.RuntimeTargetWeChatCDP,
		farmruntime.RuntimeTargetYYBCDP,
	} {
		t.Run(string(target), func(t *testing.T) {
			link := NewCDPLink(
				ProfileForTarget(target),
				CDPLinkConfig{
					ButtonScript:     "globalThis.gameCtl = {};",
					ButtonScriptPath: filepath.Join(t.TempDir(), "missing", "button.js"),
				},
				farmruntime.NewManager(),
			)

			source, err := link.buttonScriptSource()
			if err != nil {
				t.Fatalf("load embedded button script: %v", err)
			}
			if string(source) != "globalThis.gameCtl = {};" {
				t.Fatalf("unexpected embedded source %q", source)
			}
		})
	}
}

func TestCDPLinkButtonScriptSourceFallsBackToDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "button.js")
	if err := os.WriteFile(path, []byte("globalThis.gameCtl = { disk: true };"), 0o644); err != nil {
		t.Fatalf("write disk button script: %v", err)
	}
	link := NewCDPLink(
		ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		CDPLinkConfig{ButtonScriptPath: path},
		farmruntime.NewManager(),
	)

	source, err := link.buttonScriptSource()
	if err != nil {
		t.Fatalf("load disk button script: %v", err)
	}
	if string(source) != "globalThis.gameCtl = { disk: true };" {
		t.Fatalf("unexpected disk source %q", source)
	}
}
```

Add `os` and `path/filepath` to the test imports.

- [ ] **Step 2: Run the tests and verify they fail**

Run:

```powershell
go test ./internal/runtime/wmpf -run 'TestCDPLinkButtonScriptSource' -count=1
```

Expected: compilation fails because `CDPLinkConfig.ButtonScript` and
`buttonScriptSource` do not exist.

- [ ] **Step 3: Implement in-memory-first source selection**

Add the field to `CDPLinkConfig` in `internal/runtime/wmpf/link.go`:

```go
ButtonScript     string
ButtonScriptPath string
```

Replace the path-only block in `ensureGameCtlReady`:

```go
source, err := l.buttonScriptSource()
if err != nil {
	return err
}
if len(source) == 0 {
	return nil
}
```

Keep the existing hashing, injection, and readiness verification unchanged.
Add this loader immediately before `buttonScriptPath`:

```go
func (l *CDPLink) buttonScriptSource() ([]byte, error) {
	if l.cfg.ButtonScript != "" {
		return []byte(l.cfg.ButtonScript), nil
	}
	path := l.buttonScriptPath()
	if path == "" {
		return nil, nil
	}
	return os.ReadFile(path)
}
```

- [ ] **Step 4: Run focused WMPF tests**

Run:

```powershell
go test ./internal/runtime/wmpf -count=1
```

Expected: `ok Farm_Go/internal/runtime/wmpf`.

- [ ] **Step 5: Commit the source loader**

```powershell
git add internal/runtime/wmpf/link.go internal/runtime/wmpf/link_test.go
git commit -m "fix: load CDP button script from memory"
```

### Task 2: Wire the embedded script into both CDP profiles

**Files:**
- Modify: `app.go:187-270`
- Modify: `app.go:2844-2890`
- Test: `app_test.go`

- [ ] **Step 1: Write a failing application wiring test**

Add the `Farm_Go/internal/config` import and this test near
`TestNewAppExposesInitialRuntimeStatus` in `app_test.go`:

```go
func TestAppCDPLinkConfigIncludesEmbeddedButtonScript(t *testing.T) {
	cfg := config.Default()
	got := newCDPLinkConfig(cfg, "embedded-python.exe")

	if got.ButtonScript != runtimeButtonScript || got.ButtonScript == "" {
		t.Fatal("expected CDP links to receive the embedded button script")
	}
	if got.FridaRoot != embeddedFridaRoot {
		t.Fatalf("unexpected Frida root %q", got.FridaRoot)
	}
	if got.FridaPython != "embedded-python.exe" {
		t.Fatalf("unexpected Frida Python path %q", got.FridaPython)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```powershell
go test . -run TestAppCDPLinkConfigIncludesEmbeddedButtonScript -count=1
```

Expected: compilation fails because `newCDPLinkConfig` does not exist.

- [ ] **Step 3: Centralize and apply the CDP link configuration**

Add this package-level helper before `NewApp` in `app.go`:

```go
func newCDPLinkConfig(cfg config.Config, fridaPython string) wmpf.CDPLinkConfig {
	return wmpf.CDPLinkConfig{
		DebugPort:       cfg.WMPF.DebugPort,
		LegacyDebugPort: cfg.WMPF.LegacyDebugPort,
		CDPPort:         cfg.CDP.Port,
		FridaRoot:       embeddedFridaRoot,
		FridaResources:  embeddedFridaResources,
		FridaPython:     fridaPython,
		FridaEnabled:    cfg.WMPF.FridaEnabled,
		ButtonScript:    runtimeButtonScript,
	}
}
```

In `NewApp`, replace both inline `wmpf.CDPLinkConfig` literals with:

```go
newCDPLinkConfig(cfg, "")
```

In `rebuildCDPLinks`, replace both inline literals with:

```go
newCDPLinkConfig(a.cfg, a.fridaPython)
```

- [ ] **Step 4: Run main-package and focused runtime tests**

Run:

```powershell
go test . -count=1
go test ./internal/runtime/wmpf -count=1
```

Expected: both commands exit successfully.

- [ ] **Step 5: Commit the application wiring**

```powershell
git add app.go app_test.go
git commit -m "fix: wire embedded script into CDP links"
```

### Task 3: Verify the complete fix

**Files:**
- Verify: `app.go`
- Verify: `internal/runtime/wmpf/link.go`
- Verify: `app_test.go`
- Verify: `internal/runtime/wmpf/link_test.go`

- [ ] **Step 1: Format changed Go files**

Run:

```powershell
gofmt -w app.go app_test.go internal/runtime/wmpf/link.go internal/runtime/wmpf/link_test.go
```

Expected: command exits successfully.

- [ ] **Step 2: Run the full Go test suite**

Run:

```powershell
go test ./... -count=1
```

Expected: all Go packages pass.

- [ ] **Step 3: Run repository diff checks**

Run:

```powershell
git diff --check
git status --short
```

Expected: no whitespace errors; status contains only the intended plan or
implementation changes not already committed.

- [ ] **Step 4: Confirm both production construction paths are wired**

Run:

```powershell
rg -n "newCDPLinkConfig|ButtonScript" app.go internal/runtime/wmpf/link.go
```

Expected: `newCDPLinkConfig` supplies `runtimeButtonScript`; both `NewApp` CDP
links and both `rebuildCDPLinks` CDP links use that helper.

- [ ] **Step 5: Report the manual VMP smoke-test boundary**

Record that automated tests prove operation without a disk script, while final
end-to-end confirmation still requires launching a newly built VMP executable
and observing both WeChat CDP and YYB CDP reach `ready` with no adjacent
`resources` directory.
