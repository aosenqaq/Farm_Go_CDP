# Auto Farm Runtime Facade Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Connect the automation scheduler manual-run path to Farm_Go's runtime supervisor through a small Go-native facade.

**Architecture:** Keep the automation catalog as the stable UI/config model. Add a focused runtime facade under `internal/farm/automation` that accepts a task ID and a runtime caller. In this phase only `own_base` is mapped to a safe runtime probe (`gameCtl.getFarmStatus`) so the UI can verify end-to-end runtime connectivity without claiming the full one-click farming executor has been migrated.

**Tech Stack:** Go 1.25, Wails v2, React, TypeScript, Vitest.

---

## Scope Check

This phase intentionally does not port the full JavaScript `auto-farm-executor.js` task logic. Complex task families such as planting, fertilizer, friend steal/help/mischief, reward claiming, and mystery shop buying remain `not_migrated` until each family gets its own runtime slice.

## File Structure

- Modify `internal/farm/automation/catalog.go`: add runtime-aware statuses and retain the pure catalog fallback.
- Create `internal/farm/automation/runtime_test.go`: tests for `own_base` runtime call mapping, runtime-not-ready errors, and unmigrated task behavior.
- Create `internal/farm/automation/runtime.go`: runtime caller interface, task mapping, result normalization, and timeout policy.
- Modify `app.go`: route `RunFarmAutomationTask` through the runtime facade using the existing supervisor.
- Modify `app_test.go`: assert App returns `runtime_not_ready` when no runtime is connected for a migrated facade task.
- Modify `frontend/src/views/AutomationView.tsx`: show a friendlier label for `runtime_not_ready` while still showing the raw status badge.
- Modify `frontend/src/views/AutomationView.test.tsx`: assert runtime-not-ready messages render without success language.

### Task 1: Runtime Facade

- [ ] Write failing Go tests in `internal/farm/automation/runtime_test.go`.
- [ ] Run `go test ./internal/farm/automation` and verify missing facade symbols fail.
- [ ] Implement `RuntimeCaller`, `RuntimeFacade`, `NewRuntimeFacade`, and `RunTask`.
- [ ] Run `go test ./internal/farm/automation` and verify pass.

### Task 2: App Integration

- [ ] Write failing App test for `RunFarmAutomationTask("own_base")` returning `runtime_not_ready` without an active runtime.
- [ ] Run `go test ./...` and verify failure.
- [ ] Update `App.RunFarmAutomationTask` to call the runtime facade with `a.supervisor`.
- [ ] Run `go test ./...` and verify pass.

### Task 3: UI Status Copy

- [ ] Write failing frontend test for `runtime_not_ready` status rendering.
- [ ] Run `npm test -- AutomationView.test.tsx` and verify failure.
- [ ] Add compact status copy for runtime-not-ready results.
- [ ] Run `npm test -- AutomationView.test.tsx` and verify pass.

### Task 4: Full Verification

- [ ] Run `go test -count=1 ./...`.
- [ ] Run `cd frontend && npm test`.
- [ ] Run `cd frontend && npm run build`.
- [ ] Run `git diff --check`.

## Self-Review

- Spec coverage: This plan covers runtime facade wiring and one safe mapped task, matching Phase 2's narrow goal.
- Placeholder scan: No task relies on undefined placeholder behavior; later executor migration remains explicitly out of scope.
- Type consistency: Status values use existing `ActionResult.status` and JSON field names already generated for Wails.
