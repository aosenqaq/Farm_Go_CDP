# YYB Injection Launch Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the startup injection dialog's YYB launch action resolve the installed `AndrowsLauncher.exe` and directly open QQ Classic Farm whether YYB is already running or closed.

**Architecture:** Keep the React dialog and Wails API unchanged. Add a filesystem-aware launch-request resolver to the guard package, then have `App.LaunchHostProcess` supply live process snapshots to it before dispatching the existing QQ Farm shortcut parameters.

**Tech Stack:** Go 1.23, Wails v2, Windows process snapshots and `ShellExecute`, Go `testing`

---

## File Structure

- Modify `internal/runtime/guard/launch.go`: resolve and validate the YYB launcher while preserving QQ and WeChat protocol requests.
- Modify `internal/runtime/guard/host_test.go`: cover process-derived paths, common-directory fallback, missing-launcher errors, and unchanged protocol requests.
- Modify `app.go`: resolve the launch request from live snapshots before calling the platform adapter.
- Modify `app_test.go`: verify the Wails launch method dispatches the resolved YYB shortcut and reports snapshot failures.

### Task 1: Resolve The Installed YYB Launcher

**Files:**
- Modify: `internal/runtime/guard/launch.go`
- Test: `internal/runtime/guard/host_test.go`

- [ ] **Step 1: Write failing resolver tests**

Add tests in package `guard` that use injected environment and file-existence functions so they do not depend on the developer machine:

```go
func TestResolveLaunchRequestForPlatformFindsYYBLauncherFromRunningWMPFProcess(t *testing.T) {
	launcher := filepath.Clean(`E:\Apps\Tencent\Androws\Application\AndrowsLauncher.exe`)
	request, err := resolveLaunchRequestForPlatform(
		"yyb_cdp",
		[]HostProcessSnapshot{{
			ProcessName:    "WeChatAppEx.exe",
			ExecutablePath: `E:\Apps\Tencent\Androws\WmpfRuntime\5.10.2700.327\runtime\WeChatAppEx.exe`,
		}},
		func(string) string { return "" },
		func(path string) bool { return filepath.Clean(path) == launcher },
	)
	if err != nil {
		t.Fatal(err)
	}
	if request.TargetDisplayName != launcher {
		t.Fatalf("target = %q, want %q", request.TargetDisplayName, launcher)
	}
	if request.WorkingDirectory != filepath.Dir(launcher) {
		t.Fatalf("working directory = %q", request.WorkingDirectory)
	}
	if !strings.Contains(request.Parameters, "launchWithShortcut?pkgname=wx5306c5978fdb76e4") {
		t.Fatalf("parameters do not open QQ Classic Farm: %q", request.Parameters)
	}
}

func TestResolveLaunchRequestForPlatformFindsRootLevelYYBInstall(t *testing.T) {
	launcher := filepath.Clean(`D:\Androws\Application\AndrowsLauncher.exe`)
	request, err := resolveLaunchRequestForPlatform(
		"yyb",
		[]HostProcessSnapshot{{
			ProcessName:    "WeChatAppEx.exe",
			ExecutablePath: `D:\Androws\WmpfRuntime\5.10.2700.327\runtime\WeChatAppEx.exe`,
		}},
		func(string) string { return "" },
		func(path string) bool { return filepath.Clean(path) == launcher },
	)
	if err != nil || request.TargetDisplayName != launcher {
		t.Fatalf("request = %#v, err = %v", request, err)
	}
}

func TestResolveLaunchRequestForPlatformFallsBackToCommonYYBDirectory(t *testing.T) {
	launcher := filepath.Clean(`C:\CustomProgramFiles\Tencent\Androws\Application\AndrowsLauncher.exe`)
	request, err := resolveLaunchRequestForPlatform(
		"yyb",
		nil,
		func(name string) string {
			if name == "ProgramFiles" {
				return `C:\CustomProgramFiles`
			}
			return ""
		},
		func(path string) bool { return filepath.Clean(path) == launcher },
	)
	if err != nil || request.TargetDisplayName != launcher {
		t.Fatalf("request = %#v, err = %v", request, err)
	}
}

func TestResolveLaunchRequestForPlatformRejectsMissingYYBLauncher(t *testing.T) {
	request, err := resolveLaunchRequestForPlatform(
		"yyb",
		nil,
		func(string) string { return "" },
		func(string) bool { return false },
	)
	if err == nil || !strings.Contains(err.Error(), "未找到应用宝安装目录") {
		t.Fatalf("request = %#v, err = %v", request, err)
	}
}

func TestResolveLaunchRequestForPlatformKeepsProtocolLaunches(t *testing.T) {
	for _, target := range []string{"qq_ws", "wechat_cdp"} {
		request, err := resolveLaunchRequestForPlatform(
			target,
			nil,
			func(string) string { return "" },
			func(string) bool { return false },
		)
		if err != nil || request.Mode != "protocol" || request.Protocol == "" {
			t.Fatalf("%s request = %#v, err = %v", target, request, err)
		}
	}
}
```

