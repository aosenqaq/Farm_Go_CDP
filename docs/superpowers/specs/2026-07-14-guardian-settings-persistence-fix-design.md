# Guardian Settings Persistence Fix Design

## Goal

Ensure every parameter shown in the three guardian service cards displays its persisted value after saving, refreshing, and reopening the application.

## Root Cause

`SaveGuardianSettings` already accepts and persists all guardian fields. The guardian settings store also round-trips those values correctly. However, `GuardianStatus` only exposes the process timeout threshold through its runtime status. `GuardView` therefore renders hard-coded defaults for the process monitor interval, network reconnect interval, network recovery timeout, other-place-login interval, and reconnect delay. After a successful save and status refresh, those inputs display the defaults again and appear not to have been saved.

## Scope

- Return the complete normalized guardian settings snapshot from `GuardianStatus`.
- Render all three cards from that snapshot instead of hard-coded numeric defaults.
- Keep the existing `SaveGuardianSettings` API, storage keys, validation bounds, and guardian runtime behavior unchanged.
- Preserve compatibility with an absent settings snapshot by retaining the current defaults as frontend fallbacks.

## Design

### Backend Status Contract

Add a `settings` field of type `guard.Settings` to `GuardianStatusDTO`. `GuardianStatus` will populate it from the application's current normalized runtime settings through the existing `guardSettingsFromRuntimeSettings` conversion. This keeps configuration separate from worker health and process runtime status while making a refresh sufficient to reconstruct the settings UI.

### Frontend State and Rendering

Extend `GuardStatusDto` with an optional guardian settings snapshot. `normalizeGuardianStatus` will preserve the snapshot supplied by the backend. Each `WorkerSection` field will use the corresponding snapshot value:

- Process card: `timeoutThreshold`, `monitorIntervalMs`
- Network card: `networkReconnectIntervalMs`, `networkRecoveryTimeoutMs`
- Other-place-login card: `otherPlaceLoginIntervalMs`, `otherPlaceLoginDelayMin`

The process timeout threshold may continue to fall back to the process runtime threshold when the settings snapshot is unavailable. Other fields fall back to their existing defaults for compatibility with older or partially initialized responses.

Saving remains field-scoped through the existing `onSaveSettings` callback. After saving, the existing status refresh retrieves the persisted snapshot and redraws every input with the stored value.

## Error Handling

No new error path is introduced. A failed save continues to be logged by the existing frontend handler and the subsequent refresh restores the last persisted values. Missing settings data uses established defaults rather than leaving numeric inputs empty.

## Tests

- Add a backend regression test proving `GuardianStatus` returns all saved guardian numeric settings.
- Add a frontend rendering test proving all three cards display non-default values supplied by the settings snapshot.
- Add a frontend interaction test proving each numeric input submits its own field key and numeric value on blur.
- Run the focused Go and Vitest suites, followed by the broader affected test suites and frontend build.

## Success Criteria

After changing any numeric parameter in any of the three guardian cards, the saved value remains visible after the automatic status refresh, a manual refresh, and an application restart. Existing guardian enable switches and runtime behavior remain unchanged.
