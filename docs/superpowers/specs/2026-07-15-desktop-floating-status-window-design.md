# Desktop Floating Status Window Design

## Goal

Add a portable, always-on-top desktop floating status window so users can monitor Farm_Go while the main window is minimized, covered, or closed to the tray.

The floating window is a compact status card that combines:

- connection / runtime health
- guardian health
- authorization health
- automation running state and run statistics

It also supports light operations without opening the full main UI:

- open main window
- show / hide / collapse
- refresh status
- start / stop automation scheduler

## Scope

### In scope

- Independent second window (not main-window mini-mode)
- Always-on-top behavior
- Tray menu show / hide control
- Compact overview of connection, guardian, authorization, automation, and run statistics
- Light actions: open main window, collapse / expand, close/hide, refresh, start/stop automation
- Draggable positioning with persisted location
- Closing the floating window hides it and does not exit Farm_Go
- Compatibility with existing tray and close-confirmation lifecycle

### Out of scope

- Multi-account switching from the floating window
- Editing automation configuration, land details, full logs, or guardian fine-tuning
- Standalone out-of-process mini-app / float-only boot mode
- Non-Windows platforms
- Fancy edge-snap animations or multi-monitor layout intelligence beyond simple bounds correction
- Mandatory auto-popup on first install

## Success Criteria

- Main window can be hidden to tray while the floating window remains visible and refreshes.
- Tray can show / hide the floating window with one click.
- Displayed state matches main-app semantics for runtime phase, guardian, authorization, scheduler, and statistics.
- Start / stop automation uses existing `StartFarmAutomationScheduler` / `StopFarmAutomationScheduler` behavior.
- Closing the floating window never exits Farm_Go.
- Floating window remains always-on-top while visible.
- Window position, collapsed state, and last visibility preference survive app restarts.

## User Experience

### Window chrome

- Compact tool-window style with no heavy system title bar.
- Custom top bar:
  - title: `Farm_Go 状态`
  - collapse / expand
  - open main window
  - close (hide floating window)
- Default expanded size about `320x220`.
- Collapsed size about `320x56`.
- Always-on-top.
- Drag by top bar.
- Default position: work-area bottom-right with safe margin.
- Remember last user position.

### Expanded content

1. Top bar actions
2. Health row:
   - connection: `正常` / `监听中` / `等待接入` / `异常`
   - guardian: `运行中` / `已暂停` / `关闭`
   - authorization: `已授权` / `心跳异常` / `未授权`
   - automation: `运行中` / `已停止`
3. Current-run statistics:
   - duration
   - collect / farm / steal / help / mischief
   - sale estimate when `estimateReady`, otherwise `-`
4. Latest task line:
   - one latest workbench task result
   - empty copy: `暂无任务结果`
5. Actions:
   - `刷新`
   - `启动自动化` / `停止自动化` based on scheduler state

### Collapsed content

Single summary line with status dot, for example:

- `● 已就绪 · 自动化运行中`

Collapsed mode still supports drag, open main window, and close/hide.

### Display rules

| Action | Behavior |
|---|---|
| Tray "显示悬浮窗" | Create/show and focus floating window |
| Tray "隐藏悬浮窗" | Hide floating window without destroying app process |
| Floating close button | Same as hide; write `visible=false` |
| Tray "显示主窗口" | Restore main window only; leave floating window unchanged |
| Main window minimize-to-tray | Leave floating window as-is |
| App exit | Close main and floating windows together |

### Tray menu

Extend current tray menu to:

1. `显示主窗口`
2. `显示悬浮窗` / `隐藏悬浮窗` (label reflects current visibility)
3. `退出程序`

Tray double-click remains "show main window only".

### Defaults and persistence

- Default: floating window does **not** auto-open on first launch.
- After user opens it from tray, persist `visible=true` and restore on next startup.
- Persist:
  - `floatingWindow.visible`
  - `floatingWindow.collapsed`
  - `floatingWindow.x`
  - `floatingWindow.y`
- Store through existing local settings/storage path used by the app.
- On restore, clamp position into current work area if off-screen.

### Authorization UX

- Unauthorized state is visible in the card.
- Start/stop automation is disabled while unauthorized.
- Open main window, refresh (status only), hide, and collapse remain available.
- Floating window must not bypass the existing license gate.

### Feedback

- Start/stop shows button loading and ignores double clicks.
- Success/failure uses a short inline toast/message inside the floating window.
- Failures do not force the main window open.

## Data Sources

Reuse existing backend bindings and semantics; do not invent a parallel status model.

| UI item | Source |
|---|---|
| Connection phase / target | `RuntimeStatus` |
| Guardian | `GuardianStatus` |
| Authorization | existing license status |
| Automation running state | `FarmAutomationSchedulerState` |
| Start / stop automation | `StartFarmAutomationScheduler` / `StopFarmAutomationScheduler` |
| Run statistics | `FarmWorkspaceRunStatistics` |
| Latest task | `RuntimeEvents` filtered to workbench task events |

### Refresh strategy

