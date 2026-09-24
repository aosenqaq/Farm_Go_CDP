# Auto Farm Own Collect Phase 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the first real automation action slice: `own_collect` manually harvests mature own-farm lands through the runtime facade.

**Architecture:** Reuse `RuntimeFacade.RunTask` as the scheduler/manual-run entry. `own_collect` reads `gameCtl.getFarmStatus`, extracts unique own-farm harvest land IDs from `workLandIds.collect` and grid harvest flags, then calls `gameCtl.harvestLandsBatchByProtocol`. If no harvestable lands exist, it returns an OK skipped result; if runtime calls fail, it returns `runtime_not_ready` or `failed`.

**Tech Stack:** Go 1.25, Wails v2, React, TypeScript, Vitest.

---

## Scope Check

This phase only migrates the simple own-farm harvest action. It does not migrate multi-tile recovery scans, dead-crop cleanup, planting-after-harvest, fertilizer-linked harvest, friend steal, or today statistics accounting.

## File Structure

- Modify `internal/farm/automation/runtime_test.go`: add tests for harvest land extraction, no-op skip, runtime error, and App integration.
- Modify `internal/farm/automation/runtime.go`: add `own_collect` dispatch and helper functions.
- Modify `app_test.go`: assert `RunFarmAutomationTask("own_collect")` uses fake runtime methods end-to-end.

## Tasks

### Task 1: Runtime Facade Own Collect

- [ ] Write failing tests for `own_collect`.
- [ ] Run `go test ./internal/farm/automation` and verify failure.
- [ ] Implement `runOwnCollect`, `collectHarvestLandIDs`, and runtime result normalization.
- [ ] Run `go test ./internal/farm/automation` and verify pass.

### Task 2: App Integration

- [ ] Write failing App test with fake runtime responses for `gameCtl.getFarmStatus` and `gameCtl.harvestLandsBatchByProtocol`.
- [ ] Run `go test ./...` and verify failure.
- [ ] Confirm existing App facade path passes the test without additional broad rewiring.
- [ ] Run `go test ./...` and verify pass.

### Task 3: Full Verification

- [ ] Run `go test -count=1 ./...`.
- [ ] Run `cd frontend && npm test`.
- [ ] Run `cd frontend && npm run build`.
- [ ] Run `git diff --check`.

## Self-Review

- Spec coverage: Covers only `own_collect` manual run behavior.
- Placeholder scan: Later automation slices are explicitly out of scope, not left as ambiguous TODOs.
- Type consistency: Reuses `ActionResult` fields already exposed through Wails.
