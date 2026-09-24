# Multi-Season Fertilizer Design

## Goal

Make the existing `多季节作物补肥` switch execute real work. When a
successfully harvested crop advances into a later season, Farm_Go applies the
configured planting fertilizer once during that existing post-harvest flow.
There is no persistent per-land or per-season marker.

## Reference Behavior

The reference protocol project performs a harvest, reads the resulting land
state, and treats a target that remains `growing` as a multi-season crop that
advanced to its next season. It immediately reuses the configured fertilizer
mode for those targets. It does not scan the entire farm on later scheduler
rounds and does not persist fertilizer markers.

Farm_Go keeps that event-driven model, but only trusts land IDs whose protocol
result explicitly has `ok: true`.

## Eligible Targets

A post-harvest target is eligible only when all of these conditions hold:

1. Its harvest protocol result is successful and includes a positive land ID.
2. The existing post-harvest farm-status read still describes that target as a
   planted, growing crop.
3. The grid reports `isMultiSeason: true`, `totalSeason > 1`, and
   `currentSeason > 1`.
4. For a multi-tile crop, all occupied grid IDs resolve to one anchor land ID.

The action uses the existing planting fertilizer setting:
`autoFarmPlantFertilizerMode`. `normal` selects ordinary/inorganic fertilizer,
`organic` selects organic fertilizer, and `none` skips. The existing fertilizer
master enablement rule remains in force. The multi-season switch must also be
enabled.

No result IDs means no multi-season fertilizer attempt. Requested harvest IDs
are never used as a substitute, so partial or ambiguous harvest responses do
not fertilize an unrelated or unharvested crop.

## Shared Post-Harvest Contract

The feature extends the status read already required after harvest; it does not
add a new farm scan.

| Flow | Existing post-harvest read | Added use of that same state |
| --- | --- | --- |
| `own_collect` | The settled read and conditional retry used to find newly withered crops | Select advanced multi-season crops from successful harvest results |
| Automated `own_fertilizer` with linked harvest | Its current linked-harvest status read | Select advanced multi-season crops from `linkedHarvest.results` |
| Manual `FarmLandRush` with linked harvest | Its current dead-crop cleanup status read | Select advanced multi-season crops from `linkedHarvest.results` |

For ordinary automatic collection, the existing settle and retry timing remains
unchanged. The final status returned by that established scan is parsed for both
dead crops and multi-season continuations. For the two linked-harvest flows,
the existing single status read is reused as-is.

## Execution And Failure Semantics

After one shared state read:

1. Derive both the dead-crop IDs and multi-season candidate anchors from the
   same state result.
2. Execute the existing dead-crop shovel logic first for all dead targets. If
   it fails, stop the post-harvest sequence and report that failure; do not
   dispatch multi-season fertilizer in that round.
3. Only after dead-crop cleanup succeeds, dispatch
   `gameCtl.fertilizeLandsBatch` for the multi-season anchors with the planting
   fertilizer mode, `dryRun: false`, and `linkedHarvestAfterFertilize: false`.

The multi-season action must not enable another linked harvest, preventing
recursive harvest/fertilize cycles. A failed multi-season fertilizer operation
happens only after cleanup and is reported explicitly rather than presenting a
false success.

Because selection is scoped to a concrete successful harvest event, there is no
periodic multi-season scan and no durable marker to clear after replanting or
season changes.

## Implementation Boundaries

- `internal/farm/automation/runtime_helpers.go`: pure helpers for extracting
  successful protocol land IDs and selecting multi-season anchor targets from a
  farm-status payload.
- `internal/farm/automation/runtime_own.go`: reuse current post-harvest reads
  in `own_collect` and linked `own_fertilizer`; dispatch the planting-mode
  fertilizer and preserve shovel cleanup.
- `app.go`: extend manual `FarmLandRush` linked-harvest cleanup to parse its
  existing status read and run the same configuration-gated follow-up.
- Existing frontend settings remain unchanged; the checkbox and planting
  fertilizer selector already persist the required configuration.

## Verification

Focused tests must prove:

1. A successful collection advancing a multi-season crop dispatches exactly one
   planting-mode fertilizer call without an additional status read.
2. Disabled switch, disabled planting fertilizer mode, non-multi-season crops,
   first-season crops, and missing/failed harvest result IDs do not dispatch
   fertilizer.
3. Four-grid crops dispatch only their anchor ID.
4. A shared read containing both a continuation and a dead crop dispatches the
   shovel call before fertilizer; a shovel failure prevents the fertilizer call.
5. Both automated and manual linked-harvest paths reuse their existing scan and
   use only successful `linkedHarvest.results` IDs.
6. Existing ordinary harvest, linked harvest, and fertilizer tests remain
   passing, followed by the complete Go and frontend suites.
