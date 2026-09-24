# Remove Golden-Bug Direct Protocols

## Status

Approved on 2026-07-23.

## Goal

Remove all application support for directly placing golden bugs on a friend's
land or directly clearing golden bugs one land at a time. Keep golden-bug
detection and the land-details card indicator. `own_base` must continue to
handle golden bugs through the existing one-click `FARM_WORK` operation.

## Scope

### Retained

- Runtime land-state detection: `hasGoldenBug`, `needGoldenBug`,
  `needsGoldenBug`, and the `socialItemIds` fallback for item `301101`.
- The `LandDetailsItem.NeedGoldenBug` backend field, generated Wails model,
  and the gold-bug state tags in `AssetsLandView`.
- `own_base` and its generic `gameCtl.triggerOneClickOperation("FARM_WORK")`
  invocation. Golden-bug-only land states count as pending one-click care so
  this invocation is not skipped.
- The golden-bug item data in `resources/gameConfig/ItemInfo.json` and all
  resource bundles containing it.

### Removed

- The friend-placement protocol request builder and
  `putFriendGoldenBugsByProtocol` runtime API.
- The per-land clean request builder, golden-bug land ID collector,
  `cleanGoldenBugsByProtocol` runtime API, and both public `gameCtl` exports.
- The `own_base` direct protocol call, its special error paths, and its
  separate golden-bug result count.
- The runtime-cache special case for method names containing `goldenbug`.
- The unused `autoFarmFriendGoldenBug*` default configuration keys.

Historical design documents and user-owned uncommitted files are not modified.
Previously saved unknown configuration values are not rewritten; without a
default or consumer they have no supported behavior.

## Runtime Flow

`getGridState` continues to detect golden bugs and `getFarmStatus` continues
to return that state. The backend and frontend preserve the existing land card
display path.

For automation, `collectCareWorkCount` treats a detected golden bug as pending
care. `runOwnBase` then invokes only the generic one-click `FARM_WORK`
operation. It no longer derives land IDs or dispatches `CleanSocialItems`.
Its successful result reports only the total one-click care count.

## Error Handling

The existing ownership, status-read, and one-click-operation errors remain
unchanged. Removing the direct protocol removes only its dedicated error
messages and failure mode.

## Tests

1. Replace the direct-clean test with a test that starts from a
   golden-bug-only status. It must prove `own_base` calls `FARM_WORK` and never
   calls a dedicated golden-bug protocol method.
2. Add a resource-script regression test that first fails while the direct
   protocol APIs and exports exist, then verifies their removal while golden
   bug state fields remain available.
3. Keep the existing land-details UI test that verifies the `金虫` indicator.
4. Run Go tests, frontend tests, and the frontend production build. Finally,
   search source files for removed protocol names, excluding retained item
   configuration and historical documentation.

## Acceptance Criteria

- No runtime API can place golden bugs on friends or clean a specific land.
- No backend automation code calls a golden-bug-specific protocol method.
- A golden-bug-only farm still reaches the generic one-click care action.
- Land details still expose and display the golden-bug indicator.
- The golden-bug item data is unchanged.
