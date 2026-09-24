# Friend Help Daily Limit Design

## Goal

Make `autoFarmFriendHelpDailyLimit` limit the number of friends successfully helped by the automatic `friend_help` task per account and local calendar day.

## Behavior

- Count one unit for each friend whose automatic help protocol call succeeds.
- Do not count skipped friends, failed protocol calls, or manual help actions.
- Before an automatic batch, cap its candidates to the smaller of:
  - `autoFarmFriendHelpMaxFriends`, when greater than zero; and
  - the remaining daily allowance, when `autoFarmFriendHelpDailyLimit` is greater than zero.
- A daily limit of `0` means unlimited.
- When no daily allowance remains, return a successful skip result without calling the game runtime.
- Reset the effective count when the local calendar date changes.
- Keep counts isolated by normalized account key and persistent across application restarts.

## State And Data Flow

Store two runtime-owned values in each account's automation configuration:

- the local date associated with the current count;
- the number of friends successfully helped on that date.

The application passes the account configuration and trigger into `RuntimeFacade`. For an automatic `friend_help` run, the facade derives the remaining allowance, limits the batch, and returns the successful friend count as structured result data. The application then increments and persists the account's daily count. Manual runs neither read nor update this allowance.

Runtime-owned friend-help daily values must be preserved by `MergeRuntimeDailyState` so saving user settings cannot overwrite a count updated by a concurrent automation run.

## Failure Handling

- Only confirmed successful friend results consume allowance.
- A partially successful batch persists only its successful friend count.
- A persistence failure follows the existing automation-state save error path and must not be hidden by parsing human-readable messages.
- The existing scheduler serialization remains the concurrency boundary for automatic tasks.

## Testing

Add focused tests for:

- candidate truncation by remaining daily allowance;
- interaction between per-batch and daily limits;
- successful, skipped, and failed friend result counting;
- immediate skip when the daily limit is reached;
- local-date reset;
- `0` as unlimited;
- manual runs ignoring and not updating the daily limit;
- per-account isolation and persistence;
- preservation of runtime-owned counters during settings merges.

Run the focused automation and application tests, then the full Go test suite.
