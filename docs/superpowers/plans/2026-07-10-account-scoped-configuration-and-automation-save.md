# Account-Scoped Configuration and Automation Save Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Fix automation switches reverting after repeated saves, and persist all farm business configuration in SQLite under the confirmed GID while keeping machine settings global.

**Architecture:** Keep storage.RuntimeSettings as the Wails compatibility DTO, but split persistence internally between global machine keys and GID-scoped warehouse preferences. Treat automation config keys as authoritative during save normalization, route background writes through one captured account key, and copy legacy default/global business keys once when a GID is first confirmed.

**Tech Stack:** Go, modernc SQLite, Wails, React 18, TypeScript, Vitest.

---

## File Map

- Modify internal/farm/automation/catalog.go and catalog_test.go for deterministic enabled-state conversion.
- Modify frontend/src/views/AutomationView.tsx and AutomationView.test.tsx for draft synchronization.
- Modify app.go and app_test.go for account-aware saves, composed settings, and account confirmation migration.
- Modify internal/storage/settings.go to keep machine settings global.
- Create internal/storage/account_preferences.go for warehouse auto-sell preferences.
- Create internal/storage/account_migration.go for one-time legacy seeding.
- Modify internal/storage/storage_test.go for isolation and migration coverage.

### Task 1: Make Automation Enabled State Deterministic

**Files:**
- Modify: internal/farm/automation/catalog_test.go
- Modify: internal/farm/automation/catalog.go
- Modify: frontend/src/views/AutomationView.test.tsx
- Modify: frontend/src/views/AutomationView.tsx

- [ ] **Step 1: Add the failing backend regression test**

Add to internal/farm/automation/catalog_test.go:

~~~go
func TestSettingsRoundTripPreservesDetailedSwitchWhenClosingDefaultOffTask(t *testing.T) {
	state := DefaultState()
	state.Config["autoFarmFriendEnabled"] = true
	for i := range state.Scheduler.Tasks {
		if state.Scheduler.Tasks[i].ID == "friend_steal" {
			state.Scheduler.Tasks[i].Enabled = true
		}
	}
	state = StateFromSettings(SettingsFromState(state))

	state.Config["autoFarmFriendEnabled"] = false
	result := StateFromSettings(SettingsFromState(state))

	if result.Config["autoFarmFriendEnabled"] != false {
		t.Fatalf("detailed switch reverted: %#v", result.Config["autoFarmFriendEnabled"])
	}
	task := findSchedulerTaskForTest(result.Scheduler.Tasks, "friend_steal")
	if task == nil || task.Enabled {
		t.Fatalf("friend_steal should be disabled: %#v", task)
	}
}
~~~

- [ ] **Step 2: Verify RED**

~~~powershell
go test ./internal/farm/automation -run TestSettingsRoundTripPreservesDetailedSwitchWhenClosingDefaultOffTask -count=1
~~~

Expected: FAIL because stale task.Enabled overwrites the false detailed config.

- [ ] **Step 3: Make backend serialization use detailed config**

Add to internal/farm/automation/catalog.go:

~~~go
func schedulerTaskEnabledForSave(config map[string]any, task SchedulerTask) bool {
	key := schedulerTaskConfigKeys[task.ID]
	if key == "" {
		return task.Enabled
	}
	value, ok := config[key]
	if !ok {
		config[key] = task.Enabled
		return task.Enabled
	}
	enabled := boolConfig(value, task.Enabled)
	config[key] = enabled
	return enabled
}
~~~

Replace the SettingsFromState task loop with:

~~~go
for _, task := range state.Scheduler.Tasks {
	enabled := schedulerTaskEnabledForSave(config, task)
	if key := schedulerTaskIntervalConfigKeys[task.ID]; key != "" {
		config[key] = normalizedIntervalSec(task.IntervalSec)
	}
	settings.Tasks = append(settings.Tasks, TaskSettings{
		ID:          task.ID,
		Enabled:     enabled,
		Priority:    task.Priority,
		IntervalSec: normalizedIntervalSec(task.IntervalSec),
	})
}
~~~

Delete shouldWriteSchedulerTaskConfig. Keep read-side legacy compatibility in StateFromSettings.

- [ ] **Step 4: Replace the obsolete scheduler-only test**

Replace TestSettingsFromStateWritesSchedulerTaskEnabledStateToFeatureConfig:

