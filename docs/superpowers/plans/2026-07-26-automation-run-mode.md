# 自动化运行模式 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the recommended automation configuration with account-scoped safe/god run modes that control scheduling while preserving each account's task configuration.

**Architecture:** Persist a small `safe`/`god` value alongside each account's automation settings. The backend snapshots that value when work starts, applying serialized execution and paced friend actions only in safe mode. The automation page exposes the mode through a compact gear entry, choice dialog, and final confirmation dialog that call a dedicated Wails API.

**Tech Stack:** Go, SQLite-backed account storage, Wails bindings, React, TypeScript, Vitest, CSS.

---

### Task 1: Model and persist the run mode

**Files:**
- Modify: `internal/farm/automation/run_mode.go`
- Modify: `internal/storage/automation.go`
- Test: `internal/farm/automation/run_mode_test.go`
- Test: `internal/storage/storage_test.go`

- [ ] **Step 1: Write failing storage tests**

```go
func TestLoadAutomationSettingsDefaultsNewAccountToSafe(t *testing.T) {
    settings, err := store.LoadAutomationSettings(ctx, "new-account")
    if err != nil { t.Fatal(err) }
    if settings.RunMode != automation.RunModeSafe { t.Fatalf("got %q", settings.RunMode) }
}
```

- [ ] **Step 2: Run the focused storage tests and verify they fail**

Run: `go test ./internal/storage ./internal/farm/automation -run 'RunMode|AutomationSettings' -count=1`

Expected: FAIL before the `RunMode` type and persisted key exist.

- [ ] **Step 3: Implement mode normalization and account migration**

```go
type RunMode string
const (
    RunModeSafe RunMode = "safe"
    RunModeGod RunMode = "god"
)
```

Persist new accounts as `safe`; map legacy records with no mode and malformed stored modes to `god`.

- [ ] **Step 4: Run the focused storage tests and verify they pass**

Run: `go test ./internal/storage ./internal/farm/automation -run 'RunMode|AutomationSettings' -count=1`

Expected: PASS.

### Task 2: Apply mode to scheduling and execution ownership

**Files:**
- Modify: `internal/farm/automation/scheduler.go`
- Modify: `automation_execution_gate.go`
- Modify: `app.go`
- Test: `internal/farm/automation/scheduler_test.go`
- Test: `automation_execution_gate_test.go`

- [ ] **Step 1: Write failing mode-specific scheduler and gate tests**

```go
func TestSafeModeDispatchesOnlyOneDueTask(t *testing.T) { /* assert one dispatch */ }
func TestSafeModeRejectsManualTaskWhileAutomaticTaskRuns(t *testing.T) { /* assert StatusRuntimeBusy */ }
```

- [ ] **Step 2: Run them and verify they fail**

Run: `go test ./internal/farm/automation ./ -run 'SafeMode.*(Dispatches|Rejects)' -count=1`

Expected: FAIL before mode-aware dispatch and shared execution gate exist.

- [ ] **Step 3: Implement the minimum mode-aware execution behavior**

Limit safe-mode scheduler rounds to one runnable task. Acquire the shared account gate for automatic, manual, and auto-sell work only in safe mode; return `StatusRuntimeBusy` for a conflicting manual attempt. Exclude guardian services from this gate.

- [ ] **Step 4: Run the scheduler and gate tests and verify they pass**

Run: `go test ./internal/farm/automation ./ -run 'SafeMode.*(Dispatches|Rejects)' -count=1`

Expected: PASS.

### Task 3: Pace friend interactions in safe mode

**Files:**
- Modify: `internal/farm/automation/runtime.go`
- Modify: `internal/farm/automation/runtime_friend.go`
- Test: `internal/farm/automation/runtime_friend_test.go`

- [ ] **Step 1: Write failing tests for friend pacing and safe land requests**

```go
func TestSafeModeStealUsesFullFarmHarvestAfterFriendWait(t *testing.T) { /* one full land list per friend, 100-1000 between friends */ }
func TestSafeModeHelpWaitsOnlyBetweenFriends(t *testing.T) { /* one-click help per friend */ }
func TestGodModeKeepsParallelFriendPath(t *testing.T) { /* original batching */ }
```

