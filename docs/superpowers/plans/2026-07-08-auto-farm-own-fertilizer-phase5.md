# 自动农场自动施肥迁移 Phase 5

## Scope

This phase migrates a conservative runtime-backed `own_fertilizer` slice. It does not port the full reference auto-fertilizer engine yet.

## Runtime Behavior

- `own_fertilizer` calls `gameCtl.getFarmStatus` with grid data.
- The task only proceeds when `farmType` is absent or `own`; an explicit non-own farm is blocked.
- Candidate lands come from `status.grids`.
- A land is selected only when it is a growing planted land, not dead, has a positive `plantId`, and has `matureInSec` between 6 and 300 seconds.
- If no candidate lands are found, the task returns an OK skip and does not call fertilizer runtime actions.
- If candidates exist, the task calls `gameCtl.fertilizeLandsBatch` with organic fertilizer mode and harvest linking disabled for this slice.
- Runtime call errors, or an explicit runtime `ok: false` response, return `failed` or `runtime_not_ready` without claiming success.

## Deferred

- User-configured fertilizer mode.
- Smart phase/season marks.
- Fertilizer container fill and stock checks.
- Harvest-link and continuous rush loops.
- Land type filters, batch chunking, and no-effect backoff.

## Verification

- Add red tests in `internal/farm/automation/runtime_test.go`.
- Add an App-level facade test in `app_test.go`.
- Run Go package tests, full Go tests, frontend tests, frontend build, and `git diff --check`.