~~~go
func TestSettingsFromStateUsesDetailedConfigForTaskEnabledState(t *testing.T) {
	state := DefaultState()
	state.Config["autoFarmFriendEnabled"] = true
	settings := SettingsFromState(state)

	if settings.Config["autoFarmFriendEnabled"] != true {
		t.Fatalf("autoFarmFriendEnabled = %#v, want true", settings.Config["autoFarmFriendEnabled"])
	}
	task := findTaskSettingsForTest(settings.Tasks, "friend_steal")
	if task == nil || !task.Enabled {
		t.Fatalf("friend_steal should follow detailed config: %#v", task)
	}
}
~~~

- [ ] **Step 5: Verify backend GREEN**

~~~powershell
go test ./internal/farm/automation -count=1
~~~

Expected: PASS.

- [ ] **Step 6: Add the failing frontend synchronization test**

Import syncAutomationSchedulerEnabledState and add:

~~~tsx
it('syncs detailed enabled config into scheduler tasks before saving', () => {
  const synced = syncAutomationSchedulerEnabledState({
    ...state,
    config: { ...state.config, autoFarmFriendEnabled: false },
    scheduler: {
      ...state.scheduler,
      tasks: state.scheduler.tasks.map((task) =>
        task.id === 'friend_steal' ? { ...task, enabled: true } : task,
      ),
    },
  });
  expect(synced.config.autoFarmFriendEnabled).toBe(false);
  expect(synced.scheduler.tasks.find((task) => task.id === 'friend_steal')?.enabled).toBe(false);
});
~~~

- [ ] **Step 7: Verify frontend RED**

~~~powershell
Set-Location frontend
npm test -- AutomationView
~~~

Expected: FAIL because the helper does not exist.

- [ ] **Step 8: Implement frontend draft synchronization**

Add beside syncAutomationSchedulerIntervals:

~~~tsx
export function syncAutomationSchedulerEnabledState(state: FarmAutomationState): FarmAutomationState {
  const config = { ...(state.config || {}) };
  const tasks = state.scheduler.tasks.map((task) => {
    const key = schedulerTaskConfigKeyById[task.id];
    if (!key || typeof config[key] === 'undefined') return { ...task };
    return { ...task, enabled: boolFromConfig(config[key], task.enabled) };
  });
  return {
    ...state,
    config,
    summary: {
      ...state.summary,
      enabledTasks: tasks.filter((task) => task.enabled).length,
      totalTasks: tasks.length,
    },
    scheduler: { ...state.scheduler, tasks },
  };
}
~~~

Pass the interval-normalized state through syncAutomationSchedulerEnabledState in normalizeAutomationState. Keep updateTask calling schedulerConfigForTaskUpdate first, so scheduler edits update the mapped config key before normalization.

- [ ] **Step 9: Verify frontend GREEN**

~~~powershell
Set-Location frontend
npm test -- AutomationView
~~~

Expected: PASS.

- [ ] **Step 10: Commit**

~~~powershell
Set-Location ..
git add -- internal/farm/automation/catalog.go internal/farm/automation/catalog_test.go frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx
git commit -m "fix: preserve automation switches across repeated saves"
~~~

### Task 2: Save Automation Daily State Under the Current GID

**Files:**
- Modify: app_test.go
- Modify: app.go

- [ ] **Step 1: Add the failing account-scope test**

Add to app_test.go:

~~~go
func TestAutomationDailyStatePersistsForCurrentAccount(t *testing.T) {
	ctx := context.Background()
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.claimSvipDailyGift": map[string]any{
			"ok": true, "success": true, "claimedCount": float64(1), "action": "svip_daily_gift",
		},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil { t.Fatalf("open store: %v", err) }
	defer store.Close()
	app.store = store
	app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001, Nickname: "A"})

	result := app.RunFarmAutomationTask("svip_daily_gift")
	if !result.OK { t.Fatalf("run task: %#v", result) }

	key := automation.DailyOnceTaskDoneDateConfigKey("svip_daily_gift")
	accountSettings, _ := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001")
	defaultSettings, _ := store.LoadAutoFarmSettingsForAccount(ctx, storage.DefaultRuntimeAccountKey)
	if accountSettings.Config[key] != time.Now().Format("2006-01-02") {
		t.Fatalf("daily state missing from gid scope: %#v", accountSettings.Config)
	}
	if _, exists := defaultSettings.Config[key]; exists {
		t.Fatalf("daily state leaked into default scope: %#v", defaultSettings.Config)
	}
}
~~~

- [ ] **Step 2: Verify RED**

~~~powershell
go test . -run TestAutomationDailyStatePersistsForCurrentAccount -count=1
~~~

Expected: FAIL because saveAutomationDailyState writes to default.