- [ ] **Step 2: Run the resolver tests to verify RED**

Run:

```powershell
go test ./internal/runtime/guard -run 'TestResolveLaunchRequestForPlatform' -count=1
```

Expected: compilation fails because `resolveLaunchRequestForPlatform` does not exist.

- [ ] **Step 3: Implement the minimal resolver**

Update `internal/runtime/guard/launch.go` with these responsibilities:

```go
func ResolveLaunchRequestForPlatform(platform string, snapshots []HostProcessSnapshot) (LaunchRequest, error) {
	return resolveLaunchRequestForPlatform(platform, snapshots, os.Getenv, regularFileExists)
}

func resolveLaunchRequestForPlatform(
	platform string,
	snapshots []HostProcessSnapshot,
	getenv func(string) string,
	fileExists func(string) bool,
) (LaunchRequest, error) {
	if normalizePlatform(platform) != "yyb" {
		return launchRequestForPlatform(platform, nil)
	}
	for _, directory := range yybApplicationDirectoryCandidates(snapshots, getenv) {
		launcher := filepath.Join(directory, yybTargetDisplayName)
		if !fileExists(launcher) {
			continue
		}
		return LaunchRequest{
			Mode:              "yyb_shortcut",
			TargetDisplayName: launcher,
			WorkingDirectory:  directory,
			Parameters:        yybLaunchParameters,
		}, nil
	}
	return LaunchRequest{}, errors.New("未找到应用宝安装目录：请确认 Tencent\\Androws\\Application 中存在 AndrowsLauncher.exe")
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
```

Add `yybApplicationDirectoryCandidates` and a case-insensitive deduplication helper. Candidate order must be:

```go
// 1. infer from snapshot.ExecutablePath under Tencent\Androws\WmpfRuntime
//    or root-level Androws\WmpfRuntime;
// 2. ProgramFiles, ProgramFiles(x86), LOCALAPPDATA, ProgramData;
// 3. C/D fixed Program Files, Program Files (x86), and root-level Androws paths.
```

Use `filepath.Clean`, `filepath.Dir`, and `filepath.Join`; do not scan unrelated directories recursively. Preserve `LaunchRequestForPlatform` for existing callers and tests.

- [ ] **Step 4: Run focused guard tests to verify GREEN**

Run:

```powershell
go test ./internal/runtime/guard -run 'TestResolveLaunchRequestForPlatform|TestLaunchRequestForPlatform' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run the complete guard package**

Run:

```powershell
go test ./internal/runtime/guard -count=1
```

Expected: PASS with zero failures.

- [ ] **Step 6: Commit the resolver**

```powershell
git add internal/runtime/guard/launch.go internal/runtime/guard/host_test.go
git commit -m "fix: resolve installed YYB launcher"
```

### Task 2: Route The Injection Dialog Launch Through The Resolver

**Files:**
- Modify: `app.go`
- Test: `app_test.go`

- [ ] **Step 1: Write failing application tests**

Add tests near the existing host launch and restart coverage:

```go
func TestAppLaunchHostProcessResolvesYYBMiniappShortcut(t *testing.T) {
	app := newAuthorizedTestApp(t)
	app.cfg.Runtime.CurrentTarget = "yyb_cdp"
	root := t.TempDir()
	launcher := filepath.Join(root, "Tencent", "Androws", "Application", "AndrowsLauncher.exe")
	if err := os.MkdirAll(filepath.Dir(launcher), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, []byte("launcher"), 0o644); err != nil {
		t.Fatal(err)
	}
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
		return []guard.HostProcessSnapshot{{
			ProcessName: "WeChatAppEx.exe",
			ExecutablePath: filepath.Join(
				root, "Tencent", "Androws", "WmpfRuntime", "5.10.2700.327", "runtime", "WeChatAppEx.exe",
			),
		}}, nil
	}
	var launched guard.LaunchRequest
	app.launchHost = func(request guard.LaunchRequest) error {
		launched = request
		return nil
	}

	result := app.LaunchHostProcess()

	if !result.LaunchDispatched || result.Status != "launch_dispatched" {
		t.Fatalf("result = %#v", result)
	}
	if launched.TargetDisplayName != launcher || launched.WorkingDirectory != filepath.Dir(launcher) {
		t.Fatalf("launch request = %#v", launched)
	}
	if !strings.Contains(launched.Parameters, "launchWithShortcut?pkgname=wx5306c5978fdb76e4") {
		t.Fatalf("launch request does not open QQ Classic Farm: %#v", launched)
	}
}

