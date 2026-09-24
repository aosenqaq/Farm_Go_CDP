# Fertilizer Linked Harvest Design

## Goal

Rename the fertilizer harvest-link switch to "催熟联动收获" and make both automatic fertilizer tasks and the manual land rush flow use the same optimized linked-harvest semantics.

## Scope

This change covers two entry points:

- Automatic fertilizer task in the automation settings.
- Manual land rush drawer on the land details page.

Both flows should share the same behavior: the land IDs selected before fertilizer dispatch are reused by linked harvest immediately after the fertilizer protocol dispatch. After that harvest step, the app rescans farm status and only sends truly dead crop IDs to the protocol shovel operation.

## Current State

The runtime already has fast protocol primitives:

- `gameCtl.fertilizeLandsBatch(opts)` accepts `linkedHarvestAfterFertilize`.
- `fertilizeLandsBatch` calls `harvestLandsBatchByProtocol(requestedLandIds, ...)` internally when linking is enabled.
- `gameCtl.harvestLandsBatchByProtocol(landIds, opts)` dispatches direct harvest protocol requests.
- `gameCtl.shovelLandsBatch(opts)` dispatches direct erase-plant protocol requests.

The Go automatic fertilizer task currently calls `fertilizeLandsBatch` with `linkedHarvestAfterFertilize: false`, so it does not use the fast shared-ID path. The manual land rush flow already sends `harvestLinkEnabled`, but it does not perform the post-harvest dead-crop scan and protocol shovel cleanup.

## User-Facing Text

Use "催熟联动收获" as the shared feature name.

- Automation fertilizer checkbox: change "收获后催熟联动" to "催熟联动收获".
- Manual land rush checkbox: change "催熟后自动收获" to "催熟联动收获".
- Manual land rush submit button: change "催熟并收获" to "催熟联动收获".
- The land rush entry/action title "一键催熟" can remain as the broader entry point name.

## Automatic Fertilizer Flow

`runOwnFertilizer` should:

1. Read own farm status with `gameCtl.getFarmStatus`.
2. Build rush targets with the existing candidate logic.
3. Read `autoFarmFertilizerHarvestLinkEnabled` from config.
4. Call `gameCtl.fertilizeLandsBatch` with the candidate land IDs and `linkedHarvestAfterFertilize` matching the switch.
5. If the fertilizer call fails or returns `ok: false`, stop with failure.
6. If the call succeeds, rescan status with `gameCtl.getFarmStatus`.
7. Extract dead crop land IDs from the rescan result.
8. If any dead crop IDs exist, call `gameCtl.shovelLandsBatch` with `onlyDead: true`.

The linked harvest must not wait for the scheduler to run `own_collect`. The shared IDs must be passed through the fertilizer runtime call so the runtime can harvest in the same JavaScript execution flow immediately after fertilizer protocol dispatch.

## Manual Land Rush Flow

`FarmLandRush` should:

1. Normalize the selected land IDs and fertilizer mode as it does now.
2. Pass `linkedHarvestAfterFertilize` according to `harvestLinkEnabled`.
3. Call `gameCtl.fertilizeLandsBatch`.
4. If the fertilizer call fails or returns `ok: false`, return the failure.
5. If the call succeeds, rescan status with `gameCtl.getFarmStatus`.
6. Extract dead crop land IDs from the rescan result.
7. If any dead crop IDs exist, call `gameCtl.shovelLandsBatch` with `onlyDead: true`.
8. Include cleanup details in the returned payload without hiding the original fertilizer result.

Manual and automatic flows should use the same helper functions for dead-crop extraction and shovel cleanup.

## Dead Crop Detection

Add a Go helper that extracts unique sorted dead crop land IDs from farm status. It should accept multiple runtime signals because status payloads may vary:

- `stageKind == "dead"`
- `isDead == true`
- `canEraseDead == true`
- `needsEraseDead == true`
- `needEraseDead == true`

The helper must only return positive land IDs from `landId` or `id`. It should not infer dead state from old cached target IDs. The second scan after harvest is the source of truth.

## Shovel Safety

Extend `gameCtl.shovelLandsBatch(opts)` to support `onlyDead: true`.

When `onlyDead` is set, the JavaScript runtime should skip any target whose current grid state is not dead. Skipped targets should appear in the payload with a clear reason such as `not_dead`. This protects against stale Go-side data and prevents healthy crops from being shoveled.

The Go cleanup calls should pass:

- `landIds`: dead IDs from the post-harvest scan.
- `onlyDead: true`
- `silent: true`
- `dryRun: false`
- low waits suitable for protocol cleanup.
- source strings that identify the entry point.

## Error Handling

Fertilizer failures remain primary failures.

Post-harvest cleanup failures should return a failed action result for automatic tasks because the requested flow includes cleanup. Manual `FarmLandRush` should return `ok: false` if shovel cleanup fails after successful fertilizer, and include the successful fertilizer payload plus cleanup error details so the UI can show what happened.

No dead crops after the second scan is a successful no-op cleanup.

## Tests

Go tests should cover:

- Automatic fertilizer passes `linkedHarvestAfterFertilize: false` when the switch is disabled.
- Automatic fertilizer passes `linkedHarvestAfterFertilize: true` when the switch is enabled.
- Automatic fertilizer rescans after successful linked fertilizer and shovels only dead IDs.
- Automatic fertilizer skips shovel when the rescan finds no dead crops.
- Manual `FarmLandRush` performs the same post-harvest scan and shovel cleanup.
- Dead-crop extraction accepts the supported runtime signals and sorts/deduplicates IDs.

Frontend tests should cover:

- Automation settings render "催熟联动收获".
- Manual land rush checkbox and submit button render "催熟联动收获".
- Old strings "收获后催熟联动", "催熟后自动收获", and "催熟并收获" are not expected in updated tests.

Runtime JavaScript tests should cover or be verified by Go/App tests where practical:

- `shovelLandsBatch({ onlyDead: true })` skips non-dead target states.
- Dead target states remain runnable.

## Non-Goals

- Do not make `own_collect` run concurrently with `own_fertilizer`.
- Do not replace the existing internal JavaScript linked-harvest path with a slower Go-side second harvest call.
- Do not shovel based on the original fertilizer target IDs alone.
- Do not redesign the automation scheduler lanes in this change.