- [ ] **Step 3: Use a captured account key**

Replace the storage call inside saveAutomationDailyState:

~~~go
accountKey := a.accountKey()
if err := a.store.SaveAutoFarmSettingsForAccount(
	a.contextOrBackground(),
	accountKey,
	storageAutoFarmSettings(settings),
); err != nil {
	a.recordEvent(eventbus.Event{
		Level: eventbus.LevelWarn, Source: "auto_farm", Type: eventType,
		Message: errorPrefix + err.Error(), Data: map[string]any{"accountKey": accountKey},
	})
	return
}
~~~

Preserve the existing scheduler reconfiguration after a successful save.

- [ ] **Step 4: Verify GREEN**

~~~powershell
go test . -run "TestAutomationDailyStatePersistsForCurrentAccount|TestRunScheduledFarmAutomationFriendMischiefMarksDailyLimitDone|TestRunScheduledFarmAutomationDailyOnceTasksMarkDoneAndSkipToday|TestRunFarmAutomationDailyOnceTaskManualRunMarksDoneToday" -count=1
~~~

Expected: PASS.

- [ ] **Step 5: Commit**

~~~powershell
git add -- app.go app_test.go
git commit -m "fix: scope automation daily state by gid"
~~~

### Task 3: Split Warehouse Preferences From Global Settings

**Files:**
- Create: internal/storage/account_preferences.go
- Modify: internal/storage/settings.go
- Modify: internal/storage/storage_test.go
- Modify: app.go
- Modify: app_test.go

- [ ] **Step 1: Add failing storage tests**

Add to internal/storage/storage_test.go:

~~~go
func TestWarehouseAutoSellSettingsAreScopedByAccount(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil { t.Fatalf("open store: %v", err) }
	defer store.Close()

	first := WarehouseAutoSellSettings{Enabled: true, IntervalHour: 2, Categories: []string{"fruit", "seed"}, RefreshOnlyOnAutoSell: true}
	second := WarehouseAutoSellSettings{Enabled: false, IntervalHour: 9, Categories: []string{"tool"}, RefreshOnlyOnAutoSell: true}
	if err := store.SaveWarehouseAutoSellSettingsForAccount(ctx, "gid:10001", first); err != nil { t.Fatalf("save first: %v", err) }
	if err := store.SaveWarehouseAutoSellSettingsForAccount(ctx, "gid:10002", second); err != nil { t.Fatalf("save second: %v", err) }
	gotFirst, _ := store.LoadWarehouseAutoSellSettingsForAccount(ctx, "gid:10001")
	gotSecond, _ := store.LoadWarehouseAutoSellSettingsForAccount(ctx, "gid:10002")
	if !reflect.DeepEqual(gotFirst, first) { t.Fatalf("first = %#v, want %#v", gotFirst, first) }
	if !reflect.DeepEqual(gotSecond, second) { t.Fatalf("second = %#v, want %#v", gotSecond, second) }
}

func TestGlobalRuntimeSettingsDoNotPersistWarehousePreferences(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil { t.Fatalf("open store: %v", err) }
	defer store.Close()

	input := RuntimeSettings{
		DefaultTarget: "qq_ws", CurrentTarget: "qq_ws", AutoStart: true,
		CDPPort: 62000, WMPFDebugPort: 9420, AutoWarehouseSellEnabled: true,
	}
	if err := store.SaveRuntimeSettings(ctx, input); err != nil { t.Fatalf("save: %v", err) }
	values, _ := store.loadSettingsForAccount(ctx, GlobalSettingsAccountKey, []string{"warehouse.autoSellEnabled"})
	if len(values) != 0 { t.Fatalf("warehouse preferences leaked into global: %#v", values) }
}
~~~

- [ ] **Step 2: Verify RED**

~~~powershell
go test ./internal/storage -run "TestWarehouseAutoSellSettingsAreScopedByAccount|TestGlobalRuntimeSettingsDoNotPersistWarehousePreferences" -count=1
~~~

Expected: compile failure for the missing type and methods.

- [ ] **Step 3: Create account_preferences.go**

Create internal/storage/account_preferences.go:

~~~go
package storage

import (
	"context"
	"strconv"
	"strings"
)

type WarehouseAutoSellSettings struct {
	Enabled               bool
	IntervalHour          int
	Categories            []string
	RefreshOnlyOnAutoSell bool
}

func DefaultWarehouseAutoSellSettings() WarehouseAutoSellSettings {
	return WarehouseAutoSellSettings{
		IntervalHour: 6, Categories: []string{"fruit"}, RefreshOnlyOnAutoSell: true,
	}
}

