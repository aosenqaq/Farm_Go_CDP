# QQ WS Miniapp Restart Design

## Goal

Optimize only the `qq_ws` restart path so Farm_Go restarts the QQ Farm miniapp subtree without terminating the long-lived QQ main process. A restart succeeds only after the old QQEX session has disconnected, a replacement miniapp root has appeared, and a new WebSocket session reaches both `Connected` and `Ready`.

The `wechat_cdp` and `yyb_cdp` restart paths retain their current behavior and are outside this change.

## Live Evidence

The live test established the following QQ lifecycle:

1. The QQ main process remained PID `33456` for the entire observation period.
2. The first QQ Farm miniapp used root PID `37064`, with six QQ renderer, utility, and GPU children plus one crashpad child.
3. The visible window titled `QQ经典农场` belonged to root PID `37064`.
4. The miniapp network utility PID `23152` held the WebSocket connection to the Farm_Go listener on `127.0.0.1:8787`.
5. Manually closing the miniapp window removed root PID `37064` and all seven descendants. The QQ main process and Farm_Go listener remained alive, while the established WebSocket connection disappeared.
6. Relaunching from the desktop shortcut created miniapp root PID `12992` under the same QQ main PID, recreated seven descendants, and assigned the visible `QQ经典农场` window to the new root.
7. The replacement network utility PID `25260` established a new WebSocket connection from local port `24140` to Farm_Go port `8787`.

Therefore the stable restart boundary is the QQEX miniapp root and its descendants. The QQ main process is infrastructure and must never be used as a restart target.

## Current Behavior

`RestartHostProcess` delegates through the Guardian manager to `restartHostForRuntimeTarget`. For `qq_ws`, that function currently calls `AutoBindHostProcess` and then the shared `guard.RestartBoundHost` implementation.

The current path performs these actions:

1. Select any matching QQ candidate, preferring a title that looks like a miniapp but falling back to a single visible or process-only `QQ.exe` candidate.
2. Terminate only the bound PID with `TerminateProcess`.
3. Clear the binding.
4. Sleep for two seconds.
5. Dispatch the Tencent miniapp launch protocol.
6. Return `launch_dispatched` before runtime reconnection is known.

This creates three concrete risks:

- After the miniapp has already closed, the fallback candidate can be the QQ main PID. A later restart can therefore terminate all of QQ.
- Terminating only the miniapp root does not explicitly confirm that its descendants or old WebSocket session have exited before relaunch.
- A successful protocol dispatch is recorded as a successful restart even if no replacement session becomes ready during reconnect grace.

## Chosen Architecture

Add a dedicated `guard.RestartQQMiniapp` orchestration function and select it only for `qq_ws` in `restartHostForRuntimeTarget`. The function will use injected callbacks for snapshots, window closure, runtime-state observation, targeted process termination, launch dispatch, and time so its behavior can be tested deterministically.

The existing `RestartWeChatMiniapp` remains separate because its identity invariant is different: WeChat preserves one host PID, while QQ intentionally replaces the QQEX root PID. The shared `RestartBoundHost` path remains available for YYB and any unchanged legacy callers.

## QQ Miniapp Identity

A QQ restart target must satisfy all of these conditions:

1. Its process name is `QQ.exe`.
2. It owns a visible nonzero window handle whose title matches the QQ Farm miniapp rule.
3. Its parent process exists and is also `QQ.exe`.
4. It is not the QQ main process and is not a renderer, utility, GPU, or crashpad descendant selected only by process name.

The current snapshot model already supplies process name, parent PID, windows, and executable path, so command-line collection is not required for this change. The observed `QQEXMiniProgram` command line may remain a diagnostic signal but is not a binding dependency.

Initial authorization requires the visible miniapp window. Before posting `WM_CLOSE`, the operation captures a termination fingerprint containing the root PID, process name, parent PID, and executable path. If the window disappears while the root remains, timeout cleanup revalidates that stable fingerprint instead of requiring the now-closed window to remain visible. Any mismatch disables forced cleanup.

QQ candidate selection will not fall back to a generic visible or process-only `QQ.exe`. Zero strict candidates means `not_found`; more than one means `ambiguous`. Neither result may authorize process termination.

## Restart Flow With A Running Miniapp

The synchronous QQ restart operation performs the following ordered steps:

1. Strictly bind or validate exactly one QQ Farm miniapp root.
2. Capture the old root PID, parent QQ PID, visible window handles, current descendants, and current runtime `InstanceID`.
3. Post `WM_CLOSE` to the bound miniapp window handles.
4. Wait up to five seconds for both conditions: the old runtime becomes disconnected and the old root subtree disappears.
5. If the close was posted but the verified old subtree remains after the deadline, refresh the process snapshot and revalidate the old root identity.
6. Only after successful revalidation, stop current descendants whose parent chain still leads to the old root, deepest descendants first, then stop the old root. Never follow or terminate the parent QQ PID.
7. Confirm that the old root subtree is absent and the runtime is disconnected. If either condition still fails, return a final failure and do not launch.
8. Dispatch the existing Tencent protocol for app ID `1112386029` exactly once.
9. Wait up to the configured `restartReconnectGraceSec` for `qq_ws` to become `Connected && Ready` with a nonempty `InstanceID` different from the captured old instance.
10. Refresh process snapshots and require exactly one strict QQ Farm miniapp root with a PID different from the old root and the expected parent QQ PID.
11. Replace the registry binding with the new PID, window handles, titles, and identity evidence.
12. Return the final status `reconnected`.