- [ ] **Step 2: Run them and verify they fail**

Run: `go test ./internal/farm/automation -run 'SafeMode.*(Steal|Help|Mischief)|GodModeKeeps' -count=1`

Expected: FAIL before the safe-mode runtime branch exists.

- [ ] **Step 3: Implement cancellable mode-specific execution paths**

Use the task's mode snapshot. In safe mode process friends in order with a randomized 100-1000ms delay after the first friend; send each friend's eligible stolen land IDs in one Harvest request, matching the proven manual request shape; process mischief lands with a randomized 100-200ms delay after the first land. Keep help as one call per friend and retain the current parallel/batched path in god mode.

- [ ] **Step 4: Run the focused runtime tests and verify they pass**

Run: `go test ./internal/farm/automation -run 'SafeMode.*(Steal|Help|Mischief)|GodModeKeeps' -count=1`

Expected: PASS.

### Task 4: Replace the recommended-config API

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`
- Modify: `lan_rpc.go`
- Modify: `lan_rpc_test.go`
- Modify: `frontend/wailsjs/go/main/App.d.ts`
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/models.ts`
- Delete: `recommended_automation_config.go`

- [ ] **Step 1: Write a failing application API test**

```go
func TestSetFarmAutomationRunModeUpdatesOnlyCurrentAccountMode(t *testing.T) {
    // Persist a settings state, change mode, then compare canonical settings JSON.
}
```

- [ ] **Step 2: Run it and verify it fails**

Run: `go test ./ -run 'TestSetFarmAutomationRunModeUpdatesOnlyCurrentAccountMode' -count=1`

Expected: FAIL before the dedicated setter preserves the stored configuration.

- [ ] **Step 3: Implement dedicated mode switching**

Validate authorization and mode, persist only the current account's mode, reconfigure only that account's scheduler after successful storage, and return the latest automation state. Remove the recommended-config Wails and LAN RPC APIs.

- [ ] **Step 4: Run the focused API tests and verify they pass**

Run: `go test ./ -run 'Test(SetFarmAutomationRunMode|SaveFarmAutomationStatePreservesRunMode|RecommendedFarm|LANRPCRejectsInvalidArguments)' -count=1`

Expected: PASS.

### Task 5: Build the compact run-mode UI

**Files:**
- Modify: `frontend/src/views/AuthorizedApp.tsx`
- Modify: `frontend/src/views/FarmWorkspaceView.tsx`
- Modify: `frontend/src/views/AutomationView.tsx`
- Modify: `frontend/src/styles.css`
- Test: `frontend/src/views/AutomationView.test.tsx`

- [ ] **Step 1: Write failing UI tests**

```tsx
it('replaces recommended config with a compact run-mode gear', () => {
  expect(html).toContain('运行模式：安全模式')
  expect(html).not.toContain('应用推荐配置')
})
it('requires final confirmation before saving a different mode', async () => { /* select then confirm */ })
```

- [ ] **Step 2: Run the focused test and verify it fails**

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: FAIL because the recommended-config controls are still rendered.

- [ ] **Step 3: Implement the mode selection and confirmation dialogs**

Render `运行模式：安全模式/仙人模式` and a gear icon in the automation header. The first dialog presents the two approved plain-language descriptions, marks the current selection, and disables next for it. The confirmation dialog states that scheduling changes and guardian service behavior remains unchanged. Disable close/back/confirm while saving; preserve selection and show errors on failure.

- [ ] **Step 4: Run the UI test and verify it passes**

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: PASS.

### Task 6: Regression verification

**Files:**
- Verify: all changed Go and frontend files

- [ ] **Step 1: Search for removed terminology and API**

Run: `rg -n 'ApplyRecommendedFarmAutomationConfig|recommendedFarmAutomationSettings|应用推荐配置' --glob '!docs/**'`

Expected: no matches.

- [ ] **Step 2: Run backend regression suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Run frontend tests and production build**

Run: `npm test -- --run`

Run: `npm run build`

Expected: both PASS.

- [ ] **Step 4: Validate the diff**

Run: `git diff --check`

Expected: no whitespace errors.
