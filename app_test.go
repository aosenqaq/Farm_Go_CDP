package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"Farm_Go/internal/config"
	"Farm_Go/internal/eventbus"
	"Farm_Go/internal/farm"
	"Farm_Go/internal/farm/automation"
	"Farm_Go/internal/farm/social"
	"Farm_Go/internal/maintenance"
	"Farm_Go/internal/messagepush"
	farmruntime "Farm_Go/internal/runtime"
	"Farm_Go/internal/runtime/guard"
	"Farm_Go/internal/storage"
)

func TestNewAppExposesInitialRuntimeStatus(t *testing.T) {
	app := newAuthorizedTestApp(t)

	status := app.RuntimeStatus()

	if status.Target != "qq_ws" {
		t.Fatalf("unexpected target %q", status.Target)
	}
	if status.Phase != "idle" {
		t.Fatalf("unexpected phase %q", status.Phase)
	}
	if app.supervisor == nil {
		t.Fatal("expected app to initialize runtime supervisor")
	}
}

func TestAppCDPLinkConfigIncludesEmbeddedButtonScript(t *testing.T) {
	cfg := config.Default()
	got := newCDPLinkConfig(cfg, "embedded-python.exe")

	if got.ButtonScript != runtimeButtonScript || got.ButtonScript == "" {
		t.Fatal("expected CDP links to receive the embedded button script")
	}
	if got.FridaRoot != embeddedFridaRoot {
		t.Fatalf("unexpected Frida root %q", got.FridaRoot)
	}
	if got.FridaPython != "embedded-python.exe" {
		t.Fatalf("unexpected Frida Python path %q", got.FridaPython)
	}
}

func TestAppExposesCacheMaintenanceMethods(t *testing.T) {
	app := newAuthorizedTestApp(t)

	if app.maintenance == nil {
		t.Fatal("maintenance service was not initialized")
	}

	var preview func(*App) (maintenance.Summary, error) = (*App).PreviewWeChatCacheCleanup
	var wechat func(*App) (maintenance.Summary, error) = (*App).CleanWeChatCache
	var qq func(*App) (maintenance.Summary, error) = (*App).CleanQQMiniappCache
	var yyb func(*App) (maintenance.Summary, error) = (*App).CleanYYBMiniappCache
	if preview == nil || wechat == nil || qq == nil || yyb == nil {
		t.Fatal("missing maintenance method")
	}
}

func newAuthorizedTestApp(t *testing.T) *App {
	t.Helper()
	app := NewApp()
	app.authorizationForTests = true
	return app
}

func TestAppExposesConnectionInfo(t *testing.T) {
	app := newAuthorizedTestApp(t)

	info := app.ConnectionInfo()

	if info.URL == "" {
		t.Fatal("expected connection URL")
	}
	if info.TokenPreview == "" {
		t.Fatal("expected token preview")
	}
}

func TestAppRunsDiagnostic(t *testing.T) {
	app := newAuthorizedTestApp(t)

	result := app.RunDiagnostic("host.describe", map[string]any{})

	if result.OK {
		t.Fatal("diagnostic should fail before host connects")
	}
	if result.Error != "runtime link is not connected" {
		t.Fatalf("unexpected error %q", result.Error)
	}
}

func TestFarmBackpackSeedOptionsReturnsRuntimeErrorWhenDisconnected(t *testing.T) {
	app := newAuthorizedTestApp(t)

	payload := app.FarmBackpackSeedOptions(map[string]any{
		"selectedSeedIds": []any{float64(21032)},
	})

	if payload.OK {
		t.Fatalf("expected runtime error before host connects: %#v", payload)
	}
	if payload.Error == "" {
		t.Fatalf("expected error message: %#v", payload)
	}
}

func TestBuildLandRushRuntimeArgsNormalizesDialogPayload(t *testing.T) {
	args, err := buildLandRushRuntimeArgs(map[string]any{
		"landIds":            []any{float64(1), "2", 2, 0},
		"fertilizerMode":     "normal",
		"harvestLinkEnabled": true,
		"rushThresholdSec":   float64(300),
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	want := map[string]any{
		"landIds":                     []int{1, 2},
		"type":                        "normal",
		"mode":                        "normal",
		"dryRun":                      false,
		"cleanupUi":                   true,
		"linkedHarvestAfterFertilize": true,
		"rushThresholdSec":            300,
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args\n got: %#v\nwant: %#v", args, want)
	}
}

func TestBuildLandRushRuntimeArgsRejectsMissingLandIds(t *testing.T) {
	_, err := buildLandRushRuntimeArgs(map[string]any{"fertilizerMode": "organic"})
	if err == nil || !strings.Contains(err.Error(), "请选择至少 1 块需要催熟的地块") {
		t.Fatalf("expected missing land id error, got %v", err)
	}
}

func TestFarmLandRushDelayedManualSubmission(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
	})
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	state := app.FarmAutomationState()
	state.Config["autoFarmFertilizerDelayedSubmitEnabled"] = true
	state.Config["autoFarmFertilizerDelayedSubmitScopes"] = []any{"manual"}
	if _, err := app.SaveFarmAutomationState(state); err != nil {
		t.Fatalf("save automation state: %v", err)
	}

	result := app.FarmLandRush(map[string]any{
		"landIds":                   []any{float64(1), float64(2)},
		"fertilizerMode":            "organic",
		"harvestLinkEnabled":        false,
		"fertilizerSubmissionScope": "manual",
	})

	if result["ok"] != true {
		t.Fatalf("FarmLandRush should report OK, got %#v", result)
	}
	payload := mapFromAny(link.lastArgs("gameCtl.fertilizeLandsBatch")[0])
	if payload["fertilizerSubmissionMode"] != "serial" || intFromAny(payload["betweenLandWait"], 0) != 500 {
		t.Fatalf("manual bulk fertilizer payload = %#v, want serial mode with 500ms wait", payload)
	}
}

func TestFarmLandRushLeavesUnmarkedRequestInBatchMode(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
	})
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	state := app.FarmAutomationState()
	state.Config["autoFarmFertilizerDelayedSubmitEnabled"] = true
	state.Config["autoFarmFertilizerDelayedSubmitScopes"] = []any{"manual"}
	if _, err := app.SaveFarmAutomationState(state); err != nil {
		t.Fatalf("save automation state: %v", err)
	}

	result := app.FarmLandRush(map[string]any{
		"landIds":            []any{float64(1), float64(2)},
		"fertilizerMode":     "organic",
		"harvestLinkEnabled": false,
	})

	if result["ok"] != true {
		t.Fatalf("FarmLandRush should report OK, got %#v", result)
	}
	payload := mapFromAny(link.lastArgs("gameCtl.fertilizeLandsBatch")[0])
	if _, ok := payload["fertilizerSubmissionMode"]; ok {
		t.Fatalf("unmarked land rush payload should use default batch path, got %#v", payload)
	}
}

func TestFarmLandRushCleansDeadCropsAfterLinkedHarvest(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true, "linkedHarvest": map[string]any{"ok": true}},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(4), "stageKind": "dead"},
				map[string]any{"landId": float64(7), "isDead": true},
				map[string]any{"landId": float64(8), "canEraseDead": true},
				map[string]any{"landId": float64(9), "needsEraseDead": true},
				map[string]any{"landId": float64(10), "needEraseDead": true},
				map[string]any{"landId": float64(1), "stageKind": "growing", "isDead": false},
			},
		},
		"gameCtl.shovelLandsBatch": map[string]any{"ok": true, "successCount": float64(5)},
	})

	result := app.FarmLandRush(map[string]any{
		"landIds":            []any{float64(1), float64(4)},
		"fertilizerMode":     "organic",
		"harvestLinkEnabled": true,
	})

	if result["ok"] != true {
		t.Fatalf("FarmLandRush should report OK, got %#v", result)
	}
	if !link.called("gameCtl.fertilizeLandsBatch") || !link.called("gameCtl.getFarmStatus") || !link.called("gameCtl.shovelLandsBatch") {
		t.Fatalf("expected fertilizer, status and shovel calls, got %#v", link.calls)
	}
	wantShovelArgs := []any{map[string]any{
		"landIds":         []int{4, 7, 8, 9, 10},
		"onlyDead":        true,
		"silent":          true,
		"dryRun":          false,
		"waitAfterAction": 0,
		"betweenLandWait": 0,
		"source":          "farm_go_land_rush_dead_cleanup",
	}}
	if !reflect.DeepEqual(link.callArgs["gameCtl.shovelLandsBatch"][0], wantShovelArgs) {
		t.Fatalf("shovel args = %#v, want %#v", link.callArgs["gameCtl.shovelLandsBatch"][0], wantShovelArgs)
	}
	cleanup := mapFromAny(result["deadCleanup"])
	if cleanup["deadCount"] != 5 {
		t.Fatalf("deadCleanup = %#v, want deadCount 5", cleanup)
	}
}

func TestFarmLandRushFertilizesMultiSeasonAfterDeadCleanup(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.fertilizeLandsBatch": fakeRuntimeSequence{
			map[string]any{
				"ok": true,
				"linkedHarvest": map[string]any{
					"ok": true,
					"results": []any{
						map[string]any{"ok": true, "landId": float64(4)},
						map[string]any{"ok": false, "landId": float64(8)},
					},
				},
			},
			map[string]any{"ok": true, "successCount": float64(1)},
		},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(2), "stageKind": "dead"},
				map[string]any{"landId": float64(4), "hasPlant": true, "stageKind": "growing", "isMultiSeason": true, "currentSeason": float64(2), "totalSeason": float64(3)},
			},
		},
		"gameCtl.shovelLandsBatch": map[string]any{"ok": true, "successCount": float64(1)},
	})
	enableManualMultiSeasonFertilizer(t, app)

	result := app.FarmLandRush(map[string]any{
		"landIds":            []any{float64(4)},
		"fertilizerMode":     "organic",
		"harvestLinkEnabled": true,
	})

	if result["ok"] != true {
		t.Fatalf("FarmLandRush should complete cleanup and multi-season fertilizer: %#v", result)
	}
	wantMethods := []string{
		"gameCtl.fertilizeLandsBatch",
		"gameCtl.getFarmStatus",
		"gameCtl.shovelLandsBatch",
		"gameCtl.fertilizeLandsBatch",
	}
	if !reflect.DeepEqual(link.calls, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", link.calls, wantMethods)
	}
	wantFertilizerArgs := []any{map[string]any{
		"landIds":                     []int{4},
		"type":                        "normal",
		"mode":                        "normal",
		"dryRun":                      false,
		"cleanupUi":                   true,
		"linkedHarvestAfterFertilize": false,
		"silent":                      true,
		"source":                      "farm_go_land_rush_multi_season",
	}}
	if got := link.callArgs["gameCtl.fertilizeLandsBatch"][1]; !reflect.DeepEqual(got, wantFertilizerArgs) {
		t.Fatalf("multi-season fertilizer args = %#v, want %#v", got, wantFertilizerArgs)
	}
}

func TestFarmLandRushDoesNotFertilizeMultiSeasonAfterDeadCleanupFails(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.fertilizeLandsBatch": map[string]any{
			"ok": true,
			"linkedHarvest": map[string]any{
				"ok":      true,
				"results": []any{map[string]any{"ok": true, "landId": float64(4)}},
			},
		},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(2), "stageKind": "dead"},
				map[string]any{"landId": float64(4), "hasPlant": true, "stageKind": "growing", "isMultiSeason": true, "currentSeason": float64(2), "totalSeason": float64(3)},
			},
		},
		"gameCtl.shovelLandsBatch": map[string]any{"ok": false, "reason": "shovel_failed"},
	})
	enableManualMultiSeasonFertilizer(t, app)

	result := app.FarmLandRush(map[string]any{
		"landIds":            []any{float64(4)},
		"fertilizerMode":     "organic",
		"harvestLinkEnabled": true,
	})

	if result["ok"] != false {
		t.Fatalf("failed dead cleanup should fail FarmLandRush: %#v", result)
	}
	wantMethods := []string{
		"gameCtl.fertilizeLandsBatch",
		"gameCtl.getFarmStatus",
		"gameCtl.shovelLandsBatch",
	}
	if !reflect.DeepEqual(link.calls, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", link.calls, wantMethods)
	}
}

func enableManualMultiSeasonFertilizer(t *testing.T, app *App) {
	t.Helper()
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open settings store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	app.store = store

	state := app.FarmAutomationState()
	state.Config["autoFarmFertilizerEnabled"] = true
	state.Config["autoFarmFertilizerMultiSeason"] = true
	state.Config["autoFarmPlantFertilizerMode"] = "normal"
	for index := range state.FeatureGroups {
		if state.FeatureGroups[index].ID == "fertilizer" {
			state.FeatureGroups[index].Enabled = true
		}
	}
	if _, err := app.SaveFarmAutomationState(state); err != nil {
		t.Fatalf("save automation state: %v", err)
	}
}

func TestFarmLandRushSkipsDeadCleanupWhenRescanFindsNoDeadCrops(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true, "linkedHarvest": map[string]any{"ok": true}},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(1), "stageKind": "growing", "isDead": false},
			},
		},
	})

	result := app.FarmLandRush(map[string]any{
		"landIds":            []any{float64(1)},
		"fertilizerMode":     "organic",
		"harvestLinkEnabled": true,
	})

	if result["ok"] != true {
		t.Fatalf("FarmLandRush should report OK, got %#v", result)
	}
	if !link.called("gameCtl.getFarmStatus") {
		t.Fatalf("expected post-rush status rescan, got %#v", link.calls)
	}
	if link.called("gameCtl.shovelLandsBatch") {
		t.Fatalf("no dead crops should skip shovel cleanup, got %#v", link.calls)
	}
	cleanup := mapFromAny(result["deadCleanup"])
	if cleanup["deadCount"] != 0 {
		t.Fatalf("deadCleanup = %#v, want deadCount 0", cleanup)
	}
}

func TestFarmLandRushSkipsDeadCleanupWhenLinkedHarvestDisabled(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
	})

	result := app.FarmLandRush(map[string]any{
		"landIds":            []any{float64(1)},
		"fertilizerMode":     "organic",
		"harvestLinkEnabled": false,
	})

	if result["ok"] != true {
		t.Fatalf("FarmLandRush should report OK, got %#v", result)
	}
	if link.called("gameCtl.getFarmStatus") || link.called("gameCtl.shovelLandsBatch") {
		t.Fatalf("disabled linked harvest should skip cleanup, got %#v", link.calls)
	}
	if _, exists := result["deadCleanup"]; exists {
		t.Fatalf("disabled linked harvest should not attach deadCleanup, got %#v", result)
	}
}

func TestBuildFertilizeLandRuntimeArgsNormalizesCardPayload(t *testing.T) {
	args, err := buildFertilizeLandRuntimeArgs(map[string]any{
		"landId": float64(7),
		"type":   "inorganic",
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	want := map[string]any{
		"silent":           true,
		"dryRun":           false,
		"type":             "normal",
		"mode":             "normal",
		"internalFallback": true,
		"landId":           7,
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args\n got: %#v\nwant: %#v", args, want)
	}
}

func TestBuildShovelLandsRuntimeArgsNormalizesCardPayload(t *testing.T) {
	args, err := buildShovelLandsRuntimeArgs(map[string]any{
		"landIds": []any{float64(7), "7", "8", 0},
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	want := map[string]any{
		"silent":          true,
		"dryRun":          false,
		"landIds":         []int{7, 8},
		"waitAfterAction": 250,
		"betweenLandWait": 160,
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args\n got: %#v\nwant: %#v", args, want)
	}
}

func TestAppRuntimeSettingsReturnsDefaults(t *testing.T) {
	app := newAuthorizedTestApp(t)

	settings := app.RuntimeSettings()

	if settings.DefaultTarget != "qq_ws" {
		t.Fatalf("unexpected default target %#v", settings)
	}
	if settings.CurrentTarget != "qq_ws" {
		t.Fatalf("unexpected current target %#v", settings)
	}
	if settings.CDPPort != 62000 || settings.WMPFDebugPort != 9420 {
		t.Fatalf("unexpected port settings %#v", settings)
	}
}

func TestAppSaveRuntimeSettingsPersistsDefaultTarget(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	saved, err := app.SaveRuntimeSettings(storage.RuntimeSettings{
		DefaultTarget: "wechat_cdp",
		CurrentTarget: "qq_ws",
		AutoStart:     true,
		CDPPort:       62000,
		WMPFDebugPort: 9420,
	})
	if err != nil {
		t.Fatalf("save settings: %v", err)
	}

	if saved.DefaultTarget != "wechat_cdp" {
		t.Fatalf("unexpected saved settings %#v", saved)
	}
	loaded, err := store.LoadRuntimeSettings(ctx)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if loaded.DefaultTarget != "wechat_cdp" {
		t.Fatalf("settings were not persisted: %#v", loaded)
	}
}

func TestSaveRuntimeSettingsReturnsStorageErrorBeforeApplyingSettings(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app.store = store
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	input := app.runtimeSettingsFromConfig()
	input.DefaultTarget = "wechat_cdp"
	input.CDPPort++
	if _, err := app.SaveRuntimeSettings(input); err == nil {
		t.Fatal("expected runtime settings save error")
	}
	if app.cfg.Runtime.DefaultTarget != "qq_ws" {
		t.Fatalf("failed save changed runtime config: %#v", app.cfg.Runtime)
	}
	assertOnlyErrorSaveEvent(t, app.memoryEvents, "settings.save")
}

func TestRuntimeSettingsWarehousePreferencesFollowConfirmedAccount(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001, Nickname: "A"}); account.Error != "" {
		t.Fatalf("confirm account A: %s", account.Error)
	}
	first := app.RuntimeSettings()
	first.DefaultTarget = "wechat_cdp"
	if _, err := app.SaveRuntimeSettings(first); err != nil {
		t.Fatalf("save global runtime settings: %v", err)
	}
	if _, err := app.SaveWarehouseAutoSellSettings(storage.WarehouseAutoSellSettings{
		Enabled:        true,
		IntervalMinute: 120,
		Categories:     []string{"seed"},
	}); err != nil {
		t.Fatalf("save account A warehouse settings: %v", err)
	}

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002, Nickname: "B"}); account.Error != "" {
		t.Fatalf("confirm account B: %s", account.Error)
	}
	second := app.RuntimeSettings()
	if second.AutoWarehouseSellEnabled || second.AutoWarehouseSellIntervalMinute != 60 || !reflect.DeepEqual(second.AutoWarehouseSellCategories, []string{"fruit"}) {
		t.Fatalf("account B inherited account A warehouse settings: %#v", second)
	}
	if second.DefaultTarget != "wechat_cdp" {
		t.Fatalf("global default target was not shared: %#v", second)
	}

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001, Nickname: "A"}); account.Error != "" {
		t.Fatalf("switch back to account A: %s", account.Error)
	}
	gotFirst := app.RuntimeSettings()
	if !gotFirst.AutoWarehouseSellEnabled || gotFirst.AutoWarehouseSellIntervalMinute != 120 || !reflect.DeepEqual(gotFirst.AutoWarehouseSellCategories, []string{"seed"}) {
		t.Fatalf("account A warehouse settings were not preserved: %#v", gotFirst)
	}
}

func TestSaveRuntimeSettingsDoesNotOverwriteCurrentAccountWarehousePreferencesFromStaleDTO(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	if err := store.SaveWarehouseAutoSellSettingsForAccount(ctx, "gid:10001", storage.WarehouseAutoSellSettings{
		Enabled: true, IntervalMinute: 120, Categories: []string{"seed"}, RefreshOnlyOnAutoSell: true,
	}); err != nil {
		t.Fatalf("save account A warehouse settings: %v", err)
	}
	if err := store.SaveWarehouseAutoSellSettingsForAccount(ctx, "gid:10002", storage.WarehouseAutoSellSettings{
		Enabled: false, IntervalMinute: 540, Categories: []string{"tool"}, RefreshOnlyOnAutoSell: true,
	}); err != nil {
		t.Fatalf("save account B warehouse settings: %v", err)
	}

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account A: %s", account.Error)
	}
	stale := app.RuntimeSettings()
	if !stale.AutoWarehouseSellEnabled || stale.AutoWarehouseSellIntervalMinute != 120 || !reflect.DeepEqual(stale.AutoWarehouseSellCategories, []string{"seed"}) {
		t.Fatalf("account A combined DTO = %#v", stale)
	}

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		t.Fatalf("confirm account B: %s", account.Error)
	}
	stale.DefaultTarget = "wechat_cdp"
	if _, err := app.SaveRuntimeSettings(stale); err != nil {
		t.Fatalf("save stale runtime DTO: %v", err)
	}

	warehouse, err := store.LoadWarehouseAutoSellSettingsForAccount(ctx, "gid:10002")
	if err != nil {
		t.Fatalf("load account B warehouse settings: %v", err)
	}
	want := storage.WarehouseAutoSellSettings{IntervalMinute: 540, Categories: []string{"tool"}, RefreshOnlyOnAutoSell: true}
	if !reflect.DeepEqual(warehouse, want) {
		t.Fatalf("account B warehouse settings = %#v, want %#v", warehouse, want)
	}
	global, err := store.LoadRuntimeSettings(ctx)
	if err != nil {
		t.Fatalf("load global runtime settings: %v", err)
	}
	if global.DefaultTarget != "wechat_cdp" {
		t.Fatalf("global default target was not saved: %#v", global)
	}
}

func TestSaveProcessGuardSettingsDoesNotChangeWarehousePreferences(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	want := storage.WarehouseAutoSellSettings{Enabled: true, IntervalMinute: 540, Categories: []string{"tool"}, RefreshOnlyOnAutoSell: true}
	if err := store.SaveWarehouseAutoSellSettingsForAccount(ctx, "gid:10002", want); err != nil {
		t.Fatalf("save account B warehouse settings: %v", err)
	}
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		t.Fatalf("confirm account B: %s", account.Error)
	}

	if _, err := app.SaveProcessGuardSettings(map[string]any{"enabled": true, "timeoutThreshold": 2}); err != nil {
		t.Fatalf("save process guard settings: %v", err)
	}

	warehouse, err := store.LoadWarehouseAutoSellSettingsForAccount(ctx, "gid:10002")
	if err != nil {
		t.Fatalf("load account B warehouse settings: %v", err)
	}
	if !reflect.DeepEqual(warehouse, want) {
		t.Fatalf("account B warehouse settings = %#v, want %#v", warehouse, want)
	}
}

func TestSaveProcessGuardSettingsReturnsPersistenceError(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app.store = store
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	result, err := app.SaveProcessGuardSettings(map[string]any{"enabled": true})
	if err == nil {
		t.Fatalf("expected persistence error, got result %#v", result)
	}
	if result["enabled"] != false {
		t.Fatalf("failed save should report unchanged settings: %#v", result)
	}
}

func TestSaveProcessGuardSettingsFailurePreservesPreviousGuardSettings(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app.store = store

	want := map[string]any{
		"enabled":                     true,
		"timeoutThreshold":            7,
		"monitorIntervalMs":           4321,
		"restartReconnectGraceSec":    88,
		"maxRestartsPer10Min":         9,
		"scheduledRestartEnabled":     true,
		"scheduledRestartIntervalMin": 123,
	}
	_, err = app.SaveProcessGuardSettings(want)
	if err != nil {
		t.Fatalf("initial save failed: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	result, err := app.SaveProcessGuardSettings(map[string]any{
		"enabled":                     false,
		"timeoutThreshold":            2,
		"monitorIntervalMs":           1000,
		"restartReconnectGraceSec":    5,
		"maxRestartsPer10Min":         1,
		"scheduledRestartEnabled":     false,
		"scheduledRestartIntervalMin": 10,
	})
	if err == nil {
		t.Fatalf("expected persistence error: %#v", result)
	}
	for key, value := range want {
		if result[key] != value {
			t.Fatalf("failed save changed %s: got %#v want %#v (result=%#v)", key, result[key], value, result)
		}
	}
}

func TestWarehouseAutoSellSettingsAPIIsScopedAndReturnsNormalizedValues(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account A: %s", account.Error)
	}
	first, err := app.SaveWarehouseAutoSellSettings(storage.WarehouseAutoSellSettings{
		Enabled: true, IntervalMinute: 0, Categories: []string{"SEED", "seed", "invalid"},
	})
	if err != nil {
		t.Fatalf("save account A warehouse settings: %v", err)
	}
	wantFirst := storage.WarehouseAutoSellSettings{Enabled: true, IntervalMinute: 60, Categories: []string{"seed"}, RefreshOnlyOnAutoSell: true}
	if !reflect.DeepEqual(first, wantFirst) {
		t.Fatalf("normalized account A settings = %#v, want %#v", first, wantFirst)
	}

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		t.Fatalf("confirm account B: %s", account.Error)
	}
	if second := app.WarehouseAutoSellSettings(); !reflect.DeepEqual(second, storage.DefaultWarehouseAutoSellSettings()) {
		t.Fatalf("account B defaults = %#v", second)
	}
	if _, err := app.SaveWarehouseAutoSellSettings(storage.WarehouseAutoSellSettings{IntervalMinute: 540, Categories: []string{"tool"}}); err != nil {
		t.Fatalf("save account B warehouse settings: %v", err)
	}

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("switch back to account A: %s", account.Error)
	}
	if got := app.WarehouseAutoSellSettings(); !reflect.DeepEqual(got, wantFirst) {
		t.Fatalf("account A settings = %#v, want %#v", got, wantFirst)
	}
}

func TestFarmAutomationStateUsesAccountWarehouseAutoSellSettings(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	if err := store.SaveWarehouseAutoSellSettingsForAccount(ctx, "gid:10001", storage.WarehouseAutoSellSettings{
		Enabled: true, IntervalMinute: 90, Categories: []string{"seed"}, RefreshOnlyOnAutoSell: true,
	}); err != nil {
		t.Fatalf("save account A warehouse settings: %v", err)
	}
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account A: %s", account.Error)
	}

	accountATask := findAutomationTask(app.FarmAutomationState().Scheduler.Tasks, "auto_warehouse_sell")
	if accountATask == nil || !accountATask.Enabled || accountATask.IntervalSec != 90*60 {
		t.Fatalf("account A auto warehouse sell task = %#v", accountATask)
	}

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		t.Fatalf("confirm account B: %s", account.Error)
	}
	accountBTask := findAutomationTask(app.FarmAutomationState().Scheduler.Tasks, "auto_warehouse_sell")
	if accountBTask == nil || accountBTask.Enabled || accountBTask.IntervalSec != 60*60 {
		t.Fatalf("account B auto warehouse sell task = %#v", accountBTask)
	}
}

