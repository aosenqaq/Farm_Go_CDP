# Cache Maintenance Tools Design

## Goal

Port the QQ, WeChat, and YYB mini-program cache maintenance tools from the legacy Tauri project into Farm_Go's system settings. The Wails application must perform the cleanup locally, preserve recoverable backups, and avoid touching the legacy application's data directory.

## Scope

The system settings page gains a `缓存维护` section with three independent actions:

- `清理微信小程序缓存`
- `清理 QQ 小程序缓存`
- `清理应用宝小程序缓存`

Each action closes only the named client's relevant processes, moves matching cache paths into a timestamped backup directory on the Desktop, and returns the number of stopped processes, moved paths, skipped paths, and the backup directory. No action permanently deletes a cache path.

The migration must not inspect, move, or edit the legacy `site.dank1ng.farm` directory or its configuration.

## Architecture

Create `internal/maintenance` as the sole owner of destructive maintenance operations. It exposes three methods to the root Wails `App`, one per client platform. The service resolves candidates only from Windows environment locations, creates a unique backup directory named `Farm_Go-<platform>-cache-backup-<timestamp>`, closes a fixed allowlist of process image names, and moves existing allowlisted paths into that directory.

The root `App` exposes typed Wails methods. `SettingsView` calls them and renders action-local progress and result feedback. The frontend never supplies a filesystem path to the backend.

## Cleanup Boundaries

### WeChat

The target root is `%APPDATA%\Tencent\xwechat`. Candidate paths reproduce the source tool's WMPF and mini-program cache allowlist:

- `xplugin\Plugins\RadiumWMPF`
- `xplugin\Plugins\WMPFDrm`
- `radium\cache`
- selected `radium\mmkv` runtime dictionaries and XWeb storage
- `radium\users\*\applet` and `radium\users\*\xworker`
- `radium\web\profiles\game*` and `radium\web\profiles\webview*`

The service rejects candidates outside the xwechat root and rejects path segments associated with chat, media, attachment, backup, or file storage. It closes `WeChat.exe`, `WeChatAppEx.exe`, `WeChatPlayer.exe`, `Weixin.exe`, and `WeixinAppEx.exe` before applying. The UI previews the candidate list before the final confirmation.

### QQ

Candidates are limited to `%APPDATA%` and `%LOCALAPPDATA%` paths under `QQ` or `QQEX`:

- `QQ\miniapp\temps\miniapp_src`
- `QQ\miniapp\temps\miniapp_pkgs`
- `QQEX\miniapp\temps\miniapp_src`
- `QQEX\miniapp\temps\miniapp_pkgs`

It closes `QQ.exe`, `QQNT.exe`, `QQMiniProgram.exe`, `QQMiniApp.exe`, and `QQEX.exe`. The source tool's legacy discovery-cache and pinned-config reset steps are deliberately omitted.

The UI uses two explicit confirmations because the operation can close the main QQ client.

### YYB

The service discovers only directories named `Androws` or `Tencent\Androws` under `%APPDATA%`, `%LOCALAPPDATA%`, `%PROGRAMDATA%`, and standard Program Files roots. It considers the source tool's cache allowlist under those roots:

- `WmpfRuntime`, `Data`, `User Data`, `Cache`, `Temp`, and `logs`
- the same cache folders beneath child runtime directories
- `Application\Cache`, `Application\User Data`, and `Application\Temp`

It closes `AndrowsLauncher.exe`, `AndrowsStore.exe`, `WeChatAppEx.exe` only when it is running from an Androws location, and `QQMiniApp.exe` only when it is running from an Androws location. It does not scan arbitrary drive roots, does not delete application binaries, and does not touch the source tool's `site.dank1ng.farm` cache files.

The UI asks for one confirmation and explains that it closes the YYB runtime processes.

## Failure Handling

Each move validates that the resolved path is inside its approved root. A path absent at execution is counted as skipped. A failure to move an existing path returns a descriptive error and stops that operation, leaving already moved paths in the backup directory. Process termination failures are recorded in the result but do not prevent cleanup attempts; a locked cache path returns an error rather than being removed forcibly.

When no candidate exists, an operation succeeds with zero moved paths. The UI reports that no matching cache was found and still displays the intended backup directory only if it was created.

## UI And Feedback

`SettingsView` adds the three controls under a compact `缓存维护` subsection. A single active operation disables all maintenance controls. On success, the view shows stopped-process and moved-path counts plus the backup location, then asks the user to restart the affected client and open the farm mini-program once to regenerate its runtime cache. On failure, the backend error is displayed without hiding the partial-result backup location when one is available.

## Verification

- Unit tests construct temporary roots and verify each allowlist, backup move, root boundary, process list, and zero-target result.
- App-level tests verify that the root `App` exposes the three maintenance methods.
- `SettingsView` tests verify the three controls, WeChat preview confirmation, QQ double confirmation, YYB confirmation, disabled busy state, and visible result/error feedback.
- Run `go test ./...`, `npm test`, and `npm run build` from `frontend`.