func (s *Store) LoadWarehouseAutoSellSettingsForAccount(ctx context.Context, accountKey string) (WarehouseAutoSellSettings, error) {
	settings := DefaultWarehouseAutoSellSettings()
	values, err := s.loadSettingsForAccount(ctx, accountKey, []string{
		"warehouse.autoSellEnabled", "warehouse.autoSellIntervalHour",
		"warehouse.autoSellCategories", "warehouse.refreshOnlyOnAutoSell",
	})
	if err != nil { return WarehouseAutoSellSettings{}, err }
	if value := values["warehouse.autoSellEnabled"]; value != "" { settings.Enabled = value == "true" }
	if value := values["warehouse.autoSellIntervalHour"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil { return WarehouseAutoSellSettings{}, err }
		settings.IntervalHour = parsed
	}
	if value := values["warehouse.autoSellCategories"]; value != "" {
		settings.Categories = strings.Split(value, ",")
	}
	settings.Categories = normalizeWarehouseSellCategories(settings.Categories)
	if settings.IntervalHour <= 0 { settings.IntervalHour = 6 }
	settings.RefreshOnlyOnAutoSell = true
	return settings, nil
}

func (s *Store) SaveWarehouseAutoSellSettingsForAccount(ctx context.Context, accountKey string, settings WarehouseAutoSellSettings) error {
	if settings.IntervalHour <= 0 { settings.IntervalHour = 6 }
	settings.Categories = normalizeWarehouseSellCategories(settings.Categories)
	if len(settings.Categories) == 0 { settings.Categories = []string{"fruit"} }
	return s.saveSettingsForAccount(ctx, accountKey, map[string]string{
		"warehouse.autoSellEnabled": strconv.FormatBool(settings.Enabled),
		"warehouse.autoSellIntervalHour": strconv.Itoa(settings.IntervalHour),
		"warehouse.autoSellCategories": strings.Join(settings.Categories, ","),
		"warehouse.refreshOnlyOnAutoSell": "true",
	})
}
~~~

Add JSON field names enabled, intervalHour, categories, and refreshOnlyOnAutoSell to the four struct fields so the storage DTO remains usable through Wails if exposed later.

- [ ] **Step 4: Add transaction-backed bulk setting writes**

Add to internal/storage/settings.go:

~~~go
func (s *Store) saveSettingsForAccount(ctx context.Context, accountKey string, values map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()
	accountKey = NormalizeAccountKey(accountKey)
	now := time.Now().Format(time.RFC3339Nano)
	for key, value := range values {
		if _, err := tx.ExecContext(ctx, settingUpsertSQL, accountKey, key, value, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
~~~

Define settingUpsertSQL beside the helper with the same INSERT ... ON CONFLICT statement currently used by saveSettingForAccount. Make saveSettingForAccount delegate to the bulk helper to keep one write path.

Remove warehouse keys from the query list in LoadRuntimeSettings and the value map in SaveRuntimeSettings. Keep warehouse fields and defaults on RuntimeSettings for Wails compatibility.

- [ ] **Step 5: Compose global and GID settings in App**

Add conversion helpers in app.go:

~~~go
func warehouseSettingsFromRuntime(settings storage.RuntimeSettings) storage.WarehouseAutoSellSettings {
	return storage.WarehouseAutoSellSettings{
		Enabled: settings.AutoWarehouseSellEnabled,
		IntervalHour: settings.AutoWarehouseSellIntervalHour,
		Categories: append([]string(nil), settings.AutoWarehouseSellCategories...),
		RefreshOnlyOnAutoSell: true,
	}
}

func applyWarehouseSettings(settings storage.RuntimeSettings, warehouse storage.WarehouseAutoSellSettings) storage.RuntimeSettings {
	settings.AutoWarehouseSellEnabled = warehouse.Enabled
	settings.AutoWarehouseSellIntervalHour = warehouse.IntervalHour
	settings.AutoWarehouseSellCategories = append([]string(nil), warehouse.Categories...)
	settings.WarehouseRefreshOnlyOnAutoSell = true
	return settings
}
~~~

RuntimeSettings must load global machine settings, capture accountKey once, load warehouse preferences for that account, and merge them. SaveRuntimeSettings must capture accountKey once, save machine fields globally, save warehouse preferences to that account, and return the composed normalized DTO. Startup and runtime change detection continue reading only global machine settings.

- [ ] **Step 6: Add App two-account coverage**

Add to app_test.go:

~~~go
func TestRuntimeSettingsWarehousePreferencesFollowConfirmedAccount(t *testing.T) {
	ctx := context.Background()
	app := NewApp()
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil { t.Fatalf("open store: %v", err) }
	defer store.Close()
	app.store = store

	app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001, Nickname: "A"})
	first := app.RuntimeSettings()
	first.AutoWarehouseSellEnabled = true
	first.AutoWarehouseSellIntervalHour = 2
	first.AutoWarehouseSellCategories = []string{"seed"}
	app.SaveRuntimeSettings(first)

	app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002, Nickname: "B"})
	second := app.RuntimeSettings()
	if second.AutoWarehouseSellEnabled || second.AutoWarehouseSellIntervalHour != 6 {
		t.Fatalf("second account inherited first account settings: %#v", second)
	}
}
~~~

