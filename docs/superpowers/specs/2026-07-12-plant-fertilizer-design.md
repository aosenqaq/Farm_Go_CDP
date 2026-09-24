# Plant Fertilizer Design

## Goal

Make the plant fertilizer setting apply normal or organic fertilizer immediately after an automatic planting request succeeds.

## Scope

- `none` performs no plant fertilizer action.
- `normal` and `organic` call the existing `gameCtl.fertilizeLandsBatch` runtime method after the matching `gameCtl.autoPlant` request succeeds.
- The fertilizer call receives the same `emptyLandIds` supplied to that planting request. The implementation must not re-read farm status.
- Smart fertilizer options are removed from the UI. Stored legacy smart values are treated as disabled.
- Rush fertilizer behavior and its configuration remain unchanged.

## Design

`runOwnPlant` already resolves one or more planting plans and invokes `gameCtl.autoPlant` once per plan. After a successful call, it will invoke a focused helper that normalizes `autoFarmPlantFertilizerMode`, extracts that plan's `emptyLandIds`, and submits a batch fertilizer request when the mode is `normal` or `organic`.

The helper uses the plan IDs directly so each planting batch stays paired with its follow-up fertilizer batch. A planting or fertilizer runtime failure stops the task and returns an actionable failure result.

## Tests

- Normal and organic modes call fertilizer after successful planting with the plan land IDs and selected type.
- `none` does not call fertilizer.
- A failed planting request does not call fertilizer.
- The UI no longer renders smart fertilizer options.
