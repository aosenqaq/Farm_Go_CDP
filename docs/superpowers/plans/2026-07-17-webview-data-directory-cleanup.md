# WebView2 Data Directory Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Farm_Go one stable WebView2 profile directory and safely remove historical profile directories named after versioned executables.

**Architecture:** Add a focused `webview_data.go` module in the root `main` package. It derives the stable path, recognizes only direct `%APPDATA%` children matching `Farm_Go*.exe` with an `EBWebView` child, and performs best-effort removal through an injected removal function; `main.go` invokes it before `wails.Run` and supplies the stable Wails Windows options.

**Tech Stack:** Go 1.25, Wails v2.12 Windows options, Go standard library filesystem APIs, Go `testing` package.

---

## File Structure

- Create `webview_data.go`: stable WebView2 path, Wails Windows option construction, and legacy-profile cleanup.
- Create `webview_data_test.go`: temporary-filesystem behavior tests and deterministic removal-failure coverage.
- Modify `main.go`: invoke cleanup before Wails starts, log cleanup failures, and apply the stable Windows options.

### Task 1: Stable Path And Selective Legacy Cleanup

**Files:**
- Create: `webview_data.go`
- Create: `webview_data_test.go`

- [ ] **Step 1: Write failing path and filesystem behavior tests**

Create `webview_data_test.go` with tests that prove the stable subdirectory, eligible-directory removal, lookalike protection, and per-directory failure isolation:

```go
package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWebviewDataDirUsesStableApplicationRoot(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "Farm_Go")
	want := filepath.Join(dataDir, "webview2")
	if got := webviewDataDir(dataDir); got != want {
		t.Fatalf("webviewDataDir() = %q, want %q", got, want)
	}
}

func TestCleanupLegacyWebviewDataDirsRemovesOnlyRecognizedProfiles(t *testing.T) {
	appDataRoot := t.TempDir()
	dataDir := filepath.Join(appDataRoot, "Farm_Go")
	mustMkdirAll(t, filepath.Join(dataDir, "data"))
	mustWriteFile(t, filepath.Join(dataDir, "data", "farm_go.db"))

	removeNames := []string{
		"Farm_Go.exe",
		"Farm_Go_V1.0.4.exe",
		"farm_go-dev.EXE",
		"Farm_Go内测版V1.0.0.exe",
	}
	for _, name := range removeNames {
		mustMkdirAll(t, filepath.Join(appDataRoot, name, "EBWebView", "Default"))
	}

	preserveNames := []string{
		"Farm_Go_V1.0.5.exe",
		"AnotherApp.exe",
	}
	mustMkdirAll(t, filepath.Join(appDataRoot, preserveNames[0], "not-webview-data"))
	mustMkdirAll(t, filepath.Join(appDataRoot, preserveNames[1], "EBWebView"))
	mustWriteFile(t, filepath.Join(appDataRoot, "Farm_Go_file.exe"))

	if cleanupErrors := cleanupLegacyWebviewDataDirs(dataDir, os.RemoveAll); len(cleanupErrors) != 0 {
		t.Fatalf("cleanup errors = %v", cleanupErrors)
	}

	for _, name := range removeNames {
		if _, err := os.Stat(filepath.Join(appDataRoot, name)); !os.IsNotExist(err) {
			t.Errorf("legacy profile %q still exists; stat error = %v", name, err)
		}
	}
	for _, name := range preserveNames {
		if _, err := os.Stat(filepath.Join(appDataRoot, name)); err != nil {
			t.Errorf("protected directory %q was changed: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, "data", "farm_go.db")); err != nil {
		t.Fatalf("Farm_Go business data was changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDataRoot, "Farm_Go_file.exe")); err != nil {
		t.Fatalf("matching regular file was changed: %v", err)
	}
}

func TestCleanupLegacyWebviewDataDirsContinuesAfterRemovalFailure(t *testing.T) {
	appDataRoot := t.TempDir()
	dataDir := filepath.Join(appDataRoot, "Farm_Go")
	failedName := "Farm_Go_A.exe"
	removedName := "Farm_Go_B.exe"
	for _, name := range []string{failedName, removedName} {
		mustMkdirAll(t, filepath.Join(appDataRoot, name, "EBWebView"))
	}

	removeAll := func(path string) error {
		if filepath.Base(path) == failedName {
			return errors.New("profile is locked")
		}
		return os.RemoveAll(path)
	}
	cleanupErrors := cleanupLegacyWebviewDataDirs(dataDir, removeAll)

	if len(cleanupErrors) != 1 {
		t.Fatalf("cleanup errors = %v, want one error", cleanupErrors)
	}
	if _, err := os.Stat(filepath.Join(appDataRoot, failedName)); err != nil {
		t.Fatalf("failed profile should remain for a later launch: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDataRoot, removedName)); !os.IsNotExist(err) {
		t.Fatalf("later removable profile still exists; stat error = %v", err)
	}
}

func TestCleanupLegacyWebviewDataDirsReportsUnreadableParent(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "missing", "Farm_Go")
	if cleanupErrors := cleanupLegacyWebviewDataDirs(dataDir, os.RemoveAll); len(cleanupErrors) != 1 {
		t.Fatalf("cleanup errors = %v, want one parent read error", cleanupErrors)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```powershell
