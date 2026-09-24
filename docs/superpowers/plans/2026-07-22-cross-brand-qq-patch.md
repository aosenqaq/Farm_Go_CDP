# Cross-Brand QQ Patch Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure normal Farm_Go and HQ Farm installations leave exactly one active QQ debug patch, and make the HQ release build reject regressions.

**Architecture:** `qqpatch.Install` will recognize all complete branded `*_QQ_DEBUG` blocks. Only one requested-hash block is latest; every other state removes managed and legacy blocks before appending the current bundle. The HQ builder runs the focused test in its branded temporary workspace before the full Go test suite.

**Tech Stack:** Go, `regexp`, Go testing, PowerShell.

---

### Task 1: Replace Every Prior Managed QQ Patch

**Files:**
- Modify: `internal/runtime/qqpatch/patcher_test.go`
- Modify: `internal/runtime/qqpatch/patcher.go`

- [ ] **Step 1: Write a failing foreign-brand test.**

Append this test:

```go
func TestInstallReplacesForeignBrandPatch(t *testing.T) {
	dir := t.TempDir()
	gameJS := filepath.Join(dir, "game.js")
	foreignStart := "// >>> HQ_FARM_QQ_DEBUG START >>>"
	foreignEnd := "// <<< HQ_FARM_QQ_DEBUG END <<<"
	original := strings.Join([]string{
		"console.log('game');", "", MarkerStart, "// scriptHash=old-farm", MarkerEnd, "",
		foreignStart, "// scriptHash=hq", "root.__hqFarmHost = {};", foreignEnd, "",
	}, "\n")
	if err := os.WriteFile(gameJS, []byte(original), 0o644); err != nil { t.Fatal(err) }
	result := Install(Options{TargetPath: gameJS, HostScript: "normalHost();", HostVersion: "farm-go-host-1", NoBackup: true})
	if !result.OK || result.Action != "replaced" || !result.RestartRequired { t.Fatalf("%#v", result) }
	content, err := os.ReadFile(gameJS)
	if err != nil { t.Fatal(err) }
	text := string(content)
	if strings.Contains(text, foreignStart) || strings.Contains(text, "root.__hqFarmHost") { t.Fatalf("foreign patch remained: %q", text) }
	if strings.Count(text, "_QQ_DEBUG START >>>") != 1 || !strings.Contains(text, "normalHost();") { t.Fatalf("expected one normal patch: %q", text) }
}
```

- [ ] **Step 2: Verify RED.**

Run `go test ./internal/runtime/qqpatch -run '^TestInstallReplacesForeignBrandPatch$' -count=1`.

Expected: FAIL because the current patcher leaves the HQ block.

- [ ] **Step 3: Write the failing mixed-block idempotency test.**

Append this test:

```go
func TestInstallDoesNotTreatMixedBrandBlocksAsLatest(t *testing.T) {
	dir := t.TempDir()
	gameJS := filepath.Join(dir, "game.js")
	if err := os.WriteFile(gameJS, []byte("console.log('game');\n"), 0o644); err != nil { t.Fatal(err) }
	options := Options{TargetPath: gameJS, HostScript: "sameHost();", HostVersion: "farm-go-host-1", NoBackup: true}
	if first := Install(options); !first.OK { t.Fatalf("%#v", first) }
	file, err := os.OpenFile(gameJS, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil { t.Fatal(err) }
	_, err = file.WriteString("\n// >>> HQ_FARM_QQ_DEBUG START >>>\n// scriptHash=hq\n// <<< HQ_FARM_QQ_DEBUG END >>>\n")
	if closeErr := file.Close(); err == nil { err = closeErr }
	if err != nil { t.Fatal(err) }
	second := Install(options)
	if !second.OK || second.Action == "already_latest" || !second.RestartRequired { t.Fatalf("%#v", second) }
	content, err := os.ReadFile(gameJS)
	if err != nil { t.Fatal(err) }
	if strings.Count(string(content), "_QQ_DEBUG START >>>") != 1 { t.Fatalf("%q", content) }
}
```

- [ ] **Step 4: Verify both tests are RED.**

Run `go test ./internal/runtime/qqpatch -run 'TestInstall(ReplacesForeignBrandPatch|DoesNotTreatMixedBrandBlocksAsLatest)' -count=1`.

