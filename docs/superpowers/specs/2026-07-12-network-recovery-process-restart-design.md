# Network Recovery Process Restart Design

## Goal

Escalate a failed miniapp network recovery to the existing process guardian so a
persistently white or disconnected miniapp is restarted without restarting
Farm_Go.

## Scope

The runtime-local reconnect watcher already emits `network_reconnect` events.
The supervisor will treat its `failed` phase as a restartable runtime error.
This reuses the existing process manager for enabled/failure-recovery guards,
restart serialization, reconnect grace, restart quotas/circuit breaking, and
lifecycle reporting.

## Behavior

- `detected` and `waiting` events continue to request runtime-local recovery
  only. They do not increment the process restart counter.
- A `network_reconnect` event with phase `failed` reports a restartable error
  to the process manager. Its message identifies the network recovery failure
  and preserves the runtime-provided error when available.
- One terminal `failed` event immediately reserves an automatic bound-host
  restart. It does not wait for `TimeoutThreshold`, because no repeated
  `failed` event arrives for the same recovery episode.
- A successful runtime call or a ready, connected runtime snapshot retains the
  existing healthy-path behavior and resets the counter.
- Other-place-login recovery behavior is unchanged.

## Validation

Supervisor tests will prove that a transient network recovery event does not
restart the host and one terminal failed event reserves the configured host
restart while existing recovery controls remain in effect.