- [ ] **Step 7: Verify GREEN**

~~~powershell
go test ./internal/storage -count=1
go test . -run "TestAppSaveRuntimeSettingsPersistsDefaultTarget|TestRuntimeSettingsWarehousePreferencesFollowConfirmedAccount" -count=1
Set-Location frontend
npm test -- AssetsLandView SettingsView
~~~

Expected: PASS. Wails bindings remain unchanged because RuntimeSettings retains its JSON fields.

- [ ] **Step 8: Commit**

~~~powershell
Set-Location ..
git add -- internal/storage/account_preferences.go internal/storage/settings.go internal/storage/storage_test.go app.go app_test.go
git commit -m "feat: scope warehouse preferences by gid"
~~~

### Task 4: Seed Legacy Business Configuration Once Per GID

**Files:**
- Create: internal/storage/account_migration.go
- Modify: internal/storage/storage_test.go
- Modify: app.go
- Modify: app_test.go

- [ ] **Step 1: Add the failing migration test**

Add this complete test to internal/storage/storage_test.go:

~~~go
func TestSeedLegacyAccountConfigurationCopiesMissingBusinessKeysOnce(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil { t.Fatalf("open store: %v", err) }
	defer store.Close()
	legacyDefault := map[string]string{
		"autoFarm.config": "{\"autoFarmFriendEnabled\":true}",
		"autoFarm.task.friend_steal.enabled": "true",
		"messagePush.config": "{\"enabled\":true}",
	}
	legacyGlobal := map[string]string{
		"warehouse.autoSellEnabled": "true",
		"warehouse.autoSellIntervalHour": "2",
		"warehouse.autoSellCategories": "seed",
	}
	if err := store.saveSettingsForAccount(ctx, DefaultRuntimeAccountKey, legacyDefault); err != nil {
		t.Fatalf("seed default: %v", err)
	}
	if err := store.saveSettingsForAccount(ctx, GlobalSettingsAccountKey, legacyGlobal); err != nil {
		t.Fatalf("seed global: %v", err)
	}
	if err := store.saveSettingForAccount(ctx, "gid:10001", "messagePush.config", "{\"enabled\":false}"); err != nil {
		t.Fatalf("seed gid: %v", err)
	}
	if err := store.SeedLegacyAccountConfiguration(ctx, "gid:10001"); err != nil { t.Fatalf("seed account: %v", err) }
	if err := store.SeedLegacyAccountConfiguration(ctx, "gid:10001"); err != nil { t.Fatalf("repeat seed: %v", err) }
	values, err := store.loadSettingsForAccount(ctx, "gid:10001", []string{
		"autoFarm.config", "autoFarm.task.friend_steal.enabled", "messagePush.config",
		"warehouse.autoSellEnabled", "warehouse.autoSellIntervalHour",
		"warehouse.autoSellCategories", legacyAccountConfigSeedMarker,
	})
	if err != nil { t.Fatalf("load destination: %v", err) }
	if values["messagePush.config"] != "{\"enabled\":false}" {
		t.Fatalf("existing gid value was overwritten: %#v", values)
	}
	if values["autoFarm.config"] == "" ||
		values["warehouse.autoSellCategories"] != "seed" ||
		values[legacyAccountConfigSeedMarker] != "done" {
		t.Fatalf("legacy values were not copied: %#v", values)
	}
}
~~~

- [ ] **Step 2: Verify RED**

~~~powershell
go test ./internal/storage -run TestSeedLegacyAccountConfigurationCopiesMissingBusinessKeysOnce -count=1
~~~

Expected: compile failure for the missing migration function and marker.

- [ ] **Step 3: Implement account_migration.go**

Create internal/storage/account_migration.go. Define:

