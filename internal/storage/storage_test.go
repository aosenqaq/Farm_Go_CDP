package storage

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"Farm_Go/internal/eventbus"
)

func TestOpenCreatesDatabaseAndTables(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	rows, err := store.db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()

	tables := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table: %v", err)
		}
		tables[name] = true
	}

	for _, name := range []string{"schema_migrations", "settings", "runtime_events", "diagnostic_runs"} {
		if !tables[name] {
			t.Fatalf("expected table %q, got %#v", name, tables)
		}
	}
}

func TestDatabasePathUsesFarmGoFile(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if store.Path() != filepath.Join(dir, "farm_go.db") {
		t.Fatalf("unexpected path %q", store.Path())
	}
}

func TestOpenConfiguresSQLiteForSerializedWrites(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if got := store.db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("expected one sqlite connection, got %d", got)
	}

	var busyTimeout int
	if err := store.db.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy timeout: %v", err)
	}
	if busyTimeout < 5000 {
		t.Fatalf("expected busy timeout >= 5000ms, got %d", busyTimeout)
	}
}

func TestStoreRuntimeSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	input := RuntimeSettings{
		DefaultTarget: "wechat_cdp",
		CurrentTarget: "yyb_cdp",
		AutoStart:     true,
		CDPPort:       62000,
		WMPFDebugPort: 9420,
	}
	if err := store.SaveRuntimeSettings(ctx, input); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	got, err := store.LoadRuntimeSettings(ctx)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if got.DefaultTarget != input.DefaultTarget || got.CurrentTarget != input.CurrentTarget {
		t.Fatalf("settings mismatch: %#v", got)
	}
	if got.AutoStart != input.AutoStart || got.CDPPort != input.CDPPort || got.WMPFDebugPort != input.WMPFDebugPort {
		t.Fatalf("settings mismatch: %#v", got)
	}
}

func TestWarehouseAutoSellSettingsAreScopedByAccount(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	first := WarehouseAutoSellSettings{
		Enabled:               true,
		IntervalMinute:        120,
		Categories:            []string{"fruit", "seed"},
		RefreshOnlyOnAutoSell: true,
	}
	second := WarehouseAutoSellSettings{
		Enabled:               false,
		IntervalMinute:        540,
		Categories:            []string{"tool"},
		RefreshOnlyOnAutoSell: true,
	}
	if err := store.SaveWarehouseAutoSellSettingsForAccount(ctx, "gid:10001", first); err != nil {
		t.Fatalf("save first: %v", err)
	}
	if err := store.SaveWarehouseAutoSellSettingsForAccount(ctx, "gid:10002", second); err != nil {
		t.Fatalf("save second: %v", err)
	}

	gotFirst, err := store.LoadWarehouseAutoSellSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load first: %v", err)
	}
	gotSecond, err := store.LoadWarehouseAutoSellSettingsForAccount(ctx, "gid:10002")
	if err != nil {
		t.Fatalf("load second: %v", err)
	}
	if !reflect.DeepEqual(gotFirst, first) {
		t.Fatalf("first = %#v, want %#v", gotFirst, first)
	}
	if !reflect.DeepEqual(gotSecond, second) {
		t.Fatalf("second = %#v, want %#v", gotSecond, second)
	}

	defaults, err := store.LoadWarehouseAutoSellSettingsForAccount(ctx, "gid:10003")
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if !defaults.RefreshOnlyOnAutoSell {
		t.Fatalf("refresh-only default = %#v, want true", defaults)
	}
}

func TestDefaultWarehouseAutoSellSettingsUsesSixtyMinutes(t *testing.T) {
	settings := DefaultWarehouseAutoSellSettings()
	if settings.IntervalMinute != 60 {
		t.Fatalf("interval minute = %d, want 60", settings.IntervalMinute)
	}
}

