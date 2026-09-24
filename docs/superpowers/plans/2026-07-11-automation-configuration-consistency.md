# Automation Configuration Consistency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every farm automation save use one canonical configuration across the frontend draft, storage, scheduler state, and runtime execution.

**Architecture:** The Go automation catalog remains authoritative for scheduler-to-config mappings and exposes those keys on each scheduler task. The React editor uses the backend-provided keys and a synchronous draft ref before sending the whole canonical state. Runtime fertilizer execution reads the saved mode and threshold, while scheduler reconfiguration resets ordinary interval deadlines when effective task settings change.

**Tech Stack:** Go, SQLite, React 18, TypeScript, Vitest, Wails v2

---

### Task 1: Runtime fertilizer configuration

**Files:**
- Modify: `internal/farm/automation/runtime_own_test.go`
- Modify: `internal/farm/automation/runtime_own.go`

- [ ] **Step 1: Write failing runtime tests**

Add tests that construct `NewRuntimeFacadeWithConfig` with `autoFarmRushFertilizerMode: "none"` and assert no runtime call occurs, then table-test `normal` and `organic` with a custom threshold and assert the batch payload contains the selected mode and threshold.

- [ ] **Step 2: Verify the tests fail**

Run: `go test -count=1 ./internal/farm/automation -run "TestRuntimeFacadeOwnFertilizer(DisabledRushModeSkips|UsesConfiguredModeAndThreshold)$"`

Expected: FAIL because `none` still reads farm status and configured modes still produce hard-coded `organic` and `300`.

- [ ] **Step 3: Implement configured fertilizer behavior**

Read and normalize `autoFarmRushFertilizerMode`; return a successful skip before any runtime call when it is `none`. Read a positive `autoFarmFertilizerRushThresholdSec`, falling back to `300`, and pass the selected `normal` or `organic` mode into `fertilizeLandsBatch`.

- [ ] **Step 4: Verify runtime tests pass**

Run: `go test -count=1 ./internal/farm/automation -run "TestRuntimeFacadeOwnFertilizer"`

Expected: PASS.

### Task 2: Scheduler hot-reconfiguration

**Files:**
- Modify: `internal/farm/automation/scheduler_test.go`
- Modify: `internal/farm/automation/scheduler.go`

- [ ] **Step 1: Write failing scheduler tests**

Add tests that run `own_collect`, change its interval, and assert `NextRunAt` becomes `configureTime + newInterval`; also configure a disabled task as enabled and assert it becomes immediately due.

- [ ] **Step 2: Verify the tests fail**

Run: `go test -count=1 ./internal/farm/automation -run "TestSchedulerConfigure(ReschedulesChangedInterval|MakesNewlyEnabledTaskDue)$"`

Expected: FAIL because `Configure` retains the previous runtime deadline.

- [ ] **Step 3: Reconcile runtime deadlines during Configure**

Compare each new task with `previousSettings`. Preserve runtime history, but set `NextRunAt` to `now + new interval` when an enabled ordinary task's interval changes, set it to `now` when newly enabled, and retain existing daily-time reward behavior.

- [ ] **Step 4: Verify scheduler tests pass**

Run: `go test -count=1 ./internal/farm/automation -run "TestSchedulerConfigure"`

Expected: PASS.

### Task 3: One scheduler/config mapping contract

**Files:**
- Modify: `internal/farm/automation/catalog.go`
- Modify: `internal/farm/automation/catalog_test.go`
- Modify: `frontend/src/views/AutomationView.tsx`
- Modify: `frontend/src/views/AutomationView.test.tsx`

- [ ] **Step 1: Write failing catalog and frontend tests**

Add a table-driven Go test over every scheduler task definition that asserts `StateFromSettings` exposes its enable and interval config keys and that `SettingsFromState` round-trips changed task values into both representations. Add frontend tests where backend-provided config keys intentionally differ from the compatibility maps and assert synchronization follows the backend keys.

- [ ] **Step 2: Verify mapping tests fail**

Run: `go test -count=1 ./internal/farm/automation -run "TestSchedulerTaskExposesCanonicalConfigKeys"`

Run: `npm test -- src/views/AutomationView.test.tsx`

Expected: Go FAIL because task DTOs do not expose config keys; frontend FAIL because synchronization uses only hard-coded maps.

- [ ] **Step 3: Expose and consume canonical mapping keys**

Add optional JSON fields to `SchedulerTask`:

```go
EnabledConfigKey  string `json:"enabledConfigKey,omitempty"`
IntervalConfigKey string `json:"intervalConfigKey,omitempty"`
```

Populate them from the Go maps. Extend the TypeScript task type and make update/synchronization helpers prefer task-provided keys while retaining current maps as fallback for old payloads and test fixtures.

- [ ] **Step 4: Verify mapping tests pass**

Run the same Go and frontend commands from Step 2.

Expected: PASS.

### Task 4: Prevent stale frontend save snapshots and prove full round trip

**Files:**
- Modify: `frontend/src/views/AutomationView.tsx`
- Modify: `frontend/src/views/AutomationView.test.tsx`
- Modify: `app_test.go`
- Modify: `app.go`

- [ ] **Step 1: Write failing save-chain tests**

Add a frontend helper test proving a config update immediately changes the state returned to the save callback even before a render cycle. Add an App test that changes representative boolean, number, string, list, every task enable/interval, and the automatic harvest interval, saves once, then verifies returned state, storage, and active scheduler all contain the canonical values.

- [ ] **Step 2: Verify save-chain tests fail**

Run: `npm test -- src/views/AutomationView.test.tsx`

Run: `go test -count=1 . -run "TestSaveFarmAutomationStateCanonicalRoundTrip"`

Expected: frontend FAIL until draft updates are centralized through a synchronous ref; backend test may expose any remaining mapping mismatch.

- [ ] **Step 3: Centralize draft updates and enrich save evidence**

Introduce one local `replaceDraftState` function that writes both `draftStateRef.current` and React state. Route config, scheduler, and task edits through it; save `normalizeAutomationState(draftStateRef.current)`. Include the saved automatic harvest and fertilizer values in the existing structured save event so future incidents identify the failing boundary without logging unrelated configuration.

- [ ] **Step 4: Verify save-chain tests pass**

Run the same frontend and App test commands from Step 2.

Expected: PASS.

### Task 5: Full verification

**Files:**
- Verify only

- [ ] **Step 1: Format modified Go files**

Run: `gofmt -w app.go app_test.go internal/farm/automation/catalog.go internal/farm/automation/catalog_test.go internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go internal/farm/automation/scheduler.go internal/farm/automation/scheduler_test.go`

- [ ] **Step 2: Run all backend tests**

Run: `go test -count=1 ./...`

Expected: PASS.

- [ ] **Step 3: Run all frontend tests and build**

Run: `npm test`

Run: `npm run build`

Working directory: `frontend`

Expected: PASS.

- [ ] **Step 4: Inspect the final diff**

Run: `git diff --check`

Run: `git status --short`

Expected: no whitespace errors; only planned source/tests/docs plus the pre-existing Vite log modification appear.
