# Fertilizer Linked Harvest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement unified "催熟联动收获" behavior for automatic fertilizer and manual land rush, including post-harvest dead-crop protocol cleanup.

**Architecture:** Reuse the runtime JavaScript `fertilizeLandsBatch` internal linked-harvest path so pre-fertilizer land IDs are harvested immediately after fertilizer protocol dispatch. Add Go helpers for post-harvest status rescans and dead-crop cleanup, and add a JavaScript `onlyDead` guard to `shovelLandsBatch`.

**Tech Stack:** Go 1.25, Wails v2, React, TypeScript, Vitest, Cocos runtime JavaScript.

---

## File Structure

- Modify `internal/farm/automation/runtime_own_test.go`: add red tests for automatic linked harvest and dead cleanup.
- Modify `internal/farm/automation/runtime_helpers.go`: add dead-crop extraction helper and config boolean helper usage if needed.
- Modify `internal/farm/automation/runtime_own.go`: wire linked harvest config and post-harvest cleanup.
- Modify `app_test.go`: add App/manual land rush tests for cleanup after successful rush.
- Modify `app.go`: add reusable manual cleanup helpers and call them from `FarmLandRush`.
- Modify `resources/wmpf/button.js`: add `onlyDead` safety to `shovelLandsBatch`.
- Modify `frontend/src/views/AutomationView.tsx`: rename automation switch text.
- Modify `frontend/src/views/AssetsLandView.tsx`: rename manual rush checkbox and submit button text.
- Modify `frontend/src/views/AutomationView.test.tsx` and `frontend/src/views/AssetsLandView.test.tsx`: update text expectations.

## Tasks

### Task 1: Automatic Fertilizer Red Tests

- [ ] Add tests showing `own_fertilizer` passes `linkedHarvestAfterFertilize: true` when `autoFarmFertilizerHarvestLinkEnabled` is true, rescans after successful fertilizer, and calls `gameCtl.shovelLandsBatch` only with dead land IDs.
- [ ] Add a helper test for dead-crop extraction accepting `stageKind`, `isDead`, `canEraseDead`, `needsEraseDead`, and `needEraseDead`.
- [ ] Run `go test ./internal/farm/automation -run "OwnFertilizer|Dead" -count=1` and verify the new tests fail because the behavior is missing.

### Task 2: Automatic Fertilizer Implementation

- [ ] Implement `collectDeadLandIDs(status map[string]any) []int`.
- [ ] Make `runOwnFertilizer` read `autoFarmFertilizerHarvestLinkEnabled`, pass it into `fertilizeLandsBatch`, rescan after success, and call `gameCtl.shovelLandsBatch` with `onlyDead: true` when dead IDs exist.
- [ ] Run `go test ./internal/farm/automation -run "OwnFertilizer|Dead" -count=1` and verify the tests pass.

### Task 3: Manual Land Rush Red Tests

- [ ] Add App-level tests showing `FarmLandRush` calls `fertilizeLandsBatch`, then rescans status, then calls `shovelLandsBatch` with dead IDs and `onlyDead: true`.
- [ ] Add a test showing no shovel call is made when the rescan finds no dead crops.
- [ ] Run `go test . -run "FarmLandRush|Dead" -count=1` and verify the new tests fail because the cleanup hook is missing.

### Task 4: Manual Land Rush Implementation

- [ ] Add package-main helpers to rescan farm status, extract dead IDs, and call protocol shovel cleanup without hiding the original fertilizer result.
- [ ] Update `FarmLandRush` to invoke the helper after successful fertilizer.
- [ ] Run `go test . -run "FarmLandRush|Dead" -count=1` and verify the tests pass.

### Task 5: Runtime Shovel Safety

- [ ] Add a testable `onlyDead` branch in `resources/wmpf/button.js` inside `shovelLandsBatch`: when set, non-dead target states get `skipReason: "not_dead"` before dispatch.
- [ ] Verify the branch by static grep and by the Go tests' `onlyDead` argument assertions.

### Task 6: Frontend Text

- [ ] Update automation and land rush display text to "催熟联动收获".
- [ ] Update frontend tests to expect the new text.
- [ ] Run `cd frontend; npm test -- AutomationView.test.tsx AssetsLandView.test.tsx -- --runInBand` or the nearest supported Vitest command.

### Task 7: Full Verification

- [ ] Run `go test ./internal/farm/automation -count=1`.
- [ ] Run `go test ./... -count=1`.
- [ ] Run `cd frontend; npm test`.
- [ ] Run `cd frontend; npm run build`.
- [ ] Run `git diff --check`.

## Self-Review

- Spec coverage: Automatic task, manual rush, shared ID linked harvest, post-harvest dead scan, shovel protocol cleanup, and text rename are all represented.
- Placeholder scan: No task relies on a vague placeholder; each task names the files and verification commands.
- Type consistency: Runtime method names match existing code: `gameCtl.fertilizeLandsBatch`, `gameCtl.getFarmStatus`, `gameCtl.shovelLandsBatch`.