- On show: immediate full snapshot fetch
- While visible: light polling every 2–3 seconds
- Manual `刷新`: force immediate refresh
- After start/stop automation: immediately refresh scheduler state and show short result feedback

Optional later enhancement: push status change events via `EventsEmit` for lower latency. Polling is enough for v1.

## Architecture

### Process model

Keep one Farm_Go process:

```text
Farm_Go.exe
├─ Main WebView2 (existing full React app)
├─ Float WebView2 (lightweight React float root)
├─ Tray (systray, extended menu)
└─ Shared Go App bindings / runtime / guardian / automation / license
```

No second process and no remote network surface.

### Why a second WebView window

Wails v2 is single-window by default. Approach A uses a Go-managed secondary WebView2 host window so the product can keep:

- independent float visibility
- always-on-top tool-window behavior
- shared app bindings and authorization
- React visual consistency with the main product

Native HUD and main-window mini-mode were rejected because they either split the design language or violate the independent-second-window requirement.

### Frontend split

- `main.tsx` detects float entry (`#/float` or dedicated float HTML/query entry).
- Main path continues with existing `App` / `AuthorizedApp`.
- Float path mounts `FloatingStatusApp` only.
- Float root must not load sidebar, full tab shell, or heavy workspace views.

### New Go APIs

Window management bindings:

- `ShowFloatingWindow()`
- `HideFloatingWindow()`
- `ToggleFloatingWindow()`
- `GetFloatingWindowState()`
- `SaveFloatingWindowState(state)`
- `OpenMainWindowFromFloat()` (wrapper around `ShowMainWindow`)

Business bindings stay on existing App methods.

### Desktop modules

Keep float concerns beside existing desktop lifecycle code:

- `tray.go` gains show/hide floating window menu item and callback
- `desktop_lifecycle.go` remains responsible for main-window hide/show/exit approval
- new focused module(s) such as `floating_window.go` own:
  - create/show/hide/destroy
  - always-on-top and chrome flags
  - position restore / clamp
  - preference load/save coordination

Do not push business automation logic into the window host module.

### Lifecycle matrix

| Scenario | Behavior |
|---|---|
| App start | If preference `visible=true`, create/show float window after app is ready |
| Hide via tray/close | Hide window; keep process; persist `visible=false` when closed by user hide action |
| Main minimize to tray | No automatic float change |
| Exit application | Destroy float window and stop tray |
| WebView2/float creation failure | Log error; main app still runs; tray action reports failure safely |

Notes:

- Hiding is preferred over tearing down the WebView on every hide, if process memory allows and recreation is expensive.
- If hide-without-destroy is impractical in the chosen WebView host, recreate on next show; preferences still control visibility.

### Security

- Local-only, same as current app.
- Existing `requireAuthorized` checks remain authoritative for automation control.
- Float UI is a view + thin action surface, not a privilege boundary bypass.

## Error Handling

- Float create failure: log, keep main app usable, disable or fail soft on tray toggle with recoverable message.
- Binding/fetch failure: show muted/error state in card; keep last good snapshot when safe.
- Start/stop failure: keep previous running state, show short error message, allow retry.
- Off-screen restore coordinates: clamp into current monitor work area.
- Unauthorized automation action attempt: no-op with disabled UI; do not surface raw internal errors.

## Testing Strategy

### Go

- Tray menu routing includes float show/hide/toggle callbacks without requiring native tray runtime where existing fakes already cover tray wiring.
- Floating window state preference round-trip.
- Hide/close float does not approve application exit.
- Open-main-from-float routes to existing main show path.
- Bounds clamping for restored coordinates.

### Frontend

- Expanded / collapsed rendering of health, stats, and latest task.
- Unauthorized disables start/stop.
- Refresh and start/stop loading/disabled states.
- Empty latest-task and unavailable estimate display.

### Manual Windows checks

- Always-on-top over ordinary apps.
- Drag and restart restores position/collapsed/visible preference.
- Main window in tray while float remains usable.
- Start/stop automation matches main automation page outcome.
- Close float hides only float; exit still only through explicit exit path.

### Commands

```powershell
go test ./...
cd frontend
npm test
npm run build
```

## Implementation Notes

- Reuse overview/status label helpers and workbench task event helpers where practical.
- Keep visual language aligned with existing light workbench styles and status tones.
- Prefer compact grid for stats so the float card does not scroll in the default size.
- Memory trade-off of a second WebView2 is accepted for UI consistency and reuse speed.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Wails lacks first-class multi-window | Isolate custom WebView2 host behind a small desktop module with tests around state APIs |
| Extra memory from second WebView2 | Keep float frontend extremely small; avoid mounting main shell |
| State drift between main and float | Shared backend bindings + frequent snapshot refresh |
| User annoyance from auto popup | Default hidden until user opts in via tray |
| Position lost across monitor changes | Clamp on restore |

## Non-goals reminder

This feature is a portable status companion, not a second full console. If a requested action needs complex configuration or multi-step review, open the main window instead of expanding the float surface.