func TestSaveWarehouseAutoSellSettingsReconfiguresLiveScheduler(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account: %s", account.Error)
	}
	app.FarmAutomationSchedulerState()

	if _, err := app.SaveWarehouseAutoSellSettings(storage.WarehouseAutoSellSettings{
		Enabled: true, IntervalMinute: 90, Categories: []string{"fruit"}, RefreshOnlyOnAutoSell: true,
	}); err != nil {
		t.Fatalf("save warehouse settings: %v", err)
	}

	task := findAutomationTask(app.FarmAutomationSchedulerState().Tasks, "auto_warehouse_sell")
	if task == nil || !task.Enabled || task.IntervalSec != 90*60 {
		t.Fatalf("live scheduler task = %#v", task)
	}
}

func TestSaveFarmAutomationStateDoesNotOverwriteWarehouseAutoSellSettings(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account: %s", account.Error)
	}
	if _, err := app.SaveWarehouseAutoSellSettings(storage.WarehouseAutoSellSettings{
		Enabled: true, IntervalMinute: 120, Categories: []string{"seed"}, RefreshOnlyOnAutoSell: true,
	}); err != nil {
		t.Fatalf("seed warehouse settings: %v", err)
	}

	state := app.FarmAutomationState()
	state.Config["autoWarehouseSellEnabled"] = true
	state.Config["autoWarehouseSellIntervalSec"] = 60 * 60
	for index := range state.Scheduler.Tasks {
		if state.Scheduler.Tasks[index].ID == "auto_warehouse_sell" {
			state.Scheduler.Tasks[index].Enabled = true
			state.Scheduler.Tasks[index].IntervalSec = 60 * 60
		}
	}
	saved, err := app.SaveFarmAutomationState(state)
	if err != nil {
		t.Fatalf("save automation state: %v", err)
	}

	settings, err := store.LoadWarehouseAutoSellSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load warehouse settings: %v", err)
	}
	want := storage.WarehouseAutoSellSettings{Enabled: true, IntervalMinute: 120, Categories: []string{"seed"}, RefreshOnlyOnAutoSell: true}
	if !reflect.DeepEqual(settings, want) {
		t.Fatalf("warehouse settings = %#v, want %#v", settings, want)
	}
	if task := findAutomationTask(saved.Scheduler.Tasks, "auto_warehouse_sell"); task == nil || !task.Enabled || task.IntervalSec != 120*60 {
		t.Fatalf("saved warehouse scheduler task = %#v", task)
	}
}

func TestSaveWarehouseAutoSellSettingsReturnsStorageError(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account: %s", account.Error)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	if _, err := app.SaveWarehouseAutoSellSettings(storage.WarehouseAutoSellSettings{Enabled: true}); err == nil {
		t.Fatal("expected warehouse settings save error")
	}
	assertOnlyErrorSaveEvent(t, app.memoryEvents, "warehouse.auto_sell.settings.save")
}

func TestSaveWarehouseAutoSellSettingsUsesAccountCapturedAtEntry(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account A: %s", account.Error)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	app.beforeWarehouseAutoSellSettingsSave = func() {
		close(entered)
		<-release
	}
	want := storage.WarehouseAutoSellSettings{Enabled: true, IntervalMinute: 120, Categories: []string{"seed"}, RefreshOnlyOnAutoSell: true}
	saveResult := make(chan error, 1)
	go func() {
		_, err := app.SaveWarehouseAutoSellSettings(want)
		saveResult <- err
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("warehouse save did not reach persistence hook")
	}
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		close(release)
		t.Fatalf("confirm account B: %s", account.Error)
	}
	close(release)
	if err := <-saveResult; err != nil {
		t.Fatalf("save account A warehouse settings: %v", err)
	}

	accountA, err := store.LoadWarehouseAutoSellSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load account A settings: %v", err)
	}
	if !reflect.DeepEqual(accountA, want) {
		t.Fatalf("account A settings = %#v, want %#v", accountA, want)
	}
	accountB, err := store.LoadWarehouseAutoSellSettingsForAccount(ctx, "gid:10002")
	if err != nil {
		t.Fatalf("load account B settings: %v", err)
	}
	if !reflect.DeepEqual(accountB, storage.DefaultWarehouseAutoSellSettings()) {
		t.Fatalf("captured account A save leaked to account B: %#v", accountB)
	}
}

func TestRuntimeSettingsKeepsGlobalSettingsWhenWarehousePreferencesFailToLoad(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	if err := store.SaveRuntimeSettings(ctx, storage.RuntimeSettings{
		DefaultTarget: "wechat_cdp", CurrentTarget: "qq_ws", AutoStart: true, CDPPort: 62000, WMPFDebugPort: 9420,
	}); err != nil {
		t.Fatalf("save global runtime settings: %v", err)
	}
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account A: %s", account.Error)
	}

	rawDB, err := sql.Open("sqlite", store.Path())
	if err != nil {
		t.Fatalf("open raw database: %v", err)
	}
	defer rawDB.Close()
	if _, err := rawDB.ExecContext(ctx, `
		INSERT INTO settings (account_key, key, value, updated_at)
		VALUES (?, ?, ?, ?)
	`, "gid:10001", "warehouse.autoSellIntervalHour", "invalid", time.Now().Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("insert invalid warehouse setting: %v", err)
	}

	settings := app.RuntimeSettings()
	if settings.DefaultTarget != "wechat_cdp" {
		t.Fatalf("global settings were discarded: %#v", settings)
	}
	if settings.AutoWarehouseSellEnabled || settings.AutoWarehouseSellIntervalMinute != 60 || !reflect.DeepEqual(settings.AutoWarehouseSellCategories, []string{"fruit"}) || !settings.WarehouseRefreshOnlyOnAutoSell {
		t.Fatalf("warehouse defaults were not applied: %#v", settings)
	}
	if app.lastErr == nil {
		t.Fatal("warehouse load failure was not recorded")
	}
}

func TestAppMessagePushSaveAndState(t *testing.T) {
	app := newAuthorizedTestApp(t)
	ctx := context.Background()
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	app.ensureMessagePushService()

	state, err := app.SaveMessagePushConfig(messagepush.Config{
		Enabled:          true,
		SelectedChannels: []string{"webhook"},
		Channels:         messagepush.Channels{WebhookURL: "https://example.test/hook"},
	})
	if err != nil {
		t.Fatalf("save message push config: %v", err)
	}
	if !state.Config.Enabled || state.Config.SelectedChannels[0] != "webhook" {
		t.Fatalf("unexpected saved state: %#v", state)
	}
	loaded := app.MessagePushState()
	if !loaded.Config.Enabled || loaded.ConfiguredChannels[0] != "webhook" {
		t.Fatalf("unexpected loaded state: %#v", loaded)
	}
}

func TestSaveMessagePushConfigReturnsStorageError(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app.store = store
	app.ensureMessagePushService()
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	if _, err := app.SaveMessagePushConfig(messagepush.Config{Enabled: true}); err == nil {
		t.Fatal("expected message push save error")
	}
	assertOnlyErrorSaveEvent(t, app.memoryEvents, "message_push.config.save")
}

func TestAppMessagePushSendTest(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	app := newAuthorizedTestApp(t)
	ctx := context.Background()
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	app.messagePushHTTPClient = server.Client()
	app.ensureMessagePushService()

	result := app.SendMessagePushTest(messagepush.Config{
		Enabled:          true,
		SelectedChannels: []string{"webhook"},
		Channels:         messagepush.Channels{WebhookURL: server.URL},
	})
	if !result.OK || attempts != 1 {
		t.Fatalf("unexpected send result attempts=%d result=%#v", attempts, result)
	}
	if state := app.MessagePushState(); len(state.RecentPushes) != 1 || !state.RecentPushes[0].OK {
		t.Fatalf("recent push was not recorded: %#v", state.RecentPushes)
	}
}

func TestAppDailyPushUsesLiveAccountWarehouseAndSaleData(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getPlayerProfile": map[string]any{"gid": 10001, "name": "Dpo.L", "level": 118, "gold": 3891552777, "bean": 455521},
		"gameCtl.refreshWarehouseSnapshot": map[string]any{"items": []any{
			map[string]any{"itemId": 41221, "count": 330, "name": "南瓜印章", "saleUnitPrice": 351511, "canSell": true},
		}},
	})
	ctx := context.Background()
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	app.messagePushHTTPClient = server.Client()
	app.currentAccountKey = "gid:10001"
	app.currentAccount = RuntimeAccount{AccountKey: "gid:10001", GID: 10001, Confirmed: true}
	reportDateKey := messagepush.ShiftDateKey(time.Now(), -1)
	if err := store.AppendWarehouseSellRecord(ctx, "gid:10001", storage.WarehouseSellRecord{
		ID: "sale-1", DateKey: reportDateKey, OccurredAt: time.Now().AddDate(0, 0, -1).Format(time.RFC3339Nano), TotalCount: 24, TotalAmount: 10336,
	}); err != nil {
		t.Fatalf("append previous-day sale record: %v", err)
	}

	result := app.SendMessagePushDailyTest(messagepush.Config{
		Enabled: true, DailyEnabled: true, SelectedChannels: []string{"webhook"}, Channels: messagepush.Channels{WebhookURL: server.URL},
		Templates: map[messagepush.MessageType]map[string]messagepush.Template{
			messagepush.MessageTypeDaily: {"webhook": {Enabled: true, Mode: messagepush.TemplateModeJSON, Content: `{"date":"{{daily.date}}","name":"{{daily.name}}","gold":{{daily.gold}},"bean":{{daily.bean}},"warehouseEstimate":{{daily.warehouseEstimate}},"warehouseSellableCount":{{daily.warehouseSellableCount}},"sellCount":{{daily.sellCount}},"sellAmount":{{daily.sellAmount}}}`}},
		},
	})
	if !result.OK {
		t.Fatalf("daily result=%#v", result)
	}
	if body["date"] != reportDateKey || body["name"] != "Dpo.L" || body["gold"] != float64(3891552777) || body["bean"] != float64(455521) || body["warehouseEstimate"] != float64(115998630) || body["warehouseSellableCount"] != float64(330) || body["sellCount"] != float64(24) || body["sellAmount"] != float64(10336) {
		t.Fatalf("daily body=%#v", body)
	}
}

func TestAppDailyReportUsesPreviousDayActivityAndSales(t *testing.T) {
	ctx := context.Background()
	reportAt := time.Date(2026, 7, 21, 9, 0, 0, 0, time.Local)
	previousDay := "2026-07-20"
	currentDay := "2026-07-21"
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getPlayerProfile": map[string]any{"gid": 10001, "name": "Dpo.L", "level": 118, "gold": 3891552777, "bean": 455521},
		"gameCtl.refreshWarehouseSnapshot": map[string]any{"items": []any{
			map[string]any{"itemId": 41221, "count": 330, "name": "南瓜印章", "saleUnitPrice": 351511, "canSell": true},
		}},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	previousEvent := eventbus.Event{
		Timestamp: time.Date(2026, 7, 20, 12, 0, 0, 0, time.Local),
		Source:    "auto_farm",
		Type:      "task.done",
		Data:      map[string]any{"taskId": "own_collect", "ok": true, "status": "ok", "actionCount": 1},
	}
	currentEvent := eventbus.Event{
		Timestamp: time.Date(2026, 7, 21, 8, 0, 0, 0, time.Local),
		Source:    "auto_farm",
		Type:      "task.done",
		Data:      map[string]any{"taskId": "friend_help", "ok": true, "status": "ok"},
	}
	if err := store.AppendRuntimeEventForAccount(ctx, "gid:10001", previousEvent); err != nil {
		t.Fatalf("append previous event: %v", err)
	}
	if err := store.AppendRuntimeEventForAccount(ctx, "gid:10001", currentEvent); err != nil {
		t.Fatalf("append current event: %v", err)
	}
	if err := store.AppendWarehouseSellRecord(ctx, "gid:10001", storage.WarehouseSellRecord{
		ID: "previous-sale", DateKey: previousDay, OccurredAt: previousEvent.Timestamp.Format(time.RFC3339Nano), TotalCount: 3, TotalAmount: 120,
	}); err != nil {
		t.Fatalf("append previous sale: %v", err)
	}
	if err := store.AppendWarehouseSellRecord(ctx, "gid:10001", storage.WarehouseSellRecord{
		ID: "current-sale", DateKey: currentDay, OccurredAt: currentEvent.Timestamp.Format(time.RFC3339Nano), TotalCount: 9, TotalAmount: 999,
	}); err != nil {
		t.Fatalf("append current sale: %v", err)
	}

	report, err := app.dailyReportForAccount(ctx, "gid:10001", "10001", reportAt)
	if err != nil {
		t.Fatalf("daily report: %v", err)
	}
	if report.DateKey != previousDay || report.Values["runs"] != 1 || report.Values["collect"] != 1 || report.Values["help"] != 0 || report.Values["sellCount"] != 3 || report.Values["sellAmount"] != 120 {
		t.Fatalf("report=%#v", report)
	}
}

func TestFullMessagePushQueueRecordsDropWithoutBlockingPublisher(t *testing.T) {
	app, store, _ := newMessagePushApp(t)
	app.messagePushQueue = make(chan messagePushWork, 1)
	app.messagePushQueue <- messagePushWork{accountKey: "gid:10001", event: messagepush.MessageEvent{Type: messagepush.MessageTypeAbnormal}}

	started := time.Now()
	app.enqueueMessagePushEventForAccount("gid:10001", messagepush.MessageEvent{Type: messagepush.MessageTypeAbnormal})
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("full queue blocked publisher for %s", elapsed)
	}

	app.startMessagePushDeliveryWorker(context.Background())
	t.Cleanup(app.stopMessagePushDeliveryWorker)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		events, err := store.ListRuntimeEventsForAccount(context.Background(), "gid:10001", 20)
		if err == nil && containsRuntimeEventType(events, "message_push.queue_dropped") {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("queue drop audit was not persisted by delivery worker")
}

func TestFullMessagePushQueuePublisherDoesNotWaitForSlowStore(t *testing.T) {
	app, store, _ := newMessagePushApp(t)
	app.messagePushQueue = make(chan messagePushWork, 1)
	app.messagePushQueue <- messagePushWork{accountKey: "gid:10001", event: messagepush.MessageEvent{Type: messagepush.MessageTypeAbnormal}}

	rawDB, err := sql.Open("sqlite", store.Path())
	if err != nil {
		t.Fatalf("open raw database: %v", err)
	}
	defer rawDB.Close()
	conn, err := rawDB.Conn(context.Background())
	if err != nil {
		t.Fatalf("open database connection: %v", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatalf("lock database: %v", err)
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), "ROLLBACK") }()

	returned := make(chan struct{})
	go func() {
		app.enqueueMessagePushEventForAccount("gid:10001", messagepush.MessageEvent{Type: messagepush.MessageTypeAbnormal})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("full queue publisher waited for runtime event persistence")
	}
}

func TestMessagePushWorkerShutdownCancelsInFlightHTTPAndSkipsQueuedWork(t *testing.T) {
	app, _, server := newMessagePushApp(t)
	started := make(chan struct{}, 2)
	canceled := make(chan struct{}, 2)
	app.messagePushHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		started <- struct{}{}
		<-request.Context().Done()
		canceled <- struct{}{}
		return nil, request.Context().Err()
	})}
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatal(account.Error)
	}
	if _, err := app.SaveMessagePushConfig(messagepush.Config{
		Enabled:          true,
		AbnormalEnabled:  true,
		PushRetryCount:   1,
		SelectedChannels: []string{"webhook"},
		Channels:         messagepush.Channels{WebhookURL: server.URL},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	defer app.stopMessagePushDeliveryWorker()
	workerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.startMessagePushDeliveryWorker(workerCtx)
	app.enqueueMessagePushEventForAccount("gid:10001", messagepush.MessageEvent{Type: messagepush.MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "first"})
	app.enqueueMessagePushEventForAccount("gid:10001", messagepush.MessageEvent{Type: messagepush.MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "second"})
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first message push request did not start")
	}

	cancel()
	stopped := make(chan struct{})
	go func() {
		app.stopMessagePushDeliveryWorker()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("worker shutdown did not cancel in-flight HTTP")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("HTTP request did not receive cancellation")
	}
	select {
	case <-started:
		t.Fatal("worker started queued delivery after cancellation")
	default:
	}
}

func TestMessagePushDailySchedulerStopsBeforeWorkerAndDoesNotEnqueueAfterStop(t *testing.T) {
	app := NewApp()
	app.messagePushDailyInterval = time.Millisecond
	app.startMessagePushDailyScheduler(context.Background())
	deadline := time.Now().Add(time.Second)
	for len(app.messagePushQueue) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(app.messagePushQueue) == 0 {
		t.Fatal("daily scheduler did not enqueue work")
	}

	app.stopMessagePushDailyScheduler()
	for len(app.messagePushQueue) > 0 {
		<-app.messagePushQueue
	}
	time.Sleep(20 * time.Millisecond)
	if len(app.messagePushQueue) != 0 {
		t.Fatal("daily scheduler enqueued work after stop returned")
	}
	app.stopMessagePushDailyScheduler()
}

func TestMessagePushEventFromLifecycleMapsEveryDeliveryType(t *testing.T) {
	tests := []struct {
		kind guard.LifecycleKind
		want messagepush.MessageType
	}{
		{guard.LifecycleAbnormal, messagepush.MessageTypeAbnormal},
		{guard.LifecycleRecovery, messagepush.MessageTypeRecovery},
		{guard.LifecycleRestartCompleted, messagepush.MessageTypeRestart},
	}
	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			event, ok := messagePushEventFromLifecycle(guard.LifecycleEvent{
				Kind:          test.kind,
				RuntimeTarget: "qq_ws",
				Error:         "timeout",
				Trigger:       "auto",
				DurationMS:    42000,
				Result:        guard.RestartResult{Reason: "timeout"},
			})
			if !ok || event.Type != test.want {
				t.Fatalf("event=%#v ok=%v", event, ok)
			}
			if test.kind == guard.LifecycleRecovery {
				recovery, ok := event.Values["recovery"].(map[string]any)
				if !ok || recovery["duration"] != "42s" {
					t.Fatalf("recovery values=%#v", event.Values)
				}
			}
		})
	}
}

