# Guardian Restart Auto-Minimize Design

## Goal

Allow the process guardian to minimize the restarted mini-program host window after its runtime connection is stable, controlled by a dedicated persisted setting in the process restart section.

## Scope

- Add a default-disabled `processGuardAutoMinimizeAfterRestart` runtime setting.
- Show a dedicated `重启后自动最小化窗口` switch in the `进程异常与定时重启` guardian section.
- Apply the setting to all guardian restart paths: automatic failure recovery, scheduled restart, and the guardian page's manual restart action.
- Never minimize the Farm_Go window.
- Never minimize a host during its initial launch or before its runtime reports a stable connection.

## Design

### Settings and UI

`storage.RuntimeSettings` will own the new boolean. The settings store will load and save it under the `processGuard.autoMinimizeAfterRestart` key, with a false zero/default value for existing installations. The backend's guardian settings save path will accept an `autoMinimizeAfterRestart` input and return it through the guardian status DTO. `GuardView` will render its own toggle in the process guardian section and persist changes through the existing `onSaveSettings` callback.

### Restart and Stable-Connection Flow

When a guardian-initiated restart returns `launch_dispatched` and the setting is enabled, `App` records one pending minimize request for the runtime target that was restarted. It does not minimize at process launch.

When the runtime status pipeline later receives the first snapshot for that target where both `Ready` and `Connected` are true, it consumes the pending request. It refreshes the host binding, obtains the visible window handles for that bound host process, and asks the Windows host helper to minimize them. The pending request is consumed whether minimization succeeds or fails, so ordinary healthy-status polling cannot repeatedly minimize a user-restored host window.

The pending request is protected by the app's synchronization primitives and cleared if the setting is disabled before connection stabilizes. A restart that fails before its launch is dispatched creates no pending request.

### Windows Boundary

The guard Windows process helper will expose a focused function that minimizes a supplied host-window handle with the native Windows `ShowWindow(SW_MINIMIZE)` call. Non-Windows builds retain a no-op/unsupported implementation consistent with the package's existing platform split. The app layer only passes handles obtained from its current, confirmed host binding; it does not minimize windows selected by title alone.

## Error Handling and Observability

Failure to rediscover the binding or minimize a window must not alter guardian recovery state or make the successful restart appear failed. The app records a guardian event for successful and failed deferred-minimize attempts, including runtime target and error detail when applicable.

## Tests

- Storage round-trip covers the new setting and verifies legacy settings default to false.
- Backend setting save tests cover acceptance and propagation of the new input.
- App tests prove a successful restart queues one request only when enabled, a stable `Ready && Connected` snapshot consumes it and minimizes the bound host windows, and a disabled setting or unsuccessful launch does not minimize anything.
- Windows helper tests cover the handle dispatch boundary through an injectable/native-call wrapper where practical.
- `GuardView` rendering test asserts the dedicated switch is present in the process-restart section.
