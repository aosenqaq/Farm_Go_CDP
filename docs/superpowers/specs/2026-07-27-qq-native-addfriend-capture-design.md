# QQ Native Add-Friend Capture Design

## Goal

Capture the QQ mini-game client's native add-friend invocation that follows a
manual click on the stranger-farm `添加QQ好友` button. The capture must preserve
normal client behavior and expose any target ID, URL, scheme, and callback data
available at the JavaScript boundary.

## Scope

Update the runtime spy in `resources/wmpf/button.js` only.

- Discover QQ platform roots: `qq`, `GameGlobal.qq`, `BK`, `GameGlobal.BK`,
  `BK.QQ`, and `GameGlobal.BK.QQ` when present.
- Wrap callable root methods whose names indicate a native friend, profile,
  card, or external-launch action: `friend`, `add`, `profile`, `card`, `open`,
  `url`, `launch`, `scheme`, or `contact`.
- Record native calls and callback payloads without changing their arguments,
  return values, or execution order.
- Reuse `shareApiEvents` for compatibility with the existing capture listener.
  Each new event carries `apiKind: "nativeFriend"` so consumers can distinguish
  it from normal sharing events.

## Non-Goals

- Do not call, replay, block, redirect, or synthesize any QQ native API.
- Do not construct an add-friend URL when no runtime call exposes one.
- Do not modify the farm protocol or invoke friend-entry methods.
- Do not modify `resources/gameConfig.bundle.zip` or unrelated runtime code.

## Data Flow

When `gameCtl.startRuntimeSpies()` runs, the existing installer enumerates the
known QQ roots and wraps matching methods. On the user's manual click, the
wrapper appends one `shareApiEvents` item before delegating to the original
method. Callback options are wrapped to append their result in the same event
stream. `getRuntimeSpySnapshot()` and the existing protocol capture script then
persist those records to the usual raw, NDJSON, and extracted artifacts.

An event contains:

```json
{
  "apiKind": "nativeFriend",
  "action": "call",
  "platform": "GameGlobal.qq",
  "method": "addFriend",
  "args": []
}
```

Callback events add `callback: "success"`, `"fail"`, or `"complete"` and the
summarized callback arguments.

## Error Handling

Missing platform roots and inaccessible properties are skipped. A wrapper is
installed at most once and errors while summarizing a value fall back to the
existing safe runtime summarizers. The original native API is always invoked
with its original receiver and arguments.

## Tests

Add a regression test before implementation that verifies the embedded runtime
script declares the native capture installer, covers all intended QQ roots and
method-name filter categories, emits `apiKind: "nativeFriend"`, and invokes the
installer from `installRuntimeSpies()`. The test is expected to fail before the
production script changes and pass after the minimal implementation.

## Acceptance Criteria

1. A clean runtime capture after the manual button click contains a native API
   event when the QQ client exposes a matching add-friend, profile, card, or
   deep-link method.
2. The recorded event preserves the method name and summarized arguments,
   including an URL or `mqqapi` value when supplied by the runtime.
3. Existing share API capture remains intact.
4. The focused Go test and the full Go test suite pass.
