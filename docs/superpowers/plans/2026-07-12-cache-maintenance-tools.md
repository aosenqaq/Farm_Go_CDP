# Cache Maintenance Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add recoverable QQ, WeChat, and YYB mini-program cache maintenance controls to Farm_Go system settings.

**Architecture:** `internal/maintenance` owns allowlisted path discovery, backup moves, and platform-specific process closure. Root `App` exposes typed Wails methods, while `SettingsView` owns confirmations and renders operation feedback. No generated Wails binding file is edited by hand; Wails regenerates it during app build.

**Tech Stack:** Go 1.25, Wails v2, React 18, TypeScript, Vitest, Windows PowerShell.

---

### Task 1: Define And Test The Maintenance Service

**Files:**
- Create: `internal/maintenance/service.go`
- Create: `internal/maintenance/service_test.go`

- [ ] **Step 1: Write the focused failing Go tests**

```go
func TestWeChatPreviewOnlyReturnsAllowedRuntimeTargets(t *testing.T) {
    root := t.TempDir()
    mustMkdirAll(t, filepath.Join(root, "radium", "cache"))
    mustMkdirAll(t, filepath.Join(root, "WeChat Files", "msg"))

    summary, err := previewWeChat(root, filepath.Join(t.TempDir(), "backup"))

    if err != nil { t.Fatal(err) }
    if summary.TargetCount != 1 || summary.Targets[0].RelativePath != `radium\cache` {
        t.Fatalf("unexpected preview: %#v", summary)
    }
}

func TestMoveTargetsKeepsBackupAndRejectsOutsideRoot(t *testing.T) {
    root := t.TempDir()
    target := filepath.Join(root, "QQ", "miniapp", "temps", "miniapp_src")
    mustMkdirAll(t, target)
    backup := filepath.Join(t.TempDir(), "backup")

    moved, skipped, err := moveTargets(root, backup, []Target{{Path: target, RelativePath: `QQ\miniapp\temps\miniapp_src`}})

    if err != nil || moved != 1 || skipped != 0 { t.Fatalf("move = %d, %d, %v", moved, skipped, err) }
    if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) { t.Fatalf("target still exists: %v", err) }
    if _, err := os.Stat(filepath.Join(backup, "QQ", "miniapp", "temps", "miniapp_src")); err != nil { t.Fatal(err) }

    outside := filepath.Join(t.TempDir(), "outside")
    mustMkdirAll(t, outside)
    _, _, err = moveTargets(root, backup, []Target{{Path: outside, RelativePath: `outside`}})
    if err == nil { t.Fatal("expected an outside-root target error") }
}

func TestQQCandidatesNeverIncludeLegacyTauriData(t *testing.T) {
    candidates := qqCandidates(`C:\Users\me\AppData\Roaming`, `C:\Users\me\AppData\Local`)
    for _, candidate := range candidates {
        if strings.Contains(strings.ToLower(candidate.Path), "site.dank1ng.farm") {
            t.Fatalf("legacy path leaked into QQ candidates: %s", candidate.Path)
        }
    }
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./internal/maintenance -run 'Test(WeChatPreviewOnlyReturnsAllowedRuntimeTargets|MoveTargetsKeepsBackupAndRejectsOutsideRoot|QQCandidatesNeverIncludeLegacyTauriData)' -count=1`

Expected: FAIL because package `Farm_Go/internal/maintenance` does not exist.

- [ ] **Step 3: Implement the minimal service**

```go
package maintenance

type Target struct {
    Path         string `json:"path"`
    RelativePath string `json:"relativePath"`
    Reason       string `json:"reason"`
}

type Summary struct {
    Platform        string   `json:"platform"`
    Preview         bool     `json:"preview"`
    TargetCount     int      `json:"targetCount"`
    MovedCount      int      `json:"movedCount"`
    SkippedCount    int      `json:"skippedCount"`
    ClosedProcesses int      `json:"closedProcesses"`
    BackupDir       string   `json:"backupDir"`
    Targets         []Target `json:"targets"`
}

type Service struct{}

func NewService() *Service { return &Service{} }
func (s *Service) PreviewWeChat() (Summary, error) { return runWeChat(false) }
func (s *Service) CleanWeChat() (Summary, error)   { return runWeChat(true) }
func (s *Service) CleanQQ() (Summary, error)       { return runQQ(true) }
func (s *Service) CleanYYB() (Summary, error)      { return runYYB(true) }
```

