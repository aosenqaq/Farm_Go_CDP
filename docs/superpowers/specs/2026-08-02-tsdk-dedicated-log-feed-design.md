# TSDK Dedicated Log Feed Design

## Goal

Keep the most recent 100 TSDK events visible independently of the ordinary runtime-event display limit. Ordinary account logs must never evict TSDK records, and TSDK records must not consume ordinary log capacity.

## Root Cause

TSDK events are stored in the dedicated `global:tsdk` scope, but `App.RuntimeEvents(limit)` currently loads both the active account and global TSDK scopes, merges them, and applies one shared limit. The dashboard requests `RuntimeEvents(120)`. When more recent ordinary events fill that window, older TSDK events fall outside the returned array, and the frontend dialog has nothing left to filter.

## Backend Design

`App.RuntimeEvents(limit)` returns only the active account's ordinary events again. A new `App.TSDKRuntimeEvents(limit)` method reads only `global:tsdk` events. Both methods retain the existing authorization check, newest-first order, storage path, and memory fallback behavior.

The dashboard poll response contains two independent fields:

- `events`: the latest 120 ordinary events for the active account;
- `tsdkEvents`: the latest 100 global TSDK events.

The limits are applied independently at their source. No merging, reserved slots, or post-filtering is used.

## Frontend Design

`DashboardPollResponse` validates and exposes `tsdkEvents` as a required array. `AuthorizedApp` stores it in a dedicated `tsdkEvents` state and updates it on every existing 2.5-second dashboard poll.

The workspace passes ordinary and TSDK events through separate props. `OverviewView` uses `events` only for task activity and uses `tsdkEvents` for both `tsdkBlockSignal` and `TsdkBlockDialog`.

Account-scope resets continue to clear ordinary `events`. They do not clear `tsdkEvents`, because TSDK is explicitly runtime-global and shared across accounts. Application unmount or authorization teardown still discards component state normally.

## Error Handling

- If the storage query succeeds, `TSDKRuntimeEvents(100)` returns the newest 100 persisted global TSDK events.
- If storage is unavailable, it scans the existing in-memory event ring for `global:tsdk` entries only and returns at most 100.
- Unauthorized reads return an empty list, matching `RuntimeEvents` behavior.
- A malformed dashboard response without `tsdkEvents` is rejected by the frontend poll validator rather than silently falling back to the shared list.

## Verification

Tests will prove:

1. More than 120 newer ordinary events do not remove a TSDK event from `TSDKRuntimeEvents(100)`.
2. `RuntimeEvents(120)` excludes global TSDK records and preserves all 120 ordinary slots.
3. The memory fallback provides the same separation.
4. The dashboard response includes independent `events` and `tsdkEvents` arrays with limits 120 and 100.
5. The frontend rejects responses without `tsdkEvents`, stores the dedicated list, and passes it to the TSDK indicator/dialog.
6. Existing newest-first dialog ordering and live prop updates remain unchanged.

## Non-Goals

- Unlimited TSDK history in one poll response.
- A second polling endpoint or a new push transport.
- Changing TSDK interception, persistence, host rules, or feature IDs.
- Clearing TSDK logs when the active game account changes.
