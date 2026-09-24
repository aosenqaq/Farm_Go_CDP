# WebView2 Data Directory Cleanup Design

## Goal

Prevent versioned Farm_Go executable names from creating a new WebView2 data directory in `%APPDATA%` on every release, and remove the historical WebView2-only directories without touching Farm_Go business data.

## Root Cause

Farm_Go does not currently set Wails' Windows `WebviewUserDataPath`. Wails v2.12.0 therefore uses `%APPDATA%\[BinaryName.exe]`. Release files such as `Farm_Go_V1.0.4.exe` consequently create directories with the same versioned names. Inspection confirms that these directories contain an `EBWebView` child and are separate from the application's SQLite and resource data in `%APPDATA%\Farm_Go`.

## Stable Data Path

The application will set the Wails Windows `WebviewUserDataPath` to:

```text
%APPDATA%\Farm_Go\webview2
```

The path is derived from the existing `defaultDataDir()` result so the WebView2 profile and existing application data share the stable Farm_Go root while remaining in separate subdirectories. Renaming a future executable will no longer change the WebView2 profile location.

## Historical Directory Cleanup

Before starting Wails, the application performs a best-effort scan of the directory containing the stable Farm_Go data root. A directory is eligible for removal only when both conditions are true:

- Its name starts with `Farm_Go` and ends with `.exe`, using case-insensitive comparison.
- It contains an `EBWebView` directory directly beneath it.

This includes release, internal-test, verification, development, and VMP-generated names such as `Farm_Go_V1.0.4.exe`, `Farm_Go内测版V1.0.0.exe`, and `Farm_Go.vmp.exe`.

The cleaner will not remove `%APPDATA%\Farm_Go`, the new `%APPDATA%\Farm_Go\webview2` directory, a similarly named directory without `EBWebView`, regular files, or entries outside the resolved `%APPDATA%` parent. Cleanup is limited to direct children of that parent; it does not recursively search for candidates.

## Startup And Failure Handling

Cleanup runs before `wails.Run`, after the stable paths have been calculated. Failure to read the parent directory or remove an individual candidate does not prevent Farm_Go from starting. This covers locked profiles belonging to an older Farm_Go process: the locked directory is skipped and can be removed on a later launch.

The fixed WebView2 path is always applied even if historical cleanup fails. Cleanup errors are reported through standard logging with the affected directory and error, but no retry loop or forced process termination is added.

## Components

- A small path helper derives the stable WebView2 directory from the existing application data directory.
- A cleanup helper owns candidate recognition and best-effort removal. Its core accepts the application data root and a removal operation; production passes `os.RemoveAll`, while tests can deterministically simulate one failed removal.
- `main.go` supplies the stable path through `windows.Options.WebviewUserDataPath` and invokes cleanup before Wails starts.

No frontend changes, settings, user prompts, database migration, or release-script changes are required.

## Verification

Go unit tests will use temporary directories to verify:

- Versioned and non-versioned `Farm_Go*.exe` directories containing `EBWebView` are removed.
- `%APPDATA%\Farm_Go` and its contents are preserved.
- Similar names without `EBWebView`, ordinary files, and unrelated application directories are preserved.
- A simulated removal failure for one candidate does not prevent other candidates from being removed or cause startup configuration to fail.
- The derived WebView2 path is always `<dataDir>\webview2`.

After implementation, run the focused tests followed by `go test ./...` and a normal Go build to verify that the Windows Wails options compile.
