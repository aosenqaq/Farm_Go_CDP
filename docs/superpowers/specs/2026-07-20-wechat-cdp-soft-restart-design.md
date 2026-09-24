# WeChat CDP Soft Restart Design

## Goal

Optimize only the `wechat_cdp` restart path so restarting the miniapp preserves the existing WeChat and `WeChatAppEx` host processes. A restart succeeds only after a new CDP session reaches both `Connected` and `Ready`; otherwise the operation returns a final failure without falling back to process termination.

The `qq_ws` and `yyb_cdp` restart paths retain their current behavior and are outside this change.

## Live Evidence

The live test established the following lifecycle:

1. Closing the miniapp removed the active `wmpf-render-type=0` renderer and one preload renderer.
2. The WeChat main process, `WeChatAppEx` browser host, network service, GPU service, link service, and the Farm_Go listener on `127.0.0.1:9421` remained alive.
3. The established TCP connection between the WMPF network service and port `9421` disappeared while the listener remained active.
4. Relaunching from the desktop shortcut reused the host and network service PIDs, created replacement business and preload renderers, and established a new TCP connection to port `9421` from a different ephemeral port.

Therefore the normal WeChat restart boundary is the miniapp window, business renderer, and CDP session rather than the WMPF host process.

## Current Behavior

`RestartHostProcess` delegates through the Guardian manager to `restartHostForRuntimeTarget`, which currently calls the shared `guard.RestartBoundHost` implementation for every runtime target.

The shared path performs these actions:

1. Stop the bound host PID.
2. Clear the host binding.
3. Sleep for two seconds.
4. Dispatch the platform launch request.
5. Return `launch_dispatched` before runtime reconnection is known.

For `wechat_cdp`, the bound PID is the visible `WeChatAppEx` miniapp host. Terminating it destroys infrastructure that the live test showed can and should be reused.

## Chosen Architecture

`restartHostForRuntimeTarget` will select a dedicated WeChat soft-restart path only when the normalized platform is `wx`. The existing `guard.RestartBoundHost` path remains unchanged for QQ and YYB.

The WeChat path will use dependency callbacks for OS window closure, launch dispatch, runtime-state observation, host snapshot refresh, and time. Keeping those boundaries injectable allows deterministic tests without terminating real processes or sleeping for production timeouts.

The Windows implementation will close only the visible bound miniapp window handles by posting `WM_CLOSE`. It will never call `StopHostPID`. Non-Windows builds will return the existing unsupported-platform style error.

## Restart Flow

The synchronous WeChat restart operation performs the following ordered steps:

1. Auto-bind and validate the current WeChat miniapp host and its visible window.
2. Capture the bound host PID and visible miniapp window handles.
3. Post `WM_CLOSE` to those window handles.
4. Wait for the active runtime target to become disconnected. This transition is required so the pre-restart healthy status cannot be mistaken for successful reconnection.
5. If disconnection is not observed within the close timeout, return failure and do not dispatch the launch protocol.
6. Dispatch `weixin://launchapplet/?app_id=wx5306c5978fdb76e4`.
7. Wait up to the configured `restartReconnectGraceSec` for the `wechat_cdp` runtime to become both `Connected` and `Ready`.
8. Refresh the host binding from a new process/window snapshot while requiring the host PID to remain unchanged.
9. Return the final status `reconnected`.

The existing two-second launch delay becomes a maximum close/disconnect acknowledgement timeout for the WeChat path, implemented as condition-based waiting rather than an unconditional sleep. QQ and YYB retain the fixed launch delay currently passed to `RestartBoundHost`.

## Binding Rules

The host binding remains associated with the original `WeChatAppEx` PID throughout the restart. A successful refresh may replace stale window handles and titles after the miniapp window is recreated, but it must not accept a different PID.

If the original host PID disappears, the refresh fails. The operation reports that the preserved host was lost and does not terminate or launch any replacement host process.

## Result Semantics

The WeChat callback returns only a final result:

- `reconnected`: window closure was observed, launch was dispatched, CDP reconnected, runtime became ready, and the binding was refreshed.
- Error: window close failed, disconnect was not observed, launch dispatch failed, the original host disappeared, binding refresh failed, or runtime readiness timed out.

`guard.Manager.executeRestart` will recognize `reconnected` as already recovered. It will record a successful restart and return to the watching phase without entering another reconnect-grace period. Results from QQ and YYB remain `launch_dispatched` and continue through the existing asynchronous reconnect-grace flow.

`RunGuardianAction` will treat both `launch_dispatched` and `reconnected` as successful action results. The Wails `RestartHostProcess` call remains awaited by the frontend, so the existing restarting state stays active until the final WeChat result is returned. On failure, Guardian records `LastActionError`, which the Guard view already displays after status refresh.

## Concurrency

Guardian already serializes restart jobs with `restartInFlight`. Automatic and scheduled restarts execute the callback in their existing worker goroutine; manual restart waits synchronously for the same callback. No second WeChat restart can start while one is waiting for disconnection or readiness.

The existing Guardian coordinator pause remains active for the entire synchronous WeChat soft restart, preventing automation dispatch while the runtime session is being replaced.

## Failure Policy

The WeChat path has no process-kill fallback. Specifically, it must never terminate:

- The WeChat main process.
- The bound `WeChatAppEx` browser host.
- The WMPF network, GPU, link-service, shared renderer, or preload processes.
- The Farm_Go process or its port `9421` listener.

Failures are returned and recorded directly. Existing circuit-breaker and restart-quota behavior remains responsible for preventing repeated automatic attempts.

## Testing

Automated tests will verify:

1. WeChat restart closes the bound window and never invokes `StopPID`.
2. Launch occurs only after a disconnected runtime state is observed.
3. A missing disconnect transition fails without launching.
4. Launch errors are returned without process termination.
5. The call blocks until `Connected && Ready` is observed.
6. Reconnect timeout is returned as a final failure.
7. Successful restart preserves the host PID and refreshes recreated window handles.
8. Loss or replacement of the bound host PID fails.
9. Guardian treats `reconnected` as final recovery and does not enter `waiting_reconnect`.
10. QQ and YYB still use the existing stop-delay-launch path.
11. Existing CDP reconnect tests continue to pass.

After automated verification, live testing will capture the process tree and port `9421` state before, during, and after a button-triggered restart. Success requires the WeChat and WMPF infrastructure PIDs to remain stable, the business renderer PID to change, the old CDP connection to disappear, a new connection to appear, and the button call to return only after runtime readiness.

## Scope Exclusions

This change does not optimize QQ WebSocket restart behavior, YYB CDP restart behavior, general runtime switching, network reconnect workers, other-place-login recovery, WMPF injection, or process discovery beyond the binding refresh required by the WeChat soft restart.
