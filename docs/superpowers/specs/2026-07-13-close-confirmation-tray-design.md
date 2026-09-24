# Close Confirmation And Tray Design

## Goal

Prevent the native window close control from immediately terminating Farm_Go. The user must explicitly choose whether to exit the application, minimize it to the Windows notification area, or cancel.

## Scope

- Add a Windows notification-area icon with a compact menu.
- Intercept every normal application-close request before Wails shuts down the process.
- Render one frontend confirmation dialog with three actions.
- Preserve existing automation and authorized-runtime work while the window is hidden.

## Interaction

When the user clicks the native close control, the Go close hook prevents the current close attempt and emits a frontend event. The React application opens a modal dialog titled "关闭 Farm_Go" with these actions:

- `退出程序` is the primary action and receives the default keyboard focus.
- `最小化到托盘` hides the main window and leaves the process running.
- `取消` closes the dialog and leaves the window unchanged.

The tray menu includes `显示主窗口` and `退出程序`. Showing the window restores and focuses the hidden application. The tray exit command uses the same one-time explicit-exit path as the dialog.

Repeated native close requests while the dialog is already visible do not create duplicate dialogs.

## Architecture

Add a small Windows tray service around a supported Go system-tray package. It owns the tray icon and routes its two menu actions through callbacks supplied by the application. The service is started with the Wails application and stopped during shutdown.

The `App` owns close-state coordination:

- `beforeClose` prevents ordinary close requests and emits `app:close-requested`.
- `ExitApplication` marks exactly one close request as approved, then invokes Wails runtime quit.
- `MinimizeToTray` hides the Wails window through the lifecycle context.
- `ShowMainWindow` restores the Wails window for the tray callback.

The one-time approval marker prevents the Wails `OnBeforeClose` hook from reopening the dialog when an explicit exit calls runtime quit. The marker is consumed immediately, so later native closes are intercepted normally.

The frontend adds a focused `CloseConfirmationDialog` component to the authorized application boundary. It listens for `app:close-requested`, invokes the bound `ExitApplication` or `MinimizeToTray` action, and removes the event subscription on unmount. The dialog uses the existing overlay, buttons, icon, and responsive styles.

## Error Handling

Tray initialization failure is logged and does not prevent the application from starting. In that case, `最小化到托盘` must not be presented as an available action, because hiding a window with no recovery path is not acceptable. The normal close confirmation still offers `退出程序` and `取消`.

Window show/hide and explicit exit are no-op-safe when lifecycle context is unavailable during teardown. The application shutdown sequence remains responsible for stopping services and closing storage exactly once.

## Verification

- Go unit tests cover interception, one-time explicit-exit approval, and the tray callback routing without starting a native tray.
- React component tests cover dialog visibility, primary exit action, minimize action, cancel behavior, and listener cleanup.
- Run `go test ./...`, `npm test`, and `npm run build`.
- Manually verify on Windows: native close opens one dialog; exit terminates; minimize hides to the notification area; tray show restores; tray exit terminates.
