# Social View QQ Profile Design

## Goal

Add a `查看好友QQ` command to each friend row's operation menu when the active
runtime target is QQ. Clicking it must use the existing visitor protocol to
resolve the selected friend's QQ OpenID and immediately request the native QQ
friend dialog with the fixed verification text `来自QQ农场`.

The command is intentionally an invocation of QQ's native UI, not a generated
or stored add-friend link. A successful invocation must never be presented as a
completed QQ friend relationship.

## Scope

### In scope

- Show `查看好友QQ` in the existing `好友社交` row `操作` menu only for the QQ
  runtime target.
- Add the focused social action `view_qq`.
- On click, call the existing runtime operation
  `gameCtl.addFriendByGidDiagnostic` with the selected friend GID,
  `verifyMsg: "来自QQ农场"`, and `silent: true`.
- Reuse the runtime operation's guarded sequence:
  1. `VisitService.Enter(reason=5)` queries the selected target.
  2. The same reply must contain matching `basic.gid` and a valid
     `basic.open_id`.
  3. The runtime calls `qq.addFriendByOpenId` once to request QQ's native
     dialog.
- Use the existing social action toast for pending, success, and failure
  feedback.
- Add Go and frontend regression tests for the new action and menu visibility.

### Out of scope

- Generating, showing, copying, or persisting a QQ add-friend link.
- Sending a generic raw-OpenID runtime command from the frontend.
- Adding a Farm_Go confirmation dialog or editable verification message.
- Detecting, operating, or interpreting the QQ native dialog's final outcome.
- Changing the existing friend, blacklist, or automation actions.

## User Flow

1. The user runs Farm_Go against a QQ runtime and opens `好友社交`.
2. The user opens a friend row's `操作` menu and selects `查看好友QQ`.
3. The menu item becomes pending while Farm_Go sends the guarded visitor
   protocol request through the backend.
4. When the matching reply exposes a valid QQ OpenID, the runtime requests the
   QQ native friend dialog using `来自QQ农场` as its verification text.
5. Farm_Go reports that it requested the QQ native dialog. The user completes
   or dismisses the dialog in QQ itself.

The same action is used for every row in the Farm_Go friend list. QQ owns any
already-friends state and decides what its native UI displays; Farm_Go does not
try to pre-classify or infer that state.

## Architecture

```text
SocialView operation menu
    -> AuthorizedApp runSocialAction({ action: "view_qq", target: gid })
        -> App FarmSocialAction authorization and QQ-target guard
            -> social.Service Action
                -> gameCtl.addFriendByGidDiagnostic
                    -> VisitService.Enter(reason=5)
                    -> validate basic.gid and basic.open_id
                    -> qq.addFriendByOpenId native QQ dialog
```

`SocialView` receives a QQ-runtime capability flag from `AuthorizedApp`. That
flag controls whether the command is rendered. The Go boundary independently
enforces the same target restriction so a crafted Wails request cannot run the
action on WeChat or another target.

The social service calls only the named, pre-existing runtime operation. It
does not receive or return an OpenID. The service transforms the runtime reply
into a minimal action result containing the target GID, whether the native call
was invoked, and callback-state metadata only when the runtime supplied it.

## Result And Error Handling

`invoked: true` maps to the user-facing result "已请求打开 QQ 原生好友对话框。"
It means only that the native method was called. No callback is treated as a
successful friend relationship.

The backend maps known guarded-operation failures to readable messages without
returning private protocol values:

| Runtime reason | User-facing result |
| --- | --- |
| `visit_query_failed` | 无法读取该好友的 QQ 信息。 |
| `reply_gid_mismatch` | QQ 目标校验失败，已阻止打开对话框。 |
| `reply_open_id_missing` | QQ 目标信息不完整，已阻止打开对话框。 |
| `qq_add_friend_api_unavailable` | 当前 QQ 客户端不支持打开好友对话框。 |
| runtime transport error | 打开 QQ 好友对话框失败：<sanitized error> |

All other runtime failures remain failures and may expose only the existing
sanitized error text. `basic.open_id`, the raw visitor response, request bytes,
and native-call arguments are never included in the `ActionResult` returned to
the frontend.

## Testing

Backend coverage will prove that:

- `view_qq` is rejected outside QQ before calling the runtime.
- QQ calls use exactly the selected GID, `verifyMsg: "来自QQ农场"`, and
  `silent: true`.
- a successful native invocation returns a sanitized success result;
- guarded failures return readable errors without exposing OpenID or raw
  payloads.

Frontend coverage will prove that:

- `查看好友QQ` appears in the action menu for QQ and is absent for non-QQ;
- selecting it sends `{ action: "view_qq", target: String(friend.gid) }`;
- the existing action result feedback is shown and duplicate action submission
  is prevented while the request is pending.

## Verification

The implementation is ready only after all focused Go and frontend tests pass,
the existing protocol VM test passes, `go test ./...` passes, and the frontend
production build succeeds. A manual QQ check must confirm that one click opens
the native QQ dialog; it must not be used to assert the final QQ friend result.
