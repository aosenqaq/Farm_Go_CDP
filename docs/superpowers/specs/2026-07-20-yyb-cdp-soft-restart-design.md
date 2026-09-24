# YYB CDP Soft Restart Design

## Goal

Optimize only the `yyb_cdp` restart path so restarting QQ Classic Farm preserves the existing Application Treasure (`AndrowsStore`) and WMPF (`WeChatAppEx`) infrastructure. A restart succeeds only after the old CDP session disconnects, the miniapp is launched again, a new CDP session becomes both connected and ready, and the visible miniapp window is rebound to the preserved host PID.

The restart must never fall back to terminating Application Treasure, the WMPF browser host, or their child processes. The existing `wechat_cdp` and `qq_ws` restart behavior remains unchanged.

## Live Evidence

Two close-and-relaunch cycles established the Application Treasure lifecycle:

1. Closing the miniapp removed the active renderer and the `QQ经典农场` window.
2. `AndrowsStore.exe`, the WMPF browser host, network service, GPU service, link service, and one preload renderer remained alive.
3. The established connection from the WMPF network service to Farm_Go port `9421` disappeared while Farm_Go continued listening on `9421` and `62000`.
4. Relaunching from Application Treasure and from the desktop shortcut both reused the same WMPF browser and network-service PIDs.
5. Relaunch consumed the retained preload renderer, created the next preload renderer, and established new outer and inner CDP connections with different ephemeral ports.

The observed ephemeral port pairs changed from `39186/39190` to `64594/64595` and then to `21240/21241`. Restart correctness therefore cannot depend on an old socket or ephemeral port. The stable boundary is the Farm_Go listeners plus a newly connected and ready runtime session.

## Current Behavior

`App.restartHostForRuntimeTarget` has dedicated final-reconnect paths for `qq_ws` and `wechat_cdp`. The `yyb_cdp` target falls through to `guard.RestartBoundHost`, which:

1. Stops the bound host PID.
2. Clears the host binding.
3. Sleeps for two seconds.
4. Dispatches the Application Treasure launch request.
5. Returns `launch_dispatched` before reconnection is known.

For the live Application Treasure process tree, the bound PID is the visible WMPF `WeChatAppEx` browser host. Stopping it destroys infrastructure that normal close-and-relaunch behavior preserves.

The project already resolves and dispatches the installed `AndrowsLauncher.exe` shortcut with the QQ Classic Farm package parameters. This design reuses that launch path and does not change launcher discovery, CDP injection, or protocol handling.

## Chosen Architecture

Extract the platform-independent orchestration from the existing WeChat soft restart into an unexported shared WMPF CDP restart helper. Preserve the exported `RestartWeChatMiniapp` entry point and add a parallel `RestartYYBMiniapp` entry point. Each wrapper supplies its platform identity and display label while sharing the ordered close, disconnect, launch, ready, and binding-refresh behavior.

This keeps platform selection explicit in `App.restartHostForRuntimeTarget`, avoids duplicating the state machine, and preserves the tested WeChat API and result semantics.

The shared helper remains dependency-injected through callbacks for window closure, runtime-state observation, launch dispatch, snapshot refresh, and timeouts. Tests can therefore verify ordering and failures without interacting with real processes or waiting for production timeouts.

## Restart Flow

The synchronous `yyb_cdp` restart performs these ordered steps:

1. Auto-bind and validate the current Application Treasure miniapp host and visible window.
2. Capture the bound WMPF browser PID and current visible miniapp window handles.
3. Post `WM_CLOSE` only to those bound visible window handles.
4. Wait for `yyb_cdp` to report `Connected == false` within the close timeout.
5. If disconnection is not observed, return failure without launching and without terminating a process.
6. Dispatch the existing Application Treasure shortcut launch request for QQ Classic Farm.
7. Wait up to `restartReconnectGraceSec` for `yyb_cdp` to report `Connected == true && Ready == true`.
8. Refresh host snapshots and require the original WMPF browser PID and process identity to remain present.
9. Require the recreated miniapp window and non-zero window handle under that PID.
10. Refresh the existing host binding with the recreated window information.
11. Return the final status `reconnected`.

