# Runtime Memory Polling Transport Design

## Goal

Stop the WebView2 renderer memory from growing linearly during normal Farm_Go operation while preserving the current status, logs, automation, land, dog-guard, account, and patch behavior.

## Reproduction Evidence

The protected `Farm_Go_V1.0.4_yb.exe` was tested locally with WebView2 `150.0.4078.65` and the existing authorized application data.

- During an authorized workspace run, total private bytes rose from about 638 MB to 884 MB in 43 seconds.
- The Go process stayed between about 181 MB and 204 MB while the WebView2 renderer rose from about 272 MB to 526 MB.
- The process tree stayed at one renderer, one GPU process, two utility processes, and two WebView2 host/crash processes. The growth was not caused by spawning more processes.
- The same protected executable, launched with empty application data and left on the unauthorized license page, stayed near 348 MB total and 32 MB renderer private bytes for 28 seconds.
- Switching from the workspace to settings did not change the growth rate, which excludes page-specific land rendering and workspace statistics as the primary cause.
- Twenty rapid `Refresh status` actions increased renderer private bytes by about 227 MB above the normal polling trend, demonstrating that growth is proportional to Wails binding call volume.
- The latest 120 persisted runtime events serialize to only about 40-50 KB, with individual rows below 1.1 KB. A single oversized log payload is not the cause.

Wails v2.12 stores and deletes JavaScript promise callbacks correctly, but every Windows binding response is delivered by evaluating a unique script containing the complete response:

```text
window.wails.Callback(<response JSON>)
```

The go-webview2 message handler also posts each frontend request back to the page. Farm_Go currently performs several binding calls every 2.5 seconds, another patch-status call every 1.5 seconds, land polling every 5 seconds, dog-guard polling every 1.8 seconds on its tab, and repeated automatic patch calls every 10 seconds. The repeated bridge traffic is the root trigger for the renderer growth.

## Chosen Approach

Keep Wails for commands and infrequent reads, but move recurring polling to same-origin HTTP-style requests handled by the existing Wails AssetServer middleware. A browser `fetch` response does not create a new `ExecuteScript` callback and does not require a Wails or go-webview2 fork.

This approach is preferred over:

- A Wails dependency fork, which would fix the transport globally but create a large Windows-specific maintenance and release burden.
- A fully event-driven rewrite, which would touch every runtime producer and would still deliver active events through Wails script evaluation.

## Poll API

The AssetServer middleware will reserve `/farm-api/poll/` and expose exactly three fixed internal refresh resources:

```text
GET /farm-api/poll/dashboard?activeTab=<tab>
GET /farm-api/poll/land?revision=<revision>
GET /farm-api/poll/dog-guard
```

All responses use `application/json; charset=utf-8` and `Cache-Control: no-store`. Unsupported methods return `405`; unknown resources return `404`; invalid query values return `400`; failed authorization returns `401` with a small JSON error object. Internal failures return `500` without exposing paths, secrets, stack traces, or raw runtime payloads.

The route is available only through the embedded AssetServer origin. It does not open a TCP listener, accept arbitrary method names, evaluate user-provided code, or expose a generic reflection-based call endpoint. Dashboard refresh may run the existing idempotent host auto-binding scan, but no poll route can launch, terminate, or restart a process, install a patch, or persist configuration.

### Dashboard Resource

The dashboard response combines the recurring reads currently started by `AuthorizedApp`:

- runtime status;
- guardian status;
- host auto-binding result;
- the newest 120 account-scoped runtime events;
- account-scoped automation state;
- workspace run statistics when `activeTab=workspace`;
- QQ debug patch status.

The response is assembled from the same backend methods and account scope used today. The `activeTab` value is validated against the existing tab identifiers. Run statistics are omitted outside the workspace to avoid unnecessary work.

### Land Resource

The land endpoint accepts the last revision. An empty revision returns a full `LandDetailsDeltaPayload`; a current revision returns only changed lands through the existing `FarmLandDetailsSince` behavior. No image data is embedded in the JSON.

### Dog-Guard Resource

The dog-guard endpoint returns the same account-scoped state as `FarmSocialDogGuardState`. It is requested only while the social view policy enables dog-guard polling.

## Backend Components

