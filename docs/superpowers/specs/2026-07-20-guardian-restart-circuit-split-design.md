# Guardian Restart Circuit Split Design

## Goal

Prevent routine scheduled and operator-requested restarts from consuming the
automatic failure-recovery circuit quota, while retaining a fixed emergency
limit that protects the application from any runaway restart loop.

## Selected Approach

Use two independent rolling ten-minute windows:

- The existing configurable `MaxRestartsPer10Min` limit applies only to
  automatic failure-recovery restarts. Its default remains four.
- A fixed internal safety limit applies to all accepted restart attempts. Its
  limit is eight and it is not persisted or exposed as a setting.

This keeps the existing settings contract stable and avoids adding another
operator control for a value intended only as a last-resort safety boundary.

## Restart Classification

The existing restart trigger remains the source of classification:

- `auto`: counts toward both the automatic circuit window and the global
  safety window;
- `scheduled`: counts only toward the global safety window;
- `manual`: counts only toward the global safety window.

An attempt is counted when the manager reserves it, before invoking the restart
callback. Callback success or failure does not remove the attempt because both
consume process and host resources. A request rejected by either limit is not
added to either window.

## Circuit Behavior

Before reserving any restart, the manager trims entries older than ten minutes
from both windows.

- An automatic request is rejected when the automatic window already contains
  `MaxRestartsPer10Min` attempts. The automatic circuit opens with an error that
  identifies the automatic restart limit.
- Any request is rejected when the global window already contains eight
  attempts. The safety circuit opens with an error that identifies the total
  restart safety limit.
- Manual and scheduled requests bypass an open automatic circuit, but never
  bypass an open global safety circuit or the existing single-restart-in-flight
  guard.
- Automatic requests remain blocked while the automatic circuit is open.
- Each circuit closes automatically after its corresponding rolling-window
  count falls below its limit. If both causes are open, the status remains
  circuit-open until both have cleared.
- The existing missing-restart-callback circuit behavior remains unchanged and
  is not treated as a quota circuit.

The fourth automatic attempt and the eighth total attempt are accepted. The
next applicable request opens the corresponding circuit and is rejected,
preserving the existing "maximum accepted attempts" boundary semantics.

## Status And UI

Keep the existing public settings and status field names for compatibility:

- `restartCountInWindow` reports only automatic failure-recovery attempts;
- `maxRestartsPerWindow` continues to report `MaxRestartsPer10Min`;
- `circuitOpen`, `phase`, and `lastActionError` report either active circuit
  cause.

The guard view changes the metric label from `重启窗口` to `异常重启` so the
displayed count is not mistaken for a total of manual, scheduled, and automatic
history. The fixed global count is not added as a routine metric; when reached,
the existing circuit status and last-action error expose the condition.

Recent restart events continue to record all executed attempts with their
existing `manual`, `scheduled`, or `auto` trigger.

## Compatibility And Scope

- Existing persisted settings require no migration.
- Existing backend JSON field names remain stable.
- Existing restart serialization, reconnect grace, lifecycle notifications,
  scheduling, and host binding behavior remain unchanged.
- Exponential backoff, a half-open probe state, and a configurable global limit
  are outside this change.

## Testing

Focused manager tests will prove that:

- scheduled restarts do not consume the automatic quota;
- manual restarts do not consume the automatic quota;
- automatic restarts still open their circuit at the configured boundary;
- an open automatic circuit blocks automatic requests but allows manual and
  scheduled requests when the global safety window has capacity;
- mixed manual, scheduled, and automatic attempts share the fixed global safety
  window and the ninth request is rejected;
- the two quota causes expire independently with their rolling windows;
- missing callback and restart-in-flight behavior remain intact.

A focused guard-view test will verify the `异常重启` metric label. After focused
tests pass, run the full Go and frontend test suites used by this project.
