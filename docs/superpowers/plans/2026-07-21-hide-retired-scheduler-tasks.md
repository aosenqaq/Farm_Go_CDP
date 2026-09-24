# Hide Retired Scheduler Tasks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove retired He Feng tasks from the scheduler state so the scheduler center and all scheduler consumers no longer expose them.

**Architecture:** Keep persisted task definitions and retired-switch normalization for backward compatibility. Filter tasks whose enabled configuration key is registered in `retiredAutomationSwitches` while `StateFromSettings` builds the public scheduler state, allowing counts, scheduling, and frontend rendering to derive from one filtered list.

**Tech Stack:** Go 1.26, React 18, TypeScript, Vitest, Vite

---

### Task 1: Filter retired tasks from scheduler state

**Files:**
- Modify: `internal/farm/automation/catalog_test.go:156`
- Modify: `internal/farm/automation/catalog_test.go:593`
- Modify: `internal/farm/automation/catalog_test.go:608`
- Modify: `internal/farm/automation/catalog.go:231`

- [ ] **Step 1: Write the failing public-state regression test**

Change the expected public task count in `TestStateContainsReferenceSchedulerTasksSortedByPriority` from 19 to 17:

```go
	if len(state.Scheduler.Tasks) != 17 {
		t.Fatalf("scheduler task count = %d, want 17", len(state.Scheduler.Tasks))
	}
```

Add this test after `TestStateContainsReferenceSchedulerTasksSortedByPriority`:

```go
func TestStateDoesNotExposeRetiredSchedulerTasks(t *testing.T) {
	state := DefaultState()
	for _, taskID := range []string{"he_feng_travel_reward", "limited_seed_draw"} {
		if task := findSchedulerTaskForTest(state.Scheduler.Tasks, taskID); task != nil {
			t.Fatalf("retired task %s should not be exposed: %#v", taskID, task)
		}
	}

	task := findSchedulerTaskForTest(state.Scheduler.Tasks, "mystery_shop_auto_buy")
	if task == nil || task.Enabled {
		t.Fatalf("non-retired disabled task should remain visible: %#v", task)
	}
}
```

- [ ] **Step 2: Run the regression test and verify it fails**

Run:

```powershell
go test ./internal/farm/automation -run 'TestStateContainsReferenceSchedulerTasksSortedByPriority|TestStateDoesNotExposeRetiredSchedulerTasks' -count=1
```

Expected: FAIL because `DefaultState` still returns 19 tasks and exposes both retired task IDs.

- [ ] **Step 3: Add the minimal state-construction filter**

In `StateFromSettings`, add the retired-switch check at the start of the `schedulerTaskDefs` loop:

```go
	tasks := make([]SchedulerTask, 0, len(schedulerTaskDefs))
	now := time.Now()
	for _, def := range schedulerTaskDefs {
		if isRetiredAutomationSwitch(schedulerTaskConfigKeys[def.id]) {
			continue
		}
		task := taskSettings[def.id]
```

Do not remove the retired task definitions, configuration defaults, normalization, reward schedules, or runtime handlers; they remain available for loading old settings and preserving compatibility.

- [ ] **Step 4: Run the regression test and verify it passes**

Run:

```powershell
gofmt -w internal/farm/automation/catalog.go internal/farm/automation/catalog_test.go
go test ./internal/farm/automation -run 'TestStateContainsReferenceSchedulerTasksSortedByPriority|TestStateDoesNotExposeRetiredSchedulerTasks' -count=1
```

Expected: PASS with 17 public tasks, both retired IDs absent, and `mystery_shop_auto_buy` still present and disabled.

- [ ] **Step 5: Align existing catalog tests with the public-state contract**

Remove this obsolete equality assertion from `TestSchedulerTaskExposesCanonicalConfigKeys`; `schedulerTaskDefs` intentionally includes compatibility-only retired definitions after this change:

```go
	if len(state.Scheduler.Tasks) != len(schedulerTaskDefs) {
		t.Fatalf("scheduler task count = %d, want %d", len(state.Scheduler.Tasks), len(schedulerTaskDefs))
	}
```

At the start of the loop in `TestSettingsFromStateUsesAllDetailedSchedulerConfigValuesAsCanonicalSource`, skip definitions that are not part of the public state:

```go
	for index, def := range schedulerTaskDefs {
		if isRetiredAutomationSwitch(schedulerTaskConfigKeys[def.id]) {
			continue
		}
		t.Run(def.id, func(t *testing.T) {
```

Remove the now-unreachable retired-switch override inside that test:

```go
				if slices.Contains(retiredAutomationSwitches, key) {
					wantEnabled = false
				}
```

Keep the separate `TestMergeConfigWithDefaultsDisablesRetiredHeFengTasks` coverage for persisted retired switches.

- [ ] **Step 6: Run the complete automation package tests**

Run:

```powershell
gofmt -w internal/farm/automation/catalog.go internal/farm/automation/catalog_test.go
go test ./internal/farm/automation -count=1
```

Expected: PASS with the existing configuration, state conversion, scheduler, and runtime tests unchanged except for the intended public-state contract.

- [ ] **Step 7: Commit the scheduler fix**

```powershell
git add -- internal/farm/automation/catalog.go internal/farm/automation/catalog_test.go
git commit -m "fix: hide retired scheduler tasks"
```

### Task 2: Run cross-layer regression verification

**Files:**
- Verify only; no planned modifications

- [ ] **Step 1: Run all Go tests**

```powershell
go test ./... -count=1
```

Expected: all Go packages report `ok` or `[no test files]` with no failures.

- [ ] **Step 2: Run frontend tests**

Run from `frontend`:

```powershell
npm test
```

Expected: Vitest reports all test files and tests passing.

- [ ] **Step 3: Build the frontend**

Run from `frontend`:

```powershell
npm run build
```

Expected: TypeScript and Vite complete successfully.

- [ ] **Step 4: Verify scope and repository state**

```powershell
git show --stat --oneline HEAD
git diff --check HEAD^ HEAD
git status --short
```

Expected: the implementation commit contains only `catalog.go` and `catalog_test.go`, `git diff --check` is silent, and the working tree is clean apart from any explicitly preserved user changes.
