# Plant Fertilizer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply normal or organic fertilizer immediately after each successful automatic planting request, using that request's target land IDs.

**Architecture:** Keep plant fertilizer in `runOwnPlant`, immediately after `gameCtl.autoPlant`. A small helper maps only `normal` and `organic` configuration values to a fertilizer type and skips all other values. The existing rush-fertilizer flow remains isolated in `runOwnFertilizer`.

**Tech Stack:** Go runtime facade and Go tests; React/TypeScript automation settings view and Vitest.

---

### Task 1: Define Plant-Fertilizer Runtime Behavior

**Files:**
- Modify: `internal/farm/automation/runtime_own_test.go`
- Modify: `internal/farm/automation/runtime_own.go`

- [ ] **Step 1: Write failing runtime tests**

Add a table-driven test after the existing own-plant tests. For each `normal` and `organic` mode, configure an own-farm status with empty land IDs `1` and `4`, successful `gameCtl.autoPlant`, and successful `gameCtl.fertilizeLandsBatch` responses. Assert the calls are `getFarmOwnership`, `getFarmStatus`, `autoPlant`, then `fertilizeLandsBatch`; assert the fertilizer payload uses `landIds: []int{1, 4}`, the selected `type` and `mode`, `dryRun: false`, `cleanupUi: true`, `linkedHarvestAfterFertilize: false`, `silent: true`, and source `farm_go_auto_plant_fertilizer`.

Add one test for `none` and one for `smart_normal`; both must make no fertilizer call after successful planting. Add one test with `autoPlant` returning `{ok: false}` and assert fertilizer is not called.

- [ ] **Step 2: Run the new tests and verify they fail**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnPlant.*PlantFertilizer' -count=1`

Expected: FAIL because `runOwnPlant` does not call `gameCtl.fertilizeLandsBatch`.

- [ ] **Step 3: Implement the minimal runtime helper and call**

Add `plantFertilizerMode(config map[string]any) string` in `internal/farm/automation/runtime_own.go`. It returns only `normal` or `organic`, otherwise an empty string.

After each successful `gameCtl.autoPlant` result in `runOwnPlant`, read `emptyLandIds` from `plantOpts`. When the helper returns a supported fertilizer type and the ID list is non-empty, call the existing `gameCtl.fertilizeLandsBatch` with those IDs, `dryRun: false`, `cleanupUi: true`, `linkedHarvestAfterFertilize: false`, `silent: true`, and source `farm_go_auto_plant_fertilizer`. Return a failed `ActionResult` when the call errors or returns `ok: false`. Do not request farm status again.

- [ ] **Step 4: Run the runtime tests and verify they pass**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnPlant.*PlantFertilizer' -count=1`

Expected: PASS.

### Task 2: Remove Deferred Smart Modes From Settings

**Files:**
- Modify: `frontend/src/views/AutomationView.test.tsx`
- Modify: `frontend/src/views/AutomationView.tsx`

- [ ] **Step 1: Write a failing UI assertion**

In `renders editable fertilizer and rush detailed settings`, add assertions that the markup excludes `智能普通化肥` and `智能有机化肥`.

- [ ] **Step 2: Run the focused UI test and verify it fails**

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: FAIL because both smart option labels are currently rendered.

- [ ] **Step 3: Remove only the two smart options**

Delete the `smart_normal` and `smart_organic` entries from `plantFertilizerOptions` in `frontend/src/views/AutomationView.tsx`. Keep `none`, `normal`, and `organic` unchanged.

- [ ] **Step 4: Run the focused UI test and verify it passes**

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: PASS.

### Task 3: Verify the Scoped Change

**Files:**
- Verify: `internal/farm/automation/runtime_own.go`
- Verify: `internal/farm/automation/runtime_own_test.go`
- Verify: `frontend/src/views/AutomationView.tsx`
- Verify: `frontend/src/views/AutomationView.test.tsx`

- [ ] **Step 1: Format Go files**

Run: `gofmt -w internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go`

- [ ] **Step 2: Run affected backend and frontend suites**

Run: `go test ./internal/farm/automation -count=1`

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: both commands PASS.

- [ ] **Step 3: Build the frontend and run the complete Go suite**

Run: `npm run build`

Run: `go test ./...`

Expected: both commands PASS.

- [ ] **Step 4: Inspect the final diff and commit**

Run: `git diff --check` and `git status --short`.

Then commit the planned source, test, and plan files with message `feat: apply fertilizer after auto planting`.