~~~go
const legacyAccountConfigSeedMarker = "migration.accountConfigSeed.v1"

func (s *Store) SeedLegacyAccountConfiguration(ctx context.Context, accountKey string) error {
	accountKey = NormalizeAccountKey(accountKey)
	if accountKey == DefaultRuntimeAccountKey || accountKey == GlobalSettingsAccountKey {
		return errors.New("legacy account configuration requires a gid account key")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()

	var marker string
	err = tx.QueryRowContext(ctx, selectSettingSQL, accountKey, legacyAccountConfigSeedMarker).Scan(&marker)
	if err == nil && marker == "done" { return nil }
	if err != nil && !errors.Is(err, sql.ErrNoRows) { return err }

	now := time.Now().Format(time.RFC3339Nano)
	patterns := []string{"autoFarm.%", "messagePush.%", "social.rankingPreferences", "warehouse.%"}
	for _, source := range []string{DefaultRuntimeAccountKey, GlobalSettingsAccountKey} {
		for _, pattern := range patterns {
			if _, err := tx.ExecContext(ctx, copyMissingSettingsSQL, accountKey, now, source, pattern); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, settingUpsertSQL, accountKey, legacyAccountConfigSeedMarker, "done", now); err != nil {
		return err
	}
	return tx.Commit()
}
~~~

Define selectSettingSQL as the existing single-setting SELECT. Define copyMissingSettingsSQL as INSERT INTO settings SELECT from the source account with key LIKE the pattern and ON CONFLICT(account_key, key) DO NOTHING. Import context, database/sql, errors, and time.

- [ ] **Step 4: Verify storage migration GREEN**

~~~powershell
go test ./internal/storage -run "TestSeedLegacyAccountConfiguration|TestWarehouseAutoSellSettingsAreScopedByAccount|TestStoreAutoFarmSettingsAreScopedByAccount" -count=1
~~~

Expected: PASS.

- [ ] **Step 5: Seed before switching the active account**

In ConfirmRuntimeAccount, after SaveRuntimeAccount succeeds and before currentAccountKey changes:

~~~go
if a.store != nil {
	if err := a.store.SeedLegacyAccountConfiguration(a.contextOrBackground(), account.AccountKey); err != nil {
		account.Error = err.Error()
		return account
	}
}
~~~

A migration failure must leave the previous active account unchanged.

- [ ] **Step 6: Add App confirmation coverage**

Add to app_test.go:

~~~go
func TestConfirmRuntimeAccountSeedsLegacyBusinessConfiguration(t *testing.T) {
	ctx := context.Background()
	app := NewApp()
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil { t.Fatalf("open store: %v", err) }
	defer store.Close()
	app.store = store
	legacy := storage.WarehouseAutoSellSettings{
		Enabled: true, IntervalHour: 2, Categories: []string{"seed"}, RefreshOnlyOnAutoSell: true,
	}
	if err := store.SaveWarehouseAutoSellSettingsForAccount(ctx, storage.GlobalSettingsAccountKey, legacy); err != nil {
		t.Fatalf("seed legacy warehouse: %v", err)
	}
	account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001, Nickname: "A"})
	if account.Error != "" { t.Fatalf("confirm: %#v", account) }
	settings := app.RuntimeSettings()
	if !settings.AutoWarehouseSellEnabled ||
		settings.AutoWarehouseSellIntervalHour != 2 ||
		!reflect.DeepEqual(settings.AutoWarehouseSellCategories, []string{"seed"}) {
		t.Fatalf("legacy preferences were not seeded: %#v", settings)
	}
	settings.AutoWarehouseSellIntervalHour = 9
	app.SaveRuntimeSettings(settings)
	app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001, Nickname: "A"})
	if got := app.RuntimeSettings().AutoWarehouseSellIntervalHour; got != 9 {
		t.Fatalf("repeat confirmation overwrote gid settings: %d", got)
	}
}
~~~

- [ ] **Step 7: Verify App migration GREEN**

~~~powershell
go test . -run "TestConfirmRuntimeAccountSeedsLegacyBusinessConfiguration|TestConfirmRuntimeAccount|TestRuntimeSettingsWarehousePreferencesFollowConfirmedAccount" -count=1
~~~

Expected: PASS.

- [ ] **Step 8: Commit**

~~~powershell
git add -- internal/storage/account_migration.go internal/storage/storage_test.go app.go app_test.go
git commit -m "feat: seed legacy configuration for gid accounts"
~~~

### Task 5: Propagate Persistence Failures to the Frontend