The close timeout uses the existing two-second restart launch delay as a maximum disconnect acknowledgement timeout, matching the WeChat soft-restart contract. It is condition-based and does not add a fixed sleep before launch.

## Session Identity

The restart does not cache, compare, or reuse ephemeral TCP ports. The required disconnect transition prevents the pre-restart healthy state from satisfying the reconnect check. The subsequent `Connected && Ready` transition is treated as the authoritative new session signal.

The WMPF browser and network-service PIDs are expected to remain stable. Renderer PIDs are deliberately not part of the success contract because the preload pool rotates them during normal lifecycle operations and renderer promotion is not directly exposed by the current process snapshot model.

## Binding Rules

The binding remains associated with the original visible WMPF `WeChatAppEx` PID throughout the operation. A successful refresh may replace stale window handles and titles but must not bind a replacement PID.

The refreshed snapshot must:

- Match the `yyb` platform candidate rules.
- Preserve the original process name and PID.
- Contain a visible miniapp window title and at least one non-zero handle.

If the original PID disappears or its identity changes, the operation fails and leaves the previous binding intact for diagnostics. It does not bind another Application Treasure or WMPF process automatically.

## Result Semantics

The Application Treasure callback returns only a final result:

- `reconnected`: window closure was accepted, disconnection was observed, launch was dispatched, the new CDP session became ready, and the original host binding was refreshed.
- `restart_failed`: any validation, close, disconnect, launch, readiness, snapshot, identity, or binding-refresh step failed.

`Stopped` remains false. `LaunchDispatched` becomes true only after the launcher callback succeeds.

Guardian already treats `reconnected` as final recovery, records the restart result, and avoids a second asynchronous reconnect-grace phase. Manual and automatic restart serialization, circuit-breaker quotas, coordinator pausing, and event reporting remain unchanged.

## Failure Policy

There is no process-kill fallback. The optimized `yyb_cdp` path must never terminate:

- `AndrowsStore.exe` or `AndrowsSvr.exe`.
- The bound WMPF `WeChatAppEx.exe` browser host.
- WMPF renderer, network, GPU, or link-service children.
- Farm_Go or its `9421` and `62000` listeners.

A failed soft restart is returned and recorded directly. Existing Guardian retry quotas and circuit breakers remain responsible for limiting subsequent automatic attempts.

## Auto-Minimize

When `AutoMinimizeAfterRestart` is enabled, the Application Treasure path queues and consumes the existing pending-host minimize operation only after `reconnected`. Minimization uses the refreshed binding so it targets the recreated window handle, not the stale pre-close handle.

## Testing

Focused guard tests will verify:

1. Application Treasure restart closes the bound window and never invokes process termination.
2. Launch occurs only after a `yyb_cdp` disconnected state is observed.
3. Disconnect timeout fails without launching.
4. Launch failure does not wait for readiness.
5. Readiness timeout is a final failure after launch.
6. Success preserves the host PID and refreshes the recreated window handle.
7. A missing or replacement host PID is rejected.
8. The Application Treasure wrapper emits the correct shortcut launch request and platform validation.
9. Existing WeChat soft-restart tests continue to pass through the shared helper.

Application tests will verify:

1. `yyb_cdp` routes to the soft-restart path instead of `RestartBoundHost`.
2. `StopHostPID` is never called for Application Treasure soft restart.
3. The ordered calls are close, wait for disconnect, launch, wait for ready, and refresh.
4. The result is final `reconnected` and retains the original binding PID.
5. Ready-time auto-minimize uses the refreshed window.
6. QQ and WeChat restart behavior remains unchanged.

Final verification will run focused guard and application tests, all Go tests, frontend tests, the frontend production build, and a live Application Treasure restart. Live success requires stable Application Treasure/WMPF infrastructure PIDs, rotation of the active renderer, disappearance of the old CDP connection, establishment of new `9421` and `62000` connections, and a final ready result.

## Scope Exclusions

This change does not modify launcher discovery, injection-dialog launch behavior, Frida hooks, CDP protocol framing, runtime target switching, QQ WebSocket restart, WeChat CDP behavior, renderer discovery, process-guard quotas, or UI layout.
