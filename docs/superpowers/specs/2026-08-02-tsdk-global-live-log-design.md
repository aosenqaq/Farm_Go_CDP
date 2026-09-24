# TSDK Global Live Log Design

## Goal

Make TSDK interception startup and success events reliably visible in the open log dialog. TSDK logs are runtime-global rather than game-account data, and the dialog always shows the newest event first.

## Confirmed Scope

- TSDK logs may be shared across game accounts.
- Ordinary runtime and automation logs remain account-scoped.
- The existing dashboard polling transport remains the live-update mechanism.
- Existing uncommitted TSDK startup-log changes in `resources/qq/qq-host.js` and `resources/wmpf/button.js` are retained and incorporated rather than replaced wholesale.

## Root Causes

1. The QQ host creates the WebSocket asynchronously. A TSDK startup event emitted immediately after injection calls `sendLog` before the socket is open, and the failed send is discarded.
2. TSDK events currently use the active account key. Events emitted before account confirmation are stored under the default account and disappear when polling switches to the confirmed account.
3. `RuntimeEvents` already returns newest-first data, while `TsdkBlockDialog` reverses it and displays the oldest event first.

## Design

### Reliable QQ Delivery

The QQ host keeps a small bounded queue for log packets that cannot be sent because the socket is not open. `sendLog` sends immediately when possible; otherwise it appends the log payload to the queue. After the socket opens and the hello packet is sent, the host flushes queued logs in their original production order.

The queue is capped at 50 entries. When full, it drops the oldest queued entry so an extended disconnect cannot grow memory without bound. Successfully sent entries are removed exactly once. A failed flush keeps the unsent entry and the remaining tail for the next connection.

The immediate `TSDK-BLOCK v2 init` event is the startup record. Existing `Layer1 interceptors ready`, `Layer2 applied`, and CDP `Fetch interception enabled` events remain the success records.

### Global TSDK Scope

The backend assigns events whose type is `qqhost.log` and whose message contains `[TSDK-BLOCK]` to a dedicated global TSDK event scope. QQ WS and CDP paths use the same classification and storage rule.

Dashboard event reads combine two scopes:

- the currently confirmed game-account scope;
- the global TSDK scope.

The combined result is sorted newest first and limited only after merging. The memory fallback follows the same rule. Because only classified TSDK events enter the global scope, ordinary logs cannot cross account boundaries.

### Frontend Ordering And Refresh

The dialog derives a fresh filtered array without mutating the `events` prop, then orders records by descending event ID with timestamp as a fallback. New events therefore appear directly below the fixed table header.

No new frontend subscription is introduced. The existing 2.5-second dashboard poll replaces the `events` state and React rerenders an already-open dialog, so a newly stored TSDK event appears without closing or reopening it.

## Error Handling

- QQ transport disconnects preserve at most 50 pending log packets and retry them on the next successful connection.
- An invalid or empty TSDK payload continues to be ignored by the existing backend validation.
- Storage failures retain the existing in-memory fallback behavior.
- Sorting tolerates missing IDs or invalid timestamps and preserves input order for indistinguishable records.

## Verification

Tests will cover:

1. A TSDK log produced before QQ WebSocket open is delivered once after connection.
2. TSDK logs are readable after switching between confirmed game accounts, while ordinary logs remain isolated.
3. Database and memory-fallback reads return the merged scopes newest first and apply the requested limit after merging.
4. The dialog renders the newest TSDK record above older records and updates when new `events` props arrive while open.
5. Focused frontend tests, Go tests, the full frontend build, and relevant Go packages pass.

## Non-Goals

- Replacing dashboard polling with Wails event streaming or Server-Sent Events.
- Changing TSDK interception rules, host lists, or feature IDs.
- Making non-TSDK runtime logs global.
- Redesigning the dialog beyond ordering and live-data behavior.