A failure to post `WM_CLOSE` returns immediately without force termination or launch. Targeted termination is a timeout recovery for a verified but unresponsive old QQEX subtree, not a fallback for uncertain identity.

## Launch-Only Recovery

The restart callback may run after the user has already closed the miniapp. In that state, a stale binding must not cause Farm_Go to bind the QQ main process.

If no strict miniapp root exists and `qq_ws` is disconnected, the operation will:

1. Clear a stale QQ miniapp binding without stopping any process.
2. Dispatch the Tencent launch protocol once.
3. Wait for a new `Connected && Ready` session.
4. Strictly discover and bind the new miniapp root.
5. Return `reconnected`.

If no strict miniapp root exists while `qq_ws` still reports connected, return `qq_miniapp_unverified`. The operation must not launch a duplicate miniapp or terminate any QQ process when runtime and process evidence disagree.

## Result Semantics

The QQ restart callback returns only a final result:

- `reconnected`: the old session is gone when applicable, one launch was dispatched, a new WS instance became ready, and the binding was refreshed to the new miniapp root.
- Error: identity is uncertain, window closure failed, old subtree cleanup failed, disconnect was not observed, launch failed, a new runtime instance did not become ready, or the replacement process could not be strictly bound.

`guard.Manager.executeRestart` already treats `reconnected` as final recovery and returns directly to the watching phase. The QQ path will no longer enter a second asynchronous `waiting_reconnect` phase after its callback completes.

Stage-specific error text will distinguish at least:

- `qq_miniapp_unverified`
- `close_failed`
- `disconnect_timeout`
- `old_tree_exit_timeout`
- `old_tree_cleanup_failed`
- `launch_failed`
- `reconnect_timeout`
- `new_instance_not_observed`
- `new_process_not_found`
- `binding_refresh_failed`

## Concurrency And Safety

Guardian continues to serialize restart jobs with `restartInFlight`. The existing coordinator generation advance and dispatch pause remain active for the entire synchronous QQ restart, preventing automation calls from racing with session replacement.

The QQ main process is a hard safety boundary. The restart implementation must never stop:

- The parent of the verified QQEX root.
- Any process outside the descendant set captured from the verified root.
- A PID that no longer matches the refreshed process identity.
- Farm_Go or its port `8787` listener.

If PID reuse or changed parentage makes identity uncertain, the operation fails closed.

## Binding And Window Refresh

The old binding remains available as identity evidence while the old subtree is being closed. It is cleared only after the old subtree is confirmed absent or when launch-only recovery accepts a stale disconnected binding.

After readiness, the refreshed binding must use the replacement root PID and new HWNDs. Pending auto-minimize behavior runs only after this refreshed binding is installed, ensuring that it targets the replacement window rather than a stale handle.

## Testing

Automated tests will verify:

1. Strict QQ selection accepts the titled miniapp root under a QQ parent.
2. Strict QQ selection never falls back to the QQ main process, generic renderers, or process-only candidates.
3. Ambiguous strict candidates fail without stopping or launching.
4. Normal restart posts `WM_CLOSE`, observes disconnect and subtree exit, and never invokes forced termination.
5. Launch does not occur before both disconnect and old-subtree exit are confirmed.
6. A window-close dispatch failure does not terminate or launch.
7. Timeout cleanup stops only refreshed descendants of the verified old root, deepest first, with the root last.
8. Changed parentage, identity mismatch, or PID reuse prevents timeout cleanup.
9. Launch-only recovery clears a stale binding only when runtime is disconnected.
10. A connected runtime with no verified root returns `qq_miniapp_unverified` without launching.
11. A launch error is returned as a final failure.
12. Reconnect requires a nonempty `InstanceID` different from the old instance.
13. Reconnect timeout is a final failure rather than a successful `launch_dispatched` result.
14. Successful restart discovers a different root PID and refreshes new window handles.
15. Missing or ambiguous replacement processes fail binding refresh.
16. Guardian records `reconnected` as final recovery and does not enter `waiting_reconnect`.
17. Auto-minimize targets the refreshed QQ window.
18. Existing WeChat and YYB restart tests continue to pass.

Live verification will capture the QQ and Farm_Go process trees plus port `8787` before, during, and after a button-triggered restart. Success requires the QQ main PID and Farm_Go PID to remain stable, the old QQEX root and descendants to disappear, exactly one replacement QQEX root to appear, the old WS connection to close, a new connection and instance to become ready, and the UI action to return only after final readiness.

## Scope Exclusions

This change does not redesign QQ WebSocket framing, QQ patch injection, runtime switching, network reconnect workers, other-place-login recovery, WeChat restart behavior, YYB restart behavior, or general-purpose Windows process management beyond the strictly bounded QQEX subtree operations required here.
