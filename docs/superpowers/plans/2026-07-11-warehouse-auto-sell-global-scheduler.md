# Warehouse Auto Sell Global Scheduler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move warehouse auto sell from a page-local timer into the account-scoped global automation scheduler, expose it in the scheduler UI, and migrate the interval from hours to minutes with a 60-minute default.

**Architecture:** Warehouse auto sell preferences remain the single source of truth. App-level state composition maps those preferences into the generic automation scheduler task, while both warehouse settings saves and scheduler saves persist the same account-scoped preferences and reconfigure the live scheduler. The scheduled runner executes refresh-filter-sell directly with the captured account key so records cannot cross account boundaries.

**Tech Stack:** Go, SQLite storage, Wails bindings, React, TypeScript, Vitest, Go testing.

---

### Task 1: Migrate warehouse interval storage to minutes

**Files:**
- Modify: `internal/storage/account_preferences.go`
- Modify: `internal/storage/settings.go`
- Test: `internal/storage/storage_test.go`
- Test: `app_test.go`

- [ ] **Step 1: Write failing storage tests**

Add tests proving that `DefaultWarehouseAutoSellSettings().IntervalMinute == 60`, a saved minute value round-trips, legacy `warehouse.autoSellIntervalHour=6` loads as 360 minutes, and a present minute key wins over the legacy hour key.

```go
func TestWarehouseAutoSellSettingsMigratesLegacyHoursToMinutes(t *testing.T) {
    ctx := context.Background()
    store, err := Open(ctx, t.TempDir())
    if err != nil { t.Fatal(err) }
    defer store.Close()
    accountKey := "gid:10001"
    if _, err := store.db.ExecContext(ctx, settingUpsertSQL, accountKey, "warehouse.autoSellIntervalHour", "6", time.Now().Format(time.RFC3339Nano)); err != nil {
        t.Fatal(err)
    }
    got, err := store.LoadWarehouseAutoSellSettingsForAccount(ctx, accountKey)
    if err != nil { t.Fatal(err) }
    if got.IntervalMinute != 360 { t.Fatalf("interval = %d, want 360", got.IntervalMinute) }
}
```

- [ ] **Step 2: Run focused tests and verify RED**

Run: `go test ./internal/storage ./... -run "WarehouseAutoSell.*Minute|WarehouseAutoSellSettingsMigratesLegacyHours" -count=1`

Expected: compile failure because `IntervalMinute` does not exist or assertion failure because the default is still six hours.

- [ ] **Step 3: Implement minute DTO and one-time legacy migration**

Change the preference DTO and normalization:

```go
type WarehouseAutoSellSettings struct {
    Enabled               bool     `json:"enabled"`
    IntervalMinute        int      `json:"intervalMinute"`
    Categories            []string `json:"categories"`
    RefreshOnlyOnAutoSell bool     `json:"refreshOnlyOnAutoSell"`
}

func DefaultWarehouseAutoSellSettings() WarehouseAutoSellSettings {
    return WarehouseAutoSellSettings{IntervalMinute: 60, Categories: []string{"fruit"}, RefreshOnlyOnAutoSell: true}
}
```

Load both keys, prefer `warehouse.autoSellIntervalMinute`, otherwise parse the old hour value, multiply by 60, and persist the migrated minute value. Save only `warehouse.autoSellIntervalMinute`. Rename runtime compatibility fields to `AutoWarehouseSellIntervalMinute` and use a default of 60.

- [ ] **Step 4: Update app mappings and existing tests**

Replace `IntervalHour`/`AutoWarehouseSellIntervalHour` assertions and fixtures with their minute equivalents. Existing two-hour fixtures become `IntervalMinute: 120`; defaults become 60.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run: `go test ./internal/storage ./... -run "WarehouseAutoSell|RuntimeSettings" -count=1`

Expected: PASS.

### Task 2: Register warehouse auto sell in the scheduler catalog

**Files:**
- Modify: `internal/farm/automation/catalog.go`
- Modify: `internal/farm/automation/config.go`
- Modify: `internal/farm/automation/scheduler.go`
- Test: `internal/farm/automation/catalog_test.go`
- Test: `internal/farm/automation/scheduler_test.go`

- [ ] **Step 1: Write failing catalog and scheduler tests**

Assert the default state contains this task:

```go
task := findSchedulerTaskForTest(DefaultState().Scheduler.Tasks, "auto_warehouse_sell")
if task.Label != "仓库自动出售" || task.Enabled || task.IntervalSec != 3600 {
    t.Fatalf("unexpected task: %#v", task)
}
```

