# Message Push Template Design

**Status:** Approved for implementation planning

## Goal

Let each account configure and test a complete, channel-specific template for every
business message type, then send those templates from real runtime events without
notification storms.

## Scope

The feature covers six message types:

| Type | ID | Source of truth |
| --- | --- | --- |
| Abnormal alert | `abnormal` | Guardian reconnection/restart failure and circuit-open state |
| Suspected abnormal | `suspected` | Guardian timeout or connection-error threshold before recovery fails |
| Recovery notice | `recovery` | A previously unhealthy runtime returns to a healthy state |
| Restart notice | `restart` | Manual, scheduled, or automatic host restart completes |
| Daily asset report | `daily` | Existing once-per-day scheduler |
| Log monitor | `log_monitor` | Persisted `warn` and `error` runtime events that match a configured rule |

The feature does not add arbitrary user scripting, dynamic outbound destinations, or
unbounded template syntaxes.

## Architecture

All sources publish a normalized message event before any channel-specific work:

```
guardian/runtime event or daily scheduler
  -> message event mapper
  -> five-minute deduplication gate
  -> template resolver and renderer
  -> channel payload adapter
  -> existing sender and push history
```

The mapper owns the definition of each message type and creates a controlled variable
context. The renderer selects the active account's template for the event type and
chosen channel. The channel adapter validates the result and constructs the existing
HTTP request. This preserves the current sender's retry, channel selection, history,
and account-scoped persistence behavior.

## Independent Delivery Scheduling

Message delivery must not run through the farm automation scheduler or its execution
locks. `App` owns a dedicated, bounded message-push event queue and one delivery
worker. Runtime/guardian publishers enqueue a normalized event after persisting its
audit record, then return immediately; the worker performs rendering, dedupe, retries,
and HTTP delivery in order. When the queue is full, the app records an explicit
`message_push.queue_dropped` audit event and applies no unbounded goroutine fallback.

The worker has its own lifecycle context, starts with the existing daily-push timer,
and stops before application shutdown completes. Daily reports enqueue onto this same
delivery worker rather than sending from the timer goroutine. This creates one ordered
message-delivery lane while leaving the primary farm task scheduler independent.

## Template Configuration

`messagePush.config` remains the only persisted message-push configuration record and
gains a `templates` field. It is scoped by the existing account key.

Each template is addressed by message type and channel type. A template contains:

- `enabled`: whether the channel should send this message type;
- `mode`: a channel-supported payload mode;
- `content`: the complete template payload;
- `updatedAt`: display and diagnostics metadata, not a variable source.

Template modes are capability-limited per channel. `text` and `markdown` render
message content; `card` renders supported platform card payloads; `json` is reserved
for generic Webhook payloads. The UI only offers modes supported by its selected
channel. Channel adapters remain the authority that rejects invalid or unsupported
modes.

Older configurations omit `templates`. On load, the service generates the built-in
templates in memory; saving then writes the normalized configuration. Existing channel
credentials, selected-channel behavior, and channel format settings remain compatible.

## Variables and Validation

Templates use `{{path.to.value}}` placeholders only. There are no expressions,
function calls, file access, or outbound requests inside templates.

Every event includes common variables:

- `event.type`, `event.time`, `event.count`;
- `account.key`, `account.gid` when available;
- `runtime.target`.

Type-specific variables are:

| Type | Variables |
| --- | --- |
| `abnormal` | `error.message`, `error.stage`, `restart.reason` |
| `suspected` | `error.message`, `monitor.streak`, `monitor.threshold` |
| `recovery` | `recovery.duration`, `recovery.via`, `previous.error` |
| `restart` | `restart.trigger`, `restart.reason`, `restart.ok`, `restart.error` |
| `daily` | `daily.date`, `daily.summary` |
| `log_monitor` | `log.level`, `log.source`, `log.type`, `log.message`, `log.data` |

Saving and testing validate template syntax, unknown variables, required content, and
the final payload structure. JSON must parse after rendering; card templates must
conform to the selected channel's allowed structure. Renderer output is escaped or
encoded by the channel adapter so values cannot make malformed JSON or requests.

## Trigger Mapping

The guardian manager must expose durable state transitions instead of requiring the
push service to inspect log strings.

- `abnormal`: emit when a network/other-place recovery fails, an automatic restart
  fails, or restart quota opens the circuit.
- `suspected`: emit only when a restartable runtime failure crosses the configured
  monitoring threshold but has not yet produced an abnormal result.
- `recovery`: emit when a runtime with an active abnormal or suspected state first
  returns to healthy. A normal periodic healthy check does not create a notification.
- `restart`: emit after each restart attempt, including manual, scheduled, and
  automatic attempts, with the final result.
- `daily`: replace the current placeholder payload with the daily event context while
  retaining the current once-per-day scheduler semantics.
- `log_monitor`: inspect persisted runtime events at `warn` and `error` level. A rule
  can restrict matches by source, event type, and message keyword. No rule means
  matching logs are not sent.

The application event recorder is the integration boundary for log-monitor messages.
It must dispatch after the event has been assigned its timestamp and account scope,
and it must ignore push-service diagnostic events to prevent recursive notifications.

## Deduplication

Real-time events use a five-minute suppression window. The key is the normalized
combination of account, message type, runtime target, and error or rule signature.

Events that match a live key do not send again; their count is retained. The next
eligible send includes `event.count` so recipients can see repetition. Recovery and
restart-completion events bypass an earlier abnormal/suspected key because they convey
a state transition. Daily reports retain their existing one-report-per-local-date key.

Deduplication state is account-scoped and persisted with `messagePush.state` so an
application restart does not produce a duplicate burst.

## Failure Behavior

An invalid saved template cannot be used for a live send. The service records a
runtime event with the account, type, channel, and validation error, then tries the
built-in default template for that type and channel. If the default cannot be built or
sent, the result is reported through the existing push history and runtime event log.
Template faults never block guardian recovery, runtime event persistence, or daily
scheduler bookkeeping.

The template test endpoint renders against a fixed mock context and sends only that
result. It does not change guardian state, run a restart, increment deduplication, or
mark a daily report as sent.

## User Experience

The Message Push view gains a Templates workspace alongside channel configuration and
send controls:

1. Select a message type.
2. Select a configured channel.
3. Select one of that channel's supported modes.
4. Edit the full payload, browse the type's permitted variables, and inspect a mock
   rendered preview.
5. Validate, restore the built-in default, or send the current draft as a template
   test before saving.

Changing a type, channel, or mode does not overwrite any other template. Validation
messages appear at the editor and identify the template path and failure reason.

## Verification

Backend tests cover normalization and migration of old configuration, variable
allowlists, rendering and escaping, per-channel payload validation, fallback template
behavior, all six event mappings, account isolation, persisted five-minute
deduplication, recovery/restart bypass behavior, and daily idempotency.

Frontend tests cover switching message types and channels without draft loss,
mode-specific editor selection, variable discovery, preview and validation states,
restore-default behavior, test-send wiring, and save/reload of the account-scoped
configuration.

Integration tests use a controlled HTTP server plus guardian/runtime event fixtures to
verify the emitted request body, message type, account scope, suppression count, and
runtime event audit trail.
