# Scheduled Update Check Design

## Goal

Add a configurable recurring update check to the System Settings update section. It is enabled by default and runs every 120 minutes. The existing startup check and manual check remain available.

## User Experience

The existing `应用更新` panel gains three controls:

- a `定时检查更新` checkbox, enabled by default;
- a numeric interval input labelled `检查间隔`, expressed as `分钟/次`;
- an update-settings save button with its own busy, success, and error state.

The interval accepts whole numbers from 1 through 10080 minutes. The input is disabled while recurring checks are disabled. Invalid values are not saved and produce an inline validation message in the update panel. Saving update preferences does not save or apply unsaved runtime-link settings.

Changing and saving the recurring-update preference resets the countdown. Saving an enabled interval does not trigger an immediate extra check. Disabling it cancels the pending recurring check. Manual checks continue to work while recurring checks are disabled.

## Persistence And API

Add a small global update-check preference model rather than extending runtime settings:

```text
enabled: boolean
intervalMinutes: integer
```

The storage layer persists the fields in the existing global settings table under update-specific keys. Missing settings load as `enabled = true` and `intervalMinutes = 120`. Stored invalid intervals normalize to 120 so older or damaged configuration cannot create a tight loop.

Expose authorized Wails methods to load and save the preference. Save rejects an interval outside 1 through 10080 rather than silently changing a value the user just entered. The methods are added to the authorization policy and generated frontend bindings.

## Scheduling And Data Flow

`AuthorizedApp` owns the recurring timer because it remains mounted while the user changes pages. On authorization it continues to start the existing non-blocking update check and independently loads the saved recurring preference. When enabled, it schedules the next automatic check for one full configured interval after preferences load.

Automatic checks use a completion-aware `setTimeout` loop: the next timeout is scheduled only after the current check settles. Manual, startup, and recurring checks share one in-flight request so they cannot issue overlapping backend checks. Saving new preferences updates root state and recreates the timer with the new interval.

Successful automatic results update the shared update state. A newly available version therefore uses the existing non-blocking update notification and details flow. Automatic errors are swallowed and do not replace the last successful update state or display an intrusive error. Manual errors continue to render in the update panel.

The timer and any stale asynchronous result are invalidated when the authorized application unmounts, including authorization revocation.

## Components

- `internal/storage`: preference defaults, load, save, and interval validation.
- Go application API: authorized load/save methods that delegate to storage.
- generated Wails bindings: typed access to the two preference methods.
- frontend update scheduling helper: timer lifecycle and interval conversion, designed for fake-timer tests.
- `AuthorizedApp`: preference loading, check de-duplication, recurring scheduling, and state propagation.
- `SettingsView`: update preference controls, validation feedback, and independent save action.

## Error Handling

- Preference load failure falls back to the safe default of enabled every 120 minutes and does not block the workbench.
- Preference save failure leaves the active root preference unchanged and shows an inline error.
- Invalid input remains visible for correction and does not call the backend.
- Automatic update-check failures are non-blocking and silent.
- Manual update-check failures retain the existing visible behavior.

## Testing

Backend tests cover default values, save/load round trips, corrupt stored interval fallback, validation boundaries, and authorization of the new API methods.

Frontend tests cover default control rendering, disabled interval behavior, input validation, successful and failed saves, default scheduling, timer reset after save, cancellation when disabled or unmounted, no overlapping checks, silent automatic errors, and continued manual checks.

Verification includes focused Go and Vitest suites, the full relevant frontend suite, TypeScript compilation, and a production frontend build copied to `public/app` through the repository's established frontend build command.
