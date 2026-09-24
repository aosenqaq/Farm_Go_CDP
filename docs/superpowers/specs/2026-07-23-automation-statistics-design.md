# Automation Statistics Design

## Goal

Make action statistics reflect confirmed work rather than successful task completion:

- `own_collect` and `own_base` count one successful task round only when the task actually harvests or performs farm work.
- `friend_steal`, `friend_help`, and `friend_mischief` count the number of friends whose requested action succeeds.

## Data Flow

`automation.ActionResult` will expose an action count. Runtime task handlers set it only after a confirmed action:

- Own collection: `1` when one or more crops are confirmed harvested; dead-land cleanup without a harvest is `0`.
- Own farming: `1` after the one-click farm-work request succeeds; skipped work is `0`.
- Friend steal/help: the successful-candidate count.
- Friend mischief: `1` after its selected friend's mischief request succeeds; its current execution flow stops after that successful friend.

Both manual and scheduled `task.done` events will include this count. The scheduler log carries the field so the two execution paths have identical event data.

## Aggregation

The session summary and the account-history aggregation will require a positive action count for the five action metrics, then add that count rather than blindly adding one per `task.done` event. Overall `runs` remains a count of successful task rounds and is not redefined.

Existing persisted events lack an action count and cannot be distinguished from no-op rounds. They therefore contribute zero to the affected action metrics; new events use the explicit count.

## Friend Help Limit

The friend-help daily marker already advances by `SuccessfulFriends`. That matches the required per-friend meaning and remains unchanged. No-op or failed help attempts do not advance it.

## Tests

Cover runtime action counts for each task, session aggregation with no-op and multi-friend events, historical aggregation with the same event data, and propagation through scheduler logs and manual execution events.