func TestWarehouseAutoSellSettingsMigratesLegacyHoursToMinutes(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	accountKey := "gid:10001"
	if _, err := store.db.ExecContext(ctx, settingUpsertSQL, accountKey, "warehouse.autoSellIntervalHour", "6", time.Now().Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("seed legacy interval: %v", err)
	}

	settings, err := store.LoadWarehouseAutoSellSettingsForAccount(ctx, accountKey)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if settings.IntervalMinute != 360 {
		t.Fatalf("interval minute = %d, want 360", settings.IntervalMinute)
	}

	values, err := store.loadSettingsForAccount(ctx, accountKey, []string{"warehouse.autoSellIntervalMinute"})
	if err != nil {
		t.Fatalf("load migrated interval: %v", err)
	}
	if values["warehouse.autoSellIntervalMinute"] != "360" {
		t.Fatalf("migrated interval = %q, want 360", values["warehouse.autoSellIntervalMinute"])
	}
}

func TestWarehouseAutoSellSettingsPrefersMinuteIntervalOverLegacyHours(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	accountKey := "gid:10001"
	updatedAt := time.Now().Format(time.RFC3339Nano)
	for key, value := range map[string]string{
		"warehouse.autoSellIntervalHour":   "6",
		"warehouse.autoSellIntervalMinute": "90",
	} {
		if _, err := store.db.ExecContext(ctx, settingUpsertSQL, accountKey, key, value, updatedAt); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}

	settings, err := store.LoadWarehouseAutoSellSettingsForAccount(ctx, accountKey)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if settings.IntervalMinute != 90 {
		t.Fatalf("interval minute = %d, want 90", settings.IntervalMinute)
	}
}