Implement each operation with fixed candidate builders, `filepath.Rel` root containment validation, `os.Rename` into `Desktop/Farm_Go-<platform>-cache-backup-<yyyyMMdd-HHmmss>`, and an existing-path skip count. Preserve source-tool allowlists from the approved design: xwechat runtime-only paths for WeChat; `QQ`/`QQEX` `miniapp_src` and `miniapp_pkgs` for QQ; Androws runtime cache directories for YYB. Do not inspect `site.dank1ng.farm`, do not recursively scan arbitrary drives, and do not remove files.

Use `taskkill /IM <image> /T /F` only for the fixed QQ and WeChat image-name lists. For YYB, call hidden PowerShell that selects only `AndrowsLauncher`, `AndrowsStore`, `QQMiniApp`, and `WeChatAppEx` processes whose executable path is inside an `Androws` root; then stops those matching process IDs. On non-Windows, return zero closed processes without launching a command.

- [ ] **Step 4: Run the focused Go tests and verify GREEN**

Run: `go test ./internal/maintenance -run 'Test(WeChatPreviewOnlyReturnsAllowedRuntimeTargets|MoveTargetsKeepsBackupAndRejectsOutsideRoot|QQCandidatesNeverIncludeLegacyTauriData)' -count=1`

Expected: PASS.

### Task 2: Expose The Service Through Wails

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`

- [ ] **Step 1: Write the failing App delegation test**

```go
func TestAppExposesCacheMaintenanceMethods(t *testing.T) {
    app := newAuthorizedTestApp(t)

    if app.maintenance == nil { t.Fatal("maintenance service was not initialized") }

    var preview func(*App) (maintenance.Summary, error) = (*App).PreviewWeChatCacheCleanup
    var wechat func(*App) (maintenance.Summary, error) = (*App).CleanWeChatCache
    var qq func(*App) (maintenance.Summary, error) = (*App).CleanQQMiniappCache
    var yyb func(*App) (maintenance.Summary, error) = (*App).CleanYYBMiniappCache
    if preview == nil || wechat == nil || qq == nil || yyb == nil { t.Fatal("missing maintenance method") }
}
```

Add `"Farm_Go/internal/maintenance"` to the existing `app_test.go` import block for the method-signature assertions. This test intentionally does not call a cleanup method.

- [ ] **Step 2: Run the focused App test and verify RED**

Run: `go test . -run TestAppExposesCacheMaintenanceMethods -count=1`

Expected: FAIL because the three Wails methods do not exist.

- [ ] **Step 3: Add the maintenance dependency and Wails methods**

```go
import "Farm_Go/internal/maintenance"

type App struct {
    // existing fields
    maintenance *maintenance.Service
}

func (a *App) PreviewWeChatCacheCleanup() (maintenance.Summary, error) {
    return a.maintenance.PreviewWeChat()
}

func (a *App) CleanWeChatCache() (maintenance.Summary, error) {
    return a.maintenance.CleanWeChat()
}

func (a *App) CleanQQMiniappCache() (maintenance.Summary, error) {
    return a.maintenance.CleanQQ()
}

func (a *App) CleanYYBMiniappCache() (maintenance.Summary, error) {
    return a.maintenance.CleanYYB()
}
```

Initialize `maintenance` in `NewApp`. Keep method names exported so Wails generates frontend bindings during app build. Do not add an authorization gate beyond the existing application gate.

- [ ] **Step 4: Run the focused App test and verify GREEN**

Run: `go test . -run TestAppExposesCacheMaintenanceMethods -count=1`

Expected: PASS.

### Task 3: Add The System Settings Controls

**Files:**
- Modify: `frontend/src/views/SettingsView.tsx`
- Modify: `frontend/src/views/SettingsView.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write the focused failing settings-view tests**