**Files:**
- Modify: app.go
- Modify: app_test.go
- Modify: internal/farm/social/service.go
- Modify: internal/farm/social/service_test.go

- [ ] **Step 1: Add failing error-propagation tests**

Add to app_test.go:

~~~go
func TestSaveFarmAutomationStateReturnsStorageError(t *testing.T) {
	ctx := context.Background()
	app := NewApp()
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil { t.Fatalf("open store: %v", err) }
	app.store = store
	if err := store.Close(); err != nil { t.Fatalf("close store: %v", err) }

	if _, err := app.SaveFarmAutomationState(automation.DefaultState()); err == nil {
		t.Fatal("expected automation save error")
	}
}

func TestSaveRuntimeSettingsReturnsStorageError(t *testing.T) {
	ctx := context.Background()
	app := NewApp()
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil { t.Fatalf("open store: %v", err) }
	app.store = store
	if err := store.Close(); err != nil { t.Fatalf("close store: %v", err) }

	if _, err := app.SaveRuntimeSettings(app.runtimeSettingsFromConfig()); err == nil {
		t.Fatal("expected runtime settings save error")
	}
}

func TestSaveMessagePushConfigReturnsStorageError(t *testing.T) {
	ctx := context.Background()
	app := NewApp()
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil { t.Fatalf("open store: %v", err) }
	app.store = store
	if err := store.Close(); err != nil { t.Fatalf("close store: %v", err) }

	if _, err := app.SaveMessagePushConfig(messagepush.Config{Enabled: true}); err == nil {
		t.Fatal("expected message push save error")
	}
}
~~~

Add to internal/farm/social/service_test.go:

~~~go
type failingRankingPreferencesStore struct {
	*memoryStore
}

func (s *failingRankingPreferencesStore) SaveRankingPreferences(context.Context, string, RankingPreferences) error {
	return errors.New("save failed")
}

func TestServiceSaveRankingPreferencesReturnsStorageError(t *testing.T) {
	store := &failingRankingPreferencesStore{memoryStore: &memoryStore{}}
	service := NewService(store, nil, Options{AccountKey: "gid:10001"})

	if _, err := service.SaveRankingPreferences(
		context.Background(),
		RankingPreferences{StolenFromMeViewMode: "ranking"},
	); err == nil {
		t.Fatal("expected ranking preference save error")
	}
}
~~~

- [ ] **Step 2: Verify RED**

~~~powershell
go test ./internal/farm/social -run TestServiceSaveRankingPreferencesReturnsStorageError -count=1
go test . -run "TestSaveFarmAutomationStateReturnsStorageError|TestSaveRuntimeSettingsReturnsStorageError|TestSaveMessagePushConfigReturnsStorageError" -count=1
~~~

Expected: compile failure because the App methods currently return one value.

- [ ] **Step 3: Return errors from the Wails methods**

Change the signatures:

~~~go
func (a *App) SaveFarmAutomationState(state automation.State) (automation.State, error)
func (a *App) SaveRuntimeSettings(settings storage.RuntimeSettings) (storage.RuntimeSettings, error)
func (a *App) SaveMessagePushConfig(config messagepush.Config) (messagepush.ViewState, error)
func (a *App) SaveFarmSocialRankingPreferences(input social.RankingPreferences) (social.RankingPreferences, error)
~~~

SaveFarmAutomationState returns immediately on SaveAutoFarmSettingsForAccount failure and configures the scheduler only after persistence succeeds. SaveRuntimeSettings returns immediately when either the global machine write or account warehouse write fails. SaveMessagePushConfig returns the service error instead of loading a fallback state.

Change social.Service.SaveRankingPreferences to:

~~~go
func (s *Service) SaveRankingPreferences(ctx context.Context, preferences RankingPreferences) (RankingPreferences, error) {
	preferences = NormalizeRankingPreferences(preferences)
	if store, ok := s.store.(RankingPreferencesStore); ok {
		if err := store.SaveRankingPreferences(ctx, s.accountKey, preferences); err != nil {
			return RankingPreferences{}, err
		}
	}
	return preferences, nil
}
~~~

The App wrapper returns this result directly. On success, every method returns the normalized saved value and nil.

- [ ] **Step 4: Update direct Go call sites**

Update SaveProcessGuardSettings and all direct Go call sites reported by this command:

~~~powershell
rg -n "SaveFarmAutomationState\(|SaveRuntimeSettings\(|SaveMessagePushConfig\(|SaveFarmSocialRankingPreferences\(" --glob '*.go' .
~~~

Use this pattern where success is required:

