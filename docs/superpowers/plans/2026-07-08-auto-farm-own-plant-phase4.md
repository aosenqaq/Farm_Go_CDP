# 自动农场自动种植迁移 Phase 4

## Scope

This phase migrates the first runtime-backed `own_plant` slice. It keeps the UI and scheduler shell from Phase 1, the runtime facade from Phase 2, and follows the same incremental style as `own_collect`.

## Runtime Behavior

- `own_plant` calls `gameCtl.getFarmStatus` with grid data.
- The task only proceeds when `farmType` is absent or `own`; an explicit non-own farm is blocked.
- Empty lands are collected from `status.grids` when `stageKind == "empty"` and `interactable == true`.
- If no empty lands are found, the task returns an OK skip and does not call planting.
- If empty lands exist, the task calls `gameCtl.autoPlant` in `specified_seed` mode with seed `20002` as the temporary default.
- Runtime call errors, or an explicit runtime `ok: false` response, return `failed` or `runtime_not_ready` without pretending the feature succeeded.

## Deferred

- User-configurable planting strategy.
- Backpack-first and shop fallback resolution.
- Four-grid crop grouping.
- Randomized delay and richer progress telemetry.

## Verification

- Add red tests in `internal/farm/automation/runtime_test.go`.
- Add an App-level facade test in `app_test.go`.
- Run Go package tests, full Go tests, frontend tests, frontend build, and `git diff --check`.