Also assert `taskSpecByID("auto_warehouse_sell")` uses `LaneProtocol` and `ResourceProtocol`, and that enabling it makes it immediately due under the existing scheduler semantics.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/farm/automation -run "WarehouseAutoSell|AutoWarehouseSell" -count=1`

Expected: FAIL because the task is absent.

- [ ] **Step 3: Add canonical task mappings**

Register:

```go
"auto_warehouse_sell": "autoWarehouseSellEnabled"
"auto_warehouse_sell": "autoWarehouseSellIntervalSec"
```

Add a task definition with default priority below core own-farm tasks, interval 3600 seconds, and disabled by default. Add a `warehouse` feature group only if required by catalog invariants; otherwise keep the task scheduler-only to avoid duplicating the warehouse settings UI.

- [ ] **Step 4: Add the protocol task resource spec**

```go
{ID: "auto_warehouse_sell", Label: "仓库自动出售", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol}},
```

- [ ] **Step 5: Run package tests and verify GREEN**

Run: `go test ./internal/farm/automation -count=1`

Expected: PASS.

### Task 3: Compose warehouse preferences into automation state

**Files:**
- Modify: `app.go`
- Test: `app_test.go`

- [ ] **Step 1: Write failing state-composition tests**

Save warehouse settings for an account, call `FarmAutomationState`, find `auto_warehouse_sell`, and assert its enabled state and interval are derived from `IntervalMinute * 60`. Assert another account retains the 60-minute disabled default.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test . -run "FarmAutomationState.*WarehouseAutoSell|WarehouseAutoSell.*Scheduler" -count=1`

Expected: FAIL because the scheduler state does not contain the task or does not reflect warehouse preferences.

- [ ] **Step 3: Add state composition helpers**

Add focused helpers in `app.go`:

```go
func applyWarehouseAutoSellToAutomationSettings(settings automation.Settings, warehouse storage.WarehouseAutoSellSettings) automation.Settings
func warehouseAutoSellFromAutomationSettings(settings automation.Settings, fallback storage.WarehouseAutoSellSettings) storage.WarehouseAutoSellSettings
```

The first helper sets `autoWarehouseSellEnabled`, `autoWarehouseSellIntervalSec`, and the matching task fields. The second reads the task/config values, rounds seconds up to at least one whole minute, and preserves categories from the fallback preference.

- [ ] **Step 4: Load composed state everywhere scheduler settings are created**

Update `farmAutomationStateForAccount` and scheduler creation so stored auto-farm settings are composed with `LoadWarehouseAutoSellSettingsForAccount` before `StateFromSettings` or `Configure` is called.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run: `go test . -run "FarmAutomationState|WarehouseAutoSell.*Scheduler" -count=1`

Expected: PASS.

### Task 4: Execute real warehouse auto sell from the global runner

**Files:**
- Modify: `app.go`
- Test: `app_test.go`

- [ ] **Step 1: Write failing execution tests**