func TestAppRoutesWarningAndErrorRuntimeLogsToMessagePushQueue(t *testing.T) {
	app, _, server := newMessagePushApp(t)
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatal(account.Error)
	}
	if _, err := app.SaveMessagePushConfig(messagepush.Config{
		Enabled:           true,
		LogMonitorEnabled: true,
		SelectedChannels:  []string{"webhook"},
		Channels:          messagepush.Channels{WebhookURL: server.URL},
		LogMonitorRules: []messagepush.LogMonitorRule{
			{Enabled: true, Source: "warn_source"},
			{Enabled: true, Source: "error_source"},
		},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	app.startMessagePushDeliveryWorker(context.Background())
	t.Cleanup(app.stopMessagePushDeliveryWorker)
	app.recordEventForAccount("gid:10001", eventbus.Event{Level: eventbus.LevelWarn, Source: "warn_source", Type: "runtime.warn", Message: "warning"})
	app.recordEventForAccount("gid:10001", eventbus.Event{Level: eventbus.LevelError, Source: "error_source", Type: "runtime.error", Message: "error"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state := app.messagePushServiceForAccount("gid:10001", "10001")
		view, err := state.State(context.Background())
		if err == nil && len(view.RecentPushes) == 2 && view.RecentPushes[0].Kind == "log_monitor" && view.RecentPushes[1].Kind == "log_monitor" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("warning and error runtime logs were not delivered")
}

func TestAppDispatchesMessagePushEventForCapturedAccount(t *testing.T) {
	app, store, server := newMessagePushApp(t)
	account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001})
	if account.Error != "" {
		t.Fatal(account.Error)
	}
	if _, err := app.SaveMessagePushConfig(messagepush.Config{
		Enabled:          true,
		AbnormalEnabled:  true,
		SelectedChannels: []string{"webhook"},
		Channels:         messagepush.Channels{WebhookURL: server.URL},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app.enqueueMessagePushEventForAccount(account.AccountKey, messagepush.MessageEvent{Type: messagepush.MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"})
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		t.Fatal(account.Error)
	}
	app.startMessagePushDeliveryWorker(context.Background())
	t.Cleanup(app.stopMessagePushDeliveryWorker)

	waitForMessagePush(t, app, "gid:10001", "abnormal")
	state, err := store.LoadMessagePushStateJSONForAccount(context.Background(), "gid:10001")
	if err != nil || !strings.Contains(state, `"kind":"abnormal"`) {
		t.Fatalf("captured account state=%q err=%v", state, err)
	}
	other, err := store.LoadMessagePushStateJSONForAccount(context.Background(), "gid:10002")
	if err != nil || strings.Contains(other, `"kind":"abnormal"`) {
		t.Fatalf("wrong account state=%q err=%v", other, err)
	}
}

func TestMessagePushDeliveryQueueDoesNotUseFarmAutomationScheduler(t *testing.T) {
	app, _, server := newMessagePushApp(t)
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatal(account.Error)
	}
	if _, err := app.SaveMessagePushConfig(messagepush.Config{
		Enabled:          true,
		AbnormalEnabled:  true,
		SelectedChannels: []string{"webhook"},
		Channels:         messagepush.Channels{WebhookURL: server.URL},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	app.startMessagePushDeliveryWorker(context.Background())
	t.Cleanup(app.stopMessagePushDeliveryWorker)

	app.automationSchedulerActionMu.Lock()
	defer app.automationSchedulerActionMu.Unlock()
	app.enqueueMessagePushEventForAccount("gid:10001", messagepush.MessageEvent{Type: messagepush.MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"})
	waitForMessagePush(t, app, "gid:10001", "abnormal")
}

func TestAppDoesNotRecursivelyDispatchMessagePushAuditEvents(t *testing.T) {
	app, _, _ := newMessagePushApp(t)

	app.recordEventForAccount("gid:10001", eventbus.Event{Level: eventbus.LevelError, Source: "message_push", Type: "message_push.sent", Message: "sent"})
	if len(app.messagePushQueue) != 0 {
		t.Fatal("message push audit enqueued recursive delivery")
	}
}

func TestAppMapsGuardLifecycleEventsToMessagePushQueue(t *testing.T) {
	app, _, server := newMessagePushApp(t)
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatal(account.Error)
	}
	if _, err := app.SaveMessagePushConfig(messagepush.Config{
		Enabled:          true,
		SuspectedEnabled: true,
		SelectedChannels: []string{"webhook"},
		Channels:         messagepush.Channels{WebhookURL: server.URL},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	app.startMessagePushDeliveryWorker(context.Background())
	t.Cleanup(app.stopMessagePushDeliveryWorker)
	app.guard.UpdateSettings(guard.Settings{Enabled: true, FailureRecoveryEnabled: true, TimeoutThreshold: 3})
	app.guard.Arm("test")
	app.guard.NoteRuntimeError(errors.New("timeout"), guard.RuntimeSnapshot{RuntimeTarget: "qq_ws"})

	waitForMessagePush(t, app, "gid:10001", "suspected")
}

func TestAppPreviewMessagePushTemplateDoesNotSendHTTP(t *testing.T) {
	app, _, server := newMessagePushApp(t)
	requests := make(chan struct{}, 1)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		w.WriteHeader(http.StatusOK)
	})
	preview := app.PreviewMessagePushTemplate(messagepush.Config{
		SelectedChannels: []string{"webhook"},
		Channels:         messagepush.Channels{WebhookURL: server.URL},
	}, string(messagepush.MessageTypeAbnormal), "webhook")
	if !preview.OK || preview.Rendered.JSON == nil {
		t.Fatalf("preview=%#v", preview)
	}
	select {
	case <-requests:
		t.Fatal("preview made an HTTP request")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAppSendMessagePushTemplateTestOnlyUsesDraftAndSelectedChannel(t *testing.T) {
	app, store, server := newMessagePushApp(t)
	requests := 0
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	})
	config := messagepush.Config{
		SelectedChannels: []string{"webhook", "wecom"},
		Channels: messagepush.Channels{
			WebhookURL:   server.URL,
			WecomWebhook: server.URL,
		},
		Templates: map[messagepush.MessageType]map[string]messagepush.Template{
			messagepush.MessageTypeAbnormal: {
				"webhook": {Enabled: true, Mode: messagepush.TemplateModeJSON, Content: `{"message":"{{error.message}}"}`},
			},
		},
	}
	result := app.SendMessagePushTemplateTest(config, string(messagepush.MessageTypeAbnormal), "webhook")
	if !result.OK || requests != 1 {
		t.Fatalf("result=%#v requests=%d", result, requests)
	}
	state, err := store.LoadMessagePushStateJSONForAccount(context.Background(), storage.DefaultRuntimeAccountKey)
	if err != nil || strings.TrimSpace(state) != "" {
		t.Fatalf("draft test mutated state=%q err=%v", state, err)
	}
}

func newMessagePushApp(t *testing.T) (*App, *storage.Store, *httptest.Server) {
	t.Helper()
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(server.Close)
	app := NewApp()
	app.authorizationForTests = true
	app.store = store
	app.messagePushHTTPClient = server.Client()
	return app, store, server
}

func waitForMessagePush(t *testing.T, app *App, accountKey string, kind string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		service := app.messagePushServiceForAccount(accountKey, "")
		state, err := service.State(context.Background())
		if err == nil && len(state.RecentPushes) > 0 && state.RecentPushes[0].Kind == kind {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s message push for %s", kind, accountKey)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestAppMessagePushInvalidTemplateReturnsErrorSummary(t *testing.T) {
	for _, send := range []struct {
		name string
		call func(*App, messagepush.Config) messagepush.SendResult
	}{
		{"test", (*App).SendMessagePushTest},
		{"daily", (*App).SendMessagePushDailyTest},
	} {
		t.Run(send.name, func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			app := NewApp()
			app.authorizationForTests = true
			app.messagePushHTTPClient = server.Client()
			result := send.call(app, messagepush.Config{
				Enabled:          true,
				SelectedChannels: []string{"wecom"},
				Channels:         messagepush.Channels{WecomWebhook: server.URL},
				Templates: map[messagepush.MessageType]map[string]messagepush.Template{
					messagepush.MessageTypeDaily: {
						"wecom": {Enabled: true, Mode: messagepush.TemplateModeCard, Content: `{"title":"missing envelope"}`},
					},
				},
			})
			if result.OK || !strings.Contains(result.Summary, "卡片") {
				t.Fatalf("result=%#v", result)
			}
			if attempts != 0 {
				t.Fatalf("invalid config made %d requests", attempts)
			}
		})
	}
}

func TestAppMessagePushUsesCapturedAccountTemplateContext(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	app := NewApp()
	app.authorizationForTests = true
	app.messagePushHTTPClient = server.Client()
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account: %s", account.Error)
	}
	result := app.SendMessagePushTest(messagepush.Config{
		SelectedChannels: []string{"webhook"},
		Channels:         messagepush.Channels{WebhookURL: server.URL},
		Templates: map[messagepush.MessageType]map[string]messagepush.Template{
			messagepush.MessageTypeDaily: {
				"webhook": {Enabled: true, Mode: messagepush.TemplateModeJSON, Content: `{"key":"{{account.key}}","gid":"{{account.gid}}"}`},
			},
		},
	})
	if !result.OK {
		t.Fatalf("result=%#v", result)
	}
	if body["key"] != "gid:10001" || body["gid"] != "10001" {
		t.Fatalf("body=%#v", body)
	}
}

func TestMessagePushInFlightOperationKeepsCapturedAccountPersistenceAndEvents(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-releaseRequest
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	app.messagePushHTTPClient = server.Client()
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account A: %s", account.Error)
	}

	config := messagepush.Config{
		Enabled:          true,
		SelectedChannels: []string{"webhook"},
		Channels:         messagepush.Channels{WebhookURL: server.URL},
	}
	resultCh := make(chan messagepush.SendResult, 1)
	go func() {
		resultCh <- app.SendMessagePushTest(config)
	}()
	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("message push request did not start")
	}
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		close(releaseRequest)
		t.Fatalf("confirm account B: %s", account.Error)
	}
	close(releaseRequest)
	select {
	case result := <-resultCh:
		if !result.OK {
			t.Fatalf("message push result = %#v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("message push request did not finish")
	}

	stateA, err := store.LoadMessagePushStateJSONForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load account A message push state: %v", err)
	}
	stateB, err := store.LoadMessagePushStateJSONForAccount(ctx, "gid:10002")
	if err != nil {
		t.Fatalf("load account B message push state: %v", err)
	}
	if !strings.Contains(stateA, `"kind":"test"`) || strings.Contains(stateB, `"kind":"test"`) {
		t.Fatalf("in-flight push persisted to wrong scope: A=%q B=%q", stateA, stateB)
	}
	eventsA, err := store.ListRuntimeEventsForAccount(ctx, "gid:10001", 20)
	if err != nil {
		t.Fatalf("load account A events: %v", err)
	}
	eventsB, err := store.ListRuntimeEventsForAccount(ctx, "gid:10002", 20)
	if err != nil {
		t.Fatalf("load account B events: %v", err)
	}
	if !containsRuntimeEventType(eventsA, "message_push.test") || containsRuntimeEventType(eventsB, "message_push.test") {
		t.Fatalf("in-flight push event used wrong scope: A=%#v B=%#v", eventsA, eventsB)
	}
}

func TestMessagePushServiceCacheRejectsPreviousAccountAfterScopePublish(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account A: %s", account.Error)
	}
	serviceA, keyA := app.ensureMessagePushService()

	app.accountMu.Lock()
	app.currentAccountKey = "gid:10002"
	app.currentAccount = RuntimeAccount{AccountKey: "gid:10002", GID: 10002, Confirmed: true}
	app.accountMu.Unlock()
	serviceB, keyB := app.ensureMessagePushService()

	if keyA != "gid:10001" || keyB != "gid:10002" || serviceA == serviceB {
		t.Fatalf("message push cache reused stale service: serviceA=%p keyA=%q serviceB=%p keyB=%q", serviceA, keyA, serviceB, keyB)
	}
}

func TestStartupRuntimeStatusUsesPersistedDefaultTargetBeforeAutostart(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	store, err := storage.Open(ctx, filepath.Join(dataDir, "data"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.SaveRuntimeSettings(ctx, storage.RuntimeSettings{
		DefaultTarget: "wechat_cdp",
		CurrentTarget: "wechat_cdp",
		AutoStart:     false,
		CDPPort:       62000,
		WMPFDebugPort: 9420,
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	store.Close()

	app := newAuthorizedTestApp(t)
	app.dataDir = dataDir
	app.startup(ctx)
	defer app.shutdown(ctx)
	if err := app.activateAuthorizedRuntime(ctx, 1); err != nil {
		t.Fatalf("activate: %v", err)
	}

	status := app.RuntimeStatus()
	if status.Target != "wechat_cdp" || status.Phase != farmruntime.PhaseIdle {
		t.Fatalf("expected persisted target before autostart, got %#v", status)
	}
}

func TestStartupRuntimeSettingsCurrentTargetResetsToDefaultTarget(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	store, err := storage.Open(ctx, filepath.Join(dataDir, "data"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.SaveRuntimeSettings(ctx, storage.RuntimeSettings{
		DefaultTarget: "wechat_cdp",
		CurrentTarget: "yyb_cdp",
		AutoStart:     false,
		CDPPort:       62000,
		WMPFDebugPort: 9420,
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	store.Close()

	app := newAuthorizedTestApp(t)
	app.dataDir = dataDir
	app.startup(ctx)
	defer app.shutdown(ctx)
	if err := app.activateAuthorizedRuntime(ctx, 1); err != nil {
		t.Fatalf("activate: %v", err)
	}

	settings := app.RuntimeSettings()
	if settings.DefaultTarget != "wechat_cdp" || settings.CurrentTarget != "wechat_cdp" {
		t.Fatalf("expected current target to reset to default on startup, got %#v", settings)
	}
}

func TestAppExposesProcessGuardStatus(t *testing.T) {
	app := newAuthorizedTestApp(t)
	status := app.ProcessGuardStatus()
	if status.Phase == "" {
		t.Fatalf("expected guard phase, got %#v", status)
	}
}

func TestAppStartsGuardianIndependentlyFromAutomationScheduler(t *testing.T) {
	app := newAuthorizedTestApp(t)
	app.dataDir = t.TempDir()
	app.startup(context.Background())
	defer app.shutdown(context.Background())
	if err := app.store.SaveRuntimeSettings(context.Background(), storage.RuntimeSettings{DefaultTarget: "wechat_cdp", CurrentTarget: "wechat_cdp", AutoStart: false}); err != nil {
		t.Fatalf("save runtime settings: %v", err)
	}
	if err := app.activateAuthorizedRuntime(context.Background(), 1); err != nil {
		t.Fatalf("activate: %v", err)
	}

	if app.automationScheduler != nil && app.automationScheduler.State().Running {
		t.Fatal("scheduler unexpectedly running")
	}
	if app.guardian == nil || !app.guardian.Status().Running {
		t.Fatal("guardian is not running")
	}
}

func TestAppRuntimeCallerReportsRestartableErrorsToGuardian(t *testing.T) {
	guardian := newTestGuardian(t)
	caller := guardianRuntimeCaller{
		base:       fakeGuardianRuntimeCaller{err: context.DeadlineExceeded},
		guardian:   guardian,
		generation: guardian.Generation,
	}

	_, _ = caller.Call(context.Background(), "gameCtl.getFarmStatus", nil, time.Second)
	if status := guardian.Status(); status.Process.TimeoutStreak != 1 {
		t.Fatalf("guardian did not record runtime error: %#v", status)
	}
}

func TestAppProcessRecoveryPausesAndResumesAutomationDispatch(t *testing.T) {
	app := newAuthorizedTestApp(t)
	app.setGuardianDispatchPaused(true)
	blocked := app.executeFarmAutomationTaskForAccount(context.Background(), "own_collect", "auto", "default")
	if blocked.Status != automation.StatusRuntimeBusy {
		t.Fatalf("expected paused dispatch result, got %#v", blocked)
	}
	app.setGuardianDispatchPaused(false)
	if app.guardianDispatchIsPaused() {
		t.Fatal("dispatch remained paused")
	}
}

func TestAppRecordsOtherPlaceLoginEventsForCurrentAccount(t *testing.T) {
	app := newAppWithTempStore(t)
	app.currentAccountKey = "gid:10001"
	app.handleGuardianRuntimeEvent(guard.RuntimeEvent{
		Name:        "other_place_login_reconnect",
		Phase:       "waiting",
		RemainingMS: 60000,
	})

	events, err := app.store.ListRuntimeEventsForAccount(context.Background(), "gid:10001", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "guardian.other_place_login.waiting" {
		t.Fatalf("unexpected account events: %#v", events)
	}
}

type fakeGuardianRuntimeCaller struct {
	err error
}

func (c fakeGuardianRuntimeCaller) Call(context.Context, string, []any, time.Duration) (any, error) {
	return nil, c.err
}

func newTestGuardian(t *testing.T) *guard.Supervisor {
	t.Helper()
	settings := guard.DefaultSettings()
	settings.Enabled = true
	guardian := guard.NewSupervisor(guard.SupervisorOptions{Settings: settings})
	guardian.Start(context.Background())
	t.Cleanup(guardian.Close)
	return guardian
}

func newAppWithTempStore(t *testing.T) *App {
	t.Helper()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app.store = store
	t.Cleanup(func() { _ = store.Close() })
	return app
}

func TestMessagePushFallbackDiagnosticRecordsAccountScopedAudit(t *testing.T) {
	app := newAppWithTempStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(server.Close)
	app.messagePushHTTPClient = server.Client()
	service := app.messagePushServiceForAccount("gid:10001", "10001")
	config := messagepush.NormalizeConfig(messagepush.Config{
		Enabled: true, AbnormalEnabled: true, SelectedChannels: []string{"webhook"}, Channels: messagepush.Channels{WebhookURL: server.URL},
		Templates: map[messagepush.MessageType]map[string]messagepush.Template{
			messagepush.MessageTypeAbnormal: {"webhook": {Enabled: true, Mode: messagepush.TemplateModeJSON, Content: `{"title":"{{error.missing}}"}`}},
		},
	})
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.store.SaveMessagePushConfigJSONForAccount(context.Background(), "gid:10001", string(raw)); err != nil {
		t.Fatal(err)
	}
	if result, err := service.Dispatch(context.Background(), messagepush.MessageEvent{Type: messagepush.MessageTypeAbnormal, Error: "timeout"}); err != nil || !result.Sent || result.FallbackError == "" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	eventsA, err := app.store.ListRuntimeEventsForAccount(context.Background(), "gid:10001", 10)
	if err != nil {
		t.Fatal(err)
	}
	eventsB, err := app.store.ListRuntimeEventsForAccount(context.Background(), "gid:10002", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !hasRuntimeEventType(eventsA, "message_push.template_fallback") || hasRuntimeEventType(eventsB, "message_push.template_fallback") {
		t.Fatalf("eventsA=%#v eventsB=%#v", eventsA, eventsB)
	}
}

func hasRuntimeEventType(events []eventbus.Event, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func TestAppAutoBindsOnlyAvailableHostCandidate(t *testing.T) {
	app := newAuthorizedTestApp(t)
	app.cfg.Runtime.CurrentTarget = "wechat_cdp"
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
		return []guard.HostProcessSnapshot{
			{PID: 42, ProcessName: "WeChatAppEx.exe", Windows: []guard.HostWindowSnapshot{{PID: 42, Title: "QQ经典农场", Visible: true}}},
		}, nil
	}

	result := app.AutoBindHostProcess()

	if result.Status != "bound" || result.Binding == nil || result.Binding.PID != 42 {
		t.Fatalf("expected auto bound host, got %#v", result)
	}
}

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

func TestAppWeChatRestartPreservesHostAndWaitsForReconnect(t *testing.T) {
	app := newAuthorizedTestApp(t)
	var calls []string
	configureWeChatSoftRestartTest(t, app, &calls)

	result := app.RestartHostProcess()

	if result.Status != guard.RestartStatusReconnected || result.Stopped || !result.LaunchDispatched {
		t.Fatalf("expected final WeChat reconnect, got %#v", result)
	}
	if result.Binding == nil || result.Binding.PID != 42 || !reflect.DeepEqual(result.Binding.HWNDs, []uint64{421}) {
		t.Fatalf("expected refreshed same-PID binding, got %#v", result.Binding)
	}
	want := []string{"close:420", "wait:disconnected", "launch", "wait:ready", "refresh"}
	if strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected restart order: got %#v want %#v", calls, want)
	}
}

func TestRunGuardianActionAcceptsWeChatReconnected(t *testing.T) {
	app := newAuthorizedTestApp(t)
	configureWeChatSoftRestartTest(t, app, nil)

	result := app.RunGuardianAction(guard.ActionRequest{Action: "restart"})
	if !result.OK || result.Status != guard.RestartStatusReconnected || result.Error != "" {
		t.Fatalf("guardian action = %#v", result)
	}
}

func TestGuardianWeChatRestartMinimizesRefreshedWindowAfterReady(t *testing.T) {
	app := newAuthorizedTestApp(t)
	configureWeChatSoftRestartTest(t, app, nil)
	if _, err := app.SaveGuardianSettings(map[string]any{"autoMinimizeAfterRestart": true}); err != nil {
		t.Fatalf("enable auto minimize: %v", err)
	}
	var minimized []guard.HostWindowSnapshot
	app.minimizeHostWindows = func(windows []guard.HostWindowSnapshot) error {
		minimized = append(minimized, windows...)
		return nil
	}

	result := app.RestartHostProcess()
	if result.Status != guard.RestartStatusReconnected {
		t.Fatalf("restart = %#v", result)
	}
	if len(minimized) != 1 || minimized[0].PID != 42 || minimized[0].HWND != 421 {
		t.Fatalf("minimized = %#v, want refreshed HWND 421", minimized)
	}
}

func TestAppQQRestartClosesAndWaitsForReplacementReady(t *testing.T) {
	app := newAuthorizedTestApp(t)
	app.cfg.Runtime.CurrentTarget = "qq_ws"
	app.manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-1"})
	initial := qqAppSnapshots(20, 200)
	refreshed := qqAppSnapshots(30, 300)
	calls := []string{}
	app.closeHostWindows = func(windows []guard.HostWindowSnapshot) error {
		calls = append(calls, "close:"+itoa(int(windows[0].HWND)))
		return nil
	}
	app.waitForQQMiniappClosed = func(rootPID int, timeout time.Duration) (guard.QQCloseObservation, error) {
		calls = append(calls, "closed:"+itoa(rootPID)+":"+timeout.String())
		return guard.QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: qqAppMainOnlySnapshots()}, nil
	}
	app.stopHostPID = func(pid int) error {
		t.Fatalf("unexpected stop PID %d", pid)
		return nil
	}
	app.launchHost = func(guard.LaunchRequest) error { calls = append(calls, "launch"); return nil }
	app.waitForRuntimeStatus = func(timeout time.Duration, accept func(farmruntime.Status) bool) bool {
		calls = append(calls, "ready:"+timeout.String())
		app.manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-2"})
		return accept(app.manager.Status())
	}
	refreshCalls := 0
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
		refreshCalls++
		if refreshCalls == 1 {
			return initial, nil
		}
		calls = append(calls, "refresh")
		return refreshed, nil
	}

	result := app.RestartHostProcess()
	if result.Status != guard.RestartStatusReconnected || result.Binding == nil || result.Binding.PID != 30 {
		t.Fatalf("restart = %#v", result)
	}
	want := []string{"close:200", "closed:20:5s", "launch", "ready:45s", "refresh"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func qqAppSnapshots(rootPID int, hwnd uint64) []guard.HostProcessSnapshot {
	return []guard.HostProcessSnapshot{
		{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []guard.HostWindowSnapshot{{PID: 10, HWND: 100, Title: "QQ", Visible: true}}},
		{PID: rootPID, ParentPID: 10, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`, Windows: []guard.HostWindowSnapshot{{PID: rootPID, HWND: hwnd, Title: "QQ经典农场", Visible: true}}},
	}
}

func qqAppMainOnlySnapshots() []guard.HostProcessSnapshot {
	return []guard.HostProcessSnapshot{{PID: 10, ParentPID: 1, ProcessName: "QQ.exe", ExecutablePath: `D:\QQ\QQ.exe`}}
}

func configureQQFinalRestartTest(t *testing.T, app *App) {
	t.Helper()
	app.cfg.Runtime.CurrentTarget = "qq_ws"
	app.manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-1"})
	listCalls := 0
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
		listCalls++
		if listCalls == 1 {
			return qqAppSnapshots(20, 200), nil
		}
		if listCalls == 2 {
			return qqAppSnapshots(30, 300), nil
		}
		mainOnly := qqAppMainOnlySnapshots()
		mainOnly[0].Windows = []guard.HostWindowSnapshot{{PID: 10, HWND: 100, Title: "QQ", Visible: true}}
		return mainOnly, nil
	}
	app.closeHostWindows = func([]guard.HostWindowSnapshot) error { return nil }
	app.waitForQQMiniappClosed = func(int, time.Duration) (guard.QQCloseObservation, error) {
		return guard.QQCloseObservation{Disconnected: true, TreeExited: true, Snapshots: qqAppMainOnlySnapshots()}, nil
	}
	app.stopHostPID = func(pid int) error {
		t.Fatalf("unexpected forced stop PID %d", pid)
		return nil
	}
	app.launchHost = func(guard.LaunchRequest) error { return nil }
	app.waitForRuntimeStatus = func(_ time.Duration, accept func(farmruntime.Status) bool) bool {
		app.manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-2"})
		return accept(app.manager.Status())
	}
}

func TestAppQQRestartLaunchesWithoutStoppingWhenAlreadyClosed(t *testing.T) {
	app := newAuthorizedTestApp(t)
	app.cfg.Runtime.CurrentTarget = "qq_ws"
	app.manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: false, InstanceID: "qq-1"})
	listCalls := 0
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
		listCalls++
		if listCalls == 1 {
			return qqAppMainOnlySnapshots(), nil
		}
		return qqAppSnapshots(30, 300), nil
	}
	app.closeHostWindows = func([]guard.HostWindowSnapshot) error {
		t.Fatal("unexpected close")
		return nil
	}
	app.stopHostPID = func(pid int) error {
		t.Fatalf("unexpected stop PID %d", pid)
		return nil
	}
	launches := 0
	app.launchHost = func(guard.LaunchRequest) error { launches++; return nil }
	app.waitForRuntimeStatus = func(_ time.Duration, accept func(farmruntime.Status) bool) bool {
		app.manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-2"})
		return accept(app.manager.Status())
	}

	result := app.RestartHostProcess()
	if result.Status != guard.RestartStatusReconnected || result.OldPID != 0 || result.Stopped || launches != 1 {
		t.Fatalf("launch-only restart = %#v, launches = %d", result, launches)
	}
	if result.Binding == nil || result.Binding.PID != 30 {
		t.Fatalf("replacement binding = %#v", result.Binding)
	}
}

func TestAppQQRestartNeverAutoBindsQQMain(t *testing.T) {
	app := newAuthorizedTestApp(t)
	app.cfg.Runtime.CurrentTarget = "qq_ws"
	app.manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-1"})
	mainOnly := qqAppMainOnlySnapshots()
	mainOnly[0].Windows = []guard.HostWindowSnapshot{{PID: 10, HWND: 100, Title: "QQ", Visible: true}}
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) { return mainOnly, nil }
	app.closeHostWindows = func([]guard.HostWindowSnapshot) error { t.Fatal("unexpected close"); return nil }
	app.stopHostPID = func(pid int) error { t.Fatalf("unexpected stop PID %d", pid); return nil }
	app.launchHost = func(guard.LaunchRequest) error { t.Fatal("unexpected launch"); return nil }

	result, err := app.restartHostForRuntimeTarget("qq_ws")
	if err == nil || result.Status != "restart_failed" || !strings.Contains(err.Error(), "qq_miniapp_unverified") {
		t.Fatalf("rootless connected restart = %#v, err = %v", result, err)
	}
	if binding, ok := app.hosts.Binding("main"); ok {
		t.Fatalf("QQ main entered generic auto-bind: %#v", binding)
	}
}

func TestAppQQRestartRequiresAcceptedQQReadyStatus(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     farmruntime.Status
		waitResult bool
	}{
		{name: "wait timeout", status: farmruntime.Status{Target: "qq_ws", Connected: true, Ready: true, InstanceID: "qq-2"}, waitResult: false},
		{name: "wrong target", status: farmruntime.Status{Target: "wechat_cdp", Connected: true, Ready: true, InstanceID: "qq-2"}, waitResult: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := newAuthorizedTestApp(t)
			configureQQFinalRestartTest(t, app)
			app.waitForRuntimeStatus = func(_ time.Duration, _ func(farmruntime.Status) bool) bool {
				app.manager.SetStatus(test.status)
				return test.waitResult
			}
			result, err := app.restartHostForRuntimeTarget("qq_ws")
			if err == nil || result.Status != "restart_failed" || !strings.Contains(err.Error(), "reconnect_timeout") {
				t.Fatalf("restart = %#v, err = %v", result, err)
			}
			if result.Binding != nil && result.Binding.PID == 30 {
				t.Fatalf("replacement was bound without accepted QQ readiness: %#v", result.Binding)
			}
		})
	}
}

func TestAppYYBRestartPreservesHostAndWaitsForReconnect(t *testing.T) {
	app := newAuthorizedTestApp(t)
	var calls []string
	configureYYBSoftRestartTest(t, app, &calls)

	result := app.RestartHostProcess()

	if result.Status != guard.RestartStatusReconnected || result.Stopped || !result.LaunchDispatched {
		t.Fatalf("expected final YYB reconnect, got %#v", result)
	}
	if result.Binding == nil || result.Binding.PID != 42 || !reflect.DeepEqual(result.Binding.HWNDs, []uint64{421}) {
		t.Fatalf("expected refreshed same-PID binding, got %#v", result.Binding)
	}
	want := []string{"close:420", "wait:disconnected", "launch", "wait:ready", "refresh"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestAppQQManualRestartRecordsFinalRestartEvent(t *testing.T) {
	app := newAuthorizedTestApp(t)
	configureQQFinalRestartTest(t, app)

	result := app.RestartHostProcess()
	if result.Status != guard.RestartStatusReconnected {
		t.Fatalf("QQ restart = %#v", result)
	}
	events := app.ProcessGuardStatus().RecentRestartEvents
	if len(events) != 1 || events[0].Trigger != "manual" || events[0].RuntimeTarget != "qq_ws" || !events[0].OK {
		t.Fatalf("manual QQ restart events = %#v", events)
	}
	if phase := app.ProcessGuardStatus().Phase; phase == guard.PhaseWaitingReconnect {
		t.Fatalf("guardian entered a second reconnect wait after final QQ reconnect")
	}
}

func TestRunGuardianActionAcceptsQQReconnected(t *testing.T) {
	app := newAuthorizedTestApp(t)
	configureQQFinalRestartTest(t, app)

	result := app.RunGuardianAction(guard.ActionRequest{Action: "restart"})
	if !result.OK || result.Status != guard.RestartStatusReconnected || result.Error != "" {
		t.Fatalf("guardian QQ action = %#v", result)
	}
}

func TestAppManualRestartRecordsRestartEvent(t *testing.T) {
	app := newAuthorizedTestApp(t)
	configureWeChatSoftRestartTest(t, app, nil)

	result := app.RestartHostProcess()

	if result.Status != guard.RestartStatusReconnected {
		t.Fatalf("expected final reconnect, got %#v", result)
	}
	events := app.ProcessGuardStatus().RecentRestartEvents
	if len(events) != 1 {
		t.Fatalf("expected one restart event, got %#v", events)
	}
	if events[0].Trigger != "manual" || events[0].RuntimeTarget != "wechat_cdp" {
		t.Fatalf("expected manual wechat restart event, got %#v", events[0])
	}
}

func TestWaitForRuntimeStatusImmediateSuccess(t *testing.T) {
	manager := farmruntime.NewManager()
	manager.SetStatus(farmruntime.Status{Target: "wechat_cdp", Connected: true, Ready: true})
	if !waitForRuntimeStatus(manager, time.Second, func(status farmruntime.Status) bool { return status.Connected && status.Ready }) {
		t.Fatal("ready status was not accepted immediately")
	}
}

func TestWaitForRuntimeStatusObservesDelayedTransition(t *testing.T) {
	manager := farmruntime.NewManager()
	go func() {
		time.Sleep(20 * time.Millisecond)
		manager.SetStatus(farmruntime.Status{Target: "wechat_cdp", Connected: true, Ready: true})
	}()
	if !waitForRuntimeStatus(manager, 500*time.Millisecond, func(status farmruntime.Status) bool { return status.Connected && status.Ready }) {
		t.Fatal("delayed ready status was not observed")
	}
}

func TestWaitForRuntimeStatusTimesOut(t *testing.T) {
	manager := farmruntime.NewManager()
	if waitForRuntimeStatus(manager, 20*time.Millisecond, func(status farmruntime.Status) bool { return status.Ready }) {
		t.Fatal("unexpected ready status")
	}
}

func TestWaitForQQMiniappClosedImmediateSuccess(t *testing.T) {
	manager := farmruntime.NewManager()
	manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: false})
	calls := 0
	observation, err := waitForQQMiniappClosed(manager, func() ([]guard.HostProcessSnapshot, error) {
		calls++
		return qqAppMainOnlySnapshots(), nil
	}, 20, time.Second)
	if err != nil {
		t.Fatalf("wait for QQ miniapp closed: %v", err)
	}
	if !observation.Disconnected || !observation.TreeExited || calls != 1 {
		t.Fatalf("observation = %#v, snapshot calls = %d", observation, calls)
	}
}

func TestWaitForQQMiniappClosedWaitsForBothSignals(t *testing.T) {
	manager := farmruntime.NewManager()
	manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true})
	calls := 0
	observation, err := waitForQQMiniappClosed(manager, func() ([]guard.HostProcessSnapshot, error) {
		calls++
		switch calls {
		case 2:
			manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: false})
			return qqAppSnapshots(20, 200), nil
		case 3:
			return qqAppMainOnlySnapshots(), nil
		default:
			return qqAppSnapshots(20, 200), nil
		}
	}, 20, 190*time.Millisecond)
	if err != nil {
		t.Fatalf("wait for QQ miniapp closed: %v", err)
	}
	if !observation.Disconnected || !observation.TreeExited || calls < 3 {
		t.Fatalf("observation = %#v, snapshot calls = %d", observation, calls)
	}
}

func TestWaitForQQMiniappClosedTimeoutReturnsPartialState(t *testing.T) {
	for _, test := range []struct {
		name         string
		connected    bool
		snapshots    []guard.HostProcessSnapshot
		disconnected bool
		treeExited   bool
	}{
		{name: "disconnected only", connected: false, snapshots: qqAppSnapshots(20, 200), disconnected: true},
		{name: "tree exited only", connected: true, snapshots: qqAppMainOnlySnapshots(), treeExited: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager := farmruntime.NewManager()
			manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: test.connected})
			observation, err := waitForQQMiniappClosed(manager, func() ([]guard.HostProcessSnapshot, error) {
				return test.snapshots, nil
			}, 20, 70*time.Millisecond)
			if err != nil {
				t.Fatalf("wait for QQ miniapp closed: %v", err)
			}
			if observation.Disconnected != test.disconnected || observation.TreeExited != test.treeExited {
				t.Fatalf("partial observation = %#v", observation)
			}
		})
	}
}

func TestWaitForQQMiniappClosedDeadlineObservesFinalTransition(t *testing.T) {
	manager := farmruntime.NewManager()
	manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true})
	updated := make(chan struct{})
	calls := 0
	observation, err := waitForQQMiniappClosed(manager, func() ([]guard.HostProcessSnapshot, error) {
		calls++
		if calls == 1 {
			go func() {
				time.Sleep(10 * time.Millisecond)
				manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: false})
				close(updated)
			}()
		}
		return qqAppMainOnlySnapshots(), nil
	}, 20, 40*time.Millisecond)
	select {
	case <-updated:
	case <-time.After(time.Second):
		t.Fatal("runtime status update goroutine did not finish")
	}
	if err != nil {
		t.Fatalf("wait for QQ miniapp closed: %v", err)
	}
	if !observation.Disconnected || !observation.TreeExited || calls != 2 {
		t.Fatalf("deadline observation = %#v, snapshot calls = %d", observation, calls)
	}
}

func TestWaitForQQMiniappClosedDeadlineReturnsFinalSnapshotError(t *testing.T) {
	manager := farmruntime.NewManager()
	manager.SetStatus(farmruntime.Status{Target: "qq_ws", Connected: true})
	sentinel := errors.New("final snapshot failed")
	calls := 0
	observation, err := waitForQQMiniappClosed(manager, func() ([]guard.HostProcessSnapshot, error) {
		calls++
		if calls == 2 {
			return nil, sentinel
		}
		return qqAppMainOnlySnapshots(), nil
	}, 20, 40*time.Millisecond)
	if !errors.Is(err, sentinel) || calls != 2 {
		t.Fatalf("deadline observation = %#v, snapshot calls = %d, err = %v", observation, calls, err)
	}
}

func TestWaitForQQMiniappClosedReturnsSnapshotError(t *testing.T) {
	manager := farmruntime.NewManager()
	sentinel := errors.New("snapshot failed")
	observation, err := waitForQQMiniappClosed(manager, func() ([]guard.HostProcessSnapshot, error) {
		return nil, sentinel
	}, 20, time.Second)
	if !errors.Is(err, sentinel) {
		t.Fatalf("observation = %#v, err = %v", observation, err)
	}
}

func configureWeChatSoftRestartTest(t *testing.T, app *App, calls *[]string) {
	t.Helper()
	app.cfg.Runtime.CurrentTarget = "wechat_cdp"
	snapshotCalls := 0
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
		snapshotCalls++
		hwnd := uint64(420)
		if snapshotCalls >= 3 {
			hwnd = 421
			if calls != nil && snapshotCalls == 3 {
				*calls = append(*calls, "refresh")
			}
		}
		return []guard.HostProcessSnapshot{{
			PID: 42, ProcessName: "WeChatAppEx.exe",
			Windows: []guard.HostWindowSnapshot{{HWND: hwnd, PID: 42, Title: "QQ经典农场", Visible: true}},
		}}, nil
	}
	app.stopHostPID = func(pid int) error {
		t.Fatalf("WeChat soft restart stopped PID %d", pid)
		return nil
	}
	app.closeHostWindows = func(windows []guard.HostWindowSnapshot) error {
		if len(windows) != 1 || windows[0].HWND != 420 {
			t.Fatalf("close windows = %#v", windows)
		}
		if calls != nil {
			*calls = append(*calls, "close:420")
		}
		return nil
	}
	waitCalls := 0
	app.waitForRuntimeStatus = func(_ time.Duration, accept func(farmruntime.Status) bool) bool {
		waitCalls++
		status := farmruntime.Status{Target: "wechat_cdp", Phase: farmruntime.PhaseDisconnected}
		label := "wait:disconnected"
		if waitCalls == 2 {
			status = farmruntime.Status{Target: "wechat_cdp", Phase: farmruntime.PhaseReady, Connected: true, Ready: true}
			label = "wait:ready"
		}
		if calls != nil {
			*calls = append(*calls, label)
		}
		return accept(status)
	}
	app.launchHost = func(guard.LaunchRequest) error {
		if calls != nil {
			*calls = append(*calls, "launch")
		}
		return nil
	}
}

func configureYYBSoftRestartTest(t *testing.T, app *App, calls *[]string) {
	t.Helper()
	app.cfg.Runtime.CurrentTarget = "yyb_cdp"
	snapshotCalls := 0
	app.listHostSnapshots = func() ([]guard.HostProcessSnapshot, error) {
		snapshotCalls++
		hwnd := uint64(420)
		if snapshotCalls >= 3 {
			hwnd = 421
			if calls != nil && snapshotCalls == 3 {
				*calls = append(*calls, "refresh")
			}
		}
		return []guard.HostProcessSnapshot{{
			PID:            42,
			ParentPID:      7176,
			ProcessName:    "WeChatAppEx.exe",
			ExecutablePath: `E:\Program Files\Tencent\Androws\WmpfRuntime\5.10.2700.327\runtime\WeChatAppEx.exe`,
			Windows: []guard.HostWindowSnapshot{{
				HWND:  hwnd,
				PID:   42,
				Title: "QQ经典农场", Visible: true,
			}},
		}}, nil
	}
	app.stopHostPID = func(pid int) error {
		t.Fatalf("YYB soft restart stopped PID %d", pid)
		return nil
	}
	app.closeHostWindows = func(windows []guard.HostWindowSnapshot) error {
		if len(windows) != 1 || windows[0].HWND != 420 {
			t.Fatalf("close windows = %#v", windows)
		}
		if calls != nil {
			*calls = append(*calls, "close:420")
		}
		return nil
	}
	waitCalls := 0
	app.waitForRuntimeStatus = func(_ time.Duration, accept func(farmruntime.Status) bool) bool {
		waitCalls++
		status := farmruntime.Status{Target: "yyb_cdp", Phase: farmruntime.PhaseDisconnected}
		label := "wait:disconnected"
		if waitCalls == 2 {
			status = farmruntime.Status{Target: "yyb_cdp", Phase: farmruntime.PhaseReady, Connected: true, Ready: true}
			label = "wait:ready"
		}
		if calls != nil {
			*calls = append(*calls, label)
		}
		return accept(status)
	}
	app.launchHost = func(request guard.LaunchRequest) error {
		if request.Mode != "yyb_shortcut" || !strings.Contains(request.Parameters, "launchWithShortcut?pkgname=wx5306c5978fdb76e4") {
			t.Fatalf("launch request = %#v", request)
		}
		if calls != nil {
			*calls = append(*calls, "launch")
		}
		return nil
	}
}

func TestAppCanSaveProcessGuardSettings(t *testing.T) {
	app := newAuthorizedTestApp(t)
	settings, err := app.SaveProcessGuardSettings(map[string]any{
		"enabled":          true,
		"timeoutThreshold": 2,
	})
	if err != nil {
		t.Fatalf("save process guard settings: %v", err)
	}
	if settings["enabled"] != true {
		t.Fatalf("expected enabled setting, got %#v", settings)
	}
}

func TestSaveGuardianSettingsUpdatesAllWorkers(t *testing.T) {
	app := newAppWithTempStore(t)
	settings, err := app.SaveGuardianSettings(map[string]any{
		"enabled":                    true,
		"failureRecoveryEnabled":     true,
		"networkReconnectEnabled":    true,
		"otherPlaceLoginEnabled":     true,
		"networkReconnectIntervalMs": 1300,
		"networkRecoveryTimeoutMs":   18000,
		"otherPlaceLoginIntervalMs":  4500,
		"otherPlaceLoginDelayMin":    8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !settings.Enabled || !settings.NetworkReconnectEnabled || !settings.OtherPlaceLoginEnabled {
		t.Fatalf("unexpected guardian settings: %#v", settings)
	}
	status := app.GuardianStatus()
	if !status.Enabled || !status.Network.Enabled || !status.OtherPlaceLogin.Enabled {
		t.Fatalf("workers were not updated: %#v", status)
	}
}

func TestGuardianStatusReturnsPersistedSettings(t *testing.T) {
	app := newAppWithTempStore(t)
	_, err := app.SaveGuardianSettings(map[string]any{
		"timeoutThreshold":           7,
		"monitorIntervalMs":          4321,
		"networkReconnectIntervalMs": 1300,
		"networkRecoveryTimeoutMs":   18000,
		"otherPlaceLoginIntervalMs":  4500,
		"otherPlaceLoginDelayMin":    8,
	})
	if err != nil {
		t.Fatal(err)
	}

	settings := app.GuardianStatus().Settings
	if settings.TimeoutThreshold != 7 || settings.MonitorIntervalMS != 4321 ||
		settings.NetworkReconnectIntervalMS != 1300 || settings.NetworkRecoveryTimeoutMS != 18000 ||
		settings.OtherPlaceLoginIntervalMS != 4500 || settings.OtherPlaceLoginDelayMin != 8 {
		t.Fatalf("guardian status settings = %#v", settings)
	}
}

func TestSaveGuardianSettingsUpdatesOtherPlaceLoginWorkerWithoutProcessChange(t *testing.T) {
	app := newAppWithTempStore(t)
	if _, err := app.SaveGuardianSettings(map[string]any{"enabled": true, "otherPlaceLoginEnabled": false}); err != nil {
		t.Fatal(err)
	}
	if app.GuardianStatus().OtherPlaceLogin.Enabled {
		t.Fatal("other-place-login worker unexpectedly started enabled")
	}

	if _, err := app.SaveGuardianSettings(map[string]any{"otherPlaceLoginEnabled": true}); err != nil {
		t.Fatal(err)
	}
	if !app.GuardianStatus().OtherPlaceLogin.Enabled {
		t.Fatalf("other-place-login worker was not updated: %#v", app.GuardianStatus())
	}
}

func TestAppSwitchRuntimeTargetClearsConfirmedAccountBinding(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	defer app.shutdown(context.Background())
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001, Nickname: "A"}); account.Error != "" {
		t.Fatalf("confirm account: %#v", account)
	}
	if got := app.accountKey(); got != "gid:10001" {
		t.Fatalf("account key before switch = %q, want gid:10001", got)
	}
	if current := app.CurrentRuntimeAccount(); !current.Confirmed || current.GID != 10001 {
		t.Fatalf("current account before switch = %#v", current)
	}

	// Keep warehouse settings under the old account so a stale binding would still write there.
	if _, err := app.SaveWarehouseAutoSellSettings(storage.WarehouseAutoSellSettings{
		Enabled:        true,
		IntervalMinute: 30,
		Categories:     []string{"fruit"},
	}); err != nil {
		t.Fatalf("save warehouse settings under confirmed account: %v", err)
	}

	app.cfg.WMPF.LegacyDebugPort = freeTCPPort(t)
	app.cfg.WMPF.FridaEnabled = false
	status := app.SwitchRuntimeTarget("wechat_cdp")
	if status.Phase == "error" {
		t.Fatalf("switch failed: %#v", status)
	}

	if got := app.accountKey(); got != storage.DefaultRuntimeAccountKey {
		t.Fatalf("account key after switch = %q, want %q", got, storage.DefaultRuntimeAccountKey)
	}
	current := app.CurrentRuntimeAccount()
	if current.Confirmed || current.GID != 0 {
		t.Fatalf("current account after switch should be unbound, got %#v", current)
	}

	// Saving business settings after unbind must not keep writing into the previous GID scope.
	if _, err := app.SaveWarehouseAutoSellSettings(storage.WarehouseAutoSellSettings{
		Enabled:        true,
		IntervalMinute: 90,
		Categories:     []string{"seed"},
	}); err != nil {
		t.Fatalf("save warehouse settings after unbind: %v", err)
	}
	previous, err := store.LoadWarehouseAutoSellSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load previous account settings: %v", err)
	}
	if previous.IntervalMinute != 30 || !reflect.DeepEqual(previous.Categories, []string{"fruit"}) {
		t.Fatalf("previous account settings were mutated after unbind: %#v", previous)
	}
}

func TestAppSwitchRuntimeTargetChangesActiveLink(t *testing.T) {
	app := newAuthorizedTestApp(t)
	defer app.shutdown(context.Background())
	app.cfg.WMPF.LegacyDebugPort = freeTCPPort(t)
	app.cfg.WMPF.FridaEnabled = false
	if _, err := app.SaveRuntimeSettings(storage.RuntimeSettings{
		DefaultTarget: "wechat_cdp",
		CurrentTarget: "wechat_cdp",
		AutoStart:     true,
		CDPPort:       freeTCPPort(t),
		WMPFDebugPort: freeTCPPort(t),
	}); err != nil {
		t.Fatalf("save runtime settings: %v", err)
	}

	status := app.SwitchRuntimeTarget("wechat_cdp")

	if status.Target != "wechat_cdp" {
		t.Fatalf("expected active target in status, got %#v", status)
	}
	if status.Phase == "error" {
		t.Fatalf("expected successful switch, got %#v", status)
	}
}

func TestAppSaveRuntimeSettingsRebuildsCDPLinks(t *testing.T) {
	app := newAuthorizedTestApp(t)
	defer app.shutdown(context.Background())
	cdpPort := freeTCPPort(t)
	debugPort := freeTCPPort(t)
	legacyPort := freeTCPPort(t)
	app.cfg.WMPF.LegacyDebugPort = legacyPort
	app.cfg.WMPF.FridaEnabled = false

	if _, err := app.SaveRuntimeSettings(storage.RuntimeSettings{
		DefaultTarget: "wechat_cdp",
		CurrentTarget: "wechat_cdp",
		AutoStart:     true,
		CDPPort:       cdpPort,
		WMPFDebugPort: debugPort,
	}); err != nil {
		t.Fatalf("save runtime settings: %v", err)
	}

	status := app.SwitchRuntimeTarget("wechat_cdp")
	if status.Phase == "error" {
		t.Fatalf("switch failed: %#v", status)
	}
	host, err := app.supervisor.Call(context.Background(), "host.describe", nil, 1000000000)
	if err != nil {
		t.Fatalf("host.describe: %v", err)
	}
	if !strings.Contains(host.(map[string]any)["cdpURL"].(string), ":"+itoa(cdpPort)) {
		t.Fatalf("expected cdp url to use saved port, got %#v", host)
	}
}

func TestAppSaveRuntimeSettingsWarehouseOnlyDoesNotStopActiveRuntime(t *testing.T) {
	app := newAuthorizedTestApp(t)
	link := &fakeRuntimeLink{
		target: farmruntime.RuntimeTargetWeChatCDP,
		status: farmruntime.Status{
			Target:    string(farmruntime.RuntimeTargetWeChatCDP),
			Phase:     farmruntime.PhaseReady,
			Connected: true,
			Ready:     true,
		},
	}
	app.supervisor = farmruntime.NewSupervisor(app.manager, map[farmruntime.RuntimeTarget]farmruntime.RuntimeLink{
		farmruntime.RuntimeTargetWeChatCDP: link,
	})
	if err := app.supervisor.Switch(context.Background(), farmruntime.RuntimeTargetWeChatCDP); err != nil {
		t.Fatalf("switch fake runtime: %v", err)
	}

	if _, err := app.SaveRuntimeSettings(storage.RuntimeSettings{
		DefaultTarget:                   app.cfg.Runtime.DefaultTarget,
		CurrentTarget:                   app.cfg.Runtime.CurrentTarget,
		AutoStart:                       app.cfg.Runtime.AutoStart,
		CDPPort:                         app.cfg.CDP.Port,
		WMPFDebugPort:                   app.cfg.WMPF.DebugPort,
		AutoWarehouseSellEnabled:        true,
		AutoWarehouseSellIntervalMinute: 180,
		AutoWarehouseSellCategories:     []string{"fruit", "tool"},
		WarehouseRefreshOnlyOnAutoSell:  true,
	}); err != nil {
		t.Fatalf("save warehouse-only runtime DTO: %v", err)
	}

	if link.stopCount != 0 {
		t.Fatalf("warehouse-only settings save should not stop active runtime, stopped %d times", link.stopCount)
	}
	status := app.RuntimeLinkStatus()
	if status.Target != string(farmruntime.RuntimeTargetWeChatCDP) || !status.Connected {
		t.Fatalf("expected active runtime to remain connected, got %#v", status)
	}
}

func TestAppRuntimeEventsIncludesStatusChanges(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	app.manager.SetStatus(farmruntime.Status{
		Target:    "wechat_cdp",
		Phase:     farmruntime.PhaseListening,
		Connected: false,
	})

	events := app.RuntimeEvents(10)
	if len(events) != 1 {
		t.Fatalf("expected one event, got %#v", events)
	}
	if events[0].Source != "wechat_cdp" || events[0].Type != "runtime.status" {
		t.Fatalf("unexpected event identity %#v", events[0])
	}
	if events[0].Message != "wechat_cdp listening" {
		t.Fatalf("unexpected message %q", events[0].Message)
	}
	if events[0].Data["phase"] != "listening" || events[0].Data["connected"] != false {
		t.Fatalf("unexpected data %#v", events[0].Data)
	}
}

func TestAppRuntimeEventsMemoryFallbackIsScopedToCurrentAccount(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app.store = store
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	app.recordEventForAccount("gid:10001", eventbus.Event{Type: "account-a", Message: "account A"})
	app.recordEventForAccount("gid:10002", eventbus.Event{Type: "account-b", Message: "account B"})
	app.currentAccountKey = "gid:10002"

	events := app.RuntimeEvents(10)
	if len(events) != 1 {
		t.Fatalf("expected one current-account fallback event, got %#v", events)
	}
	if events[0].Type != "account-b" {
		t.Fatalf("fallback exposed another account event: %#v", events)
	}
}

func TestAppRuntimeEventsKeepPersistentTSDKFeedIndependent(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	app.currentAccountKey = "gid:10001"
	for index := 0; index < 105; index++ {
		app.recordRuntimeLogEvent(eventbus.Event{
			Source:  "qq_ws",
			Type:    "qqhost.log",
			Message: fmt.Sprintf("[TSDK-BLOCK] persistent-%03d", index),
		})
	}
	app.currentAccountKey = "gid:10002"
	for index := 0; index < 130; index++ {
		app.recordEvent(eventbus.Event{
			Source:  "account",
			Type:    "ordinary",
			Message: fmt.Sprintf("ordinary-%03d", index),
		})
	}

	ordinary := app.RuntimeEvents(120)
	if len(ordinary) != 120 {
		t.Fatalf("ordinary event count = %d, want 120", len(ordinary))
	}
	for _, event := range ordinary {
		if isTSDKBlockRuntimeEvent(event) {
			t.Fatalf("ordinary feed contains TSDK event: %#v", event)
		}
	}

	tsdk := app.TSDKRuntimeEvents(100)
	if len(tsdk) != 100 {
		t.Fatalf("TSDK event count = %d, want 100", len(tsdk))
	}
	if !strings.Contains(tsdk[0].Message, "persistent-104") || !strings.Contains(tsdk[99].Message, "persistent-005") {
		t.Fatalf("TSDK feed is not the newest 100 records: first=%q last=%q", tsdk[0].Message, tsdk[99].Message)
	}
}

func TestAppRuntimeEventsKeepMemoryTSDKFeedIndependent(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app.store = store
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	app.currentAccountKey = "gid:10001"
	for index := 0; index < 105; index++ {
		app.recordRuntimeLogEvent(eventbus.Event{
			Type:    "qqhost.log",
			Message: fmt.Sprintf("[TSDK-BLOCK] memory-%03d", index),
		})
	}
	app.currentAccountKey = "gid:10002"
	for index := 0; index < 130; index++ {
		app.recordEvent(eventbus.Event{
			Type:    "ordinary",
			Message: fmt.Sprintf("ordinary-%03d", index),
		})
	}

	ordinary := app.RuntimeEvents(120)
	if len(ordinary) != 120 {
		t.Fatalf("ordinary fallback count = %d, want 120", len(ordinary))
	}
	for _, event := range ordinary {
		if isTSDKBlockRuntimeEvent(event) {
			t.Fatalf("ordinary fallback contains TSDK event: %#v", event)
		}
	}

	tsdk := app.TSDKRuntimeEvents(100)
	if len(tsdk) != 100 {
		t.Fatalf("TSDK fallback count = %d, want 100", len(tsdk))
	}
	if !strings.Contains(tsdk[0].Message, "memory-104") || !strings.Contains(tsdk[99].Message, "memory-005") {
		t.Fatalf("TSDK fallback is not the newest 100 records: first=%q last=%q", tsdk[0].Message, tsdk[99].Message)
	}
}

func TestAppTSDKRuntimeEventsRequiresAuthorization(t *testing.T) {
	app := NewApp()
	if events := app.TSDKRuntimeEvents(100); len(events) != 0 {
		t.Fatalf("unauthorized TSDK events = %#v, want empty", events)
	}
}

func TestAppInstallsQQDebugPatchToExplicitGameJS(t *testing.T) {
	dir := t.TempDir()
	gameJS := filepath.Join(dir, "game.js")
	hostGameJS := filepath.Join(dir, "cocos-js", "assets", "game.js")
	if err := os.WriteFile(gameJS, []byte("console.log('game');\n"), 0o644); err != nil {
		t.Fatalf("write game.js: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "game.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write game.json: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(hostGameJS), 0o755); err != nil {
		t.Fatalf("create Cocos assets directory: %v", err)
	}
	if err := os.WriteFile(hostGameJS, []byte("console.log('wasm assets');\n"), 0o644); err != nil {
		t.Fatalf("write Cocos assets game.js: %v", err)
	}
	app := newAuthorizedTestApp(t)

	result := app.InstallQQDebugPatch(gameJS)

	if !result.OK {
		t.Fatalf("expected patch ok, got %#v", result)
	}
	if result.TargetPath != hostGameJS {
		t.Fatalf("expected Cocos assets host target %q, got %q", hostGameJS, result.TargetPath)
	}
	content, err := os.ReadFile(hostGameJS)
	if err != nil {
		t.Fatalf("read patched Cocos assets game.js: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "ws://127.0.0.1:8787/runtime/qqws") {
		t.Fatalf("expected Farm_Go ws url in patch: %q", text)
	}
	if !strings.Contains(text, "farm-go-host-1") {
		t.Fatalf("expected Farm_Go host version in patch: %q", text)
	}
	if !strings.Contains(text, "G.gameCtl =") {
		t.Fatalf("expected Farm_Go runtime button layer in patch: %q", text)
	}
	rootContent, err := os.ReadFile(gameJS)
	if err != nil {
		t.Fatalf("read root game.js: %v", err)
	}
	if strings.Contains(string(rootContent), "// >>> FARM_GO_QQ_DEBUG START >>>") {
		t.Fatalf("root game.js must remain unpatched: %q", rootContent)
	}
}

func TestAppTracksQQDebugPatchStatus(t *testing.T) {
	dir := t.TempDir()
	gameJS := filepath.Join(dir, "game.js")
	hostGameJS := filepath.Join(dir, "cocos-js", "assets", "game.js")
	if err := os.WriteFile(gameJS, []byte("console.log('game');\n"), 0o644); err != nil {
		t.Fatalf("write game.js: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(hostGameJS), 0o755); err != nil {
		t.Fatalf("create Cocos assets directory: %v", err)
	}
	if err := os.WriteFile(hostGameJS, []byte("console.log('wasm assets');\n"), 0o644); err != nil {
		t.Fatalf("write Cocos assets game.js: %v", err)
	}
	app := newAuthorizedTestApp(t)

	initial := app.QQDebugPatchStatus()
	if initial.Phase != "idle" {
		t.Fatalf("expected idle initial status, got %#v", initial)
	}

	result := app.InstallQQDebugPatch(gameJS)
	if !result.OK {
		t.Fatalf("expected patch ok, got %#v", result)
	}

	status := app.QQDebugPatchStatus()
	if status.Phase != "ready" {
		t.Fatalf("expected ready status, got %#v", status)
	}
	if !status.Injected {
		t.Fatalf("expected injected status, got %#v", status)
	}
	if status.TargetPath != hostGameJS {
		t.Fatalf("unexpected status target %q", status.TargetPath)
	}
	if status.LastTrigger != "manual" {
		t.Fatalf("unexpected trigger %q", status.LastTrigger)
	}
	if !status.RestartRequired {
		t.Fatalf("expected changed patch to request miniapp restart, got %#v", status)
	}
}

func TestFarmFeatureCatalogIsExposed(t *testing.T) {
	app := newAuthorizedTestApp(t)

	catalog := app.FarmFeatureCatalog()

	if len(catalog.Groups) != 5 {
		t.Fatalf("group count = %d, want 5", len(catalog.Groups))
	}
}

func TestFarmAutomationStateIsExposed(t *testing.T) {
	app := newAuthorizedTestApp(t)

	state := app.FarmAutomationState()

	if len(state.FeatureGroups) != 6 {
		t.Fatalf("feature group count = %d, want 6", len(state.FeatureGroups))
	}
	for _, group := range state.FeatureGroups {
		if group.ID == "runtime" || group.Label == "任务全局运行设置" {
			t.Fatalf("runtime settings should live in scheduler center, got feature group %#v", group)
		}
	}
	if len(state.Scheduler.Tasks) != 19 {
		t.Fatalf("scheduler task count = %d, want 19", len(state.Scheduler.Tasks))
	}
}

func TestFarmSocialStateIsExposed(t *testing.T) {
	app := newAuthorizedTestApp(t)

	state := app.FarmSocialState(false)

	if state.Status == "" {
		t.Fatalf("social state should expose a structured status: %#v", state)
	}
	if state.Friends == nil {
		t.Fatalf("social state should use an empty friend list instead of nil: %#v", state)
	}
}

func TestFarmSocialActionIsExposed(t *testing.T) {
	app := newAuthorizedTestApp(t)

	result := app.FarmSocialAction(social.FriendActionRequest{Action: "steal", Target: "1184649322"})

	if !result.OK || result.Status != social.StatusSkipped {
		t.Fatalf("protected social action should skip before runtime: %#v", result)
	}
}

func TestFarmSocialActionViewQQRejectsNonQQRuntime(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.addFriendByGidDiagnostic": map[string]any{"ok": true, "invoked": true},
	})
	app.manager.SetStatus(farmruntime.Status{Target: string(farmruntime.RuntimeTargetWeChatCDP)})

	result := app.FarmSocialAction(social.FriendActionRequest{Action: "view_qq", Target: "10001"})

	if result.OK || result.Status != social.StatusUnsupported || result.Message != "查看好友QQ仅支持 QQ 运行链路。" {
		t.Fatalf("result = %#v", result)
	}
	if link.called("gameCtl.addFriendByGidDiagnostic") {
		t.Fatalf("non-QQ request must not reach runtime: %#v", link.calls)
	}
}

func TestFarmSocialProtocolBlockListIsExposed(t *testing.T) {
	app := newAuthorizedTestApp(t)

	result := app.FarmSocialProtocolBlockList(false)

	if result.Status == "" || result.Friends == nil {
		t.Fatalf("protocol block list should be structured: %#v", result)
	}
}

func TestFarmSocialRankingsIsExposed(t *testing.T) {
	app := newAuthorizedTestApp(t)

	result := app.FarmSocialRankings(social.RankingRequest{Tab: "visitors", ViewMode: "timeline", DateRange: "all"})

	if result.Status == "" || result.Rows == nil {
		t.Fatalf("rankings should be structured: %#v", result)
	}
}

func TestFarmSocialRefreshVisitorsIsExposed(t *testing.T) {
	app := newAuthorizedTestApp(t)

	result := app.FarmSocialRefreshVisitors()

	if result.Status == "" {
		t.Fatalf("refresh visitors = %#v", result)
	}
}

func TestFarmSocialRankingPreferencesAreExposed(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	saved, err := app.SaveFarmSocialRankingPreferences(storage.DefaultRuntimeAccountKey, social.RankingPreferences{StolenByMeViewMode: "ranking", StolenFromMeViewMode: "timeline"})
	if err != nil {
		t.Fatalf("save ranking preferences: %v", err)
	}
	loaded := app.FarmSocialRankingPreferences()

	if saved.StolenByMeViewMode != "ranking" || saved.StolenFromMeViewMode != "timeline" ||
		loaded.StolenByMeViewMode != "ranking" || loaded.StolenFromMeViewMode != "timeline" {
		t.Fatalf("preferences were not exposed: saved=%#v loaded=%#v", saved, loaded)
	}
}

func TestSaveFarmSocialRankingPreferencesReturnsStorageError(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app.store = store
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	if _, err := app.SaveFarmSocialRankingPreferences(storage.DefaultRuntimeAccountKey, social.RankingPreferences{StolenByMeViewMode: "ranking", StolenFromMeViewMode: "timeline"}); err == nil {
		t.Fatal("expected ranking preferences save error")
	}
}

func TestSaveFarmSocialRankingPreferencesRejectsStaleScopeWithoutWritingEitherAccount(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	if err := store.SaveSocialRankingPreferences(ctx, "gid:10001", storage.SocialRankingPreferences{StolenByMeViewMode: "ranking", StolenFromMeViewMode: "timeline"}); err != nil {
		t.Fatalf("seed account A preferences: %v", err)
	}
	if err := store.SaveSocialRankingPreferences(ctx, "gid:10002", storage.SocialRankingPreferences{StolenByMeViewMode: "timeline", StolenFromMeViewMode: "ranking"}); err != nil {
		t.Fatalf("seed account B preferences: %v", err)
	}
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		t.Fatalf("confirm account B: %s", account.Error)
	}

	if _, err := app.SaveFarmSocialRankingPreferences("gid:10001", social.RankingPreferences{StolenByMeViewMode: "timeline", StolenFromMeViewMode: "ranking"}); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("expected stale-scope error, got %v", err)
	}
	accountA, err := store.LoadSocialRankingPreferences(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load account A preferences: %v", err)
	}
	accountB, err := store.LoadSocialRankingPreferences(ctx, "gid:10002")
	if err != nil {
		t.Fatalf("load account B preferences: %v", err)
	}
	if accountA.StolenByMeViewMode != "ranking" || accountA.StolenFromMeViewMode != "timeline" ||
		accountB.StolenByMeViewMode != "timeline" || accountB.StolenFromMeViewMode != "ranking" {
		t.Fatalf("stale save changed preferences: A=%#v B=%#v", accountA, accountB)
	}
}

func TestFarmSocialDogGuardMethodsAreExposed(t *testing.T) {
	app := newAuthorizedTestApp(t)

	state := app.FarmSocialDogGuardState()
	if state.Results == nil {
		t.Fatalf("dog guard state should expose an empty result list instead of nil: %#v", state)
	}

	result := app.FarmSocialDogGuardAction(social.DogGuardActionRequest{Action: "clear"})
	if result.Results == nil || result.Running {
		t.Fatalf("dog guard clear should return a structured idle state: %#v", result)
	}
}

func TestFarmSocialImportExportIsExposed(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	rules := social.FriendRules{Blacklist: []string{"10002"}}
	result := app.FarmSocialImport(social.ImportExportPayload{Version: 1, Groups: []string{"rules"}, Rules: &rules})
	if !result.OK {
		t.Fatalf("import result = %#v", result)
	}

	payload := app.FarmSocialExport(social.ExportRequest{Groups: []string{"rules"}})
	if !payload.OK || payload.Version != 1 || payload.Rules == nil || payload.Rules.Blacklist[0] != "10002" {
		t.Fatalf("export payload = %#v", payload)
	}
}

func TestFarmAutomationStateUsesPersistedSchedulerSettings(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if err := store.SaveAutoFarmSettings(ctx, storage.AutoFarmSettings{
		SchedulerEnabled:  false,
		SchedulerMinGapMS: 750,
		Config: map[string]any{
			"autoFarmOneClickEnabled": false,
			"autoFarmPlantSeedId":     float64(100123),
		},
		Tasks: []storage.AutoFarmTaskSettings{
			{ID: "friend_steal", Enabled: true, Priority: 120, IntervalSec: 45},
			{ID: "own_collect", Enabled: false, Priority: 11, IntervalSec: 300},
		},
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	state := app.FarmAutomationState()

	if state.Scheduler.Enabled || state.Scheduler.MinGapMs != 750 {
		t.Fatalf("scheduler settings were not applied: %#v", state.Scheduler)
	}
	if state.Config["autoFarmOneClickEnabled"] != false || state.Config["autoFarmPlantSeedId"] != float64(100123) {
		t.Fatalf("detailed config was not applied: %#v", state.Config)
	}
	friendSteal := findAutomationTask(state.Scheduler.Tasks, "friend_steal")
	if friendSteal == nil || !friendSteal.Enabled || friendSteal.Priority != 120 || friendSteal.IntervalSec != 45 {
		t.Fatalf("friend_steal settings were not applied: %#v", friendSteal)
	}
	ownCollect := findAutomationTask(state.Scheduler.Tasks, "own_collect")
	if ownCollect == nil || ownCollect.Enabled || ownCollect.Priority != 11 || ownCollect.IntervalSec != 300 {
		t.Fatalf("own_collect settings were not applied: %#v", ownCollect)
	}
}

func TestSaveFarmAutomationStatePersistsSchedulerSettings(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	state := automation.DefaultState()
	state.Scheduler.Enabled = false
	state.Scheduler.MinGapMs = 900
	state.Config["autoFarmOneClickEnabled"] = false
	state.Config["autoFarmPlantEnabled"] = false
	state.Config["autoFarmPlantIntervalSec"] = 120
	state.Config["autoFarmPlantSeedId"] = 100123
	for i := range state.Scheduler.Tasks {
		if state.Scheduler.Tasks[i].ID == "own_plant" {
			state.Scheduler.Tasks[i].Enabled = false
			state.Scheduler.Tasks[i].Priority = 42
			state.Scheduler.Tasks[i].IntervalSec = 120
		}
	}

	saved, err := app.SaveFarmAutomationState(state)
	if err != nil {
		t.Fatalf("save automation state: %v", err)
	}

	if saved.Scheduler.Enabled || saved.Scheduler.MinGapMs != 900 {
		t.Fatalf("unexpected saved state: %#v", saved.Scheduler)
	}
	loaded, err := store.LoadAutoFarmSettings(ctx)
	if err != nil {
		t.Fatalf("load auto farm settings: %v", err)
	}
	task := findAutoFarmTaskSettings(loaded.Tasks, "own_plant")
	if task == nil || task.Enabled || task.Priority != 42 || task.IntervalSec != 120 {
		t.Fatalf("task settings were not persisted: %#v", task)
	}
	if loaded.Config["autoFarmOneClickEnabled"] != false || loaded.Config["autoFarmPlantEnabled"] != false || loaded.Config["autoFarmPlantSeedId"] != float64(100123) {
		t.Fatalf("detailed config was not persisted: %#v", loaded.Config)
	}
}

func TestSaveFarmAutomationStateCanonicalRoundTrip(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	_ = app.FarmAutomationSchedulerState()

	input := app.FarmAutomationState()
	for index := range input.FeatureGroups {
		input.FeatureGroups[index].Enabled = true
	}
	type expectedTaskState struct {
		enabled     bool
		intervalSec int
		enabledKey  string
		intervalKey string
	}
	expectedTasks := make(map[string]expectedTaskState, len(input.Scheduler.Tasks))
	stableTaskIndex := 0
	for index := range input.Scheduler.Tasks {
		task := &input.Scheduler.Tasks[index]
		if task.ID == "auto_warehouse_sell" {
			continue
		}
		// Keep existing test preferences stable when a new scheduler row is introduced.
		requestedEnabled := stableTaskIndex%2 == 0
		enabled := requestedEnabled
		if task.ID == "he_feng_travel_reward" || task.ID == "limited_seed_draw" {
			enabled = false
		}
		intervalSec := task.IntervalSec + 137 + stableTaskIndex
		if task.ID == "own_collect" {
			intervalSec = 30
			task.IntervalSec = 120
		}
		if task.EnabledConfigKey != "" {
			input.Config[task.EnabledConfigKey] = requestedEnabled
		}
		if task.IntervalConfigKey != "" {
			input.Config[task.IntervalConfigKey] = intervalSec
		}
		expectedTasks[task.ID] = expectedTaskState{
			enabled:     enabled,
			intervalSec: intervalSec,
			enabledKey:  task.EnabledConfigKey,
			intervalKey: task.IntervalConfigKey,
		}
		if task.ID != "qian_xing_travel_reward" && task.ID != "xing_su_auto_light_up" {
			stableTaskIndex++
		}
	}
	input.Config["autoFarmFourGridPlantEnabled"] = true
	input.Config["autoFarmRushFertilizerMode"] = "normal"
	input.Config["autoFarmFertilizerRushThresholdSec"] = 450
	input.Config["autoFarmPlantBackpackSeedPriority"] = []any{float64(20133), float64(21032)}
	input.Config["autoFarmFertilizerLandTypes"] = []any{"gold", "normal"}

	saved, err := app.SaveFarmAutomationState(input)
	if err != nil {
		t.Fatalf("save automation state: %v", err)
	}
	assertConfigTypes := func(source string, config map[string]any) {
		t.Helper()
		if !boolFromAny(config["autoFarmFourGridPlantEnabled"], false) {
			t.Fatalf("%s boolean config was not preserved", source)
		}
		if intFromAny(config["autoFarmFertilizerRushThresholdSec"], 0) != 450 || stringFromAny(config["autoFarmRushFertilizerMode"]) != "normal" {
			t.Fatalf("%s scalar configs were not preserved", source)
		}
		numberList := make([]int, 0)
		for _, value := range sliceFromAny(config["autoFarmPlantBackpackSeedPriority"]) {
			numberList = append(numberList, intFromAny(value, 0))
		}
		if !reflect.DeepEqual(numberList, []int{20133, 21032}) {
			t.Fatalf("%s number-list config = %#v", source, numberList)
		}
		stringList := make([]string, 0)
		for _, value := range sliceFromAny(config["autoFarmFertilizerLandTypes"]) {
			stringList = append(stringList, stringFromAny(value))
		}
		if !reflect.DeepEqual(stringList, []string{"gold", "normal"}) {
			t.Fatalf("%s string-list config = %#v", source, stringList)
		}
	}
	assertStateTasks := func(source string, state automation.State) {
		t.Helper()
		assertConfigTypes(source, state.Config)
		for taskID, expected := range expectedTasks {
			task := findAutomationTask(state.Scheduler.Tasks, taskID)
			if task == nil || task.Enabled != expected.enabled || task.IntervalSec != expected.intervalSec {
				t.Errorf("%s task %s = %#v, want enabled %v interval %d", source, taskID, task, expected.enabled, expected.intervalSec)
			}
			if expected.enabledKey != "" && boolFromAny(state.Config[expected.enabledKey], !expected.enabled) != expected.enabled {
				t.Errorf("%s config %s = %#v, want %v", source, expected.enabledKey, state.Config[expected.enabledKey], expected.enabled)
			}
			if expected.intervalKey != "" && intFromAny(state.Config[expected.intervalKey], 0) != expected.intervalSec {
				t.Errorf("%s config %s = %#v, want %d", source, expected.intervalKey, state.Config[expected.intervalKey], expected.intervalSec)
			}
		}
	}
	assertStateTasks("saved", saved)

	loaded, err := store.LoadAutoFarmSettings(ctx)
	if err != nil {
		t.Fatalf("load automation settings: %v", err)
	}
	assertConfigTypes("persisted", loaded.Config)
	for taskID, expected := range expectedTasks {
		loadedTask := findAutoFarmTaskSettings(loaded.Tasks, taskID)
		if loadedTask == nil || loadedTask.Enabled != expected.enabled || loadedTask.IntervalSec != expected.intervalSec {
			t.Errorf("persisted task %s = %#v, want enabled %v interval %d", taskID, loadedTask, expected.enabled, expected.intervalSec)
		}
		if expected.intervalKey != "" && intFromAny(loaded.Config[expected.intervalKey], 0) != expected.intervalSec {
			t.Errorf("persisted config %s = %#v, want %d", expected.intervalKey, loaded.Config[expected.intervalKey], expected.intervalSec)
		}
	}

	running := app.automationScheduler.State()
	assertStateTasks("scheduler", running)

	var saveEvent *eventbus.Event
	for index := range app.memoryEvents {
		if app.memoryEvents[index].event.Type == "auto_farm.settings.save" && app.memoryEvents[index].event.Level == eventbus.LevelInfo {
			saveEvent = &app.memoryEvents[index].event
		}
	}
	if saveEvent == nil {
		t.Fatal("missing successful automation settings event")
	}
	collectExpected := expectedTasks["own_collect"]
	if intFromAny(saveEvent.Data["ownCollectIntervalSec"], 0) != collectExpected.intervalSec || boolFromAny(saveEvent.Data["ownCollectEnabled"], !collectExpected.enabled) != collectExpected.enabled || saveEvent.Data["rushFertilizerMode"] != "normal" {
		t.Fatalf("save event is missing canonical config evidence: %#v", saveEvent.Data)
	}
}

func TestSaveFarmAutomationStatePreservesEveryDetailedConfigKey(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	_ = app.FarmAutomationSchedulerState()

	input := automation.DefaultState()
	input.FeatureGroups = nil
	schedulerKeys := map[string]bool{}
	for _, task := range input.Scheduler.Tasks {
		schedulerKeys[task.EnabledConfigKey] = task.EnabledConfigKey != ""
		schedulerKeys[task.IntervalConfigKey] = task.IntervalConfigKey != ""
	}
	expected := map[string]any{}
	numberListKeys := map[string]bool{}
	retiredAutomationKeys := map[string]bool{
		"autoFarmHeFengTravelRewardEnabled":  true,
		"autoFarmLimitedSeedDrawEnabled":     true,
		"autoFarmLimitedSeedDrawPaidEnabled": true,
	}
	for key, value := range automation.DefaultConfig() {
		if schedulerKeys[key] {
			continue
		}
		if key == "autoWarehouseSellIntervalMinute" || key == "autoWarehouseSellCategories" || key == "warehouseRefreshOnlyOnAutoSell" {
			continue
		}
		switch typed := value.(type) {
		case bool:
			expected[key] = !typed
		case int:
			expected[key] = typed + 17
		case string:
			expected[key] = "round-trip:" + key
		case []int:
			expected[key] = []any{float64(7001), float64(7002)}
			numberListKeys[key] = true
		case []string:
			expected[key] = []any{"round-trip-a", "round-trip-b"}
		default:
			t.Fatalf("unsupported default config type for %s: %T", key, value)
		}
		input.Config[key] = expected[key]
		if retiredAutomationKeys[key] {
			input.Config[key] = true
			expected[key] = false
		}
	}
	if len(expected) < 90 {
		t.Fatalf("detailed config coverage = %d keys, want at least 90", len(expected))
	}

	saved, err := app.SaveFarmAutomationState(input)
	if err != nil {
		t.Fatalf("save automation state: %v", err)
	}
	loaded, err := store.LoadAutoFarmSettings(ctx)
	if err != nil {
		t.Fatalf("load automation settings: %v", err)
	}
	running := app.automationScheduler.State()

	assertConfig := func(source string, config map[string]any) {
		t.Helper()
		for key, want := range expected {
			got := config[key]
			switch typed := want.(type) {
			case bool:
				if boolFromAny(got, !typed) != typed {
					t.Errorf("%s config %s = %#v, want %v", source, key, got, typed)
				}
			case int:
				if intFromAny(got, typed-1) != typed {
					t.Errorf("%s config %s = %#v, want %d", source, key, got, typed)
				}
			case string:
				if stringFromAny(got) != typed {
					t.Errorf("%s config %s = %#v, want %q", source, key, got, typed)
				}
			case []any:
				gotValues := sliceFromAny(got)
				if len(typed) == 0 || len(gotValues) != len(typed) {
					t.Errorf("%s config %s = %#v, want %#v", source, key, got, typed)
					continue
				}
				for index := range typed {
					matches := stringFromAny(gotValues[index]) == stringFromAny(typed[index])
					if numberListKeys[key] {
						matches = intFromAny(gotValues[index], 0) == intFromAny(typed[index], -1)
					}
					if !matches {
						t.Errorf("%s config %s = %#v, want %#v", source, key, got, typed)
						break
					}
				}
			}
		}
	}
	assertConfig("saved", saved.Config)
	assertConfig("persisted", loaded.Config)
	assertConfig("scheduler", running.Config)
}

func TestSaveFarmAutomationStateReturnsStorageErrorWithoutReconfiguringScheduler(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app.store = store
	before := app.FarmAutomationSchedulerState()
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	input := automation.DefaultState()
	for index := range input.Scheduler.Tasks {
		if input.Scheduler.Tasks[index].ID == "friend_steal" {
			input.Scheduler.Tasks[index].Enabled = true
			input.Scheduler.Tasks[index].Priority = 200
		}
	}
	if _, err := app.SaveFarmAutomationState(input); err == nil {
		t.Fatal("expected automation save error")
	}
	after := app.automationScheduler.State().Scheduler
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("failed save reconfigured scheduler:\n before=%#v\n after=%#v", before, after)
	}
	assertOnlyErrorSaveEvent(t, app.memoryEvents, "auto_farm.settings.save")
}

func TestSetFarmAutomationRunModeUpdatesOnlyCurrentAccountMode(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account: %#v", account)
	}

	before := app.FarmAutomationState()
	if before.RunMode != automation.RunModeSafe {
		t.Fatalf("fresh mode = %q, want safe", before.RunMode)
	}
	_ = app.FarmAutomationSchedulerState()
	saved, err := app.SetFarmAutomationRunMode("god")
	if err != nil {
		t.Fatalf("set run mode: %v", err)
	}
	if saved.RunMode != automation.RunModeGod {
		t.Fatalf("saved mode = %q, want god", saved.RunMode)
	}
	if !reflect.DeepEqual(saved.Config, before.Config) || !reflect.DeepEqual(saved.Scheduler.Tasks, before.Scheduler.Tasks) {
		t.Fatalf("mode switch changed automation configuration:\n before=%#v\n saved=%#v", before, saved)
	}
	persisted, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load account A: %v", err)
	}
	beforeConfig, err := json.Marshal(before.Config)
	if err != nil {
		t.Fatalf("marshal configuration before mode change: %v", err)
	}
	persistedConfig, err := json.Marshal(persisted.Config)
	if err != nil {
		t.Fatalf("marshal persisted configuration: %v", err)
	}
	if persisted.RunMode != "god" || !bytes.Equal(persistedConfig, beforeConfig) {
		t.Fatalf("persisted account A = %#v", persisted)
	}
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		t.Fatalf("confirm account B: %#v", account)
	}
	if other := app.FarmAutomationState(); other.RunMode != automation.RunModeSafe {
		t.Fatalf("account B mode = %q, want safe", other.RunMode)
	}
	if persisted, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001"); err != nil || persisted.RunMode != "god" {
		t.Fatalf("account A mode leaked or changed: %#v, %v", persisted, err)
	}
}

func TestSetFarmAutomationRunModeRejectsInvalidModeAndStorageFailure(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		_ = store.Close()
		t.Fatalf("confirm account: %#v", account)
	}
	before := app.FarmAutomationSchedulerState()
	if _, err := app.SetFarmAutomationRunMode("fast"); err == nil {
		_ = store.Close()
		t.Fatal("invalid run mode should fail")
	}
	if after := app.automationScheduler.State().Scheduler; !reflect.DeepEqual(after, before) {
		_ = store.Close()
		t.Fatalf("invalid mode reconfigured scheduler:\n before=%#v\n after=%#v", before, after)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	if _, err := app.SetFarmAutomationRunMode("god"); err == nil {
		t.Fatal("storage failure should fail mode update")
	}
	if after := app.automationScheduler.State().Scheduler; !reflect.DeepEqual(after, before) {
		t.Fatalf("failed mode update reconfigured scheduler:\n before=%#v\n after=%#v", before, after)
	}
}

func TestSaveFarmAutomationStatePreservesCurrentRunMode(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account: %#v", account)
	}
	input := app.FarmAutomationState()
	input.RunMode = automation.RunModeGod
	saved, err := app.SaveFarmAutomationState(input)
	if err != nil {
		t.Fatalf("save automation state: %v", err)
	}
	if saved.RunMode != automation.RunModeSafe {
		t.Fatalf("saved mode = %q, want current safe mode", saved.RunMode)
	}
	persisted, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001")
	if err != nil || persisted.RunMode != "safe" {
		t.Fatalf("persisted mode = %#v, %v", persisted, err)
	}
}

func assertOnlyErrorSaveEvent(t *testing.T, events []accountRuntimeEvent, eventType string) {
	t.Helper()
	foundError := false
	for _, entry := range events {
		event := entry.event
		if event.Type != eventType {
			continue
		}
		if event.Level == eventbus.LevelInfo {
			t.Fatalf("failed save recorded success event: %#v", event)
		}
		if event.Level == eventbus.LevelError {
			foundError = true
		}
	}
	if !foundError {
		t.Fatalf("missing error event %q: %#v", eventType, events)
	}
}

func TestRunFarmAutomationTaskReturnsNotMigrated(t *testing.T) {
	app := newAuthorizedTestApp(t)

	result := app.RunFarmAutomationTask("unknown_task")

	if result.OK {
		t.Fatalf("expected not migrated result, got %#v", result)
	}
	if result.Status != "not_migrated" {
		t.Fatalf("status = %q, want not_migrated", result.Status)
	}
}

func TestRunFarmAutomationTaskRecordsManualTaskEvents(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	result := app.RunFarmAutomationTask("unknown_task")

	if result.Status != automation.StatusNotMigrated {
		t.Fatalf("status = %q, want not_migrated", result.Status)
	}
	events := app.RuntimeEvents(10)
	if len(events) != 2 {
		t.Fatalf("expected start and finish events, got %#v", events)
	}
	if events[0].Source != "auto_farm" || events[0].Type != "task.failed" {
		t.Fatalf("unexpected finish event %#v", events[0])
	}
	if events[0].Data["taskId"] != "unknown_task" || events[0].Data["trigger"] != "manual" || events[0].Data["status"] != string(automation.StatusNotMigrated) {
		t.Fatalf("unexpected finish event data %#v", events[0].Data)
	}
	if events[1].Source != "auto_farm" || events[1].Type != "task.start" {
		t.Fatalf("unexpected start event %#v", events[1])
	}
	if events[1].Data["taskId"] != "unknown_task" || events[1].Data["trigger"] != "manual" {
		t.Fatalf("unexpected start event data %#v", events[1].Data)
	}
}

func TestRunFarmAutomationTaskRecordsAutoTriggerEvents(t *testing.T) {
	app := newAuthorizedTestApp(t)

	result := app.runFarmAutomationTask("unknown_task", "auto")

	if result.Status != automation.StatusNotMigrated {
		t.Fatalf("status = %q, want not_migrated", result.Status)
	}
	events := app.RuntimeEvents(10)
	if len(events) != 2 {
		t.Fatalf("expected start and finish events, got %#v", events)
	}
	if events[0].Type != "task.failed" || events[0].Data["trigger"] != "auto" {
		t.Fatalf("unexpected auto finish event %#v", events[0])
	}
	if events[1].Type != "task.start" || events[1].Data["trigger"] != "auto" {
		t.Fatalf("unexpected auto start event %#v", events[1])
	}
}

func TestFarmAutomationSchedulerStateIsExposed(t *testing.T) {
	app := newAuthorizedTestApp(t)

	state := app.FarmAutomationSchedulerState()

	if len(state.Tasks) == 0 {
		t.Fatalf("scheduler state has no tasks: %#v", state)
	}
	if !state.Enabled {
		t.Fatalf("scheduler should default to enabled: %#v", state)
	}
}

func TestFarmAutomationSchedulerRunDueRecordsTaskEvents(t *testing.T) {
	app := newAuthorizedTestApp(t)

	app.RunDueFarmAutomationTasks()

	events := app.RuntimeEvents(10)
	if len(events) < 2 {
		t.Fatalf("expected scheduler task events, got %#v", events)
	}
	foundStart := false
	foundFinish := false
	for _, event := range events {
		if event.Source != "auto_farm" || event.Data["trigger"] != "auto" {
			continue
		}
		if event.Type == "task.start" {
			foundStart = true
		}
		if event.Type == "task.done" || event.Type == "task.failed" {
			foundFinish = true
		}
	}
	if !foundStart || !foundFinish {
		t.Fatalf("expected scheduler start and finish events, got %#v", events)
	}
}

func TestFarmAutomationSchedulerStartStopUpdatesSessionStateAndRecordsEvents(t *testing.T) {
	app := newAuthorizedTestApp(t)

	started := app.StartFarmAutomationScheduler()
	if !app.FarmAutomationState().Running {
		t.Fatalf("FarmAutomationState should report running after start: %#v", started)
	}

	stopped := app.StopFarmAutomationScheduler()
	if app.FarmAutomationState().Running {
		t.Fatalf("FarmAutomationState should report stopped after stop: %#v", stopped)
	}

	events := app.RuntimeEvents(10)
	foundStart := false
	foundStop := false
	for _, event := range events {
		if event.Source != "auto_farm" {
			continue
		}
		if event.Type == "scheduler.started" {
			foundStart = true
		}
		if event.Type == "scheduler.stopped" {
			foundStop = true
		}
	}
	if !foundStart || !foundStop {
		t.Fatalf("expected scheduler start/stop events, got %#v", events)
	}
}

func TestStartFarmAutomationSchedulerRetriesAfterAccountSwitchBeforeAction(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	entered := make(chan struct{})
	release := make(chan struct{})
	firstAction := true
	app.automationSchedulerMu.Lock()
	app.beforeAutomationSchedulerAction = func() {
		if !firstAction {
			return
		}
		firstAction = false
		close(entered)
		<-release
	}
	app.automationSchedulerMu.Unlock()

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account A: %s", account.Error)
	}
	stateCh := make(chan automation.SchedulerState, 1)
	go func() {
		stateCh <- app.StartFarmAutomationScheduler()
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("scheduler start did not reach pre-action hook")
	}

	app.automationSchedulerMu.Lock()
	accountAScheduler := app.automationScheduler
	accountASchedulerKey := app.automationSchedulerAccountKey
	app.automationSchedulerMu.Unlock()
	if accountAScheduler == nil || accountASchedulerKey != "gid:10001" {
		close(release)
		t.Fatalf("expected captured account A scheduler, got scheduler=%p key=%q", accountAScheduler, accountASchedulerKey)
	}
	defer accountAScheduler.Stop()
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		close(release)
		t.Fatalf("confirm account B: %s", account.Error)
	}
	close(release)
	startedState := <-stateCh
	if !startedState.Enabled {
		t.Fatalf("started scheduler state should remain enabled: %#v", startedState)
	}

	app.automationSchedulerMu.Lock()
	currentScheduler := app.automationScheduler
	currentSchedulerKey := app.automationSchedulerAccountKey
	app.automationSchedulerMu.Unlock()
	if currentScheduler == nil || currentScheduler == accountAScheduler || currentSchedulerKey != "gid:10002" {
		t.Fatalf("scheduler start did not retry for account B: scheduler=%p old=%p key=%q", currentScheduler, accountAScheduler, currentSchedulerKey)
	}
	defer currentScheduler.Stop()
	if accountAScheduler.State().Running {
		t.Fatal("orphaned account A scheduler was started after account switch")
	}
	if !currentScheduler.State().Running {
		t.Fatal("account B scheduler was not started")
	}

	eventsA, err := store.ListRuntimeEventsForAccount(ctx, "gid:10001", 20)
	if err != nil {
		t.Fatalf("load account A events: %v", err)
	}
	eventsB, err := store.ListRuntimeEventsForAccount(ctx, "gid:10002", 20)
	if err != nil {
		t.Fatalf("load account B events: %v", err)
	}
	for _, event := range eventsA {
		if event.Type == "scheduler.started" {
			t.Fatalf("account A unexpectedly contains scheduler.started: %#v", event)
		}
	}
	foundBStart := false
	for _, event := range eventsB {
		if event.Type == "scheduler.started" {
			foundBStart = true
		}
	}
	if !foundBStart {
		t.Fatalf("account B is missing scheduler.started: %#v", eventsB)
	}
}

func TestSaveFarmAutomationStateReconfiguresActiveScheduler(t *testing.T) {
	app := newAuthorizedTestApp(t)
	_ = app.FarmAutomationSchedulerState()
	state := app.FarmAutomationState()
	state.Config["autoFarmFriendEnabled"] = true
	for index := range state.FeatureGroups {
		if state.FeatureGroups[index].ID == "friends" {
			state.FeatureGroups[index].Enabled = true
		}
	}
	for index := range state.Scheduler.Tasks {
		switch state.Scheduler.Tasks[index].ID {
		case "own_base":
			state.Scheduler.Tasks[index].Priority = 1
		case "friend_steal":
			state.Scheduler.Tasks[index].Enabled = true
			state.Scheduler.Tasks[index].Priority = 200
		}
	}

	if _, err := app.SaveFarmAutomationState(state); err != nil {
		t.Fatalf("save automation state: %v", err)
	}

	schedulerState := app.automationScheduler.State().Scheduler
	if schedulerState.Tasks[0].ID != "friend_steal" {
		t.Fatalf("scheduler was not reconfigured after save: %#v", schedulerState.Tasks[:3])
	}
}

func TestAutomationSettingsWritesSerializeUserSaveAndDailyMarker(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account: %s", account.Error)
	}
	_ = app.FarmAutomationSchedulerState()

	userState := app.FarmAutomationState()
	userState.Config["autoFarmPlantSeedId"] = 20002
	firstWriteEntered := make(chan struct{})
	secondWriteEntered := make(chan struct{})
	releaseFirstWrite := make(chan struct{})
	writeCalls := 0
	app.beforeAutomationSettingsWrite = func() {
		writeCalls++
		if writeCalls == 1 {
			close(firstWriteEntered)
			<-releaseFirstWrite
			return
		}
		if writeCalls == 2 {
			close(secondWriteEntered)
		}
	}

	saveDone := make(chan error, 1)
	go func() {
		_, err := app.SaveFarmAutomationState(userState)
		saveDone <- err
	}()
	select {
	case <-firstWriteEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("user save did not acquire settings write lock")
	}

	dailyDone := make(chan struct{})
	markerTime := time.Date(2026, 7, 10, 12, 0, 0, 0, time.Local)
	go func() {
		app.markFriendMischiefDailyDoneForAccount("gid:10001", markerTime)
		close(dailyDone)
	}()
	select {
	case <-secondWriteEntered:
		t.Fatal("daily marker entered persistence while user save held the write lock")
	default:
	}
	close(releaseFirstWrite)
	if err := <-saveDone; err != nil {
		t.Fatalf("save user state: %v", err)
	}
	select {
	case <-secondWriteEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("daily marker did not acquire write lock after user save")
	}
	select {
	case <-dailyDone:
	case <-time.After(2 * time.Second):
		t.Fatal("daily marker did not finish")
	}

	persisted, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load persisted settings: %v", err)
	}
	if intFromAny(persisted.Config["autoFarmPlantSeedId"], 0) != 20002 {
		t.Fatalf("daily marker overwrote user config: %#v", persisted.Config)
	}
	if persisted.Config[automation.FriendMischiefDailyDoneDateConfigKey] != "2026-07-10" {
		t.Fatalf("daily marker missing from persisted config: %#v", persisted.Config)
	}
	schedulerState := app.FarmAutomationState()
	if intFromAny(schedulerState.Config["autoFarmPlantSeedId"], 0) != 20002 || schedulerState.Config[automation.FriendMischiefDailyDoneDateConfigKey] != "2026-07-10" {
		t.Fatalf("scheduler state diverged from persisted settings: %#v", schedulerState.Config)
	}
}

func TestSaveFarmAutomationStatePreservesRuntimeOwnedDailyMarkers(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account: %s", account.Error)
	}
	markerTime := time.Date(2026, 7, 10, 12, 0, 0, 0, time.Local)
	app.markFriendMischiefDailyDoneForAccount("gid:10001", markerTime)

	staleUIState := automation.DefaultState()
	staleUIState.Config["autoFarmPlantSeedId"] = 20003
	if _, err := app.SaveFarmAutomationState(staleUIState); err != nil {
		t.Fatalf("save stale UI state: %v", err)
	}
	persisted, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load persisted settings: %v", err)
	}
	if persisted.Config[automation.FriendMischiefDailyDoneDateConfigKey] != "2026-07-10" {
		t.Fatalf("UI save removed runtime-owned daily marker: %#v", persisted.Config)
	}
}

func TestConcurrentAutomationDailyMarkersDoNotOverwriteEachOther(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account: %s", account.Error)
	}

	firstWriteEntered := make(chan struct{})
	secondWriteEntered := make(chan struct{})
	releaseFirstWrite := make(chan struct{})
	writeCalls := 0
	app.beforeAutomationSettingsWrite = func() {
		writeCalls++
		if writeCalls == 1 {
			close(firstWriteEntered)
			<-releaseFirstWrite
			return
		}
		if writeCalls == 2 {
			close(secondWriteEntered)
		}
	}
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.Local)
	friendDone := make(chan struct{})
	go func() {
		app.markFriendMischiefDailyDoneForAccount("gid:10001", now)
		close(friendDone)
	}()
	select {
	case <-firstWriteEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("first daily marker did not reach persistence")
	}
	giftDone := make(chan struct{})
	go func() {
		app.markDailyOnceTaskDoneForAccount("gid:10001", "svip_daily_gift", now)
		close(giftDone)
	}()
	select {
	case <-secondWriteEntered:
		t.Fatal("second daily marker entered persistence while first held the write lock")
	default:
	}
	close(releaseFirstWrite)
	select {
	case <-secondWriteEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("second daily marker did not acquire write lock")
	}
	for name, done := range map[string]<-chan struct{}{"friend": friendDone, "gift": giftDone} {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s daily marker did not finish", name)
		}
	}

	persisted, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load persisted settings: %v", err)
	}
	if persisted.Config[automation.FriendMischiefDailyDoneDateConfigKey] != "2026-07-10" || persisted.Config[automation.DailyOnceTaskDoneDateConfigKey("svip_daily_gift")] != "2026-07-10" {
		t.Fatalf("concurrent daily marker was lost: %#v", persisted.Config)
	}
}

func TestRunFarmAutomationOwnBaseUsesRuntimeFacade(t *testing.T) {
	app := newAuthorizedTestApp(t)

	result := app.RunFarmAutomationTask("own_base")

	if result.OK {
		t.Fatalf("runtime probe should not claim full task execution, got %#v", result)
	}
	if result.Status != "runtime_not_ready" {
		t.Fatalf("status = %q, want runtime_not_ready", result.Status)
	}
	if result.TaskID != "own_base" {
		t.Fatalf("taskID = %q, want own_base", result.TaskID)
	}
}

func TestRunFarmAutomationOwnCollectUsesRuntimeFacade(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"workLandIds": map[string]any{
				"collect": []any{float64(9), float64(2)},
			},
		},
		"gameCtl.harvestLandsBatchByProtocol": map[string]any{"ok": true, "results": []any{map[string]any{"ok": true, "landId": float64(2)}}},
	})

	result := app.RunFarmAutomationTask("own_collect")

	if !result.OK {
		t.Fatalf("expected own_collect runtime facade OK, got %#v", result)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if result.ActionCount != 1 {
		t.Fatalf("action count = %d, want 1", result.ActionCount)
	}
	if !link.called("gameCtl.getFarmStatus") || !link.called("gameCtl.harvestLandsBatchByProtocol") {
		t.Fatalf("expected status and harvest calls, got %#v", link.calls)
	}
	for _, entry := range app.memoryEvents {
		if entry.event.Type == "task.done" && entry.event.Data["taskId"] == "own_collect" {
			if got := entry.event.Data["actionCount"]; got != 1 {
				t.Fatalf("manual task event action count = %#v, want 1", got)
			}
			return
		}
	}
	t.Fatal("manual own_collect completion event was not recorded")
}

func TestRunFarmAutomationOwnPlantUsesRuntimeFacade(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(9), "stageKind": "empty", "interactable": true},
			},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	})
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	state := app.FarmAutomationState()
	state.Config["autoFarmPlantPrimaryMode"] = "specified_seed"
	state.Config["autoFarmPlantSeedId"] = 20002
	if _, err := app.SaveFarmAutomationState(state); err != nil {
		t.Fatalf("save automation state: %v", err)
	}

	result := app.RunFarmAutomationTask("own_plant")

	if !result.OK {
		t.Fatalf("expected own_plant runtime facade OK, got %#v", result)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if !link.called("gameCtl.getFarmOwnership") || !link.called("gameCtl.getFarmStatus") || !link.called("gameCtl.autoPlant") {
		t.Fatalf("expected ownership, status and autoPlant calls, got %#v", link.calls)
	}
}

func TestRunFarmAutomationOwnFertilizerUsesRuntimeFacade(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(9), "stageKind": "growing", "hasPlant": true, "plantId": float64(20002), "matureInSec": float64(120)},
			},
		},
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
	})
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	state := app.FarmAutomationState()
	for index := range state.FeatureGroups {
		if state.FeatureGroups[index].ID == "fertilizer" {
			state.FeatureGroups[index].Enabled = true
		}
	}
	state.Config["autoFarmRushFertilizerMode"] = "organic"
	if _, err := app.SaveFarmAutomationState(state); err != nil {
		t.Fatalf("save automation state: %v", err)
	}

	result := app.RunFarmAutomationTask("own_fertilizer")

	if !result.OK {
		t.Fatalf("expected own_fertilizer runtime facade OK, got %#v", result)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if !link.called("gameCtl.getFarmStatus") || !link.called("gameCtl.fertilizeLandsBatch") {
		t.Fatalf("expected status and fertilizer batch calls, got %#v", link.calls)
	}
}

func TestRunScheduledFarmAutomationFriendMischiefMarksDailyLimitDone(t *testing.T) {
	ctx := context.Background()
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFriendList":               []any{map[string]any{"gid": float64(10001), "workCounts": map[string]any{"mischief": float64(1)}}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true, "workLandIds": map[string]any{"bug": []any{float64(1)}, "grass": []any{}}},
		"gameCtl.friendMischiefLandsBatch":    map[string]any{"ok": false, "reason": "dispatch_failed", "results": []any{map[string]any{"error": "操作次数已达上限"}}},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	result := app.RunScheduledFarmAutomationTask("friend_mischief")

	if !result.OK || result.Status != automation.StatusOK {
		t.Fatalf("daily limit should be recorded as completed, got %#v", result)
	}
	state := app.FarmAutomationState()
	if state.Config[automation.FriendMischiefDailyDoneDateConfigKey] != time.Now().Format("2006-01-02") {
		t.Fatalf("daily done date was not persisted: %#v", state.Config[automation.FriendMischiefDailyDoneDateConfigKey])
	}
	friendMischiefTaskDone := false
	for _, task := range state.Scheduler.Tasks {
		if task.ID == "friend_mischief" {
			friendMischiefTaskDone = task.DailyDoneToday
			break
		}
	}
	if !friendMischiefTaskDone {
		t.Fatalf("friend mischief task was not marked 今日已完成: %#v", state.Scheduler.Tasks)
	}
	if !link.called("gameCtl.inspectFriendFarmByProtocol") {
		t.Fatalf("expected inspect friend farm protocol call, got %#v", link.calls)
	}

	link.calls = nil
	second := app.RunScheduledFarmAutomationTask("friend_mischief")

	if !second.OK || !strings.Contains(second.Message, "跳过") {
		t.Fatalf("second scheduled run should skip, got %#v", second)
	}
	if len(link.calls) != 0 {
		t.Fatalf("second scheduled run should not call runtime, got %#v", link.calls)
	}
}

func TestRunScheduledFarmAutomationFriendHelpPersistsDailyCountAndSkips(t *testing.T) {
	ctx := context.Background()
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "workCounts": map[string]any{"water": float64(1)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"water": []any{float64(1)}},
		},
		"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	state := app.FarmAutomationState()
	state.Config["autoFarmFriendHelpDailyLimit"] = 1
	state.Config["autoFarmFriendHelpMaxFriends"] = 5
	if _, err := app.SaveFarmAutomationState(state); err != nil {
		t.Fatalf("save automation state: %v", err)
	}

	result := app.RunScheduledFarmAutomationTask("friend_help")

	if !result.OK || result.SuccessfulFriends != 1 {
		t.Fatalf("scheduled friend help result = %#v", result)
	}
	state = app.FarmAutomationState()
	if state.Config[automation.FriendHelpDailyCountDateConfigKey] != time.Now().Format("2006-01-02") {
		t.Fatalf("daily date was not persisted: %#v", state.Config)
	}
	if intFromAny(state.Config[automation.FriendHelpDailyCountConfigKey], 0) != 1 {
		t.Fatalf("daily count was not persisted: %#v", state.Config)
	}
	link.calls = nil
	second := app.RunScheduledFarmAutomationTask("friend_help")
	if !second.OK || !strings.Contains(second.Message, "每日上限") {
		t.Fatalf("second scheduled run should skip: %#v", second)
	}
	if len(link.calls) != 0 {
		t.Fatalf("exhausted daily limit should not call runtime: %#v", link.calls)
	}
}

func TestRunFarmAutomationFriendHelpManualRunDoesNotCount(t *testing.T) {
	ctx := context.Background()
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "workCounts": map[string]any{"water": float64(1)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"water": []any{float64(1)}},
		},
		"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	state := app.FarmAutomationState()
	state.Config["autoFarmFriendHelpDailyLimit"] = 1
	if _, err := app.SaveFarmAutomationState(state); err != nil {
		t.Fatalf("save automation state: %v", err)
	}

	result := app.RunFarmAutomationTask("friend_help")

	if !result.OK || result.SuccessfulFriends != 1 {
		t.Fatalf("manual friend help result = %#v", result)
	}
	state = app.FarmAutomationState()
	if _, ok := state.Config[automation.FriendHelpDailyCountDateConfigKey]; ok {
		t.Fatalf("manual run persisted daily date: %#v", state.Config)
	}
	if _, ok := state.Config[automation.FriendHelpDailyCountConfigKey]; ok {
		t.Fatalf("manual run persisted daily count: %#v", state.Config)
	}
}

func TestScheduledFriendHelpDailyCountIsAccountScoped(t *testing.T) {
	ctx := context.Background()
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(20001), "workCounts": map[string]any{"water": float64(1)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"water": []any{float64(1)}},
		},
		"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account: %s", account.Error)
	}
	state := app.FarmAutomationState()
	state.Config["autoFarmFriendHelpDailyLimit"] = 1
	if _, err := app.SaveFarmAutomationState(state); err != nil {
		t.Fatalf("save automation state: %v", err)
	}

	result := app.RunScheduledFarmAutomationTask("friend_help")

	if !result.OK || result.SuccessfulFriends != 1 {
		t.Fatalf("scheduled friend help result = %#v", result)
	}
	accountA, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load account A: %v", err)
	}
	accountB, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10002")
	if err != nil {
		t.Fatalf("load account B: %v", err)
	}
	defaultSettings, err := store.LoadAutoFarmSettingsForAccount(ctx, storage.DefaultRuntimeAccountKey)
	if err != nil {
		t.Fatalf("load default account: %v", err)
	}
	if intFromAny(accountA.Config[automation.FriendHelpDailyCountConfigKey], 0) != 1 {
		t.Fatalf("account A daily count = %#v", accountA.Config)
	}
	for name, settings := range map[string]storage.AutoFarmSettings{"account B": accountB, "default": defaultSettings} {
		if _, ok := settings.Config[automation.FriendHelpDailyCountConfigKey]; ok {
			t.Fatalf("%s unexpectedly contains friend help daily count: %#v", name, settings.Config)
		}
	}
}

func TestRunScheduledFarmAutomationDailyOnceTasksMarkDoneAndSkipToday(t *testing.T) {
	tests := []struct {
		name   string
		taskID string
		method string
		result map[string]any
	}{
		{
			name:   "svip_daily_gift",
			taskID: "svip_daily_gift",
			method: "gameCtl.claimSvipDailyGift",
			result: map[string]any{"ok": true, "success": true, "claimedCount": float64(1), "action": "svip_daily_gift"},
		},
		{
			name:   "monthly_card_reward",
			taskID: "monthly_card_reward",
			method: "gameCtl.claimMonthlyCardReward",
			result: map[string]any{"ok": true, "success": true, "skipped": true, "reason": "already_claimed", "action": "monthly_card_reward"},
		},
		{
			name:   "mall_daily_fertilizer",
			taskID: "mall_daily_fertilizer",
			method: "gameCtl.claimMallDailyFertilizerGift",
			result: map[string]any{"ok": true, "success": true, "skipped": true, "reason": "限购次数已用完", "action": "claim_mall_daily_fertilizer"},
		},
		{
			name:   "share_reward",
			taskID: "share_reward",
			method: "gameCtl.claimShareRewardByProtocol",
			result: map[string]any{"ok": true, "success": true, "skipped": true, "reason": "already_claimed", "action": "claim_share_reward"},
		},
		{
			name:   "limited_seed_draw",
			taskID: "limited_seed_draw",
			method: "gameCtl.claimLimitedSeedDraw",
			result: map[string]any{"ok": true, "success": true, "itemUpdateCount": float64(1), "action": "claim_limited_seed_draw"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			app, link := newAppWithFakeRuntime(t, map[string]any{tt.method: tt.result})
			store, err := storage.Open(ctx, t.TempDir())
			if err != nil {
				t.Fatalf("open store: %v", err)
			}
			defer store.Close()
			app.store = store

			result := app.RunScheduledFarmAutomationTask(tt.taskID)

			if !result.OK || result.Status != automation.StatusOK {
				t.Fatalf("daily-once task should report OK, got %#v", result)
			}
			key := automation.DailyOnceTaskDoneDateConfigKey(tt.taskID)
			if state := app.FarmAutomationState(); state.Config[key] != time.Now().Format("2006-01-02") {
				t.Fatalf("daily done date was not persisted for %s: %#v", tt.taskID, state.Config[key])
			}
			if !link.called(tt.method) {
				t.Fatalf("first scheduled run should call %s, got %#v", tt.method, link.calls)
			}

			link.calls = nil
			second := app.RunScheduledFarmAutomationTask(tt.taskID)

			if !second.OK || !strings.Contains(second.Message, "跳过") {
				t.Fatalf("second scheduled run should skip, got %#v", second)
			}
			if len(link.calls) != 0 {
				t.Fatalf("second scheduled run should not call runtime, got %#v", link.calls)
			}
		})
	}
}

func TestRunFarmAutomationDailyOnceTaskManualRunMarksDoneToday(t *testing.T) {
	ctx := context.Background()
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.claimSvipDailyGift": map[string]any{
			"ok":           true,
			"success":      true,
			"claimedCount": float64(1),
			"action":       "svip_daily_gift",
		},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	result := app.RunFarmAutomationTask("svip_daily_gift")

	if !result.OK || result.Status != automation.StatusOK {
		t.Fatalf("manual daily-once task should report OK, got %#v", result)
	}
	state := app.FarmAutomationState()
	key := automation.DailyOnceTaskDoneDateConfigKey("svip_daily_gift")
	if state.Config[key] != time.Now().Format("2006-01-02") {
		t.Fatalf("manual run did not persist daily done date: %#v", state.Config[key])
	}
	found := false
	for _, task := range state.Scheduler.Tasks {
		if task.ID == "svip_daily_gift" {
			found = true
			if !task.DailyDoneToday {
				t.Fatalf("manual run did not surface DailyDoneToday in state: %#v", task)
			}
		}
	}
	if !found {
		t.Fatal("svip_daily_gift task not found in scheduler state")
	}
}

func TestAutomationDailyStatePersistsForCurrentAccount(t *testing.T) {
	ctx := context.Background()
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.claimSvipDailyGift": map[string]any{
			"ok":           true,
			"success":      true,
			"claimedCount": float64(1),
			"action":       "svip_daily_gift",
		},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001})
	if account.Error != "" {
		t.Fatalf("confirm runtime account: %s", account.Error)
	}
	result := app.RunFarmAutomationTask("svip_daily_gift")
	if !result.OK || result.Status != automation.StatusOK {
		t.Fatalf("manual daily-once task should report OK, got %#v", result)
	}

	key := automation.DailyOnceTaskDoneDateConfigKey("svip_daily_gift")
	accountSettings, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load current account settings: %v", err)
	}
	defaultSettings, err := store.LoadAutoFarmSettingsForAccount(ctx, storage.DefaultRuntimeAccountKey)
	if err != nil {
		t.Fatalf("load default account settings: %v", err)
	}
	if _, exists := defaultSettings.Config[key]; exists {
		t.Fatalf("default account unexpectedly contains daily done date: %#v", defaultSettings.Config[key])
	}
	if accountSettings.Config[key] != time.Now().Format("2006-01-02") {
		t.Fatalf("current account daily done date = %#v", accountSettings.Config[key])
	}
}

func TestAutomationRunKeepsCapturedAccountAfterSwitch(t *testing.T) {
	ctx := context.Background()
	started := make(chan struct{})
	release := make(chan struct{})
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.claimSvipDailyGift": fakeRuntimeBlockingResponse{
			started: started,
			release: release,
			value: map[string]any{
				"ok":           true,
				"success":      true,
				"claimedCount": float64(1),
				"action":       "svip_daily_gift",
			},
		},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account A: %s", account.Error)
	}
	resultCh := make(chan automation.ActionResult, 1)
	go func() {
		resultCh <- app.RunFarmAutomationTask("svip_daily_gift")
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("automation task did not reach blocked runtime call")
	}
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		close(release)
		t.Fatalf("confirm account B: %s", account.Error)
	}
	close(release)
	result := <-resultCh
	if !result.OK || result.Status != automation.StatusOK {
		t.Fatalf("manual daily-once task should report OK, got %#v", result)
	}

	key := automation.DailyOnceTaskDoneDateConfigKey("svip_daily_gift")
	accountA, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load account A settings: %v", err)
	}
	accountB, err := store.LoadAutoFarmSettingsForAccount(ctx, "gid:10002")
	if err != nil {
		t.Fatalf("load account B settings: %v", err)
	}
	defaultSettings, err := store.LoadAutoFarmSettingsForAccount(ctx, storage.DefaultRuntimeAccountKey)
	if err != nil {
		t.Fatalf("load default settings: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	if accountA.Config[key] != today {
		t.Fatalf("account A daily done date = %#v, want %q", accountA.Config[key], today)
	}
	if _, exists := accountB.Config[key]; exists {
		t.Fatalf("account B unexpectedly contains daily done date: %#v", accountB.Config[key])
	}
	if _, exists := defaultSettings.Config[key]; exists {
		t.Fatalf("default unexpectedly contains daily done date: %#v", defaultSettings.Config[key])
	}

	eventsA, err := store.ListRuntimeEventsForAccount(ctx, "gid:10001", 20)
	if err != nil {
		t.Fatalf("load account A events: %v", err)
	}
	eventsB, err := store.ListRuntimeEventsForAccount(ctx, "gid:10002", 20)
	if err != nil {
		t.Fatalf("load account B events: %v", err)
	}
	accountATaskEvents := 0
	for _, event := range eventsA {
		if event.Type == "task.start" || event.Type == "task.done" {
			accountATaskEvents++
		}
	}
	if accountATaskEvents != 2 {
		t.Fatalf("account A task events = %d, want start and done: %#v", accountATaskEvents, eventsA)
	}
	for _, event := range eventsB {
		if event.Type == "task.start" || event.Type == "task.done" {
			t.Fatalf("account B unexpectedly contains task event: %#v", event)
		}
	}
}

func TestSafeModeRejectsManualTaskWhileAnotherTaskRuns(t *testing.T) {
	ctx := context.Background()
	started := make(chan struct{})
	release := make(chan struct{})
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.claimSvipDailyGift": fakeRuntimeBlockingResponse{
			started: started,
			release: release,
			value: map[string]any{
				"ok":           true,
				"success":      true,
				"claimedCount": float64(1),
				"action":       "svip_daily_gift",
			},
		},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account: %s", account.Error)
	}
	if state := app.FarmAutomationState(); state.RunMode != automation.RunModeSafe {
		t.Fatalf("fresh account mode = %q, want safe", state.RunMode)
	}

	firstResult := make(chan automation.ActionResult, 1)
	go func() {
		firstResult <- app.RunFarmAutomationTask("svip_daily_gift")
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("first automation task did not reach blocked runtime call")
	}

	result := app.RunFarmAutomationTask("reward_claim")
	if result.OK || result.Status != automation.StatusRuntimeBusy || result.Message != "当前有任务正在执行，请稍后再试。" {
		close(release)
		t.Fatalf("manual result = %#v, want runtime busy", result)
	}
	close(release)
	if result := <-firstResult; !result.OK {
		t.Fatalf("first task result = %#v", result)
	}
}

func TestAutomationDailySaveDoesNotReconfigureDifferentAccountScheduler(t *testing.T) {
	ctx := context.Background()
	started := make(chan struct{})
	release := make(chan struct{})
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.claimSvipDailyGift": fakeRuntimeBlockingResponse{
			started: started,
			release: release,
			value: map[string]any{
				"ok":           true,
				"success":      true,
				"claimedCount": float64(1),
				"action":       "svip_daily_gift",
			},
		},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account A: %s", account.Error)
	}
	resultCh := make(chan automation.ActionResult, 1)
	go func() {
		resultCh <- app.RunFarmAutomationTask("svip_daily_gift")
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("automation task did not reach blocked runtime call")
	}
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		close(release)
		t.Fatalf("confirm account B: %s", account.Error)
	}
	_ = app.FarmAutomationSchedulerState()
	close(release)
	result := <-resultCh
	if !result.OK || result.Status != automation.StatusOK {
		t.Fatalf("manual daily-once task should report OK, got %#v", result)
	}

	state := app.automationScheduler.State().Scheduler
	for _, task := range state.Tasks {
		if task.ID == "svip_daily_gift" && task.DailyDoneToday {
			t.Fatalf("account B scheduler was reconfigured by account A daily save: %#v", task)
		}
	}
}

func TestAppExposesCropAnalytics(t *testing.T) {
	app := newAuthorizedTestApp(t)

	payload := app.FarmCropAnalytics()

	if len(payload.Items) == 0 {
		t.Fatalf("expected crop analytics items, got %#v", payload)
	}
	if payload.Source != "resources/gameConfig" {
		t.Fatalf("unexpected crop analytics source %q", payload.Source)
	}
}

func TestAppCropAnalyticsUsesRuntimeProfileForLevelRecommendations(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getPlayerProfile": map[string]any{"plantLevel": 2, "nickname": "tester"},
		"gameCtl.getSeedList":      []any{map[string]any{"itemId": 20002, "count": 3, "name": "白萝卜种子"}},
		"gameCtl.requestShopData":  map[string]any{"ok": true},
		"gameCtl.getShopSeedList":  []any{map[string]any{"itemId": 20003, "goodsId": 90003, "price": 2, "level": 2}},
	})

	payload := app.FarmCropAnalytics()

	if payload.EffectiveMaxLevel != 2 || payload.LevelSource != "profile" {
		t.Fatalf("expected runtime profile level, got %#v", payload)
	}
	if len(payload.Recommendations) < 5 {
		t.Fatalf("expected strategy recommendations, got %#v", payload.Recommendations)
	}
	for _, item := range payload.Items {
		if item.Level != nil && *item.Level > 2 {
			t.Fatalf("expected item filtered by profile level, got %#v", item)
		}
	}
	if !link.called("gameCtl.getPlayerProfile") || !link.called("gameCtl.getSeedList") || !link.called("gameCtl.getShopSeedList") {
		t.Fatalf("expected runtime profile and availability calls, got %#v", link.calls)
	}
}

func TestAppExposesAtlasPreview(t *testing.T) {
	app := newAuthorizedTestApp(t)

	payload := app.FarmAtlasPreview()

	if payload.Status != "static_preview" {
		t.Fatalf("unexpected atlas status %#v", payload)
	}
	if payload.RefreshEnabled || payload.BuyEnabled {
		t.Fatalf("runtime atlas actions should stay disabled: %#v", payload)
	}
}

func TestAppAtlasPreviewRefreshesRuntimeRows(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.requestAtlasUnlockRowsByProtocol": map[string]any{
			"collectedAt": float64(1234),
			"crop": map[string]any{
				"complete": true,
				"items": []any{
					map[string]any{"fruitId": 40002, "name": "白萝卜", "locked": false, "unlocked": true},
				},
			},
		},
	})

	payload := app.FarmAtlasPreview()

	if payload.Status != "runtime" || !payload.RefreshEnabled {
		t.Fatalf("expected runtime atlas payload, got %#v", payload)
	}
	crop := findAppAtlasSection(payload.Sections, "crop")
	if crop == nil || len(crop.Items) != 1 || crop.Items[0].Name != "白萝卜" || !crop.Items[0].Unlocked {
		t.Fatalf("unexpected runtime atlas section %#v", crop)
	}
	if !link.called("gameCtl.requestAtlasUnlockRowsByProtocol") {
		t.Fatalf("expected atlas runtime call, got %#v", link.calls)
	}
}

func TestAppExposesLandDetailsGate(t *testing.T) {
	app := newAuthorizedTestApp(t)

	payload := app.FarmLandDetails()

	if payload.Status != "not_migrated" {
		t.Fatalf("unexpected land details payload %#v", payload)
	}
	if len(payload.Lands) != 0 {
		t.Fatalf("expected no fake land details, got %#v", payload.Lands)
	}
}

func TestAppLandDetailsReadsRuntimeFarmStatus(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType":   "own",
			"totalGrids": float64(3),
			"grids": []any{
				map[string]any{"landId": float64(5), "stageKind": "empty"},
				map[string]any{
					"landId":                   float64(1),
					"landLevel":                float64(1),
					"landType":                 "purplegold",
					"plantName":                "白萝卜",
					"displayPlantName":         "黄金白萝卜",
					"imageUrl":                 "/api/plant-image?seedId=20002",
					"stageKind":                "mature",
					"canHarvest":               true,
					"matureInSec":              float64(0),
					"currentSeason":            float64(2),
					"totalSeason":              float64(3),
					"landSize":                 float64(4),
					"occupancyPlantSize":       float64(4),
					"occupancyAnchorLandId":    float64(5),
					"needsWater":               true,
					"needsEraseGrass":          true,
					"needsKillBug":             true,
					"needGoldenBug":            true,
					"isMultiSeason":            true,
					"matureAtMs":               float64(1720000000000),
					"occupiedByMultiTilePlant": true,
				},
				map[string]any{"landId": float64(2), "stageKind": "empty"},
			},
		},
	})

	payload := app.FarmLandDetails()

	if payload.Status != "runtime" || len(payload.Lands) != 3 {
		t.Fatalf("expected runtime lands, got %#v", payload)
	}
	gotOrder := []int{payload.Lands[0].LandID, payload.Lands[1].LandID, payload.Lands[2].LandID}
	if gotOrder[0] != 1 || gotOrder[1] != 2 || gotOrder[2] != 5 {
		t.Fatalf("expected lands sorted by land id, got %#v", gotOrder)
	}
	if payload.Lands[0].PlantName != "白萝卜" || payload.Lands[0].Status != "mature" {
		t.Fatalf("unexpected normalized land %#v", payload.Lands[0])
	}
	if payload.Lands[0].DisplayPlantName != "黄金白萝卜" || payload.Lands[0].LandTypeLabel != "紫金土地" {
		t.Fatalf("expected enriched land identity fields, got %#v", payload.Lands[0])
	}
	if !strings.HasPrefix(payload.Lands[0].ImageURL, "/farm-assets/") {
		t.Fatalf("expected registered local land image url, got %#v", payload.Lands[0])
	}
	if payload.Lands[0].MatureEtaText != "已成熟" || payload.Lands[0].CurrentSeason != 2 || payload.Lands[0].TotalSeason != 3 {
		t.Fatalf("expected maturity and season fields, got %#v", payload.Lands[0])
	}
	if payload.Lands[0].LandSize != 4 || payload.Lands[0].OccupancyPlantSize != 4 || payload.Lands[0].OccupancyAnchorLandID != 5 || !payload.Lands[0].OccupiedByMultiTilePlant {
		t.Fatalf("expected occupancy fields, got %#v", payload.Lands[0])
	}
	if !payload.Lands[0].NeedWater || !payload.Lands[0].NeedWeed || !payload.Lands[0].NeedBug || !payload.Lands[0].NeedGoldenBug {
		t.Fatalf("expected work flags, got %#v", payload.Lands[0])
	}
	if !payload.Actions[0].Enabled {
		t.Fatalf("expected land refresh action enabled, got %#v", payload.Actions)
	}
	if !link.called("gameCtl.getFarmStatus") {
		t.Fatalf("expected farm status runtime call, got %#v", link.calls)
	}
}

func TestAppLandDetailsDeltaSkipsCountdownOnlyChanges(t *testing.T) {
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFarmStatus": fakeRuntimeSequence{
			map[string]any{"farmType": "own", "grids": []any{map[string]any{
				"landId": 1, "plantName": "白萝卜", "seedId": 20002, "stageKind": "growing", "currentStage": 2, "matureInSec": 120,
			}}},
			map[string]any{"farmType": "own", "grids": []any{map[string]any{
				"landId": 1, "plantName": "白萝卜", "seedId": 20002, "stageKind": "growing", "currentStage": 2, "matureInSec": 115,
			}}},
		},
	})

	first := app.FarmLandDetails()
	if first.Revision == "" {
		t.Fatalf("expected full snapshot revision: %#v", first)
	}
	delta := app.FarmLandDetailsSince(first.Revision)
	if delta.Full || len(delta.Lands) != 0 || len(delta.RemovedLandIDs) != 0 {
		t.Fatalf("expected empty countdown delta: %#v", delta)
	}
}

func TestAppLandDetailsDeltaPreservesSnapshotWhenBackgroundReadFails(t *testing.T) {
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFarmStatus": fakeRuntimeSequence{
			map[string]any{"farmType": "own", "grids": []any{map[string]any{
				"landId": 1, "plantName": "白萝卜", "seedId": 20002, "stageKind": "growing", "currentStage": 2, "matureInSec": 120,
			}}},
			os.ErrDeadlineExceeded,
		},
	})

	first := app.FarmLandDetails()
	delta := app.FarmLandDetailsSince(first.Revision)

	if delta.Full || delta.Revision != first.Revision || len(delta.Lands) != 0 || len(delta.RemovedLandIDs) != 0 {
		t.Fatalf("expected background read failure to preserve the previous snapshot, got %#v", delta)
	}
}

func TestAppLandDetailsDoesNotPublishOlderPollOverNewerRefresh(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	oldStatus := map[string]any{"farmType": "own", "grids": []any{map[string]any{
		"landId": 1, "plantName": "白萝卜", "seedId": 20002, "stageKind": "growing", "currentStage": 2,
	}}}
	newStatus := map[string]any{"farmType": "own", "grids": []any{map[string]any{
		"landId": 1, "plantName": "白萝卜", "seedId": 20002, "stageKind": "growing", "currentStage": 3,
	}}}
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFarmStatus": fakeRuntimeSequence{
			oldStatus,
			fakeRuntimeBlockingResponse{started: started, release: release, value: oldStatus},
			newStatus,
		},
	})

	first := app.FarmLandDetails()
	pollDone := make(chan struct{})
	go func() {
		app.FarmLandDetailsSince(first.Revision)
		close(pollDone)
	}()
	<-started
	refreshDone := make(chan struct{})
	go func() {
		app.FarmLandDetails()
		close(refreshDone)
	}()
	// Without transaction ordering, the refresh can publish stage 3 while the
	// earlier poll remains blocked, then the stale poll overwrites it on release.
	time.Sleep(50 * time.Millisecond)
	close(release)
	<-pollDone
	<-refreshDone

	if len(app.landDetailsSnapshot.Lands) != 1 || app.landDetailsSnapshot.Lands[0].CurrentStage != 3 {
		t.Fatalf("expected newest refresh to remain published, got %#v", app.landDetailsSnapshot)
	}
}

func TestAppLandDetailsReturnsChangedStage(t *testing.T) {
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFarmStatus": fakeRuntimeSequence{
			map[string]any{"farmType": "own", "grids": []any{map[string]any{
				"landId": 1, "plantName": "白萝卜", "seedId": 20002, "stageKind": "growing", "currentStage": 2, "matureInSec": 120,
			}}},
			map[string]any{"farmType": "own", "grids": []any{map[string]any{
				"landId": 1, "plantName": "白萝卜", "seedId": 20002, "stageKind": "growing", "currentStage": 3, "matureInSec": 115,
			}}},
		},
	})

	first := app.FarmLandDetails()
	delta := app.FarmLandDetailsSince(first.Revision)
	if delta.Full || len(delta.Lands) != 1 || delta.Lands[0].LandID != 1 || delta.Lands[0].CurrentStage != 3 {
		t.Fatalf("expected changed stage row: %#v", delta)
	}
	if delta.Lands[0].ImageURL == first.Lands[0].ImageURL || !strings.HasPrefix(delta.Lands[0].ImageURL, "/farm-assets/") {
		t.Fatalf("expected changed local stage image: first=%q delta=%q", first.Lands[0].ImageURL, delta.Lands[0].ImageURL)
	}
}

func TestAppExposesWarehouseGate(t *testing.T) {
	app := newAuthorizedTestApp(t)

	payload := app.FarmWarehouse()

	if payload.Status != "not_migrated" {
		t.Fatalf("unexpected warehouse payload %#v", payload)
	}
	if len(payload.Items) != 0 {
		t.Fatalf("expected no fake warehouse items, got %#v", payload.Items)
	}
}

func TestFarmWorkspaceRunStatisticsUsesCurrentSessionEventsAndCurrentRunSellRecords(t *testing.T) {
	ctx := context.Background()
	app, _ := newAppWithFakeRuntime(t, map[string]any{})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	startedAt := time.Now().Add(-2 * time.Minute)
	app.runStatisticsStartedAt = startedAt
	app.recordEventForAccount(app.accountKey(), eventbus.Event{Source: "auto_farm", Type: "task.done", Data: map[string]any{"taskId": "own_collect", "ok": true, "status": "ok", "actionCount": 1}})
	app.recordEventForAccount(app.accountKey(), eventbus.Event{Source: "auto_farm", Type: "task.done", Data: map[string]any{"taskId": "own_fertilizer", "ok": true, "status": "ok", "actionCount": 2}})
	app.recordEventForAccount(app.accountKey(), eventbus.Event{Source: "auto_farm", Type: "task.done", Data: map[string]any{"taskId": "friend_help", "ok": true, "status": "ok", "actionCount": 2}})
	app.recordEventForAccount("gid:other", eventbus.Event{Source: "auto_farm", Type: "task.done", Data: map[string]any{"taskId": "friend_steal", "ok": true, "status": "ok", "actionCount": 2}})

	today := time.Now().Local().Format("2006-01-02")
	if err := store.AppendWarehouseSellRecord(ctx, app.accountKey(), storage.WarehouseSellRecord{
		ID: "sale-before-run", DateKey: today, OccurredAt: startedAt.Add(-time.Second).Format(time.RFC3339Nano), TotalCount: 1, TotalAmount: 999,
	}); err != nil {
		t.Fatalf("append pre-run sell record: %v", err)
	}
	if err := store.AppendWarehouseSellRecord(ctx, app.accountKey(), storage.WarehouseSellRecord{
		ID: "sale-today-1", DateKey: today, OccurredAt: time.Now().Format(time.RFC3339Nano), TotalCount: 3, TotalAmount: 1200,
	}); err != nil {
		t.Fatalf("append today sell record: %v", err)
	}
	if err := store.AppendWarehouseSellRecord(ctx, app.accountKey(), storage.WarehouseSellRecord{
		ID: "sale-today-2", DateKey: today, OccurredAt: time.Now().Format(time.RFC3339Nano), TotalCount: 1, TotalAmount: 80,
	}); err != nil {
		t.Fatalf("append today sell record 2: %v", err)
	}
	if err := store.AppendWarehouseSellRecord(ctx, app.accountKey(), storage.WarehouseSellRecord{
		ID: "sale-yesterday", DateKey: time.Now().Local().AddDate(0, 0, -1).Format("2006-01-02"), OccurredAt: time.Now().Add(-24 * time.Hour).Format(time.RFC3339Nano), TotalCount: 9, TotalAmount: 9999,
	}); err != nil {
		t.Fatalf("append yesterday sell record: %v", err)
	}
	if err := store.AppendWarehouseSellRecord(ctx, "gid:other", storage.WarehouseSellRecord{
		ID: "sale-other", DateKey: today, OccurredAt: time.Now().Format(time.RFC3339Nano), TotalCount: 2, TotalAmount: 500,
	}); err != nil {
		t.Fatalf("append other account sell record: %v", err)
	}

	stats := app.FarmWorkspaceRunStatistics()

	if stats.Collect != 3 || stats.Help != 2 || stats.Steal != 0 {
		t.Fatalf("unexpected task statistics: %#v", stats)
	}
	if stats.DurationSeconds < 119 {
		t.Fatalf("duration = %d, want at least 119", stats.DurationSeconds)
	}
	if !stats.EstimateReady || stats.SaleEstimate != 1280 {
		t.Fatalf("unexpected sale estimate: %#v", stats)
	}
}

func TestAppWarehouseReadsRuntimeSnapshot(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.refreshWarehouseSnapshot": map[string]any{
			"ok": true,
			"items": []any{
				map[string]any{"itemId": float64(40002), "count": float64(5), "name": "白萝卜", "saleRewards": []any{map[string]any{"itemId": float64(1), "amount": float64(2)}}},
			},
		},
	})

	payload := app.FarmWarehouse()

	if payload.Status != "runtime" || len(payload.Items) != 1 {
		t.Fatalf("expected runtime warehouse items, got %#v", payload)
	}
	if payload.Items[0].ItemID != 40002 || payload.Items[0].Count != 5 || !payload.Items[0].CanSell {
		t.Fatalf("unexpected warehouse item %#v", payload.Items[0])
	}
	if payload.Items[0].ImageURL == "" {
		t.Fatalf("expected warehouse item image url, got %#v", payload.Items[0])
	}
	if !link.called("gameCtl.refreshWarehouseSnapshot") {
		t.Fatalf("expected warehouse runtime call, got %#v", link.calls)
	}
}

func TestAppWarehouseRefreshAndSellUseRuntime(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.refreshWarehouseSnapshot": map[string]any{
			"ok": true,
			"items": []any{
				map[string]any{"itemId": float64(40002), "count": float64(5), "name": "白萝卜", "warehouseKey": "40002:open:1", "saleRewards": []any{map[string]any{"itemId": float64(1), "amount": float64(2)}}},
			},
		},
		"gameCtl.sellWarehouseItems": map[string]any{
			"ok":             true,
			"observedChange": true,
			"beforeItems": []any{
				map[string]any{"itemId": float64(40002), "count": float64(5), "name": "白萝卜", "warehouseKey": "40002:open:1", "saleRewards": []any{map[string]any{"itemId": float64(1), "amount": float64(2)}}},
			},
			"afterItems": []any{},
		},
	})

	refreshed := app.FarmWarehouseRefresh()
	sold := app.FarmWarehouseSell(map[string]any{"itemIds": []any{float64(40002)}, "itemKeys": []any{"40002:open:1"}})

	if refreshed.Status != "runtime" || len(refreshed.Items) != 1 {
		t.Fatalf("expected refreshed warehouse payload, got %#v", refreshed)
	}
	if !sold.OK || sold.Warehouse.Status != "runtime" {
		t.Fatalf("expected successful sell payload, got %#v", sold)
	}
	if !link.called("gameCtl.refreshWarehouseSnapshot") || !link.called("gameCtl.sellWarehouseItems") {
		t.Fatalf("expected refresh and sell runtime calls, got %#v", link.calls)
	}
}

func TestAutoWarehouseSellTaskRefreshesFiltersAndStoresRecordForCapturedAccount(t *testing.T) {
	ctx := context.Background()
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.refreshWarehouseSnapshot": map[string]any{
			"ok": true,
			"items": []any{
				map[string]any{"itemId": float64(40002), "count": float64(5), "warehouseKey": "fruit:eligible", "saleUnitPrice": float64(2)},
				map[string]any{"itemId": float64(40002), "count": float64(3), "warehouseKey": "fruit:locked", "saleUnitPrice": float64(2), "locked": true},
				map[string]any{"itemId": float64(20002), "count": float64(4), "warehouseKey": "seed:other", "saleUnitPrice": float64(1)},
				map[string]any{"itemId": float64(40002), "count": float64(2), "warehouseKey": "fruit:unsellable"},
			},
		},
		"gameCtl.sellWarehouseItems": map[string]any{
			"ok": true,
			"soldDiff": []any{
				map[string]any{"itemId": float64(40002), "name": "白萝卜", "soldCount": float64(5), "saleUnitPrice": float64(2)},
			},
			"afterItems": []any{},
		},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if err := store.SaveWarehouseAutoSellSettingsForAccount(ctx, "gid:10001", storage.WarehouseAutoSellSettings{
		Enabled: true, IntervalMinute: 60, Categories: []string{"fruit"}, RefreshOnlyOnAutoSell: true,
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		t.Fatalf("confirm other account: %s", account.Error)
	}

	result := app.executeFarmAutomationTaskForAccount(ctx, "auto_warehouse_sell", "auto", "gid:10001")
	if !result.OK || result.TaskID != "auto_warehouse_sell" {
		t.Fatalf("auto sell result = %#v", result)
	}
	args := link.lastArgs("gameCtl.sellWarehouseItems")
	if len(args) != 1 {
		t.Fatalf("sell args = %#v", args)
	}
	options := args[0].(map[string]any)
	if !reflect.DeepEqual(options["itemKeys"], []string{"fruit:eligible"}) || options["mode"] != "auto" {
		t.Fatalf("sell options = %#v", options)
	}
	records, err := store.ListWarehouseSellRecords(ctx, "gid:10001", "", 10)
	if err != nil {
		t.Fatalf("list captured account records: %v", err)
	}
	if len(records) != 1 || records[0].Mode != "auto" {
		t.Fatalf("captured account records = %#v", records)
	}
	otherRecords, err := store.ListWarehouseSellRecords(ctx, "gid:10002", "", 10)
	if err != nil {
		t.Fatalf("list current account records: %v", err)
	}
	if len(otherRecords) != 0 {
		t.Fatalf("auto sell record leaked to current account: %#v", otherRecords)
	}
}

func TestAutoWarehouseSellTaskSkipsSellWhenNoEligibleItems(t *testing.T) {
	ctx := context.Background()
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.refreshWarehouseSnapshot": map[string]any{
			"ok": true,
			"items": []any{
				map[string]any{"itemId": float64(40002), "count": float64(5), "warehouseKey": "fruit:locked", "saleUnitPrice": float64(2), "locked": true},
			},
		},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if err := store.SaveWarehouseAutoSellSettingsForAccount(ctx, "gid:10001", storage.WarehouseAutoSellSettings{
		Enabled: true, IntervalMinute: 60, Categories: []string{"fruit"}, RefreshOnlyOnAutoSell: true,
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	result := app.executeFarmAutomationTaskForAccount(ctx, "auto_warehouse_sell", "auto", "gid:10001")
	if !result.OK || !strings.Contains(result.Message, "没有可出售") {
		t.Fatalf("auto sell skip result = %#v", result)
	}
	if link.called("gameCtl.sellWarehouseItems") {
		t.Fatalf("sell should not run: %#v", link.calls)
	}
}

func TestAppWarehouseRefreshPassesProtocolPreference(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.refreshWarehouseSnapshot": map[string]any{
			"ok":     true,
			"source": "protocol",
			"items": []any{
				map[string]any{
					"itemId":        float64(40002),
					"count":         float64(5),
					"name":          "白萝卜",
					"warehouseKey":  "40002:open:123456",
					"saleUnitPrice": float64(2),
					"uid":           float64(123456),
				},
			},
			"originalItems": []any{
				map[string]any{"id": float64(40002), "count": float64(5), "uid": float64(123456)},
			},
		},
	})

	payload := app.FarmWarehouseRefresh()

	if payload.Status != "runtime" || len(payload.Items) != 1 {
		t.Fatalf("expected protocol warehouse payload, got %#v", payload)
	}
	if payload.Items[0].ID != "40002:open:123456" || !payload.Items[0].CanSell {
		t.Fatalf("unexpected protocol item %#v", payload.Items[0])
	}
	args := link.lastArgs("gameCtl.refreshWarehouseSnapshot")
	if len(args) != 1 {
		t.Fatalf("expected one refresh arg payload, got %#v", args)
	}
	options := args[0].(map[string]any)
	if options["preferProtocol"] != true || options["protocolWaitMs"] != 1200 {
		t.Fatalf("refresh should prefer protocol, got %#v", options)
	}
}

func TestAppWarehouseSellPassesProtocolPreference(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.sellWarehouseItems": map[string]any{
			"ok":              true,
			"source":          "protocol",
			"requestPayload":  []any{map[string]any{"id": float64(40002), "count": float64(5), "uid": float64(123456)}},
			"protocolRewards": []any{map[string]any{"itemId": float64(1001), "amount": float64(10)}},
			"afterItems":      []any{},
		},
	})

	sold := app.FarmWarehouseSell(map[string]any{"itemKeys": []any{"40002:open:123456"}})

	if !sold.OK || sold.Sell["source"] != "protocol" {
		t.Fatalf("expected protocol sell payload, got %#v", sold)
	}
	args := link.lastArgs("gameCtl.sellWarehouseItems")
	if len(args) != 1 {
		t.Fatalf("expected one sell arg payload, got %#v", args)
	}
	options := args[0].(map[string]any)
	if options["preferProtocol"] != true || options["protocolWaitMs"] != 1200 {
		t.Fatalf("sell should prefer protocol, got %#v", options)
	}
}

func TestAppWarehouseSellStoresRecordForCurrentAccount(t *testing.T) {
	ctx := context.Background()
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.sellWarehouseItems": map[string]any{
			"ok": true,
			"soldDiff": []any{
				map[string]any{
					"itemId":        float64(41221),
					"name":          "青梅",
					"soldCount":     float64(3),
					"saleUnitPrice": float64(240),
				},
			},
			"afterItems": []any{},
		},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	app.ConfirmRuntimeAccount(RuntimeAccount{GID: 123456, Nickname: "Dpo.L"})

	sold := app.FarmWarehouseSell(map[string]any{
		"itemKeys": []any{"41221:open:9001"},
		"mode":     "auto",
	})

	if !sold.OK {
		t.Fatalf("expected successful sell payload, got %#v", sold)
	}
	records, err := store.ListWarehouseSellRecords(ctx, "gid:123456", "", 10)
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one sell record, got %#v", records)
	}
	record := records[0]
	if record.Mode != "auto" || record.TotalCount != 3 || record.TotalAmount != 720 || record.ItemKinds != 1 {
		t.Fatalf("unexpected record summary: %#v", record)
	}
	if len(record.Items) != 1 || record.Items[0].Name != "青梅" || record.Items[0].Amount != 720 {
		t.Fatalf("unexpected record items: %#v", record.Items)
	}
	listed := app.FarmWarehouseSellRecords(map[string]any{"date": record.DateKey})
	if len(listed) != 1 || listed[0].ID != record.ID {
		t.Fatalf("app record query should filter by date, got %#v", listed)
	}
}

func TestFarmMysteryShopPurchaseRecordsUseCurrentAccount(t *testing.T) {
	ctx := context.Background()
	app, _ := newAppWithFakeRuntime(t, map[string]any{})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001, Nickname: "账号一"}); !account.Confirmed {
		t.Fatalf("confirm first account: %#v", account)
	}
	if err := app.store.AppendMysteryShopPurchaseRecord(ctx, "gid:10001", storage.MysteryShopPurchaseRecord{
		ID: "first", OccurredAt: "2026-07-26T10:00:00Z", ItemName: "高级化肥", CurrencyName: "金豆豆",
	}); err != nil {
		t.Fatalf("append first account record: %v", err)
	}
	if err := app.store.AppendMysteryShopPurchaseRecord(ctx, "gid:10002", storage.MysteryShopPurchaseRecord{
		ID: "other", OccurredAt: "2026-07-26T11:00:00Z", ItemName: "其他商品",
	}); err != nil {
		t.Fatalf("append second account record: %v", err)
	}

	records := app.FarmMysteryShopPurchaseRecords()
	if len(records) != 1 || records[0].ID != "first" {
		t.Fatalf("records for current account = %#v", records)
	}
}

func TestAppWarehouseSellStoresRecordForAccountCapturedAtEntry(t *testing.T) {
	ctx := context.Background()
	started := make(chan struct{})
	release := make(chan struct{})
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.sellWarehouseItems": fakeRuntimeBlockingResponse{
			started: started,
			release: release,
			value: map[string]any{
				"ok": true,
				"soldDiff": []any{
					map[string]any{"itemId": float64(41221), "name": "青梅", "soldCount": float64(3), "saleUnitPrice": float64(240)},
				},
				"afterItems": []any{},
			},
		},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001}); account.Error != "" {
		t.Fatalf("confirm account A: %s", account.Error)
	}

	done := make(chan farm.WarehouseSellPayload, 1)
	go func() {
		done <- app.FarmWarehouseSell(map[string]any{"itemKeys": []any{"41221:open:9001"}})
	}()
	<-started
	if account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002}); account.Error != "" {
		t.Fatalf("confirm account B: %s", account.Error)
	}
	close(release)
	if sold := <-done; !sold.OK {
		t.Fatalf("expected successful sell payload, got %#v", sold)
	}

	accountA, err := store.ListWarehouseSellRecords(ctx, "gid:10001", "", 10)
	if err != nil {
		t.Fatalf("list account A records: %v", err)
	}
	accountB, err := store.ListWarehouseSellRecords(ctx, "gid:10002", "", 10)
	if err != nil {
		t.Fatalf("list account B records: %v", err)
	}
	if len(accountA) != 1 || len(accountB) != 0 {
		t.Fatalf("sell record followed switched account: accountA=%#v accountB=%#v", accountA, accountB)
	}
}

func TestAppAccountStatusReadsRuntimeProfileAndFertilizer(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getPlayerProfile": map[string]any{
			"name":         "Dpo.L",
			"level":        float64(118),
			"gold":         float64(3780797635),
			"bean":         float64(446032),
			"coupon":       float64(10104),
			"diamond":      float64(250),
			"exp":          float64(108855),
			"nextLevelExp": float64(823800),
		},
		"gameCtl.getFertilizerContainerStatus": map[string]any{
			"normal":  map[string]any{"available": false, "remainingSec": float64(0)},
			"organic": map[string]any{"available": true, "remainingSec": float64(382320)},
		},
	})

	payload := app.FarmAccountStatus()

	if payload.Profile.Name != "Dpo.L" || payload.Profile.Level != 118 {
		t.Fatalf("unexpected account profile %#v", payload.Profile)
	}
	if payload.Profile.LevelProgress.Current != 108855 || payload.Profile.LevelProgress.Needed != 823800 {
		t.Fatalf("unexpected level progress %#v", payload.Profile.LevelProgress)
	}
	if payload.Fertilizer.Organic.RemainingHours != 106.2 {
		t.Fatalf("unexpected fertilizer status %#v", payload.Fertilizer)
	}
	if !link.called("gameCtl.getPlayerProfile") || !link.called("gameCtl.getFertilizerContainerStatus") {
		t.Fatalf("expected account runtime calls, got %#v", link.calls)
	}
}

func TestAppAccountStatusAggregatesSuccessfulAutomationStats(t *testing.T) {
	ctx := context.Background()
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getPlayerProfile": map[string]any{"name": "Dpo.L", "level": float64(118)},
		"gameCtl.getFertilizerContainerStatus": map[string]any{
			"normal":  map[string]any{"available": true},
			"organic": map[string]any{"available": true},
		},
	})
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	now := time.Now()
	for _, event := range []eventbus.Event{
		successAutomationEvent(now, "own_base", "manual", 1),
		successAutomationEvent(now, "own_collect", "auto", 1),
		successAutomationEvent(now, "own_fertilizer", "auto", 2),
		successAutomationEvent(now, "friend_steal", "auto", 2),
		successAutomationEvent(now, "friend_help", "manual", 3),
		successAutomationEvent(now, "friend_mischief", "auto", 1),
		successAutomationEvent(now, "own_collect", "auto", 0),
		{Timestamp: now, Level: eventbus.LevelInfo, Source: "auto_farm", Type: "task.done", Data: map[string]any{"taskId": "own_collect", "trigger": "auto", "ok": true, "status": string(automation.StatusOK)}},
		{
			Timestamp: now,
			Level:     eventbus.LevelWarn,
			Source:    "auto_farm",
			Type:      "task.failed",
			Message:   "automation task failed",
			Data: map[string]any{
				"taskId":  "own_collect",
				"trigger": "manual",
				"ok":      false,
				"status":  string(automation.StatusFailed),
			},
		},
		{
			Timestamp: now,
			Level:     eventbus.LevelInfo,
			Source:    "auto_farm",
			Type:      "task.done",
			Message:   "automation task skipped",
			Data: map[string]any{
				"taskId":  "own_collect",
				"trigger": "auto",
				"ok":      true,
				"status":  string(automation.StatusRuntimeNotReady),
			},
		},
	} {
		app.recordEvent(event)
	}

	payload := app.FarmAccountStatus()

	if payload.Profile.TodayStats.Runs != 8 {
		t.Fatalf("runs = %d, want 8", payload.Profile.TodayStats.Runs)
	}
	if payload.Profile.TodayStats.Collect != 3 || payload.Profile.TodayStats.Water != 1 {
		t.Fatalf("unexpected own stats %#v", payload.Profile.TodayStats)
	}
	if payload.Profile.TodayStats.Steal != 2 || payload.Profile.TodayStats.Help != 3 {
		t.Fatalf("unexpected friend stats %#v", payload.Profile.TodayStats)
	}
	if payload.Profile.TodayStats.MischiefGrass+payload.Profile.TodayStats.MischiefBug != 1 {
		t.Fatalf("unexpected mischief stats %#v", payload.Profile.TodayStats)
	}
	if payload.Profile.StatsHistory.TodayKey != now.Format("2006-01-02") || len(payload.Profile.StatsHistory.Days) == 0 {
		t.Fatalf("unexpected stats history %#v", payload.Profile.StatsHistory)
	}
}

func TestAutomationStatsHistoryIncludesDailySaleEstimates(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.Local)
	for _, record := range []storage.WarehouseSellRecord{
		{ID: "today", DateKey: "2026-07-23", OccurredAt: "2026-07-23T10:00:00+08:00", TotalAmount: 500},
		{ID: "previous", DateKey: "2026-07-22", OccurredAt: "2026-07-22T10:00:00+08:00", TotalAmount: 300},
	} {
		if err := store.AppendWarehouseSellRecord(ctx, app.accountKey(), record); err != nil {
			t.Fatalf("append sale record: %v", err)
		}
	}

	history := app.automationStatsHistoryForAccount(ctx, app.accountKey(), now, 3)
	if len(history.Days) != 3 {
		t.Fatalf("expected one entry per calendar day, got %#v", history.Days)
	}
	want := map[string]int64{"2026-07-21": 0, "2026-07-22": 300, "2026-07-23": 500}
	for _, day := range history.Days {
		if !day.EstimateReady || day.SaleEstimate != want[day.DateKey] {
			t.Fatalf("unexpected estimate for %s: %#v", day.DateKey, day)
		}
	}
}

func successAutomationEvent(ts time.Time, taskID string, trigger string, actionCount int) eventbus.Event {
	return eventbus.Event{
		Timestamp: ts,
		Level:     eventbus.LevelInfo,
		Source:    "auto_farm",
		Type:      "task.done",
		Message:   "automation task completed",
		Data: map[string]any{
			"taskId":      taskID,
			"trigger":     trigger,
			"ok":          true,
			"status":      string(automation.StatusOK),
			"actionCount": actionCount,
		},
	}
}

func TestAppAccountStatusSanitizesGameCtlNotReadyErrors(t *testing.T) {
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getPlayerProfile":             errors.New("Runtime.evaluate failed: map[description:Error: gameCtl_not_ready stackTrace:...]"),
		"gameCtl.getFertilizerContainerStatus": errors.New("Runtime.evaluate failed: map[description:Error: gameCtl_not_ready stackTrace:...]"),
	})

	payload := app.FarmAccountStatus()

	if payload.ProfileError != "游戏运行时尚未就绪" || payload.FertilizerError != "游戏运行时尚未就绪" {
		t.Fatalf("expected sanitized account errors, got %#v", payload)
	}
}

func TestIdentifyRuntimeAccountReadsRuntimeProfile(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getRuntimeAccountIdentity": map[string]any{
			"gid":       float64(123456),
			"nickname":  "Dpo.L",
			"avatarUrl": "https://example.test/avatar.png",
		},
	})

	account := app.IdentifyRuntimeAccount()

	if account.GID != 123456 || account.AccountKey != "gid:123456" {
		t.Fatalf("unexpected account identity: %#v", account)
	}
	if account.Nickname != "Dpo.L" || account.AvatarURL != "https://example.test/avatar.png" {
		t.Fatalf("unexpected account profile fields: %#v", account)
	}
	if account.Confirmed {
		t.Fatalf("identified account should wait for user confirmation: %#v", account)
	}
	if !link.called("gameCtl.getRuntimeAccountIdentity") {
		t.Fatalf("expected runtime profile call, got %#v", link.calls)
	}
}

func TestIdentifyRuntimeAccountFallsBackToSelfGIDWhenProfileIsUnavailable(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getRuntimeAccountIdentity": errors.New("call_path_not_ready: gameCtl.getRuntimeAccountIdentity"),
		"gameCtl.getSelfGid":                float64(123456),
	})

	account := app.IdentifyRuntimeAccount()

	if account.GID != 123456 || account.AccountKey != "gid:123456" {
		t.Fatalf("expected fallback self gid to identify account, got %#v", account)
	}
	if !link.called("gameCtl.getRuntimeAccountIdentity") || !link.called("gameCtl.getSelfGid") {
		t.Fatalf("expected identity and self gid calls, got %#v", link.calls)
	}
	if link.called("gameCtl.getPlayerProfile") {
		t.Fatalf("fallback self gid should not require full profile call, got %#v", link.calls)
	}
}

func TestIdentifyRuntimeAccountRetriesUntilRuntimeProfileHasGID(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getRuntimeAccountIdentity": fakeRuntimeSequence{
			map[string]any{
				"name": "Dpo.L",
			},
			map[string]any{
				"gid":      float64(123456),
				"nickname": "Dpo.L",
			},
		},
	})

	account := app.IdentifyRuntimeAccount()

	if account.GID != 123456 || account.AccountKey != "gid:123456" {
		t.Fatalf("expected retry to identify account gid, got %#v", account)
	}
	if got := link.callCount("gameCtl.getRuntimeAccountIdentity"); got != 2 {
		t.Fatalf("expected two profile attempts, got %d (%#v)", got, link.calls)
	}
}

func TestConfirmRuntimeAccountSwitchesStorageScope(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	account := app.ConfirmRuntimeAccount(RuntimeAccount{
		GID:       123456,
		Nickname:  "Dpo.L",
		AvatarURL: "https://example.test/avatar.png",
	})

	if !account.Confirmed || account.AccountKey != "gid:123456" {
		t.Fatalf("expected confirmed gid scope, got %#v", account)
	}
	if app.accountKey() != "gid:123456" {
		t.Fatalf("app did not switch account scope: %q", app.accountKey())
	}
	loaded, err := store.LoadRuntimeAccount(ctx, "gid:123456")
	if err != nil {
		t.Fatalf("load runtime account: %v", err)
	}
	if loaded.GID != 123456 || loaded.Nickname != "Dpo.L" {
		t.Fatalf("confirmed account was not persisted: %#v", loaded)
	}
}

func TestConfirmRuntimeAccountSeedsLegacyBusinessConfiguration(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	legacy := storage.WarehouseAutoSellSettings{
		Enabled:               true,
		IntervalMinute:        120,
		Categories:            []string{"seed"},
		RefreshOnlyOnAutoSell: true,
	}
	if err := store.SaveWarehouseAutoSellSettingsForAccount(ctx, storage.GlobalSettingsAccountKey, legacy); err != nil {
		t.Fatalf("seed legacy warehouse: %v", err)
	}

	account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001, Nickname: "A"})
	if account.Error != "" {
		t.Fatalf("confirm: %#v", account)
	}
	settings := app.RuntimeSettings()
	if !settings.AutoWarehouseSellEnabled ||
		settings.AutoWarehouseSellIntervalMinute != 120 ||
		!reflect.DeepEqual(settings.AutoWarehouseSellCategories, []string{"seed"}) {
		t.Fatalf("legacy preferences were not seeded: %#v", settings)
	}

	if _, err := app.SaveWarehouseAutoSellSettings(storage.WarehouseAutoSellSettings{
		Enabled:        true,
		IntervalMinute: 540,
		Categories:     []string{"seed"},
	}); err != nil {
		t.Fatalf("save gid warehouse settings: %v", err)
	}
	account = app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001, Nickname: "A"})
	if account.Error != "" {
		t.Fatalf("repeat confirm: %#v", account)
	}
	if got := app.RuntimeSettings().AutoWarehouseSellIntervalMinute; got != 540 {
		t.Fatalf("repeat confirmation overwrote gid settings: %d", got)
	}
}

func TestConfirmRuntimeAccountSeedFailureKeepsPreviousAccountActive(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	previous := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001, Nickname: "A"})
	if previous.Error != "" {
		t.Fatalf("confirm previous account: %#v", previous)
	}
	rawDB, err := sql.Open("sqlite", store.Path())
	if err != nil {
		t.Fatalf("open raw database: %v", err)
	}
	defer rawDB.Close()
	if _, err := rawDB.ExecContext(ctx, `DROP TABLE settings`); err != nil {
		t.Fatalf("drop settings table: %v", err)
	}

	failed := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002, Nickname: "B"})
	if failed.Error == "" {
		t.Fatalf("expected seed failure, got %#v", failed)
	}
	if failed.Confirmed {
		t.Fatalf("failed account must not be confirmed: %#v", failed)
	}
	if got := app.accountKey(); got != previous.AccountKey {
		t.Fatalf("active account key changed after seed failure: %q, want %q", got, previous.AccountKey)
	}
	if got := app.CurrentRuntimeAccount(); !reflect.DeepEqual(got, previous) {
		t.Fatalf("current runtime account changed after seed failure: %#v, want %#v", got, previous)
	}
}

func TestConfirmRuntimeAccountSaveFailureKeepsPreviousAccountActive(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app.store = store
	previous := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001, Nickname: "A"})
	if previous.Error != "" {
		t.Fatalf("confirm previous account: %#v", previous)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	failed := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10002, Nickname: "B"})
	if failed.Error == "" || failed.Confirmed {
		t.Fatalf("expected unconfirmed save failure, got %#v", failed)
	}
	if got := app.accountKey(); got != previous.AccountKey {
		t.Fatalf("active account key changed after save failure: %q, want %q", got, previous.AccountKey)
	}
	if got := app.CurrentRuntimeAccount(); !reflect.DeepEqual(got, previous) {
		t.Fatalf("current runtime account changed after save failure: %#v, want %#v", got, previous)
	}
}

func containsRuntimeEventType(events []eventbus.Event, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func TestEmbeddedFridaResourcesPreserveSourceBytes(t *testing.T) {
	root := filepath.Join("resources", "wmpf", "frida")
	err := filepath.WalkDir(root, func(sourcePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		want, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(sourcePath)
		got, err := embeddedFridaResources.ReadFile(name)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("embedded resource %s differs from its source bytes", name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("verify embedded Frida resources: %v", err)
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

type fakeRuntimeLink struct {
	target    farmruntime.RuntimeTarget
	status    farmruntime.Status
	responses map[string]any
	calls     []string
	callArgs  map[string][][]any
	stopCount int
}

type fakeRuntimeSequence []any

type fakeRuntimeBlockingResponse struct {
	started chan struct{}
	release chan struct{}
	value   any
}

func newAppWithFakeRuntime(t *testing.T, responses map[string]any) (*App, *fakeRuntimeLink) {
	t.Helper()
	app := newAuthorizedTestApp(t)
	link := &fakeRuntimeLink{
		target: farmruntime.RuntimeTargetQQWS,
		status: farmruntime.Status{
			Target:    string(farmruntime.RuntimeTargetQQWS),
			Phase:     farmruntime.PhaseReady,
			Connected: true,
			Ready:     true,
		},
		responses: responses,
	}
	app.supervisor = farmruntime.NewSupervisor(app.manager, map[farmruntime.RuntimeTarget]farmruntime.RuntimeLink{
		farmruntime.RuntimeTargetQQWS: link,
	})
	if err := app.supervisor.Switch(context.Background(), farmruntime.RuntimeTargetQQWS); err != nil {
		t.Fatalf("switch fake runtime: %v", err)
	}
	return app, link
}

func (l *fakeRuntimeLink) Target() farmruntime.RuntimeTarget { return l.target }
func (l *fakeRuntimeLink) Start(context.Context) error       { return nil }
func (l *fakeRuntimeLink) Stop(context.Context) error {
	l.stopCount++
	return nil
}
func (l *fakeRuntimeLink) Status() farmruntime.Status { return l.status }
func (l *fakeRuntimeLink) Call(_ context.Context, method string, args []any, _ time.Duration) (any, error) {
	l.calls = append(l.calls, method)
	if l.callArgs == nil {
		l.callArgs = map[string][][]any{}
	}
	l.callArgs[method] = append(l.callArgs[method], args)
	if value, ok := l.responses[method]; ok {
		if response, ok := value.(fakeRuntimeBlockingResponse); ok {
			close(response.started)
			<-response.release
			return response.value, nil
		}
		if sequence, ok := value.(fakeRuntimeSequence); ok {
			if len(sequence) == 0 {
				return nil, os.ErrNotExist
			}
			next := sequence[0]
			l.responses[method] = sequence[1:]
			if response, ok := next.(fakeRuntimeBlockingResponse); ok {
				close(response.started)
				<-response.release
				return response.value, nil
			}
			if err, ok := next.(error); ok {
				return nil, err
			}
			return next, nil
		}
		if err, ok := value.(error); ok {
			return nil, err
		}
		return value, nil
	}
	return nil, os.ErrNotExist
}
func (l *fakeRuntimeLink) called(method string) bool {
	for _, call := range l.calls {
		if call == method {
			return true
		}
	}
	return false
}
func (l *fakeRuntimeLink) callCount(method string) int {
	count := 0
	for _, call := range l.calls {
		if call == method {
			count++
		}
	}
	return count
}
func (l *fakeRuntimeLink) lastArgs(method string) []any {
	calls := l.callArgs[method]
	if len(calls) == 0 {
		return nil
	}
	return calls[len(calls)-1]
}

func findAppAtlasSection(sections []farm.AtlasSection, id string) *farm.AtlasSection {
	for i := range sections {
		if sections[i].ID == id {
			return &sections[i]
		}
	}
	return nil
}

func findAutomationTask(tasks []automation.SchedulerTask, id string) *automation.SchedulerTask {
	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i]
		}
	}
	return nil
}

func findAutoFarmTaskSettings(tasks []storage.AutoFarmTaskSettings, id string) *storage.AutoFarmTaskSettings {
	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i]
		}
	}
	return nil
}
