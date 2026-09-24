# Runtime Reconnect And Process Guard Design

## Goal

Farm_Go should recover when a connected miniapp is closed and later reopened across all supported runtime targets: QQ WS, WeChat CDP, and YingYongBao CDP. It should also port the reference project's process guard behavior from `E:\desktop\farm-tauri`, adapted to Farm_Go's Wails/Go architecture.

## Scope

This work covers connection recovery and host-process guarding only. It does not migrate farm automation task logic, account logic, message push, or the reference project's full Node gateway.

The guard is allowed to terminate the currently bound host process and relaunch the miniapp. It must not kill unbound or ambiguous host processes.

## Reference Behavior

The reference project splits the feature into two responsibilities:

- `core/src/process-guard.js`: state machine for health, consecutive timeout detection, restart throttling, manual restart, scheduled restart, reconnect grace windows, and recent restart events.
- `app/src-tauri/src/host_process.rs`: OS-facing host process discovery, PID binding, restart preview, termination, relaunch, and rebinding.

Farm_Go should preserve those behaviors while rewriting them in Go and exposing them through Wails methods instead of Tauri commands or HTTP endpoints.

## Runtime Recovery

QQ WS already has client-side socket reconnect while the miniapp JS context is alive. Closing the miniapp destroys that context, so Farm_Go must make reopening reliable by patching all likely QQ `game.js` candidates, not just the most recently touched one.

WeChat CDP and YYB CDP should keep their debug bridge and Frida watcher running while the miniapp is absent. When a new miniapp debug client connects, the link should return to `handshaking`, find `gameContext`, connect the CDP evaluator again, and set runtime status back to `ready`.

If a link loses its runtime evaluator or miniapp bridge after it was ready, calls should fail with a recoverable disconnected/context error rather than permanently poisoning the link.

## Process Guard

Add a Go process guard manager with these states:

- `disabled`
- `standby`
- `watching`
- `degraded`
- `restarting`
- `waiting_reconnect`
- `circuit_open`

The guard has these inputs:

- Runtime status changes from `runtime.Manager`.
- Diagnostic/runtime call errors.
- Manual restart requests from Wails.
- Scheduled restart timer.

The guard has these actions:

- Auto-bind a single safe host candidate for the current runtime target.
- Preview whether the bound PID is still safe to restart.
- Terminate only the bound PID.
- Relaunch the current miniapp:
  - QQ: use the QQ Farm `tencent://ntqq-open` protocol.
  - WeChat: use the `weixin://launchapplet/?app_id=wx5306c5978fdb76e4` protocol.
  - YYB: resolve and launch `AndrowsLauncher.exe` with the reference launch parameters.
- Wait for the runtime to reconnect during a grace window.
- Record recent restart events.

The guard must throttle automatic restarts with a "max restarts per 10 minutes" circuit breaker.

## Host Process Discovery

Create a Go host-process package based on the reference candidate rules:

- QQ candidates: process name contains `qq` or `ntqq`, or visible window title mentions the miniapp.
- WeChat candidates: `WeChatAppEx.exe` that is not an YYB WMPF path.
- YYB candidates: `WeChatAppEx.exe` associated with an Androws/WMPF runtime path, including inferred child executable paths.

Each candidate should include PID, parent PID, process name, executable path, visible window titles, window handles, confidence, availability, bound owner, and evidence.

On Windows, process and window snapshots should use native APIs or a small PowerShell fallback if native API wiring blocks progress. Non-Windows should return `unsupported_platform`.

## Public App Surface

Add Wails methods:

- `ProcessGuardStatus()`
- `SaveProcessGuardSettings(settings)`
- `ListHostCandidates()`
- `BindHostProcess(pid)`
- `ClearHostBinding()`
- `PreviewHostRestart()`
- `RestartHostProcess()`
- `LaunchHostProcess()`

Persist guard settings and restart events in the existing storage layer.

## UI

Add a guard/recovery section that follows the current Farm_Go UI style. It should show:

- Guard phase and current runtime target.
- Bound PID and process name.
- Timeout streak and restart count.
- Last healthy time and last restart time.
- Recent restart events.
- Buttons for refresh, bind, clear binding, launch miniapp, and restart current miniapp.
- Settings for enable guard, timeout threshold, monitor interval, restart grace, maximum restarts, and scheduled restart.

Do not port the farm-tauri MUI layout. Keep the Wails/React UI visually consistent with the current app.

## Testing

Testing should be TDD-first:

- QQ patcher patches multiple discovered candidates and reports all candidate paths.
- CDP links can recover after miniapp disconnect/reconnect.
- Guard state machine transitions through degraded, restarting, waiting reconnect, and circuit open.
- Host candidate selection refuses ambiguous processes.
- Restart logic terminates only the bound PID and rebinding ignores old or unrelated candidates.
- Wails app methods expose guard state and do not break existing runtime settings.
- Frontend renders guard controls without overlapping text at desktop and mobile widths.

## Completion Criteria

The work is complete when:

- QQ, WeChat CDP, and YYB CDP can reconnect after closing and reopening the miniapp.
- The guard can bind, preview, launch, manually restart, auto restart after repeated recoverable failures, and wait for reconnection.
- Restart throttling prevents runaway process kills.
- The UI exposes the guard in the current project style.
- Go tests and frontend tests pass.
