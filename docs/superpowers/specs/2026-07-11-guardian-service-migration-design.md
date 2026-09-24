# Guardian Service Migration Design

## Goal

Migrate the complete guardian-service behavior from
`E:\desktop\farm-tauri-core-copy-20260707-155223\farm-tauri` into Farm_Go.
The guardian must keep detecting and handling failures while ordinary automation
tasks are running or while the automation scheduler is stopped.

The migrated service contains three independently configurable capabilities:

1. Process failure recovery and scheduled miniapp restart.
2. Immediate recovery from in-game network error prompts.
3. Delayed recovery from other-place-login prompts.

## Confirmed Product Decisions

- The guardian has one master switch and three independent child switches.
- The guardian lifecycle is independent from the farm automation scheduler.
- The UI uses one unified console rather than tabs or a secondary sidebar.
- The UI must reuse Farm_Go's existing visual system and responsive behavior.
- Detection remains concurrent, but destructive recovery actions are serialized.

## Existing State

Farm_Go already contains useful parts of the reference implementation:

- `internal/runtime/guard` provides host binding, process restart operations,
  restart events, phases, throttling fields, and a partial state machine.
- `frontend/src/views/GuardView.tsx` provides a basic process-guard page.
- `resources/wmpf/button.js` already contains network reconnect and
  other-place-login watchers.
- The automation scheduler already runs resource-compatible tasks concurrently.

The current implementation is incomplete:

- The Go guard has no active monitor loop or scheduled-restart timer.
- Scheduled restart status fields are stored but never driven.
- Runtime calls do not consistently report healthy/error results to the guard.
- The in-game reconnect watcher is present but is not activated by Farm_Go.
- Other-place-login configuration is not stored or synchronized to the runtime.
- Generic QQ runtime events are ignored, so reconnect lifecycle events are lost.
- The UI exposes only the process-guard summary and manual launch/restart actions.

## Architecture

### Guardian Supervisor

Add a long-lived `GuardianSupervisor` owned by `App`. It starts with the
application and remains alive independently of the automation scheduler. The
master switch controls whether its child workers are active.

The supervisor owns:

- current configuration;
- child-worker lifecycle;
- aggregate status;
- runtime generation tracking;
- event publication;
- the recovery coordinator.

### Process Guard Worker

The process worker is implemented in Go because it owns desktop-process and
runtime-link recovery. It is responsible for:

- monitoring runtime health at the configured interval;
- classifying restartable runtime failures;
- tracking consecutive failures;
- scheduled miniapp restart;
- host candidate binding and restart preview;
- reconnect grace periods;
- restart-window throttling and circuit breaking;
- automatically clearing the circuit when the ten-minute window expires;
- recording manual, automatic, and scheduled restart events.

It must preserve the existing safe-host boundary: only the currently bound and
revalidated process may be terminated.

### Runtime Network Reconnect Worker

Network prompt detection remains inside `button.js`. This is the closest layer
to `ServerKickOutUICom`, `NetNode`, and the current game state, so it can react
without waiting for a Go scheduler cycle.

When enabled, the worker:

- checks once per second by default;
- detects reconnect, relogin, and restart-game network prompts;
- invokes the most reliable available UI handler or `NetNode.reconnect()`;
- waits up to twenty seconds for the prompt, network state, and game state to
  recover;
- emits a lifecycle event for success or failure;
- requests escalation when runtime-local recovery fails.

The Go supervisor synchronizes the worker configuration whenever a runtime
becomes ready or the injected script is replaced.

### Other-Place-Login Worker

Other-place-login detection also remains inside `button.js`. When enabled, it:

- checks every five seconds by default;
- records the first prompt detection time;
- preserves the countdown if the prompt is temporarily inactive;
- emits one waiting event per detection period;
- rechecks the prompt after the configured delay;
- invokes the reconnect handler and emits a reconnected event;
- clears the detection timestamp after successful handling.

Go stores the configuration, resynchronizes it after reconnects, receives the
runtime events, deduplicates them, and writes account-scoped warning events.

### Recovery Coordinator

All workers may detect concurrently. Destructive recovery actions go through a
single `RecoveryCoordinator` so only one recovery action is active at a time.

Recovery priority is:

1. Immediate in-game network reconnect.
2. Due other-place-login reconnect.
3. Process-level miniapp restart.

The coordinator does not block detection loops. Workers submit recovery
requests to it and continue monitoring.

## Concurrency And Task Interaction

Guardian detection does not acquire automation `scene` or `protocol` resources.
It therefore continues while compatible or incompatible farm tasks are running.

Runtime-local watchers execute independently inside the injected game script.
Go process monitoring and scheduled restart use independent goroutines.

Before a process-level restart, the coordinator:

1. prevents dispatch of new automation work;
2. cancels pending runtime calls where cancellation is supported;
3. increments the runtime generation;
4. revalidates and terminates only the bound process;
5. launches the current runtime target;
6. waits for the runtime to become ready;
7. resynchronizes both runtime-local watchers;
8. resumes automation dispatch.

Results from an older runtime generation are discarded so a task that finishes
after restart cannot update current state.

## State Model

The aggregate guardian phases are:

- `disabled`
- `standby`
- `watching`
- `degraded`
- `recovering`
- `waiting_reconnect`
- `circuit_open`

Each child worker also exposes its own enabled, running, busy, last-check,
last-handled, last-result, and error fields. The other-place-login worker adds
first-detected, due-at, and remaining-time fields. The process worker adds
restart quota, grace-period, binding, and scheduled-restart fields.

## Failure Rules