```tsx
it('renders three cache maintenance actions in system settings', () => {
  const html = renderToStaticMarkup(<SettingsView events={[]} onRefreshEvents={() => undefined} />);
  expect(html).toContain('缓存维护');
  expect(html).toContain('清理微信小程序缓存');
  expect(html).toContain('清理 QQ 小程序缓存');
  expect(html).toContain('清理应用宝小程序缓存');
});

it('uses a WeChat preview and requires a second confirmation before apply', () => {
  const source = readFileSync(new URL('./SettingsView.tsx', import.meta.url), 'utf8');
  expect(source).toContain('PreviewWeChatCacheCleanup');
  expect(source).toContain('CleanWeChatCache');
  expect(source).toContain('将移动');
});

it('requires two QQ confirmations and one YYB confirmation', () => {
  const source = readFileSync(new URL('./SettingsView.tsx', import.meta.url), 'utf8');
  expect(source).toContain('再次确认');
  expect(source).toContain('CleanQQMiniappCache');
  expect(source).toContain('CleanYYBMiniappCache');
});
```

Extend the existing Wails module mock with the four new functions so static rendering remains isolated from the desktop bridge.

- [ ] **Step 2: Run the focused view test and verify RED**

Run: `Set-Location frontend; npm test -- SettingsView`

Expected: FAIL because the maintenance controls and handlers do not exist.

- [ ] **Step 3: Implement the narrow UI addition**

```tsx
const [maintenanceBusy, setMaintenanceBusy] = useState<'wechat-preview' | 'wechat-apply' | 'qq' | 'yyb' | null>(null);
const [maintenanceMessage, setMaintenanceMessage] = useState('');
const [maintenanceError, setMaintenanceError] = useState('');

type MaintenanceSummary = {
  targetCount: number;
  movedCount: number;
  skippedCount: number;
  closedProcesses: number;
  backupDir: string;
  targets: Array<{ relativePath: string }>;
};
```

Import `PreviewWeChatCacheCleanup`, `CleanWeChatCache`, `CleanQQMiniappCache`, and `CleanYYBMiniappCache` from the generated Wails module. Add a `缓存维护` panel after the existing application-update panel. Use `DeleteSweep` for each destructive action, disable all three while `maintenanceBusy` is non-null, and retain the established `settings-message` styles for result/error text.

`handleWechatCleanup` must ask for a first acknowledgement, request the preview, display up to twelve relative paths and the backup location in a second `window.confirm`, then apply only after that confirmation. `handleQQCleanup` must use two confirmations and call the QQ method. `handleYYBCleanup` must use one confirmation and call the YYB method. On success, produce the platform-specific restart/open-mini-program instruction and include the returned backup directory when non-empty.

Add compact CSS for `.settings-maintenance-panel`, `.maintenance-actions`, and `.maintenance-summary` using the existing settings-panel sizing and the mobile single-column breakpoint. Do not introduce a nested card or change existing settings layout.

- [ ] **Step 4: Run the focused view test and verify GREEN**

Run: `Set-Location frontend; npm test -- SettingsView`

Expected: PASS.

### Task 4: Generate Bindings And Verify The Integration

**Files:**
- Generated during Wails app build: `frontend/wailsjs/go/main/App.js`
- Generated during Wails app build: `frontend/wailsjs/go/main/App.d.ts`

- [ ] **Step 1: Run the Go suite**

Run: `go test ./...`

Expected: PASS with the new maintenance package and existing tests.

- [ ] **Step 2: Run the frontend suite**

Run: `Set-Location frontend; npm test`

Expected: PASS.

- [ ] **Step 3: Regenerate bindings and build the Wails application**

Run: `wails build -s`

Expected: Wails regenerates `frontend/wailsjs/go/main/App.js` and `App.d.ts`, TypeScript compilation succeeds, and the Windows app build completes. If release-license environment variables are absent, use `wails dev -s` long enough to regenerate bindings, then run `Set-Location frontend; npm run build` and report the release-build prerequisite rather than editing generated bindings manually.

- [ ] **Step 4: Inspect the final change set**

Run: `git diff --check; git status --short`

Expected: no whitespace errors; changed source is limited to the maintenance service, App bridge, settings view/tests/styles, generated Wails bindings, and this plan.