go test . -run 'Test(WebviewDataDir|CleanupLegacyWebviewDataDirs)' -count=1
```

Expected: build failure because `webviewDataDir` and `cleanupLegacyWebviewDataDirs` do not exist.

- [ ] **Step 3: Implement the minimum path and cleanup helpers**

Create `webview_data.go`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func webviewDataDir(dataDir string) string {
	return filepath.Join(dataDir, "webview2")
}

func cleanupLegacyWebviewDataDirs(dataDir string, removeAll func(string) error) []error {
	parent := filepath.Dir(filepath.Clean(dataDir))
	entries, err := os.ReadDir(parent)
	if err != nil {
		return []error{fmt.Errorf("read legacy WebView2 data parent %q: %w", parent, err)}
	}

	var cleanupErrors []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasPrefix(name, "farm_go") || !strings.HasSuffix(name, ".exe") {
			continue
		}

		candidate := filepath.Join(parent, entry.Name())
		webviewInfo, err := os.Stat(filepath.Join(candidate, "EBWebView"))
		if err != nil || !webviewInfo.IsDir() {
			continue
		}
		if err := removeAll(candidate); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove legacy WebView2 data %q: %w", candidate, err))
		}
	}
	return cleanupErrors
}
```

- [ ] **Step 4: Format and verify GREEN**

Run:

```powershell
gofmt -w webview_data.go webview_data_test.go
go test . -run 'Test(WebviewDataDir|CleanupLegacyWebviewDataDirs)' -count=1
```

Expected: all focused tests pass.

- [ ] **Step 5: Commit the tested cleanup core**

```powershell
git add -- webview_data.go webview_data_test.go
git commit -m "feat: clean legacy WebView2 data directories"
```

### Task 2: Wire Stable WebView2 Options Into Startup

**Files:**
- Modify: `webview_data.go`
- Modify: `webview_data_test.go`
- Modify: `main.go:3-13,37-65`

- [ ] **Step 1: Write the failing Windows-options test**

Add this test to `webview_data_test.go`:

```go
func TestAppWindowsOptionsUsesStableWebviewDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "Farm_Go")
	got := appWindowsOptions(dataDir)
	want := filepath.Join(dataDir, "webview2")
	if got.WebviewUserDataPath != want {
		t.Fatalf("WebviewUserDataPath = %q, want %q", got.WebviewUserDataPath, want)
	}
}
```

- [ ] **Step 2: Run the new test and verify RED**

Run:

```powershell
go test . -run TestAppWindowsOptionsUsesStableWebviewDataDir -count=1
```

Expected: build failure because `appWindowsOptions` does not exist.

- [ ] **Step 3: Add the tested Wails options constructor**

Add the Wails Windows options import to `webview_data.go`:

```go
import "github.com/wailsapp/wails/v2/pkg/options/windows"
```

Add:

```go
func appWindowsOptions(dataDir string) *windows.Options {
	return &windows.Options{WebviewUserDataPath: webviewDataDir(dataDir)}
}
```

- [ ] **Step 4: Wire cleanup and stable options into `main.go`**

Add standard-library imports:

```go
"log/slog"
"os"
```

Immediately after `app := NewApp()`, add:

```go
for _, cleanupErr := range cleanupLegacyWebviewDataDirs(app.dataDir, os.RemoveAll) {
	slog.Warn("clean legacy WebView2 data directory", "error", cleanupErr)
}
```

Add the Windows field to `options.App`:

```go
Windows: appWindowsOptions(app.dataDir),
```

- [ ] **Step 5: Format and verify the startup integration**

Run:

```powershell
gofmt -w main.go webview_data.go webview_data_test.go
go test . -run 'Test(AppWindowsOptions|WebviewDataDir|CleanupLegacyWebviewDataDirs)' -count=1
$buildCheck = Join-Path $env:TEMP "farm-go-build-check-$PID.exe"
go build -o $buildCheck .
Remove-Item -LiteralPath $buildCheck -Force
```

Expected: focused tests pass and the root package builds successfully with Wails' `Windows` option populated.

- [ ] **Step 6: Commit the startup wiring**

```powershell
git add -- main.go webview_data.go webview_data_test.go
git commit -m "fix: use stable WebView2 profile directory"
```

### Task 3: Full Regression Verification

**Files:**
- Verify only; no planned file changes.

- [ ] **Step 1: Run the complete Go test suite**

```powershell
go test ./...
```

Expected: all packages pass.

- [ ] **Step 2: Run a clean root build**

```powershell
$buildCheck = Join-Path $env:TEMP "farm-go-build-check-$PID.exe"
go build -o $buildCheck .
Remove-Item -LiteralPath $buildCheck -Force
```

Expected: command exits successfully and produces no compile errors.

- [ ] **Step 3: Check patch hygiene and repository state**

```powershell
git diff --check
git status --short
git log -4 --oneline
```

Expected: `git diff --check` is silent; the status contains no uncommitted implementation files; recent history contains the design, plan, and two focused implementation commits.

- [ ] **Step 4: Record the runtime expectation**

Do not launch the desktop app as part of automated verification because doing so would remove real `%APPDATA%` profiles. Record that on the next normal Farm_Go launch:

```text
WebView2 profile: %APPDATA%\Farm_Go\webview2
Legacy cleanup: direct %APPDATA% children matching Farm_Go*.exe with EBWebView
```

Existing locked profiles may remain until no older Farm_Go process is using them and a later launch retries cleanup.
