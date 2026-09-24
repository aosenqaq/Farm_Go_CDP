# Automation Configuration Consistency Design

## Goal

Make every setting exposed by the farm automation page persist consistently. A successful save must return the canonical state that the database, scheduler center, and runtime receive.

## Scope

- All settings edited in `AutomationView`, including feature switches, scheduler task switches, priorities, intervals, and detailed runtime options.
- Account-scoped persistence through `SaveFarmAutomationState`.
- Hot reconfiguration of the active scheduler.
- Runtime consumption of the fertilizer and rush settings proven to be ignored in this incident.
- System settings, message push, social rule storage, warehouse settings, and other independently persisted modules remain out of scope.

## Chosen Approach

The backend becomes the authority for automation state normalization. `SettingsFromState` converts the submitted state into one canonical `Settings` value, synchronizes every scheduler-bound enable/interval key with its task, persists that exact value, configures the active scheduler with it, and returns `StateFromSettings(settings)`.

The frontend keeps its immediate draft synchronization for usability, but correctness no longer depends on it. After save, the returned backend state replaces the draft and parent state.

This is preferred over a frontend-only patch because saves may come from generated bindings, future clients, or stale UI snapshots. A fully typed replacement for the existing configuration map would remove more ambiguity, but it is too broad for this bug fix.

## Behavior

### Canonical save

- Every scheduler task enable flag and interval has one canonical value during conversion.
- The canonical value is written to both the task record and its compatibility config key.
- A save/load round trip preserves every editable configuration value.
- Saving one account cannot reconfigure or overwrite another account.

### Runtime settings

- `autoFarmRushFertilizerMode = none` performs no fertilizer batch call.
- `normal` and `organic` select the corresponding runtime fertilizer type and mode.
- `autoFarmFertilizerRushThresholdSec` controls target selection and the runtime payload.
- Existing linked-harvest behavior remains controlled by `autoFarmFertilizerHarvestLinkEnabled`.
- Other automation algorithms are unchanged; their editable values are still covered by the canonical save/load contract.

### Scheduler hot reload

- If a normal interval task changes interval while idle, its `NextRunAt` is recalculated from the configuration time using the new interval.
- Enabling a task makes it immediately eligible.
- Disabling a task prevents future starts; an already running invocation is allowed to finish.
- Daily-time reward scheduling retains its existing specified-time calculation.

## Validation

- Table-driven backend tests cover every entry in the scheduler enable and interval mappings.
- A table-driven round-trip test changes every editable `AutomationView` config key away from its default and proves it survives state conversion and storage.
- A save/load/reconfigure test changes automatic harvest and verifies config, persisted task, returned state, and active scheduler state.
- Runtime tests prove `none` skips and both fertilizer modes and custom thresholds reach the runtime payload.
- Scheduler tests prove interval changes update `NextRunAt` and enabled-state changes take effect.
- Frontend interaction tests cover changing a detailed interval and saving the resulting canonical state.
- Full Go and frontend test suites must pass.

## Non-Goals

- Migrating `map[string]any` to a new typed configuration schema.
- Changing unrelated automation algorithms or UI layout.
- Rewriting settings storage outside farm automation.
