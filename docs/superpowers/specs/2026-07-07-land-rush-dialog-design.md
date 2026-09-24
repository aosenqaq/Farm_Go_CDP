# Land Rush Dialog Design

## Goal

Optimize the land details page by removing redundant top-level land actions and replacing the old rush action with a reference-project-style one-click ripening flow.

## User-Facing Behavior

- The land details top action area shows only `一键催熟`.
- `刷新土地`, `升级土地`, and top-level `铲除` are removed from this area.
- The land cards continue to show their per-land actions, including the existing per-card shovel control.
- Clicking `一键催熟` opens a right-side dialog.
- The dialog resets on open to automatic selection, threshold `300`, organic fertilizer, and linked harvest enabled.
- The user can switch between automatic filtering and manual land selection.
- The user can choose `无机` or `有机` ripening fertilizer.
- The user can enable or disable `催熟后自动收获`.
- Manual mode lists currently eligible lands, supports toggling each land, and supports selecting all eligible lands.

## Eligibility Rules

A land is eligible when:

- It has a positive `landId` or `id`.
- `matureInSec` is finite, greater than `5`, and less than or equal to the selected threshold.
- It is not `mature`, `dead`, or `empty`.
- It has `displayPlantName`, `plantName`, or `seedId`.

These rules mirror the reference project dialog behavior.

## Runtime Flow

The frontend submits:

```json
{
  "landIds": [1, 2],
  "rushThresholdSec": 300,
  "harvestLinkEnabled": true,
  "continuousRushEnabled": false,
  "fertilizerMode": "organic"
}
```

The Wails app exposes a `FarmLandRush` method. Because Farm_Go currently uses injected `gameCtl` calls rather than the reference project's Node `autoFarmManager.runLandRushOnce`, the backend bridges the request through current runtime APIs:

- Validate the selected land IDs and fertilizer mode.
- Resolve the mode to `normal` or `organic`.
- Call `gameCtl.fertilizeLandsBatch` with the selected land IDs, selected fertilizer mode, `dryRun: false`, `cleanupUi: true`, and `linkedHarvestAfterFertilize` mapped from `harvestLinkEnabled`.
- Refresh land details after success.

## Error Handling

- The submit button is disabled while busy, when runtime is disabled, or when there are no selected eligible lands.
- Runtime errors are shown in the land panel error area.
- If a background refresh fails, existing land cards remain visible.

## Testing

- Frontend render tests cover action removal, dialog controls, automatic filtering copy, manual selectable land rows, and absence of removed top-level actions.
- Go tests cover payload validation and the runtime call arguments passed to `gameCtl.fertilizeLandsBatch`.
- Existing land polling and card rendering tests must continue to pass.