func TestGlobalRuntimeSettingsDoNotPersistWarehousePreferences(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	input := RuntimeSettings{
		DefaultTarget:                   "qq_ws",
		CurrentTarget:                   "qq_ws",
		AutoStart:                       true,
		CDPPort:                         62000,
		WMPFDebugPort:                   9420,
		AutoWarehouseSellEnabled:        true,
		AutoWarehouseSellIntervalMinute: 120,
		AutoWarehouseSellCategories:     []string{"seed"},
		WarehouseRefreshOnlyOnAutoSell:  true,
	}
	if err := store.SaveRuntimeSettings(ctx, input); err != nil {
		t.Fatalf("save: %v", err)
	}

	var count int
	if err := store.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM settings
		WHERE account_key = ? AND key LIKE 'warehouse.%'
	`, GlobalSettingsAccountKey).Scan(&count); err != nil {
		t.Fatalf("query global warehouse settings: %v", err)
	}
	if count != 0 {
		t.Fatalf("global settings contain %d warehouse preference keys", count)
	}
}

func TestStoreMessagePushJSONRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	config := `{"enabled":true,"selectedChannels":["webhook"],"channels":{"webhookUrl":"https://example.test/hook"}}`
	if err := store.SaveMessagePushConfigJSON(ctx, config); err != nil {
		t.Fatalf("save message push config: %v", err)
	}
	gotConfig, err := store.LoadMessagePushConfigJSON(ctx)
	if err != nil {
		t.Fatalf("load message push config: %v", err)
	}
	if gotConfig != config {
		t.Fatalf("config mismatch: %s", gotConfig)
	}

	state := `{"recentPushes":[{"kind":"test","title":"农场测试推送","ok":true}]}`
	if err := store.SaveMessagePushStateJSON(ctx, state); err != nil {
		t.Fatalf("save message push state: %v", err)
	}
	gotState, err := store.LoadMessagePushStateJSON(ctx)
	if err != nil {
		t.Fatalf("load message push state: %v", err)
	}
	if gotState != state {
		t.Fatalf("state mismatch: %s", gotState)
	}
}

func TestRuntimeSettingsPersistCompleteGuardianConfiguration(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	input := RuntimeSettings{
		DefaultTarget:                           "qq_ws",
		CurrentTarget:                           "qq_ws",
		AutoStart:                               true,
		CDPPort:                                 62000,
		WMPFDebugPort:                           9420,
		ProcessGuardEnabled:                     true,
		ProcessGuardFailureRecoveryEnabled:      true,
		ProcessGuardTimeoutThreshold:            2,
		ProcessGuardMonitorIntervalMS:           1500,
		ProcessGuardRestartReconnectGraceSec:    30,
		ProcessGuardMaxRestartsPer10Min:         3,
		ProcessGuardScheduledRestartEnabled:     true,
		ProcessGuardScheduledRestartIntervalMin: 45,
		ProcessGuardAutoMinimizeAfterRestart:    true,
		NetworkReconnectEnabled:                 true,
		NetworkReconnectIntervalMS:              1300,
		NetworkReconnectRecoveryTimeoutMS:       18000,
		OtherPlaceLoginReconnectEnabled:         true,
		OtherPlaceLoginCheckIntervalMS:          4500,
		OtherPlaceLoginReconnectDelayMin:        8,
	}
	if err := store.SaveRuntimeSettings(ctx, input); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	got, err := store.LoadRuntimeSettings(ctx)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if !reflect.DeepEqual(guardianSettingsOnly(got), guardianSettingsOnly(input)) {
		t.Fatalf("guardian settings = %#v, want %#v", got, input)
	}
}

func TestRuntimeSettingsGuardianDefaults(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	got, err := store.LoadRuntimeSettings(ctx)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	want := RuntimeSettings{
		ProcessGuardEnabled:                     false,
		ProcessGuardFailureRecoveryEnabled:      true,
		ProcessGuardTimeoutThreshold:            3,
		ProcessGuardMonitorIntervalMS:           3000,
		ProcessGuardRestartReconnectGraceSec:    45,
		ProcessGuardMaxRestartsPer10Min:         4,
		ProcessGuardScheduledRestartEnabled:     false,
		ProcessGuardScheduledRestartIntervalMin: 60,
		ProcessGuardAutoMinimizeAfterRestart:    false,
		NetworkReconnectEnabled:                 true,
		NetworkReconnectIntervalMS:              1000,
		NetworkReconnectRecoveryTimeoutMS:       20000,
		OtherPlaceLoginReconnectEnabled:         false,
		OtherPlaceLoginCheckIntervalMS:          5000,
		OtherPlaceLoginReconnectDelayMin:        5,
	}
	if !reflect.DeepEqual(guardianSettingsOnly(got), want) {
		t.Fatalf("guardian defaults = %#v, want %#v", guardianSettingsOnly(got), want)
	}
}

func TestRuntimeSettingsPersistDisabledDefaultEnabledGuardians(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	input := RuntimeSettings{
		ProcessGuardFailureRecoveryEnabled: false,
		NetworkReconnectEnabled:            false,
	}
	if err := store.SaveRuntimeSettings(ctx, input); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	got, err := store.LoadRuntimeSettings(ctx)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if got.ProcessGuardFailureRecoveryEnabled || got.NetworkReconnectEnabled {
		t.Fatalf("disabled guardian settings were restored to defaults: %#v", got)
	}
}

func guardianSettingsOnly(settings RuntimeSettings) RuntimeSettings {
	return RuntimeSettings{
		ProcessGuardEnabled:                     settings.ProcessGuardEnabled,
		ProcessGuardFailureRecoveryEnabled:      settings.ProcessGuardFailureRecoveryEnabled,
		ProcessGuardTimeoutThreshold:            settings.ProcessGuardTimeoutThreshold,
		ProcessGuardMonitorIntervalMS:           settings.ProcessGuardMonitorIntervalMS,
		ProcessGuardRestartReconnectGraceSec:    settings.ProcessGuardRestartReconnectGraceSec,
		ProcessGuardMaxRestartsPer10Min:         settings.ProcessGuardMaxRestartsPer10Min,
		ProcessGuardScheduledRestartEnabled:     settings.ProcessGuardScheduledRestartEnabled,
		ProcessGuardScheduledRestartIntervalMin: settings.ProcessGuardScheduledRestartIntervalMin,
		ProcessGuardAutoMinimizeAfterRestart:    settings.ProcessGuardAutoMinimizeAfterRestart,
		NetworkReconnectEnabled:                 settings.NetworkReconnectEnabled,
		NetworkReconnectIntervalMS:              settings.NetworkReconnectIntervalMS,
		NetworkReconnectRecoveryTimeoutMS:       settings.NetworkReconnectRecoveryTimeoutMS,
		OtherPlaceLoginReconnectEnabled:         settings.OtherPlaceLoginReconnectEnabled,
		OtherPlaceLoginCheckIntervalMS:          settings.OtherPlaceLoginCheckIntervalMS,
		OtherPlaceLoginReconnectDelayMin:        settings.OtherPlaceLoginReconnectDelayMin,
	}
}

func TestRuntimeSettingsNormalizeGuardianBounds(t *testing.T) {
	tests := []struct {
		name  string
		input RuntimeSettings
		want  RuntimeSettings
	}{
		{
			name: "minimums",
			input: RuntimeSettings{
				ProcessGuardMonitorIntervalMS:     1,
				NetworkReconnectIntervalMS:        1,
				NetworkReconnectRecoveryTimeoutMS: 1,
				OtherPlaceLoginCheckIntervalMS:    1,
				OtherPlaceLoginReconnectDelayMin:  -1,
			},
			want: RuntimeSettings{
				ProcessGuardMonitorIntervalMS:     500,
				NetworkReconnectIntervalMS:        300,
				NetworkReconnectRecoveryTimeoutMS: 1000,
				OtherPlaceLoginCheckIntervalMS:    1000,
				OtherPlaceLoginReconnectDelayMin:  0,
			},
		},
		{
			name: "maximums",
			input: RuntimeSettings{
				ProcessGuardMonitorIntervalMS:     60001,
				NetworkReconnectIntervalMS:        60001,
				NetworkReconnectRecoveryTimeoutMS: 120001,
				OtherPlaceLoginCheckIntervalMS:    60001,
				OtherPlaceLoginReconnectDelayMin:  1441,
			},
			want: RuntimeSettings{
				ProcessGuardMonitorIntervalMS:     60000,
				NetworkReconnectIntervalMS:        60000,
				NetworkReconnectRecoveryTimeoutMS: 120000,
				OtherPlaceLoginCheckIntervalMS:    60000,
				OtherPlaceLoginReconnectDelayMin:  1440,
			},
		},
		{
			name: "zero delay is valid",
			input: RuntimeSettings{
				OtherPlaceLoginReconnectDelayMin: 0,
			},
			want: RuntimeSettings{
				ProcessGuardMonitorIntervalMS:     3000,
				NetworkReconnectIntervalMS:        1000,
				NetworkReconnectRecoveryTimeoutMS: 20000,
				OtherPlaceLoginCheckIntervalMS:    5000,
				OtherPlaceLoginReconnectDelayMin:  0,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			store, err := Open(ctx, t.TempDir())
			if err != nil {
				t.Fatalf("open store: %v", err)
			}
			defer store.Close()

			if err := store.SaveRuntimeSettings(ctx, test.input); err != nil {
				t.Fatalf("save settings: %v", err)
			}
			got, err := store.LoadRuntimeSettings(ctx)
			if err != nil {
				t.Fatalf("load settings: %v", err)
			}
			if got.ProcessGuardMonitorIntervalMS != test.want.ProcessGuardMonitorIntervalMS ||
				got.NetworkReconnectIntervalMS != test.want.NetworkReconnectIntervalMS ||
				got.NetworkReconnectRecoveryTimeoutMS != test.want.NetworkReconnectRecoveryTimeoutMS ||
				got.OtherPlaceLoginCheckIntervalMS != test.want.OtherPlaceLoginCheckIntervalMS ||
				got.OtherPlaceLoginReconnectDelayMin != test.want.OtherPlaceLoginReconnectDelayMin {
				t.Fatalf("guardian bounds = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestStoreAutoFarmSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	input := AutoFarmSettings{
		SchedulerEnabled:  true,
		SchedulerMinGapMS: 750,
		RunMode:           "safe",
		Config: map[string]any{
			"autoFarmOneClickEnabled":              false,
			"autoFarmPlantSeedId":                  float64(100123),
			"autoFarmMysteryShopCurrencyIds":       []any{float64(1001), float64(1004)},
			"autoFarmFriendQuietHoursStart":        "22:30",
			"autoFarmFertilizerHarvestLinkEnabled": true,
		},
		Tasks: []AutoFarmTaskSettings{
			{ID: "friend_steal", Enabled: true, Priority: 120, IntervalSec: 45},
			{ID: "own_collect", Enabled: false, Priority: 11, IntervalSec: 300},
		},
	}
	if err := store.SaveAutoFarmSettings(ctx, input); err != nil {
		t.Fatalf("save auto farm settings: %v", err)
	}

	got, err := store.LoadAutoFarmSettings(ctx)
	if err != nil {
		t.Fatalf("load auto farm settings: %v", err)
	}
	if got.SchedulerEnabled != input.SchedulerEnabled || got.SchedulerMinGapMS != input.SchedulerMinGapMS || got.RunMode != input.RunMode {
		t.Fatalf("scheduler settings mismatch: %#v", got)
	}
	if !reflect.DeepEqual(got.Config, input.Config) {
		t.Fatalf("detailed config mismatch: %#v", got.Config)
	}
	if !reflect.DeepEqual(got.Tasks, input.Tasks) {
		t.Fatalf("task settings mismatch: %#v", got.Tasks)
	}
}

func TestAutoFarmRunModeInitializesFreshAndHistoricalAccounts(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	fresh, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load fresh settings: %v", err)
	}
	if fresh.RunMode != "safe" {
		t.Fatalf("fresh run mode = %q, want safe", fresh.RunMode)
	}
	values, err := store.loadSettingsForAccount(ctx, "gid:10001", []string{"autoFarm.runMode"})
	if err != nil {
		t.Fatalf("load fresh mode: %v", err)
	}
	if values["autoFarm.runMode"] != "safe" {
		t.Fatalf("persisted fresh run mode = %q, want safe", values["autoFarm.runMode"])
	}

	if err := store.saveSettingForAccount(ctx, "gid:10002", "autoFarm.config", "{}"); err != nil {
		t.Fatalf("seed historical settings: %v", err)
	}
	historical, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10002")
	if err != nil {
		t.Fatalf("load historical settings: %v", err)
	}
	if historical.RunMode != "god" {
		t.Fatalf("historical run mode = %q, want god", historical.RunMode)
	}
}

func TestAutoFarmRunModeNormalizesInvalidValuesWithoutChangingTasksOrConfig(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	input := AutoFarmSettings{
		SchedulerEnabled:  true,
		SchedulerMinGapMS: 750,
		RunMode:           "safe",
		Config:            map[string]any{"autoFarmOneClickEnabled": false},
		Tasks:             []AutoFarmTaskSettings{{ID: "own_collect", Enabled: true, Priority: 11, IntervalSec: 300}},
	}
	if err := store.SaveAutoFarmSettingsForAccount(ctx, "gid:10001", input); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	if err := store.saveSettingForAccount(ctx, "gid:10001", "autoFarm.runMode", "invalid"); err != nil {
		t.Fatalf("seed invalid mode: %v", err)
	}

	got, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if got.RunMode != "god" {
		t.Fatalf("normalized mode = %q, want god", got.RunMode)
	}
	if !reflect.DeepEqual(got.Config, input.Config) || !reflect.DeepEqual(got.Tasks, input.Tasks) {
		t.Fatalf("mode migration changed account settings: %#v", got)
	}
	values, err := store.loadSettingsForAccount(ctx, "gid:10001", []string{"autoFarm.runMode"})
	if err != nil {
		t.Fatalf("load normalized mode: %v", err)
	}
	if values["autoFarm.runMode"] != "god" {
		t.Fatalf("persisted normalized mode = %q, want god", values["autoFarm.runMode"])
	}
}

func TestStoreAutoFarmSettingsAreScopedByAccount(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	first := AutoFarmSettings{
		SchedulerEnabled:  true,
		SchedulerMinGapMS: 111,
		Config:            map[string]any{"autoFarmOneClickEnabled": true},
		Tasks:             []AutoFarmTaskSettings{{ID: "own_collect", Enabled: true, Priority: 10, IntervalSec: 60}},
	}
	second := AutoFarmSettings{
		SchedulerEnabled:  false,
		SchedulerMinGapMS: 222,
		Config:            map[string]any{"autoFarmOneClickEnabled": false},
		Tasks:             []AutoFarmTaskSettings{{ID: "friend_steal", Enabled: true, Priority: 20, IntervalSec: 120}},
	}

	if err := store.SaveAutoFarmSettingsForAccount(ctx, "gid:10001", first); err != nil {
		t.Fatalf("save first account settings: %v", err)
	}
	if err := store.SaveAutoFarmSettingsForAccount(ctx, "gid:10002", second); err != nil {
		t.Fatalf("save second account settings: %v", err)
	}

	gotFirst, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load first account settings: %v", err)
	}
	gotSecond, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10002")
	if err != nil {
		t.Fatalf("load second account settings: %v", err)
	}

	if gotFirst.SchedulerMinGapMS != 111 || gotFirst.Tasks[0].ID != "own_collect" {
		t.Fatalf("first account settings leaked or changed: %#v", gotFirst)
	}
	if gotSecond.SchedulerMinGapMS != 222 || gotSecond.Tasks[0].ID != "friend_steal" {
		t.Fatalf("second account settings leaked or changed: %#v", gotSecond)
	}
}

func TestSeedLegacyAccountConfigurationCopiesMissingBusinessKeysOnce(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	legacyDefault := map[string]string{
		"autoFarm.config":                    `{"autoFarmFriendEnabled":true}`,
		"autoFarm.task.friend_steal.enabled": "true",
		"messagePush.config":                 `{"enabled":true}`,
	}
	legacyGlobal := map[string]string{
		"warehouse.autoSellEnabled":      "true",
		"warehouse.autoSellIntervalHour": "2",
		"warehouse.autoSellCategories":   "seed",
		"social.rankingPreferences":      `{"stolenFromMeViewMode":"ranking"}`,
	}
	if err := store.saveSettingsForAccount(ctx, DefaultRuntimeAccountKey, legacyDefault); err != nil {
		t.Fatalf("seed default: %v", err)
	}
	if err := store.saveSettingsForAccount(ctx, GlobalSettingsAccountKey, legacyGlobal); err != nil {
		t.Fatalf("seed global: %v", err)
	}
	if err := store.saveSettingForAccount(ctx, "gid:10001", "messagePush.config", `{"enabled":false}`); err != nil {
		t.Fatalf("seed gid: %v", err)
	}

	if err := store.SeedLegacyAccountConfiguration(ctx, " gid:10001 "); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if err := store.SeedLegacyAccountConfiguration(ctx, "gid:10001"); err != nil {
		t.Fatalf("repeat seed: %v", err)
	}

	keys := []string{
		"autoFarm.config",
		"autoFarm.task.friend_steal.enabled",
		"messagePush.config",
		"warehouse.autoSellEnabled",
		"warehouse.autoSellIntervalHour",
		"warehouse.autoSellCategories",
		"social.rankingPreferences",
		legacyAccountConfigSeedMarker,
	}
	values, err := store.loadSettingsForAccount(ctx, "gid:10001", keys)
	if err != nil {
		t.Fatalf("load destination: %v", err)
	}
	want := map[string]string{
		"autoFarm.config":                    `{"autoFarmFriendEnabled":true}`,
		"autoFarm.task.friend_steal.enabled": "true",
		"messagePush.config":                 `{"enabled":false}`,
		"warehouse.autoSellEnabled":          "true",
		"warehouse.autoSellIntervalHour":     "2",
		"warehouse.autoSellCategories":       "seed",
		"social.rankingPreferences":          `{"stolenFromMeViewMode":"ranking"}`,
		legacyAccountConfigSeedMarker:        "done",
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("seeded values = %#v, want %#v", values, want)
	}

	if err := store.saveSettingsForAccount(ctx, DefaultRuntimeAccountKey, map[string]string{
		"autoFarm.config":    `{"autoFarmFriendEnabled":false}`,
		"messagePush.config": `{"enabled":"changed"}`,
	}); err != nil {
		t.Fatalf("change default source: %v", err)
	}
	if err := store.saveSettingsForAccount(ctx, GlobalSettingsAccountKey, map[string]string{
		"warehouse.autoSellIntervalHour": "9",
		"social.rankingPreferences":      `{"stolenFromMeViewMode":"timeline"}`,
	}); err != nil {
		t.Fatalf("change global source: %v", err)
	}
	if err := store.SeedLegacyAccountConfiguration(ctx, "gid:10001"); err != nil {
		t.Fatalf("seed after source change: %v", err)
	}
	afterSourceChange, err := store.loadSettingsForAccount(ctx, "gid:10001", keys)
	if err != nil {
		t.Fatalf("load destination after source change: %v", err)
	}
	if !reflect.DeepEqual(afterSourceChange, want) {
		t.Fatalf("seeded values followed changed legacy sources: %#v, want %#v", afterSourceChange, want)
	}
}

func TestSeedLegacyAccountConfigurationRejectsLegacyTargetScopes(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	for _, accountKey := range []string{DefaultRuntimeAccountKey, GlobalSettingsAccountKey} {
		t.Run(accountKey, func(t *testing.T) {
			if err := store.SeedLegacyAccountConfiguration(ctx, accountKey); err == nil {
				t.Fatal("expected legacy target scope error")
			}
			values, err := store.loadSettingsForAccount(ctx, accountKey, []string{legacyAccountConfigSeedMarker})
			if err != nil {
				t.Fatalf("load marker: %v", err)
			}
			if _, ok := values[legacyAccountConfigSeedMarker]; ok {
				t.Fatalf("legacy target marker was written: %#v", values)
			}
		})
	}
}

func TestSeedLegacyAccountConfigurationDoesNotCopyCaseVariantsOfSocialRankingPreferences(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if err := store.saveSettingsForAccount(ctx, GlobalSettingsAccountKey, map[string]string{
		"social.rankingPreferences": `{"stolenFromMeViewMode":"ranking"}`,
		"SOCIAL.RANKINGPREFERENCES": `{"unexpected":"upper"}`,
		"Social.RankingPreferences": `{"unexpected":"mixed"}`,
	}); err != nil {
		t.Fatalf("seed global: %v", err)
	}
	if err := store.SeedLegacyAccountConfiguration(ctx, "gid:10001"); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	values, err := store.loadSettingsForAccount(ctx, "gid:10001", []string{
		"social.rankingPreferences",
		"SOCIAL.RANKINGPREFERENCES",
		"Social.RankingPreferences",
	})
	if err != nil {
		t.Fatalf("load destination: %v", err)
	}
	want := map[string]string{
		"social.rankingPreferences": `{"stolenFromMeViewMode":"ranking"}`,
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("seeded social ranking keys = %#v, want %#v", values, want)
	}
}

func TestStoreRuntimeEventsRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	first := eventbus.Event{
		Timestamp: time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC),
		Level:     eventbus.LevelInfo,
		Source:    "runtime",
		Type:      "status",
		Message:   "QQ WS listening",
		Data:      map[string]any{"target": "qq_ws", "connected": false},
	}
	second := eventbus.Event{
		Timestamp: time.Date(2026, 7, 6, 12, 1, 0, 0, time.UTC),
		Level:     eventbus.LevelWarn,
		Source:    "wechat_cdp",
		Type:      "bridge",
		Message:   "Bridge listening but miniapp not connected",
		Data:      map[string]any{"port": float64(9420)},
	}

	if err := store.AppendRuntimeEvent(ctx, first); err != nil {
		t.Fatalf("append first event: %v", err)
	}
	if err := store.AppendRuntimeEvent(ctx, second); err != nil {
		t.Fatalf("append second event: %v", err)
	}

	events, err := store.ListRuntimeEvents(ctx, 1)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %#v", events)
	}
	if events[0].Type != "bridge" || events[0].Level != eventbus.LevelWarn {
		t.Fatalf("expected newest event first, got %#v", events[0])
	}
	if events[0].ID == 0 {
		t.Fatalf("expected database id, got %#v", events[0])
	}
	if events[0].Data["port"] != float64(9420) {
		t.Fatalf("expected data json round trip, got %#v", events[0].Data)
	}
}

func TestStoreRuntimeEventsAreScopedByAccount(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	first := eventbus.Event{
		Timestamp: time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC),
		Level:     eventbus.LevelInfo,
		Source:    "runtime",
		Type:      "account.first",
		Message:   "first account event",
	}
	second := eventbus.Event{
		Timestamp: time.Date(2026, 7, 9, 10, 1, 0, 0, time.UTC),
		Level:     eventbus.LevelInfo,
		Source:    "runtime",
		Type:      "account.second",
		Message:   "second account event",
	}

	if err := store.AppendRuntimeEventForAccount(ctx, "gid:10001", first); err != nil {
		t.Fatalf("append first account event: %v", err)
	}
	if err := store.AppendRuntimeEventForAccount(ctx, "gid:10002", second); err != nil {
		t.Fatalf("append second account event: %v", err)
	}

	events, err := store.ListRuntimeEventsForAccount(ctx, "gid:10001", 10)
	if err != nil {
		t.Fatalf("list scoped events: %v", err)
	}
	if len(events) != 1 || events[0].Type != "account.first" {
		t.Fatalf("expected only first account event, got %#v", events)
	}
}