Expected: both FAIL: foreign residue remains and mixed blocks return `already_latest`.

- [ ] **Step 5: Implement generic cleanup.**

Add `regexp` and these helpers in `patcher.go`:

```go
var managedPatchBlockPattern = regexp.MustCompile(`(?ms)^// >>> [A-Z0-9_]+_QQ_DEBUG START >>>\r?\n.*?^// <<< [A-Z0-9_]+_QQ_DEBUG END >>>`)

func managedPatchBlocks(text string) []string { return managedPatchBlockPattern.FindAllString(text, -1) }

func stripManagedPatchBlocks(text string) (string, bool) {
	if !managedPatchBlockPattern.MatchString(text) { return text, false }
	return strings.TrimSpace(managedPatchBlockPattern.ReplaceAllString(text, "")), true
}
```

Change `hasLatestPatch` to require exactly one `managedPatchBlocks(text)` item containing `// scriptHash=<hash>`. In `patchGameJS`, remove legacy blocks, remove managed blocks, append the bundle, and return the managed-removal flag as `replaced`.

- [ ] **Step 6: Verify GREEN.**

Run `go test ./internal/runtime/qqpatch -count=1`.

Expected: PASS, including legacy cleanup and single-brand idempotency.

- [ ] **Step 7: Commit.**

Run `git add internal/runtime/qqpatch/patcher.go internal/runtime/qqpatch/patcher_test.go` then `git commit -m "fix: replace foreign QQ debug patches"`.

### Task 2: Require the Cross-Brand Test in HQ Builds

**Files:**
- Modify: `scripts/build-hq-farm-agent-release.test.ps1`
- Modify: `scripts/build-hq-farm-agent-release.ps1`

- [ ] **Step 1: Write a failing builder-contract test.**

After `$builder` is assigned in the PowerShell test, add:

```powershell
$builderSource = Get-Content -Raw -LiteralPath $builder
$focusedPatchTest = "go test ./internal/runtime/qqpatch -run '^TestInstallReplacesForeignBrandPatch$' -count=1"
if (-not $builderSource.Contains($focusedPatchTest)) {
  throw 'HQ builder must run the focused cross-brand QQ patch regression.'
}
```

- [ ] **Step 2: Verify RED.**

Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-hq-farm-agent-release.test.ps1`.

Expected: FAIL with `HQ builder must run the focused cross-brand QQ patch regression.`

- [ ] **Step 3: Implement the disposable-workspace release gate.**

Inside the builder's existing `Push-Location $workspace` / `if (-not $SkipRegressionTests)` block, before `go test ./...`, add:

```powershell
go test ./internal/runtime/qqpatch -run '^TestInstallReplacesForeignBrandPatch$' -count=1
if ($LASTEXITCODE -ne 0) { throw 'HQ Farm copied cross-brand QQ patch regression failed.' }
```

- [ ] **Step 4: Verify GREEN.**

Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-hq-farm-agent-release.test.ps1`.

Expected: PASS without credential output.

- [ ] **Step 5: Commit.**

Run `git add scripts/build-hq-farm-agent-release.ps1 scripts/build-hq-farm-agent-release.test.ps1` then `git commit -m "test: guard HQ builds against patch residue"`.

### Task 3: Verify Recovery

**Files:** No production changes.

- [ ] **Step 1: Run `go test ./...`.** Expected: PASS.
- [ ] **Step 2: In `frontend`, run `npm test` and `npm run build`.** Expected: both exit 0.
- [ ] **Step 3: Run `git status --short` and `git log --oneline -3`.** Expected: two implementation commits; existing Wails generated-file changes remain unstaged.
- [ ] **Step 4: Restart normal Farm_Go, then restart QQ miniapp.** Expected: runtime event reaches `qq_ws ready` with host version `farm-go-host-1`, never `hq-farm-host-1`.

## Plan Self-Review

- Coverage: Task 1 prevents mixed managed patches; Task 2 makes the agent build enforce it; Task 3 verifies code and live recovery.
- No placeholders: all changes, commands, outcomes, and commits are explicit.
- Consistency: tests use existing `Install`, `Options`, `MarkerStart`, and `MarkerEnd`; helpers are limited to `managedPatchBlocks` and `stripManagedPatchBlocks`.