A focused poll handler will own method validation, route dispatch, authorization, query validation, JSON encoding, and response headers. Snapshot assembly remains separate from HTTP concerns so it can be unit tested without a WebView.

The existing `/farm-assets/` middleware behavior remains unchanged. Normal embedded frontend assets and all other requests continue to the next AssetServer handler.

The handler will not add caches of complete responses or retain request objects. Each request creates one bounded response and releases it after encoding.

## Frontend Lifecycle

A small poll client will provide typed functions for dashboard, land, and dog-guard reads. It will use same-origin relative URLs and verify both HTTP status and JSON shape before returning data.

`AuthorizedApp` will replace the 2.5-second group of Wails calls and the separate 1.5-second patch-status interval with one dashboard request every 2.5 seconds. Manual refresh uses the same request path. Applying a dashboard response keeps the current account-generation checks so a late result from a previous account cannot update the active account.

Every periodic reader has these lifecycle rules:

- at most one request may be in flight;
- starting an already running poller is a no-op;
- stopping clears its timer and aborts the current request;
- a generation token prevents late completions from mutating state;
- tab changes and component unmount stop tab-scoped polling;
- a background failure keeps the last good state and is logged once per failure transition rather than on every tick.

Land polling and dog-guard polling retain their existing cadences and UI behavior while using the poll client instead of Wails bindings.

## Automatic Patch Check

The automatic QQ patch check will no longer run forever every 10 seconds. It will run once when an eligible runtime becomes ready, keyed by runtime target and instance ID. Leaving the ready state clears the key, so a later reconnect or a new instance is checked again.

The finite automatic check may continue to use the existing Wails command bindings because it performs diagnostics and may install a patch. Manual patch actions are unchanged. The dashboard poll supplies the passive patch status.

## Error Handling

- `401` stops the affected poller and lets the existing license lifecycle move the UI to its locked state.
- `400`, `404`, and malformed JSON are treated as programming errors and reported without retrying rapidly.
- Transient `500` or fetch failures preserve the last state. The next scheduled tick retries only after the normal interval.
- Aborted requests are silent and never surface as user-facing failures.
- Dashboard fields are applied only after the whole response has passed shape validation, preventing mixed snapshots.

## Tests

Backend tests will prove:

- only the three documented `GET` resources are routed;
- method, route, query, and authorization failures return the expected status and bounded JSON error;
- the dashboard snapshot includes the expected fields and omits run statistics outside the workspace;
- land revisions delegate to the existing delta behavior;
- dog-guard state remains account scoped;
- normal asset requests still fall through to the existing handler.

Frontend tests will prove:

- poll URLs and query encoding are correct;
- non-success and malformed responses are rejected;
- a poller never overlaps requests;
- stop aborts an in-flight request and suppresses late results;
- `AuthorizedApp` no longer schedules recurring Wails calls for dashboard or patch status;
- automatic patch checking occurs once per ready runtime instance and is rearmed after readiness is lost;
- land and dog-guard tab cleanup still stops their pollers.

Existing Go and frontend suites must remain green. The protected frontend build must also succeed.

## Runtime Verification

The replacement protected executable will be tested on the same machine, with the same authorized application data and WebView2 version used for the reproduction.

1. Record the root process and all descendants every 5 seconds for at least 10 minutes.
2. Test the default workspace for one run and a tab-scoped poll view for another run.
3. Record process count, private bytes, working set, handles, and threads by process type.
4. Ignore the first two minutes as warm-up and calculate renderer start/end change and least-squares slope over the final eight minutes.
5. Confirm manual refresh, tab switching, land changes, dog-guard updates, runtime reconnect, and patch status still update correctly.

## Acceptance Criteria

- No recurring dashboard, patch-status, land, or dog-guard read uses a Wails binding.
- The automatic patch check does not run indefinitely for an unchanged ready runtime instance.
- The renderer process count stays constant during each observation.
- After warm-up, renderer private bytes do not show the previous linear growth: final-minus-start is at most 64 MB and the fitted slope is at most 0.05 MB/second over the final eight minutes.
- The Go process does not develop a new positive linear memory trend.
- All existing user-visible status, log, automation, land, dog-guard, account, reconnect, and manual patch behavior remains available.
