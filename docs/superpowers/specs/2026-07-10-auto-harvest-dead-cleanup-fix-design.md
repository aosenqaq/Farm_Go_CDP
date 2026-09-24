# Auto Harvest Dead Crop Cleanup Fix Design

## Context

The runtime event log shows the broken sequence clearly:

- `own_base` harvested 24 lands even though one-click farming should only handle farm care.
- The next `own_collect` run reported `清理枯萎 24 块`, but the crops remained.

The cleanup count currently reports requested dead-land IDs, not confirmed shovel successes. The JavaScript runtime also detects dead crops with `getFarmStatus` using `workSummary.sets.eraseDead`, then validates the same lands inside `shovelLandsBatch` without those action sets. A land can therefore be detected as dead and immediately skipped as `not_dead`.

## Responsibilities

### One-click farming (`own_base`)

`own_base` must not harvest crops. It only runs the existing farm-care operations: watering, removing grass, killing normal bugs, and cleaning golden bugs. Its result message must not contain a harvest count.

### Auto harvest (`own_collect`)

`own_collect` owns the complete harvest flow:

1. Read own-farm status.
2. Harvest detected mature lands.
3. Read farm status again after a successful harvest.
4. Detect dead crops from the refreshed status.
5. Shovel only confirmed dead crops.
6. Report confirmed harvest and shovel success counts.

A later `own_collect` run must also shovel dead crops even when there is nothing new to harvest.

## Runtime State Consistency

`shovelLandsBatch({ onlyDead: true })` will build the current farm work summary and pass the same `actionSets` into its grid-state reads that `getFarmStatus` uses. This keeps dead-crop detection and the runtime safety check consistent while retaining the protection against shoveling healthy crops.

Targets that became empty or healthy before dispatch remain safe no-op skips. A dispatched target with no observed shovel effect remains a failure. The payload continues to expose requested, runnable, skipped, success, and failure counts.

## Logging

The successful `own_collect` message will use this form:

`一键收获 X 块，清理枯萎 Y 块。`

`X` and `Y` are successful operation counts from runtime results, not merely requested land counts. The `own_base` log will describe care work only and will never report `一键务农收获` or any equivalent harvest wording.

The successful `own_base` message will use this form:

`一键务农已执行：照料 X 项，清理金虫 Y 处。`

## Error Handling

- Failure to read farm status, harvest, rescan, or shovel returns a failed automation result.
- Explicit runtime `ok: false` remains a failure and includes the most specific per-land reason available.
- Safe skips caused by state changing between scan and dispatch are not failures, but they are not counted as completed work.

## Tests

Go regression tests will verify:

- `own_base` never calls the harvest runtime method.
- `own_base` messages contain only care statistics.
- `own_collect` harvests, rescans, and shovels in order.
- A dead-only later round still calls shovel.
- The success message is exactly based on runtime harvest and shovel success counts.

JavaScript regression coverage will verify that an `eraseDead` action-set member passes `onlyDead` validation and dispatches `REQUEST_ERASE_PLANT`, even when the plant stage alone does not expose a reliable dead flag.

## Scope

No scheduler priority, interval, UI control, planting behavior, or non-dead shovel behavior changes are included.