Use the existing fake runtime link to return a warehouse snapshot containing matching, non-matching, locked, and unsellable items. Execute `auto_warehouse_sell` and assert only eligible keys are sent to `gameCtl.sellWarehouseItems`, mode is `auto`, the result is successful, and the sell record belongs to the captured account. Add a no-eligible-items test that does not call sell and returns success.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test . -run "AutoWarehouseSellTask|ScheduledWarehouseAutoSell" -count=1`

Expected: FAIL with the task reported as not migrated.

- [ ] **Step 3: Implement account-safe refresh-filter-sell**

Branch early in `executeFarmAutomationTaskForAccount`:

```go
if taskID == "auto_warehouse_sell" {
    return a.executeWarehouseAutoSellForAccount(ctx, trigger, accountKey)
}
```

The helper loads account-scoped settings, refreshes via `gameCtl.refreshWarehouseSnapshot`, builds the warehouse payload with game config, filters categories plus `CanSell && !Locked`, calls `gameCtl.sellWarehouseItems`, saves a record with the captured account key and mode `auto`, and converts all outcomes to `automation.ActionResult`.

- [ ] **Step 4: Run focused execution tests and verify GREEN**

Run: `go test . -run "AutoWarehouseSellTask|ScheduledWarehouseAutoSell|FarmWarehouseSell" -count=1`

Expected: PASS.

### Task 5: Synchronize warehouse and scheduler save entry points

**Files:**
- Modify: `app.go`
- Test: `app_test.go`

- [ ] **Step 1: Write failing synchronization tests**

Test both directions:

```go
saved, err := app.SaveWarehouseAutoSellSettings(storage.WarehouseAutoSellSettings{Enabled: true, IntervalMinute: 90, Categories: []string{"fruit"}})
// live scheduler task becomes enabled with IntervalSec 5400
```

Then edit the scheduler task to disabled with `IntervalSec: 7200`, call `SaveFarmAutomationState`, and assert warehouse settings persist `Enabled: false` and `IntervalMinute: 120` while categories remain unchanged.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `go test . -run "SaveWarehouseAutoSell.*Scheduler|SaveFarmAutomationState.*Warehouse" -count=1`

Expected: FAIL because the stores and live scheduler are not synchronized.

- [ ] **Step 3: Reconfigure after warehouse settings save**

After the account-scoped preference save succeeds, load the current auto-farm settings, compose the saved warehouse settings, and call `scheduler.Configure` only when the live scheduler belongs to the same captured account.

- [ ] **Step 4: Persist warehouse settings during scheduler save**

Inside the existing automation settings write lock, derive the warehouse preference before writing. Save the warehouse preference first; if it fails, return without applying scheduler changes. Then save the auto-farm settings and configure the live scheduler with the composed settings.

- [ ] **Step 5: Run synchronization and race tests**

Run: `go test . -run "WarehouseAutoSell|SaveFarmAutomationState|AccountCapturedAtEntry" -count=1`

Expected: PASS.

### Task 6: Replace the page timer with minute-based settings UI

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx`
- Modify: `frontend/src/views/AssetsLandView.test.tsx`
- Modify: `frontend/src/views/AutomationView.tsx`
- Modify: `frontend/src/views/AutomationView.test.tsx`
- Regenerate: `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Write failing frontend tests**

Assert the warehouse source contains `出售间隔(分钟)`, uses `intervalMinute`, defaults to 60, and no longer contains `window.setInterval` for auto sell. Add scheduler mapping assertions for `auto_warehouse_sell`, `autoWarehouseSellEnabled`, and `autoWarehouseSellIntervalSec`.

- [ ] **Step 2: Run frontend tests and verify RED**

Run: `npm test -- --run src/views/AssetsLandView.test.tsx src/views/AutomationView.test.tsx`

Working directory: `frontend`

Expected: FAIL because hour fields and the page timer still exist.

- [ ] **Step 3: Update the warehouse settings UI**

Rename the local type and payload property to `intervalMinute`, set the default to 60, change the label to minutes, and remove the `useEffect` that creates the page-local auto sell interval. Keep the settings load/save dialog and category selection.

- [ ] **Step 4: Update scheduler key fallbacks and minute normalization**

Add the task mappings to the two scheduler maps. When editing the generic seconds field for `auto_warehouse_sell`, normalize to multiples of 60 with a minimum of 60 so scheduler saves always produce whole minutes.

- [ ] **Step 5: Regenerate Wails bindings**

Run: `wails generate module`

Expected: `frontend/wailsjs/go/models.ts` exposes `intervalMinute` and no longer exposes `intervalHour` for `WarehouseAutoSellSettings`.

- [ ] **Step 6: Run frontend tests and verify GREEN**

Run: `npm test -- --run src/views/AssetsLandView.test.tsx src/views/AutomationView.test.tsx`

Working directory: `frontend`

Expected: PASS.

### Task 7: Full verification

**Files:**
- Verify all changed files

- [ ] **Step 1: Format Go code**

Run: `gofmt -w app.go app_test.go internal/storage/account_preferences.go internal/storage/settings.go internal/storage/storage_test.go internal/farm/automation/catalog.go internal/farm/automation/catalog_test.go internal/farm/automation/config.go internal/farm/automation/scheduler.go internal/farm/automation/scheduler_test.go`

- [ ] **Step 2: Run all Go tests**

Run: `go test ./... -count=1`

Expected: PASS with zero failures.

- [ ] **Step 3: Run all frontend tests**

Run: `npm test -- --run`

Working directory: `frontend`

Expected: PASS with zero failures.

- [ ] **Step 4: Build the frontend**

Run: `npm run build`

Working directory: `frontend`

Expected: exit code 0.

- [ ] **Step 5: Inspect the final diff**

Run: `git diff --check && git status --short && git diff --stat`

Expected: no whitespace errors; only intended source, tests, generated binding, plan, and existing user-owned dirty files are listed.
