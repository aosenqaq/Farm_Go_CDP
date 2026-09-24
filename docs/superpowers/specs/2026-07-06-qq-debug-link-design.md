# QQ Debug Link Design

## Goal

Complete the first-stage QQ miniapp debug link for Farm_Go by making the Wails/Go runtime accept the QQ host script protocol used by the reference project at `E:\desktop\farm-tauri`.

This phase is only a connection and diagnostics foundation. It does not migrate farm automation tasks, WMPF, Frida, CDP, Tauri sidecars, Node gateway code, or browser WebUI behavior.

## Scope

- Keep Farm_Go as a Wails + Go desktop app.
- Keep the listener local-only at `127.0.0.1:8787/runtime/qqws`.
- Copy only the minimal QQ host script needed by the miniapp runtime to connect back to Farm_Go.
- Do not modify `E:\desktop\farm-tauri`.
- Make the diagnostics panel able to call `host.describe` and `gameCtl.probe` through the connected QQ host.
- Preserve structured failure responses when no QQ host is connected.

## Reference Protocol

The compatible protocol comes from `E:\desktop\farm-tauri\core\qq-host.js` and `E:\desktop\farm-tauri\core\src\qq-ws-session.js`.

Client to server:

- `hello`: announces host version, platform, script hash, available methods, transport kind, and `gameCtlReady`.
- `event`: reports runtime state changes such as `gameCtlReadyChanged`.
- `log`: forwards runtime logs.
- `ping`: heartbeat from the host script.
- `result`: returns the outcome of a server `call`.
- `error`: reports a host-side error.

Server to client:

- `helloAck`: acknowledges the `hello` packet and assigns a client ID.
- `call`: requests a host method such as `host.describe` or `gameCtl.probe`.
- `pong`: responds to ping packets.

## Go Runtime Design

`internal/runtime/qqws` becomes a small session manager instead of a handshake-only adapter.

- Track one active ready client, with room for multiple connected clients.
- Mark the runtime `ready` only when a compatible `hello` is accepted.
- Reply to `hello` with `helloAck`, not `hello.ok`.
- Handle `event`, `log`, `ping`, `result`, and `error` packets.
- Provide a `Call(ctx, path, args, timeout)` method that sends a `call` packet and waits for the matching `result`.
- Reject calls cleanly when there is no active ready client.
- Clear pending calls when a client disconnects.

The runtime status should continue to feed the existing UI fields: phase, connected, ready, instance/client ID, host version, last seen time, and last error.

## Diagnostics Design

`internal/diagnostics.Service` should depend on a runtime caller interface rather than hard-coding "not connected".

Supported methods stay intentionally narrow:

- `host.describe`
- `gameCtl.probe`

The service validates the method, forwards the call to QQ WS, measures duration, and returns the result or a structured error.

## Host Script Asset

Copy `E:\desktop\farm-tauri\core\qq-host.js` into Farm_Go as a necessary runtime asset. The copied script must be adapted only where placeholders are required:

- WebSocket URL: `ws://127.0.0.1:8787/runtime/qqws`
- Host version expected by Farm_Go
- Allowed RPC paths for first-stage diagnostics

The copied file is intentionally treated as an asset, not as a source for bringing in the reference project's Node runtime.

## Testing

Add Go tests before implementation:

- A QQ host-compatible `hello` receives `helloAck` and sets runtime status to ready.
- `host.describe` sends a `call` packet and resolves from a matching `result`.
- A supported diagnostic returns `runtime is not connected` before any host connects.
- Unsupported diagnostic methods are rejected before touching the runtime.
- A pending diagnostic fails if the client disconnects.

Run verification:

- `go test ./...`
- `cd frontend && npm test`
- `cd frontend && npm run build`

## Acceptance

The task is complete when a QQ miniapp host script can connect to Farm_Go, complete the compatible hello handshake, and respond to at least `host.describe` through the diagnostics panel. `gameCtl.probe` should use the same call path and report a host-side structured error if the method is not available in the current miniapp runtime.