- A network prompt is handled immediately. Recovery is allowed twenty seconds.
- Failed network recovery is recorded and escalated to the process worker for a
  fresh health decision.
- Process restart occurs after three consecutive restartable failures.
- A successful restart creates a forty-five-second reconnect grace period.
- No repeated process restart is allowed during that grace period.
- At most four automatic or scheduled restarts may occur in ten minutes.
- The circuit automatically closes when enough restart timestamps expire.
- Manual restart uses the same coordinator and reentrancy protection.
- Scheduled restart restarts only the current miniapp host, not Farm_Go.

## Configuration Defaults

The master switch defaults to disabled so an upgrade cannot unexpectedly close
a user process.

When the master switch is enabled, child defaults are:

| Capability | Default | Settings |
|---|---:|---|
| Process failure guard | Enabled | 3-second check, threshold 3, grace 45 seconds, maximum 4 restarts per 10 minutes |
| Network error reconnect | Enabled | 1-second check, recovery timeout 20 seconds |
| Other-place-login reconnect | Disabled | 5-second check, reconnect delay 5 minutes |
| Scheduled restart | Disabled | Interval 60 minutes |

All numeric settings must be normalized to bounded values before storage and
again before use.

## Data And Event Flow

1. App startup loads guardian settings and starts `GuardianSupervisor`.
2. Runtime status changes update the process worker and runtime generation.
3. Runtime readiness triggers configuration synchronization for both in-game
   workers.
4. Runtime calls report success or classified errors to the process worker.
5. In-game workers emit lifecycle events through QQ WS events or the WMPF/CDP
   runtime event bridge.
6. The supervisor deduplicates events, updates worker state, stores account
   events, and publishes UI refresh events.
7. Recovery requests enter the coordinator and produce unified event records.

## Persistence

Extend runtime settings with:

- guardian master enabled;
- process worker enabled and existing process-guard settings;
- network reconnect enabled, interval, and recovery timeout;
- other-place-login enabled, interval, and reconnect delay;
- scheduled restart enabled and interval.

Restart and reconnect events must be persisted with bounded retention. Events
associated with a known runtime account use account-scoped storage. Process
events without a resolved account remain available in the global guardian log.

## Unified Console UI

The existing Guard page becomes a unified operational console.

### Header

- Page title and short status description.
- Aggregate phase badge.
- Guardian master switch.
- Refresh icon action.

### Summary Strip

- Current runtime target.
- Bound process and PID.
- Restart quota.
- Next scheduled restart.
- Most recent abnormal condition.

### Process And Scheduled Restart Section

- Child switch and current phase.
- Monitor interval, threshold, timeout streak, reconnect grace, restart quota,
  next scheduled restart, and last restart result.
- Manual restart and launch actions.
- Advanced settings disclosure for bounded numeric controls.

### Network Reconnect Section

- Child switch and running/busy state.
- Detection interval, latest check, current prompt state, latest handling time,
  and latest result.

### Other-Place-Login Section

- Child switch and running/busy state.
- Detection interval and reconnect delay.
- Prompt visibility, first detection time, due time, countdown, latest handling
  time, and latest result.

### Unified Event Table

The table shows time, capability, runtime/account, trigger, result, duration,
and reason. Filters cover all events, process, network, and other-place login.

The page must reuse existing Farm_Go layout primitives, CSS variables, type
scale, icon buttons, switches, status badges, log-table styling, spacing, and
responsive breakpoints. It must not introduce a second component library or a
visually separate settings theme.

## Error Handling

- Worker-loop errors update worker status and are logged without terminating the
  supervisor.
- Configuration synchronization retries with bounded exponential backoff when a
  runtime is connected but not ready.
- Missing runtime methods are reported explicitly and retried after reinjection.
- Recovery action failures leave the worker degraded unless the restart circuit
  is open.
- Host ambiguity never falls back to terminating an unbound process.
- Application shutdown cancels workers, runtime calls, and timers cleanly.

## Testing

### Go Tests

- Process worker monitor loop and scheduled restart timing.
- Grace-period behavior and automatic circuit recovery.
- Concurrent detection with serialized recovery execution.
- Recovery priority, duplicate suppression, and generation invalidation.
- Runtime call health/error reporting.
- Settings normalization and persistence.
- QQ generic runtime-event delivery and account-scoped event logging.
- Startup, runtime reconnect, and script-reinjection configuration sync.
- Clean worker shutdown without goroutine or timer leaks.

### Runtime Script Tests

- Network prompt classification and handler fallbacks.
- Recovery success and timeout events.
- Other-place-login countdown persistence and delayed handling.
- Start/stop idempotency and generation cancellation.
- Event deduplication fields and account metadata.

### Frontend Tests

- Master and child switch behavior.
- All worker states, countdowns, scheduled times, and circuit-open warnings.
- Unified event filtering.
- Disabled, loading, recovery, error, and empty states.
- Desktop and narrow-viewport layout without text overlap.

### Integration Verification

- Run an ordinary long automation task while each watcher detects an anomaly.
- Verify immediate network recovery without waiting for the task to finish.
- Verify a due other-place-login action executes while other tasks are active.
- Verify process restart pauses dispatch, invalidates old results, reconnects,
  resynchronizes watchers, and resumes automation.
- Verify the guardian remains active while the automation scheduler is stopped.

## Completion Criteria

The migration is complete when all three child capabilities work independently
and concurrently, their settings survive restart, runtime reconnects restore the
watchers, process recovery is safely serialized and throttled, account events
are visible, and the unified console matches the rest of Farm_Go.
