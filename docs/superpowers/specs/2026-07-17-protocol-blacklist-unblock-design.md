# Protocol Blacklist Unblock Design

## Goal

Add single-friend and checked batch unblock controls to the existing protocol blacklist dialog.

## Scope

- Keep the existing "查看协议拉黑好友" entry point and protocol blacklist dialog.
- Add an "解除拉黑" command to every protocol-blacklisted friend row.
- Add row selection, "全选当前列表", "清空选择", and "解除选中" controls.
- Refresh the protocol blacklist from the runtime after every single or batch operation.
- Do not change local blacklist or whitelist rules.
- Do not add a new injected runtime protocol method; reuse `gameCtl.unblockFriendByProtocol`.

## Backend Design

The existing `unblock_friend` action remains the single-friend path. It validates one positive numeric gid and calls `gameCtl.unblockFriendByProtocol` with `friendGid`, `dryRun`, and `silent`.

Add `unblock_friend_batch` to `social.Service.Action`. The action accepts `FriendActionRequest.Targets`, trims and deduplicates positive numeric gids while preserving their first-seen order, then processes them serially. Each gid calls the existing runtime method independently. A failed item is recorded and does not stop later items.

The batch action returns an `ActionResult` whose data contains:

- `action`
- normalized `targets`
- per-gid `results`
- `successCount`
- `failureCount`

An empty or fully invalid selection fails without calling the runtime. Complete success reports status `ok`; partial or complete failure reports status `failed` with a message containing both counts. The frontend refreshes the protocol list for either outcome because successful items may have changed runtime state.

## Frontend Design

`SocialView` owns the protocol selection and pending state because the controls live inside its protocol blacklist dialog.

Each row contains a checkbox, the existing friend name and gid, the "协议拉黑" status, and an "解除拉黑" button with an unlock icon. While a single request is pending, that row shows an in-progress label and cannot be submitted again.

A compact toolbar above the list shows the selected count and provides:

- "全选当前列表"
- "清空选择"
- "解除选中"

"解除选中" is disabled when nothing is selected or an unblock request is running. Batch submission uses `action: "unblock_friend_batch"` and the selected gids. Single submission uses `action: "unblock_friend"` and one gid.

After either request settles, `SocialView` calls the existing `onProtocolBlockList` callback. Completed batch requests clear the selection. Selection is also reconciled whenever a refreshed list removes friends that are no longer protocol-blacklisted.

The dialog remains usable on narrow screens: row content may wrap, command buttons retain stable height, and long names or gids cannot overlap the action area.

## Error Handling

- Invalid or empty batch targets are rejected before runtime calls.
- Single-request failures keep the dialog open and leave the friend visible after refresh.
- Batch processing continues after individual failures and reports success and failure counts.
- Controls remain disabled only for the active operation and are restored in `finally` paths.
- The existing social action toast displays the backend result message.

## Testing

Backend tests cover single unblock compatibility, batch gid normalization and order, serial runtime calls, partial failure continuation, result counts, and rejection of empty selections.

Frontend tests cover row-level unblock submission, selection toggling, select all, clear selection, batch payloads, disabled pending states, and protocol list refresh after success or failure.

Verification includes focused Go and Vitest tests, the full relevant test suites, `npm run build` from `frontend`, and confirmation that `frontend/dist/index.html` references the newly generated asset embedded by `main.go`.