func TestAppLaunchHostProcessReportsYYBSnapshotFailure(t *testing.T) {
	app := newAuthorizedTestApp(t)
	app.cfg.Runtime.CurrentTarget = "yyb_cdp"
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
		return nil, errors.New("snapshot failed")
	}
	called := false
	app.launchHost = func(guard.LaunchRequest) error {
		called = true
		return nil
	}

	result := app.LaunchHostProcess()

	if result.Status != "launch_failed" || !strings.Contains(result.Reason, "snapshot failed") {
		t.Fatalf("result = %#v", result)
	}
	if called {
		t.Fatal("launcher was called after snapshot failure")
	}
}
```

- [ ] **Step 2: Run the application tests to verify RED**

Run:

```powershell
go test . -run 'TestAppLaunchHostProcess' -count=1
```

Expected: the YYB request still contains the relative `AndrowsLauncher.exe` and fixed D: working directory, so the first test fails.

- [ ] **Step 3: Implement resolver integration**

Update `App.LaunchHostProcess()` in `app.go`:

```go
func (a *App) LaunchHostProcess() guard.HostLaunchResult {
	if a.requireAuthorized("LaunchHostProcess") != nil {
		return guard.HostLaunchResult{Owner: "main", Status: "license_required", Reason: ErrLicenseRequired.Error()}
	}
	platform := a.guardPlatform()
	var snapshots []guard.HostProcessSnapshot
	if platform == "yyb" {
		var err error
		snapshots, err = a.listHostSnapshots()
		if err != nil {
			a.lastErr = err
			return guard.HostLaunchResult{Owner: "main", Platform: platform, Status: "launch_failed", Reason: err.Error()}
		}
	}
	request, err := guard.ResolveLaunchRequestForPlatform(platform, snapshots)
	if err != nil {
		a.lastErr = err
		return guard.HostLaunchResult{Owner: "main", Platform: platform, Status: "launch_failed", Reason: err.Error()}
	}
	if err := a.launchHost(request); err != nil {
		a.lastErr = err
		return guard.HostLaunchResult{Owner: "main", Platform: platform, Status: "launch_failed", Reason: err.Error()}
	}
	return guard.HostLaunchResult{Owner: "main", Platform: platform, Status: "launch_dispatched", Reason: "launch was dispatched", LaunchDispatched: true}
}
```

The React callback remains `onLaunch={launchHost}` and the Wails signature remains `LaunchHostProcess(): Promise<guard.HostLaunchResult>`.

- [ ] **Step 4: Run focused application tests to verify GREEN**

Run:

```powershell
go test . -run 'TestAppLaunchHostProcess' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run all Go tests**

Run:

```powershell
go test ./... -count=1
```

Expected: PASS with zero failures.

- [ ] **Step 6: Commit the application integration**

```powershell
git add app.go app_test.go
git commit -m "fix: launch YYB miniapp from injection dialog"
```

### Task 3: Final Regression Verification

**Files:**
- Verify only; no planned source changes.

- [ ] **Step 1: Run frontend tests**

Run:

```powershell
npm test -- --run
```

Working directory: `frontend`

Expected: all Vitest suites pass.

- [ ] **Step 2: Build the frontend production bundle**

Run:

```powershell
npm run build
```

Working directory: `frontend`

Expected: TypeScript and Vite complete with exit code 0.

- [ ] **Step 3: Run final Go verification**

Run:

```powershell
go test ./... -count=1
```

Expected: PASS with zero failures.

- [ ] **Step 4: Inspect the final diff**

Run:

```powershell
git diff HEAD~2 --check
git status --short
```

Expected: no whitespace errors; only the planned implementation files are changed or committed.