~~~go
saved, err := app.SaveFarmAutomationState(state)
if err != nil { t.Fatalf("save automation state: %v", err) }
~~~

Use the equivalent two-value pattern for runtime settings, message push, and social preferences. In SaveProcessGuardSettings, include error text in the returned map when SaveRuntimeSettings fails. Frontend TypeScript signatures remain Promise of the saved DTO because Wails converts a non-nil Go error into a rejected Promise.

- [ ] **Step 5: Verify GREEN**

~~~powershell
go test ./internal/farm/social -count=1
go test . -run "TestSaveFarmAutomationStateReturnsStorageError|TestSaveRuntimeSettingsReturnsStorageError|TestSaveMessagePushConfigReturnsStorageError|TestSaveFarmAutomationStatePersistsSchedulerSettings|TestAppSaveRuntimeSettingsPersistsDefaultTarget|TestAppMessagePushSaveAndState|TestFarmSocialRankingPreferencesAreExposed" -count=1
Set-Location frontend
npm test -- AutomationView SettingsView AssetsLandView MessagePushView SocialView
~~~

Expected: PASS, and existing frontend catch blocks receive rejected saves.

- [ ] **Step 6: Commit**

~~~powershell
Set-Location ..
git add -- app.go app_test.go internal/farm/social/service.go internal/farm/social/service_test.go
git commit -m "fix: propagate configuration persistence errors"
~~~

### Task 6: Audit Explicit Scope and JSON Boundaries

**Files:**
- Modify only if a production match is found: app.go and internal packages

- [ ] **Step 1: Find implicit default business-storage calls**

~~~powershell
rg -n "\.(SaveAutoFarmSettings|LoadAutoFarmSettings|AppendRuntimeEvent|ListRuntimeEvents|SaveMessagePushConfigJSON|LoadMessagePushConfigJSON|SaveMessagePushStateJSON|LoadMessagePushStateJSON)\(" --glob '*.go' --glob '!**/*_test.go' .
~~~

Expected: only compatibility method definitions under internal/storage. No application or service call site.

- [ ] **Step 2: Correct every production match**

For each application match, capture accountKey := a.accountKey() once per operation. Use ForAccount automation and event methods, accountMessagePushStore for message push, and existing account-key parameters for social and warehouse methods.

- [ ] **Step 3: Verify runtime config is not written to project JSON**

~~~powershell
rg -n "os\.(WriteFile|Create|OpenFile).*\.json|json\.NewEncoder\(.*os\." --glob '*.go' .
~~~

Expected: no application configuration writer. User-requested export files and static resource readers are allowed.

- [ ] **Step 4: Run account-focused suites**

~~~powershell
go test ./internal/storage -count=1
go test ./internal/farm/social -count=1
go test ./internal/messagepush -count=1
go test . -run "Account|Automation|Warehouse|MessagePush" -count=1
~~~

Expected: PASS.

- [ ] **Step 5: Commit audit corrections only if files changed**

~~~powershell
git add -- app.go internal
git commit -m "refactor: require explicit account scope for business data"
~~~

Do not create an empty commit.

### Task 7: Full Verification

**Files:**
- Verify all modified files.
- Verify design: docs/superpowers/specs/2026-07-10-account-scoped-configuration-and-automation-save-design.md

- [ ] **Step 1: Format Go files**

~~~powershell
gofmt -w app.go app_test.go internal/farm/automation/catalog.go internal/farm/automation/catalog_test.go internal/farm/social/service.go internal/farm/social/service_test.go internal/storage/settings.go internal/storage/account_preferences.go internal/storage/account_migration.go internal/storage/storage_test.go
~~~

- [ ] **Step 2: Run all Go tests**

~~~powershell
go test ./... -count=1
~~~

Expected: PASS.

- [ ] **Step 3: Run all frontend tests**

~~~powershell
Set-Location frontend
npm test
~~~

Expected: PASS.

- [ ] **Step 4: Build the frontend**

~~~powershell
npm run build
~~~

Expected: TypeScript and Vite build complete successfully.

- [ ] **Step 5: Check diff integrity**

~~~powershell
Set-Location ..
git diff --check
git status --short
~~~

Expected: no diff-check errors. Untracked graphify-out directories remain excluded.

- [ ] **Step 6: Confirm the final storage contract**

- Machine settings write only to global.
- Farm business settings write to the captured GID account, or temporary default before confirmation.
- Legacy values copy once and never become a live fallback.
- Repeated automation saves keep config and scheduler task enabled values identical.
- Project JSON files remain static resources, build configuration, or explicit import/export artifacts.
