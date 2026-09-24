package automation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRuntimeFacadeRunsOwnBaseThroughRuntimeActions(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"workLandIds": map[string]any{
				"collect":    []any{float64(3), "1"},
				"water":      []any{float64(4)},
				"eraseGrass": []any{},
				"killBug":    []any{float64(5)},
			},
		},
		"gameCtl.triggerOneClickOperation": map[string]any{"ok": true, "type": "FARM_WORK"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_base")

	if !result.OK {
		t.Fatalf("own_base should report OK after real runtime actions: %#v", result)
	}
	if result.Status != StatusOK {
		t.Fatalf("status = %q, want %q", result.Status, StatusOK)
	}
	if result.TaskID != "own_base" {
		t.Fatalf("taskID = %q, want own_base", result.TaskID)
	}
	if result.ActionCount != 1 {
		t.Fatalf("action count = %d, want 1", result.ActionCount)
	}
	wantMethods := []string{
		"gameCtl.getFarmOwnership",
		"gameCtl.getFarmStatus",
		"gameCtl.triggerOneClickOperation",
	}
	if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
		t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
	}
	wantCareArgs := []any{"FARM_WORK", map[string]any{
		"silent":        true,
		"includeBefore": false,
		"includeAfter":  false,
		"source":        "farm_go_auto_base",
	}}
	if !reflect.DeepEqual(caller.calls[2].args, wantCareArgs) {
		t.Fatalf("care args = %#v, want %#v", caller.calls[2].args, wantCareArgs)
	}
	if result.Message != "一键务农已执行：照料 2 项。" {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestRuntimeFacadeRunsOwnBaseFromRuntimeLandIds(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"landIds": map[string]any{
				"collect":    []any{float64(9)},
				"water":      []any{float64(4)},
				"eraseGrass": []any{float64(5)},
				"killBug":    []any{},
			},
			"workCounts": map[string]any{
				"collect":    float64(1),
				"water":      float64(1),
				"eraseGrass": float64(1),
				"killBug":    float64(0),
			},
		},
		"gameCtl.triggerOneClickOperation": map[string]any{"ok": true, "type": "FARM_WORK"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_base")

	if !result.OK {
		t.Fatalf("own_base should report OK from runtime landIds: %#v", result)
	}
	wantMethods := []string{
		"gameCtl.getFarmOwnership",
		"gameCtl.getFarmStatus",
		"gameCtl.triggerOneClickOperation",
	}
	if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
		t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
	}
	if strings.Contains(result.Message, "收获") {
		t.Fatalf("one-click farming message must not contain harvest statistics: %q", result.Message)
	}
}

func TestRuntimeFacadeOwnBaseUsesOneClickForGoldenBugs(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "stageKind": "growing", "needGoldenBug": true},
				map[string]any{"landId": float64(3), "stageKind": "growing", "socialItemIds": []any{float64(301101)}},
			},
		},
		"gameCtl.triggerOneClickOperation": map[string]any{"ok": true, "type": "FARM_WORK"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_base")

	if !result.OK {
		t.Fatalf("own_base should report OK after one-click golden bug care: %#v", result)
	}
	if result.Message != "一键务农已执行：照料 2 项。" {
		t.Fatalf("message = %q", result.Message)
	}
	wantMethods := []string{
		"gameCtl.getFarmOwnership",
		"gameCtl.getFarmStatus",
		"gameCtl.triggerOneClickOperation",
	}
	if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", got, wantMethods)
	}
}

func TestRuntimeFacadeReturnsRuntimeNotReadyWhenOwnBaseCallFails(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": errors.New("runtime link is not connected"),
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_base")

	if result.OK {
		t.Fatalf("disconnected runtime should not be OK: %#v", result)
	}
	if result.Status != StatusRuntimeNotReady {
		t.Fatalf("status = %q, want %q", result.Status, StatusRuntimeNotReady)
	}
	if result.TaskID != "own_base" {
		t.Fatalf("taskID = %q, want own_base", result.TaskID)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("runtime call count = %d, want 1", len(caller.calls))
	}
}

func TestRuntimeFacadeOwnBaseReportsRuntimeNotOKResult(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType":    "own",
			"workLandIds": map[string]any{"water": []any{float64(4)}},
		},
		"gameCtl.triggerOneClickOperation": map[string]any{"ok": false, "reason": "one_click_missing"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_base")

	if result.OK {
		t.Fatalf("explicit own_base runtime ok=false should not be OK: %#v", result)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, StatusFailed)
	}
}

func TestRuntimeFacadeOwnCollectHarvestsCollectableOwnFarmLands(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"workLandIds": map[string]any{
				"collect": []any{float64(3), float64(1), float64(3), "2"},
			},
			"grids": []any{
				map[string]any{"landId": float64(4), "canHarvest": true},
				map[string]any{"landId": float64(5), "canCollect": true},
				map[string]any{"landId": float64(6), "canHarvest": false},
			},
		},
		"gameCtl.harvestLandsBatchByProtocol": map[string]any{
			"ok": true,
			"results": []any{
				map[string]any{"ok": true, "landId": float64(1)},
				map[string]any{"ok": true, "landId": float64(2)},
				map[string]any{"ok": true, "landId": float64(2)},
				map[string]any{"ok": false, "landId": float64(3)},
			},
		},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_collect")

	if !result.OK {
		t.Fatalf("own_collect should report OK on successful harvest call: %#v", result)
	}
	if result.Status != StatusOK {
		t.Fatalf("status = %q, want %q", result.Status, StatusOK)
	}
	if result.TaskID != "own_collect" {
		t.Fatalf("taskID = %q, want own_collect", result.TaskID)
	}
	wantMethods := []string{
		"gameCtl.getFarmStatus",
		"gameCtl.harvestLandsBatchByProtocol",
		"gameCtl.getFarmStatus",
		"gameCtl.getFarmStatus",
	}
	if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
		t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
	}
	harvest := caller.calls[1]
	wantArgs := []any{[]int{1, 2, 3, 4, 5}, map[string]any{
		"silent":        true,
		"closeAfter":    true,
		"waitReplyMs":   600,
		"source":        "farm_go_auto_collect",
		"betweenLandMs": 120,
	}}
	if !reflect.DeepEqual(harvest.args, wantArgs) {
		t.Fatalf("harvest args = %#v, want %#v", harvest.args, wantArgs)
	}
	wantRescanArgs := []any{map[string]any{
		"includeGrids":          true,
		"includeLandIds":        true,
		"includeRawGrid":        true,
		"includeRawLandRuntime": true,
		"silent":                true,
		"forceRefresh":          true,
	}}
	if !reflect.DeepEqual(caller.calls[2].args, wantRescanArgs) {
		t.Fatalf("first post-harvest status args = %#v, want %#v", caller.calls[2].args, wantRescanArgs)
	}
	if !reflect.DeepEqual(caller.calls[3].args, wantRescanArgs) {
		t.Fatalf("second post-harvest status args = %#v, want %#v", caller.calls[3].args, wantRescanArgs)
	}
	if result.Message != "一键收获 2 块，清理枯萎 0 块。" {
		t.Fatalf("message = %q", result.Message)
	}
	if result.ActionCount != 1 {
		t.Fatalf("action count = %d, want 1", result.ActionCount)
	}
}

func TestRuntimeFacadeOwnCollectSkipsWhenNoCollectableLands(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"workLandIds": map[string]any{
				"collect": []any{},
			},
			"grids": []any{map[string]any{"landId": float64(7), "canHarvest": false}},
		},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_collect")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("no collectable lands should be an OK skip, got %#v", result)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("no-op collect should not call harvest, got %#v", caller.calls)
	}
	if result.Message == "" {
		t.Fatalf("expected skip message, got %#v", result)
	}
}

func TestRuntimeFacadeOwnCollectFertilizesMultiSeasonAfterDeadCleanup(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": fakeRuntimeResponseSequence{
			map[string]any{
				"farmType": "own",
				"workLandIds": map[string]any{
					"collect": []any{float64(5)},
				},
			},
			map[string]any{
				"farmType": "own",
				"grids": []any{
					map[string]any{"landId": float64(5), "hasPlant": true, "stageKind": "growing", "isMultiSeason": true, "currentSeason": float64(2), "totalSeason": float64(3)},
					map[string]any{"landId": float64(9), "stageKind": "dead"},
				},
			},
		},
		"gameCtl.harvestLandsBatchByProtocol": map[string]any{
			"ok": true,
			"results": []any{
				map[string]any{"ok": true, "landId": float64(5)},
			},
		},
		"gameCtl.shovelLandsBatch": map[string]any{"ok": true, "successCount": float64(1)},
		"gameCtl.fertilizeLandsBatch": map[string]any{
			"ok":           true,
			"successCount": float64(1),
		},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":     true,
		"autoFarmFertilizerMultiSeason": true,
		"autoFarmPlantFertilizerMode":   "normal",
	})

	result := facade.RunTask(context.Background(), "own_collect")

	if !result.OK {
		t.Fatalf("own_collect should complete post-harvest cleanup and fertilizer: %#v", result)
	}
	wantMethods := []string{
		"gameCtl.getFarmStatus",
		"gameCtl.harvestLandsBatchByProtocol",
		"gameCtl.getFarmStatus",
		"gameCtl.shovelLandsBatch",
		"gameCtl.fertilizeLandsBatch",
	}
	if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", got, wantMethods)
	}
	fertilizer := caller.calls[4]
	wantArgs := []any{map[string]any{
		"landIds":                     []int{5},
		"type":                        "normal",
		"mode":                        "normal",
		"dryRun":                      false,
		"cleanupUi":                   true,
		"linkedHarvestAfterFertilize": false,
		"silent":                      true,
		"source":                      "farm_go_auto_collect_multi_season",
	}}
	if !reflect.DeepEqual(fertilizer.args, wantArgs) {
		t.Fatalf("multi-season fertilizer args = %#v, want %#v", fertilizer.args, wantArgs)
	}
}

func TestRuntimeFacadeOwnCollectRequiresOwnFarmStatus(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{"farmType": "friend"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_collect")

	if result.OK {
		t.Fatalf("friend farm should not be harvested by own_collect: %#v", result)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, StatusFailed)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("friend farm should not call harvest, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeOwnCollectReportsHarvestFailure(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"workLandIds": map[string]any{
				"collect": []any{float64(8)},
			},
		},
		"gameCtl.harvestLandsBatchByProtocol": errors.New("harvest method not ready"),
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_collect")

	if result.OK {
		t.Fatalf("harvest failure should not be OK: %#v", result)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, StatusFailed)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("expected status and harvest calls, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeOwnCollectReportsRuntimeNotOKResult(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType":    "own",
			"workLandIds": map[string]any{"collect": []any{float64(8)}},
		},
		"gameCtl.harvestLandsBatchByProtocol": map[string]any{"ok": false, "reason": "bag_full"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_collect")

	if result.OK {
		t.Fatalf("explicit harvest ok=false should not be OK: %#v", result)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, StatusFailed)
	}
}

func TestRuntimeFacadeOwnCollectCleansDeadLandsAfterHarvest(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": fakeRuntimeResponseSequence{
			map[string]any{
				"farmType":    "own",
				"workLandIds": map[string]any{"collect": []any{float64(8), float64(10)}},
			},
			map[string]any{
				"farmType": "own",
				"grids": []any{
					map[string]any{"landId": float64(8), "stageKind": "dead"},
					map[string]any{"landId": float64(9), "stageKind": "empty"},
				},
			},
		},
		"gameCtl.harvestLandsBatchByProtocol": map[string]any{
			"ok": true,
			"results": []any{
				map[string]any{"ok": true, "landId": float64(8)},
				map[string]any{"ok": true, "landId": float64(10)},
			},
		},
		"gameCtl.shovelLandsBatch": map[string]any{"ok": true, "successCount": float64(1)},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_collect")

	if !result.OK {
		t.Fatalf("own_collect should clean dead lands after harvest: %#v", result)
	}
	wantMethods := []string{
		"gameCtl.getFarmStatus",
		"gameCtl.harvestLandsBatchByProtocol",
		"gameCtl.getFarmStatus",
		"gameCtl.shovelLandsBatch",
	}
	if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
		t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
	}
	wantRescanArgs := []any{map[string]any{
		"includeGrids":          true,
		"includeLandIds":        true,
		"includeRawGrid":        true,
		"includeRawLandRuntime": true,
		"silent":                true,
		"forceRefresh":          true,
	}}
	if !reflect.DeepEqual(caller.calls[2].args, wantRescanArgs) {
		t.Fatalf("post-harvest status args = %#v, want %#v", caller.calls[2].args, wantRescanArgs)
	}
	wantShovelArgs := []any{map[string]any{
		"landIds":         []int{8},
		"onlyDead":        true,
		"silent":          true,
		"dryRun":          false,
		"waitAfterAction": 0,
		"betweenLandWait": 0,
		"source":          "farm_go_auto_collect_dead_cleanup",
	}}
	if !reflect.DeepEqual(caller.calls[3].args, wantShovelArgs) {
		t.Fatalf("shovel args = %#v, want %#v", caller.calls[3].args, wantShovelArgs)
	}
	if result.Message != "一键收获 2 块，清理枯萎 1 块。" {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestRuntimeFacadeOwnCollectCleansDeadLandsWithoutHarvest(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(6), "stageKind": "dead"},
			},
		},
		"gameCtl.shovelLandsBatch": map[string]any{"ok": true, "successCount": float64(1)},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_collect")

	if !result.OK {
		t.Fatalf("own_collect should clean dead-only farms: %#v", result)
	}
	wantMethods := []string{"gameCtl.getFarmStatus", "gameCtl.shovelLandsBatch"}
	if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
		t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
	}
	if result.Message != "一键收获 0 块，清理枯萎 1 块。" {
		t.Fatalf("message = %q", result.Message)
	}
	if result.ActionCount != 0 {
		t.Fatalf("action count = %d, want 0 for dead-land cleanup only", result.ActionCount)
	}
}

func TestRuntimeSuccessfulOperationCountDeduplicatesLandResults(t *testing.T) {
	got := runtimeSuccessfulOperationCount(map[string]any{
		"results": []any{
			map[string]any{"ok": true, "landId": float64(1)},
			map[string]any{"ok": true, "landId": float64(1)},
			map[string]any{"ok": true, "action": "skipped", "landId": float64(2)},
			map[string]any{"ok": false, "landId": float64(3)},
		},
	})
	if got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
}

func TestRuntimeFacadeOwnPlantPlantsDefaultSeedOnEmptyOwnFarmLands(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(4), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": "1", "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(2), "stageKind": "empty", "interactable": false},
				map[string]any{"landId": float64(3), "stageKind": "growing", "interactable": true},
			},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("own_plant should report OK on successful plant call: %#v", result)
	}
	if result.Status != StatusOK {
		t.Fatalf("status = %q, want %q", result.Status, StatusOK)
	}
	if result.TaskID != "own_plant" {
		t.Fatalf("taskID = %q, want own_plant", result.TaskID)
	}
	if len(caller.calls) != 3 {
		t.Fatalf("runtime call count = %d, want 3: %#v", len(caller.calls), caller.calls)
	}
	if caller.calls[1].method != "gameCtl.getFarmStatus" {
		t.Fatalf("second method = %q, want getFarmStatus", caller.calls[1].method)
	}
	plant := caller.calls[2]
	if plant.method != "gameCtl.autoPlant" {
		t.Fatalf("third method = %q, want gameCtl.autoPlant", plant.method)
	}
	if len(plant.args) != 1 {
		t.Fatalf("plant args = %#v, want one payload", plant.args)
	}
	payload := plant.args[0].(map[string]any)
	for key, want := range map[string]any{
		"mode":             "specified_seed",
		"seedId":           20002,
		"autoPlantSeedId":  20002,
		"protocolOnly":     true,
		"n":                true,
		"silent":           true,
		"waitAfterPlantMs": 300,
	} {
		if payload[key] != want {
			t.Fatalf("plant payload[%s] = %#v, want %#v; payload %#v", key, payload[key], want, payload)
		}
	}
	if !reflect.DeepEqual(payload["emptyLandIds"], []int{1, 4}) {
		t.Fatalf("autoPlant payload should pass only plantable empty lands, got %#v", payload)
	}
}

func TestRuntimeFacadeOwnPlantAppliesPlantFertilizerAfterSuccessfulPlant(t *testing.T) {
	for _, fertilizerMode := range []string{"normal", "organic"} {
		t.Run(fertilizerMode, func(t *testing.T) {
			caller := &fakeRuntimeCaller{responses: map[string]any{
				"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
				"gameCtl.getFarmStatus": map[string]any{
					"farmType": "own",
					"grids": []any{
						map[string]any{"landId": float64(4), "stageKind": "empty", "interactable": true},
						map[string]any{"landId": "1", "stageKind": "empty", "interactable": true},
					},
				},
				"gameCtl.autoPlant":           map[string]any{"ok": true},
				"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
			}}
			facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
				"autoFarmFertilizerEnabled":   true,
				"autoFarmPlantPrimaryMode":    "specified_seed",
				"autoFarmPlantSeedId":         20002,
				"autoFarmPlantFertilizerMode": fertilizerMode,
			})

			result := facade.RunTask(context.Background(), "own_plant")

			if !result.OK {
				t.Fatalf("own_plant should succeed: %#v", result)
			}
			wantMethods := []string{
				"gameCtl.getFarmOwnership",
				"gameCtl.getFarmStatus",
				"gameCtl.autoPlant",
				"gameCtl.fertilizeLandsBatch",
			}
			if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
				t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
			}
			payload := caller.calls[3].args[0].(map[string]any)
			wantPayload := map[string]any{
				"landIds":                     []int{1, 4},
				"type":                        fertilizerMode,
				"mode":                        fertilizerMode,
				"dryRun":                      false,
				"cleanupUi":                   true,
				"linkedHarvestAfterFertilize": false,
				"silent":                      true,
				"source":                      "farm_go_auto_plant_fertilizer",
			}
			if !reflect.DeepEqual(payload, wantPayload) {
				t.Fatalf("plant fertilizer payload = %#v, want %#v", payload, wantPayload)
			}
		})
	}
}

func TestRuntimeFacadeDelayedPlantingSubmissionUsesPlantingScope(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids":    []any{map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true}},
		},
		"gameCtl.autoPlant":           map[string]any{"ok": true},
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":              true,
		"autoFarmPlantPrimaryMode":               "specified_seed",
		"autoFarmPlantSeedId":                    20002,
		"autoFarmPlantFertilizerMode":            "normal",
		"autoFarmFertilizerDelayedSubmitEnabled": true,
		"autoFarmFertilizerDelayedSubmitScopes":  []any{"planting"},
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("own_plant should succeed: %#v", result)
	}
	payload := mapFromAny(caller.calls[3].args[0])
	if payload["fertilizerSubmissionMode"] != "serial" || intFromAny(payload["betweenLandWait"]) != 500 {
		t.Fatalf("plant fertilizer payload = %#v, want serial mode with 500ms wait", payload)
	}
}

func TestRuntimeFacadeMultiSeasonDelayedSubmissionUsesPlantingScope(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true, "successCount": float64(1)},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":              true,
		"autoFarmFertilizerMultiSeason":          true,
		"autoFarmPlantFertilizerMode":            "normal",
		"autoFarmFertilizerDelayedSubmitEnabled": true,
		"autoFarmFertilizerDelayedSubmitScopes":  []any{"planting"},
	})
	status := map[string]any{"grids": []any{map[string]any{
		"landId": float64(4), "plantId": float64(20002), "hasPlant": true,
		"stageKind": "growing", "isMultiSeason": true, "currentSeason": float64(2), "totalSeason": float64(3),
	}}}

	count, failed := facade.fertilizeMultiSeasonAfterHarvest(context.Background(), "own_collect", status, []int{4}, "test")

	if failed != nil || count != 1 {
		t.Fatalf("count=%d failed=%#v", count, failed)
	}
	payload := mapFromAny(caller.calls[0].args[0])
	if payload["fertilizerSubmissionMode"] != "serial" || intFromAny(payload["betweenLandWait"]) != 500 {
		t.Fatalf("multi-season fertilizer payload = %#v, want serial planting policy", payload)
	}
}

func TestRuntimeFacadeOwnPlantSmartFertilizerDefersLargeLeafCrop(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids":    []any{map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true}},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":   true,
		"autoFarmPlantPrimaryMode":    "specified_seed",
		"autoFarmPlantSeedId":         20003,
		"autoFarmPlantFertilizerMode": "smart_normal",
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("smart plant fertilizer should defer carrot: %#v", result)
	}
	wantMethods := []string{"gameCtl.getFarmOwnership", "gameCtl.getFarmStatus", "gameCtl.autoPlant"}
	if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", got, wantMethods)
	}
}

func TestRuntimeFacadeOwnPlantSmartFertilizerFallsBackWithoutLargeLeaf(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids":    []any{map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true}},
		},
		"gameCtl.autoPlant":           map[string]any{"ok": true},
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":   true,
		"autoFarmPlantPrimaryMode":    "specified_seed",
		"autoFarmPlantSeedId":         20002,
		"autoFarmPlantFertilizerMode": "smart_normal",
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("smart plant fertilizer fallback should succeed: %#v", result)
	}
	wantMethods := []string{"gameCtl.getFarmOwnership", "gameCtl.getFarmStatus", "gameCtl.autoPlant", "gameCtl.fertilizeLandsBatch"}
	if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", got, wantMethods)
	}
	payload := mapFromAny(caller.calls[3].args[0])
	if payload["type"] != "normal" || payload["mode"] != "normal" || !reflect.DeepEqual(payload["landIds"], []int{1}) {
		t.Fatalf("fallback fertilizer payload = %#v", payload)
	}
}

func TestRuntimeFacadeMultiSeasonSmartFertilizerDefersLargeLeafCrop(t *testing.T) {
	caller := &fakeRuntimeCaller{}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":     true,
		"autoFarmFertilizerMultiSeason": true,
		"autoFarmPlantFertilizerMode":   "smart_organic",
	})
	status := map[string]any{"grids": []any{map[string]any{
		"landId": float64(4), "plantId": float64(1020003), "hasPlant": true,
		"stageKind": "growing", "isMultiSeason": true, "currentSeason": float64(2), "totalSeason": float64(3),
	}}}

	count, failed := facade.fertilizeMultiSeasonAfterHarvest(context.Background(), "own_fertilizer", status, []int{4}, "test")

	if failed != nil || count != 0 || len(caller.calls) != 0 {
		t.Fatalf("count=%d failed=%#v calls=%#v", count, failed, caller.calls)
	}
}

func TestRuntimeFacadeMultiSeasonSmartFertilizerFallsBackWithoutLargeLeaf(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true, "successCount": float64(1)},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":     true,
		"autoFarmFertilizerMultiSeason": true,
		"autoFarmPlantFertilizerMode":   "smart_organic",
	})
	status := map[string]any{"grids": []any{map[string]any{
		"landId": float64(4), "plantId": float64(1020002), "hasPlant": true,
		"stageKind": "growing", "isMultiSeason": true, "currentSeason": float64(2), "totalSeason": float64(3),
	}}}

	count, failed := facade.fertilizeMultiSeasonAfterHarvest(context.Background(), "own_fertilizer", status, []int{4}, "test")

	if failed != nil || count != 1 {
		t.Fatalf("count=%d failed=%#v", count, failed)
	}
	if got := mapFromAny(caller.calls[0].args[0])["type"]; got != "organic" {
		t.Fatalf("type = %#v, want organic", got)
	}
}

func TestRuntimeFacadeOwnPlantFertilizesFourGridAnchors(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true, "gridPos": map[string]any{"x": float64(0), "y": float64(5)}},
				map[string]any{"landId": float64(2), "stageKind": "empty", "interactable": true, "gridPos": map[string]any{"x": float64(1), "y": float64(5)}},
				map[string]any{"landId": float64(5), "stageKind": "empty", "interactable": true, "gridPos": map[string]any{"x": float64(0), "y": float64(4)}},
				map[string]any{"landId": float64(6), "stageKind": "empty", "interactable": true, "gridPos": map[string]any{"x": float64(1), "y": float64(4)}},
			},
		},
		"gameCtl.getSeedList": []any{
			map[string]any{"itemId": float64(20416), "count": float64(1), "name": "哈哈南瓜种子"},
		},
		"gameCtl.autoPlant": map[string]any{
			"ok": true,
			"fourGridPlantDecision": map[string]any{
				"groups": []any{[]any{float64(5), float64(6), float64(1), float64(2)}},
			},
		},
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":         true,
		"autoFarmPlantPrimaryMode":          "backpack_first",
		"autoFarmFourGridPlantEnabled":      true,
		"autoFarmPlantFertilizerMode":       "normal",
		"autoFarmPlantBackpackSeedPriority": []int{20416},
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("four-grid planting fertilizer should succeed: %#v", result)
	}
	wantMethods := []string{
		"gameCtl.getFarmOwnership",
		"gameCtl.getFarmStatus",
		"gameCtl.getSeedList",
		"gameCtl.autoPlant",
		"gameCtl.fertilizeLandsBatch",
	}
	if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
		t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
	}
	payload := caller.calls[4].args[0].(map[string]any)
	if !reflect.DeepEqual(payload["landIds"], []int{5}) {
		t.Fatalf("four-grid fertilizer landIds = %#v, want anchor [5]", payload["landIds"])
	}
}

func TestRuntimeFacadeOwnPlantSkipsPlantFertilizerForUnsupportedModes(t *testing.T) {
	for _, fertilizerMode := range []string{"none", "smart_normal"} {
		t.Run(fertilizerMode, func(t *testing.T) {
			caller := &fakeRuntimeCaller{responses: map[string]any{
				"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
				"gameCtl.getFarmStatus": map[string]any{
					"farmType": "own",
					"grids":    []any{map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true}},
				},
				"gameCtl.autoPlant": map[string]any{"ok": true},
			}}
			facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
				"autoFarmPlantPrimaryMode":    "specified_seed",
				"autoFarmPlantSeedId":         20002,
				"autoFarmPlantFertilizerMode": fertilizerMode,
			})

			result := facade.RunTask(context.Background(), "own_plant")

			if !result.OK {
				t.Fatalf("own_plant should succeed: %#v", result)
			}
			wantMethods := []string{"gameCtl.getFarmOwnership", "gameCtl.getFarmStatus", "gameCtl.autoPlant"}
			if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
				t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
			}
		})
	}
}

func TestRuntimeFacadeOwnPlantSkipsPlantFertilizerWhenMasterDisabled(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true},
			},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":   false,
		"autoFarmPlantPrimaryMode":    "specified_seed",
		"autoFarmPlantSeedId":         20002,
		"autoFarmPlantFertilizerMode": "normal",
	})

	result := facade.RunTask(context.Background(), "own_plant")

	fertilizerCalled := false
	for _, call := range caller.calls {
		if call.method == "gameCtl.fertilizeLandsBatch" {
			fertilizerCalled = true
		}
	}
	if !result.OK || fertilizerCalled {
		t.Fatalf("disabled fertilizer master should skip plant fertilizer: result=%#v calls=%#v", result, caller.calls)
	}
}

func TestRuntimeFacadeOwnPlantSkipsPlantFertilizerWhenPlantFails(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids":    []any{map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true}},
		},
		"gameCtl.autoPlant": map[string]any{"ok": false, "reason": "no_goods"},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode":    "specified_seed",
		"autoFarmPlantSeedId":         20002,
		"autoFarmPlantFertilizerMode": "normal",
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("failed plant should fail the task: %#v", result)
	}
	wantMethods := []string{"gameCtl.getFarmOwnership", "gameCtl.getFarmStatus", "gameCtl.autoPlant"}
	if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
		t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
	}
}

func TestRuntimeFacadeOwnPlantReadsEmptyLandsAfterEnteringOwnFarm(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "friend"},
		"gameCtl.enterOwnFarm":     map[string]any{"ok": true, "farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(9), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(10), "stageKind": "growing", "interactable": true},
			},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true, "emptyCount": 1},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode": "specified_seed",
		"autoFarmPlantSeedId":      20002,
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("own_plant should report OK after entering own farm: %#v", result)
	}
	if len(caller.calls) != 4 {
		t.Fatalf("runtime call count = %d, want 4: %#v", len(caller.calls), caller.calls)
	}
	if caller.calls[0].method != "gameCtl.getFarmOwnership" || caller.calls[1].method != "gameCtl.enterOwnFarm" || caller.calls[2].method != "gameCtl.getFarmStatus" || caller.calls[3].method != "gameCtl.autoPlant" {
		t.Fatalf("unexpected call order: %#v", caller.calls)
	}
	payload := caller.calls[3].args[0].(map[string]any)
	if !reflect.DeepEqual(payload["emptyLandIds"], []int{9}) {
		t.Fatalf("autoPlant payload should use empty lands read after entering own farm, got %#v", payload)
	}
}

func TestRuntimeFacadeOwnPlantUsesConfiguredStrategySeedInsteadOfDefaultSeed(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "stageKind": "empty", "interactable": true},
			},
		},
		"gameCtl.getPlayerProfile": map[string]any{"plantLevel": 99},
		"gameCtl.getSeedList":      []any{map[string]any{"itemId": 20003, "count": 2, "name": "胡萝卜种子"}},
		"gameCtl.requestShopData":  map[string]any{"ok": true},
		"gameCtl.getShopSeedList":  []any{},
		"gameCtl.autoPlant":        map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode": "max_exp",
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("own_plant should report OK on configured strategy plant call: %#v", result)
	}
	plant := caller.calls[len(caller.calls)-1]
	args := plant.args[0].(map[string]any)
	if args["mode"] != "max_exp" || args["seedId"] != 20003 {
		t.Fatalf("plant args should use configured strategy seed 20003, got %#v", args)
	}
	if args["seedId"] == 20002 {
		t.Fatalf("plant args still use hard-coded default seed: %#v", args)
	}
}

func TestRuntimeFacadeOwnPlantSkipsWhenConfiguredStrategyIsNone(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "stageKind": "empty", "interactable": true},
			},
		},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode":   "none",
		"autoFarmPlantSecondaryMode": "none",
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("none strategy should be an OK skip, got %#v", result)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("none strategy should not call runtime, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeOwnPlantPassesBackpackFirstConfigToRuntime(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(4), "stageKind": "empty", "interactable": true},
			},
		},
		"gameCtl.getSeedList": []any{
			map[string]any{"itemId": 20002, "count": 4, "name": "白萝卜种子"},
			map[string]any{"itemId": 20133, "count": 1, "name": "优先种子"},
			map[string]any{"itemId": 20176, "count": 1, "name": "禁用种子"},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode":           "backpack_first",
		"autoFarmPlantSecondaryMode":         "max_profit",
		"autoFarmPlantBackpackSeedPriority":  []int{20133, 21032},
		"autoFarmPlantBackpackSeedDisabled":  []int{20176},
		"autoFarmPlantBackpackForcePriority": true,
		"autoFarmFourGridPlantEnabled":       true,
		"autoFarmPlantRandomOrderEnabled":    true,
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("own_plant should report OK on backpack-first plant call: %#v", result)
	}
	plant := caller.calls[len(caller.calls)-1]
	args := plant.args[0].(map[string]any)
	if args["mode"] != "backpack_first" || args["seedId"] != 20133 {
		t.Fatalf("backpack_first should use configured priority seed 20133, got %#v", args)
	}
	if !reflect.DeepEqual(args["autoPlantBackpackSeedPriority"], []int{20133, 21032}) {
		t.Fatalf("priority list not passed: %#v", args)
	}
	if !reflect.DeepEqual(args["autoPlantBackpackSeedDisabled"], []int{20176}) {
		t.Fatalf("disabled list not passed: %#v", args)
	}
	if args["autoPlantBackpackForcePriority"] != true || args["fourGridPlantEnabled"] != true || args["autoPlantRandomOrderEnabled"] != true {
		t.Fatalf("plant config flags not passed: %#v", args)
	}
}

func TestOwnPlantRuntimeTimingUsesMaximumInterLandDelay(t *testing.T) {
	timeout, deadline := ownPlantRuntimeTiming(map[string]any{
		"autoPlantRandomizedEnabled":    true,
		"autoPlantRandomizedDelayMaxMs": 500,
		"emptyLandIds":                  []int{1, 2, 3},
	}, time.UnixMilli(1000))
	if timeout != 61*time.Second {
		t.Fatalf("timeout = %v, want %v", timeout, 61*time.Second)
	}
	if deadline != 61000 {
		t.Fatalf("deadline = %d, want 61000", deadline)
	}
}

func TestRuntimeFacadeOwnPlantUsesOneBackpackReadAndOneCallPerSeedPlan(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": 1, "stageKind": "empty", "interactable": true},
				map[string]any{"landId": 2, "stageKind": "empty", "interactable": true},
				map[string]any{"landId": 3, "stageKind": "empty", "interactable": true},
			},
		},
		"gameCtl.getSeedList": []any{
			map[string]any{"itemId": 20133, "count": 1, "name": "优先种子"},
			map[string]any{"itemId": 20002, "count": 2, "name": "白萝卜种子"},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	}}
	result := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode":          "backpack_first",
		"autoFarmPlantBackpackSeedPriority": []int{20133, 20002},
		"autoFarmPlantRandomizedEnabled":    true,
		"autoFarmPlantRandomizedDelayMinMs": 100,
		"autoFarmPlantRandomizedDelayMaxMs": 500,
	}).RunTask(context.Background(), "own_plant")
	if !result.OK {
		t.Fatalf("own_plant result = %#v", result)
	}
	seedListCalls := 0
	for _, call := range caller.calls {
		if call.method == "gameCtl.getSeedList" {
			seedListCalls++
		}
	}
	if seedListCalls != 1 {
		t.Fatalf("getSeedList calls = %d, want 1; calls = %#v", seedListCalls, caller.calls)
	}
	plans := collectAutoPlantPayloads(caller.calls)
	if len(plans) != 2 {
		t.Fatalf("autoPlant plans = %#v, want two seed plans", plans)
	}
	for _, plan := range plans {
		if intFromAny(plan["plantExecutionDeadlineAtMs"]) <= 0 {
			t.Fatalf("missing execution deadline: %#v", plan)
		}
	}
}

func TestRuntimeFacadeOwnPlantAllocatesFourLandsToOneFourGridSeed(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(2), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(5), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(6), "stageKind": "empty", "interactable": true},
			},
		},
		"gameCtl.getSeedList": []any{
			map[string]any{"itemId": float64(20416), "count": float64(1), "name": "哈哈南瓜种子"},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode":     "backpack_first",
		"autoFarmFourGridPlantEnabled": true,
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("four-grid plant should succeed: %#v", result)
	}
	payloads := collectAutoPlantPayloads(caller.calls)
	if len(payloads) != 1 {
		t.Fatalf("autoPlant call count = %d, want 1; calls=%#v", len(payloads), caller.calls)
	}
	if !reflect.DeepEqual(payloads[0]["emptyLandIds"], []int{1, 2, 5, 6}) {
		t.Fatalf("four-grid payload lands = %#v, want [1 2 5 6]", payloads[0]["emptyLandIds"])
	}
}

func TestRuntimeFacadeOwnPlantAllocatesSpatialFourGridFromFullRows(t *testing.T) {
	grids := []any{}
	for x := 0; x < 4; x++ {
		grids = append(grids,
			map[string]any{
				"landId": float64(x + 1), "stageKind": "empty", "interactable": true,
				"gridPos": map[string]any{"x": float64(x), "y": float64(5)},
			},
			map[string]any{
				"landId": float64(x + 5), "stageKind": "empty", "interactable": true,
				"gridPos": map[string]any{"x": float64(x), "y": float64(4)},
			},
		)
	}
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids":    grids,
		},
		"gameCtl.getSeedList": []any{
			map[string]any{"itemId": float64(21032), "count": float64(2), "name": "琉璃宝荷种子"},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode":     "backpack_first",
		"autoFarmFourGridPlantEnabled": true,
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("four-grid plant should succeed: %#v", result)
	}
	payloads := collectAutoPlantPayloads(caller.calls)
	want := []int{1, 2, 5, 6, 3, 4, 7, 8}
	if len(payloads) != 1 || !reflect.DeepEqual(payloads[0]["emptyLandIds"], want) {
		t.Fatalf("four-grid payloads = %#v, want grouped lands %#v", payloads, want)
	}
}

func TestRuntimeFacadeOwnPlantDoesNotRunNormalSeedAfterFourGridConsumesAllLands(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(2), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(5), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(6), "stageKind": "empty", "interactable": true},
			},
		},
		"gameCtl.getSeedList": []any{
			map[string]any{"itemId": float64(20416), "count": float64(1), "name": "哈哈南瓜种子"},
			map[string]any{"itemId": float64(20002), "count": float64(4), "name": "白萝卜种子"},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode":          "backpack_first",
		"autoFarmFourGridPlantEnabled":      true,
		"autoFarmPlantBackpackSeedPriority": []int{20416, 20002},
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("four-grid plant should succeed: %#v", result)
	}
	payloads := collectAutoPlantPayloads(caller.calls)
	if len(payloads) != 1 {
		t.Fatalf("autoPlant call count = %d, want 1; calls=%#v", len(payloads), caller.calls)
	}
	if payloads[0]["seedId"] != 20416 {
		t.Fatalf("seedId = %#v, want 20416", payloads[0]["seedId"])
	}
}

func TestRuntimeFacadeOwnPlantContinuesAfterNoCompleteFourGridGroup(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(2), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(5), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(6), "stageKind": "empty", "interactable": true},
			},
		},
		"gameCtl.getSeedList": []any{
			map[string]any{"itemId": float64(20416), "count": float64(1), "name": "哈哈南瓜种子"},
			map[string]any{"itemId": float64(20002), "count": float64(4), "name": "白萝卜种子"},
		},
		"gameCtl.autoPlant": fakeRuntimeResponseSequence{
			map[string]any{"ok": false, "plantResult": map[string]any{"reason": "no_complete_multi_land_group"}},
			map[string]any{"ok": true},
		},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode":          "backpack_first",
		"autoFarmFourGridPlantEnabled":      true,
		"autoFarmPlantBackpackSeedPriority": []int{20416, 20002},
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("normal seed should run after an ungroupable four-grid seed: %#v", result)
	}
	payloads := collectAutoPlantPayloads(caller.calls)
	if len(payloads) != 2 {
		t.Fatalf("autoPlant call count = %d, want 2; calls=%#v", len(payloads), caller.calls)
	}
	if payloads[0]["seedId"] != 20416 || payloads[1]["seedId"] != 20002 {
		t.Fatalf("seed order = %#v, want four-grid then normal", payloads)
	}
}

func TestRuntimeFacadeOwnPlantSplitsEmptyLandsAcrossBackpackSeeds(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(2), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(3), "stageKind": "empty", "interactable": true},
			},
		},
		"gameCtl.getSeedList": []any{
			map[string]any{"itemId": 20133, "count": 1, "name": "优先种子"},
			map[string]any{"itemId": 20002, "count": 2, "name": "白萝卜种子"},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode":          "backpack_first",
		"autoFarmPlantBackpackSeedPriority": []int{20133, 20002},
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("own_plant should split backpack seeds across empty lands: %#v", result)
	}
	plantPayloads := collectAutoPlantPayloads(caller.calls)
	if len(plantPayloads) != 2 {
		t.Fatalf("autoPlant call count = %d, want 2; calls=%#v", len(plantPayloads), caller.calls)
	}
	if plantPayloads[0]["seedId"] != 20133 || !reflect.DeepEqual(plantPayloads[0]["emptyLandIds"], []int{1}) {
		t.Fatalf("first plant payload = %#v, want seed 20133 on land 1", plantPayloads[0])
	}
	if plantPayloads[1]["seedId"] != 20002 || !reflect.DeepEqual(plantPayloads[1]["emptyLandIds"], []int{2, 3}) {
		t.Fatalf("second plant payload = %#v, want seed 20002 on lands 2,3", plantPayloads[1])
	}
}

func TestRuntimeFacadeOwnPlantFallsBackToSecondaryStrategyWhenBackpackSeedsAreInsufficient(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(2), "stageKind": "empty", "interactable": true},
				map[string]any{"landId": float64(3), "stageKind": "empty", "interactable": true},
			},
		},
		"gameCtl.getSeedList": []any{
			map[string]any{"itemId": 20133, "count": 1, "name": "优先种子"},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode":          "backpack_first",
		"autoFarmPlantSecondaryMode":        "specified_seed",
		"autoFarmPlantSeedId":               29999,
		"autoFarmPlantBackpackSeedPriority": []int{20133},
	})

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK {
		t.Fatalf("own_plant should fall back to secondary strategy: %#v", result)
	}
	plantPayloads := collectAutoPlantPayloads(caller.calls)
	if len(plantPayloads) != 2 {
		t.Fatalf("autoPlant call count = %d, want 2; calls=%#v", len(plantPayloads), caller.calls)
	}
	if plantPayloads[0]["seedId"] != 20133 || !reflect.DeepEqual(plantPayloads[0]["emptyLandIds"], []int{1}) {
		t.Fatalf("first plant payload = %#v, want backpack seed on land 1", plantPayloads[0])
	}
	if plantPayloads[1]["mode"] != "specified_seed" || plantPayloads[1]["seedId"] != 29999 || !reflect.DeepEqual(plantPayloads[1]["emptyLandIds"], []int{2, 3}) {
		t.Fatalf("fallback payload = %#v, want specified seed 29999 on lands 2,3", plantPayloads[1])
	}
}

func collectAutoPlantPayloads(calls []runtimeCall) []map[string]any {
	payloads := []map[string]any{}
	for _, call := range calls {
		if call.method == "gameCtl.autoPlant" {
			payloads = append(payloads, call.args[0].(map[string]any))
		}
	}
	return payloads
}

func TestRuntimeFacadeOwnPlantSkipsWhenNoEmptyLands(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "stageKind": "growing", "interactable": true},
			},
		},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_plant")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("no empty lands should be an OK skip, got %#v", result)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("no-empty runtime path should call ownership and status only, got %#v", caller.calls)
	}
	if caller.calls[1].method != "gameCtl.getFarmStatus" {
		t.Fatalf("second method = %q, want getFarmStatus", caller.calls[1].method)
	}
}

func TestRuntimeFacadeOwnPlantRequiresOwnFarmStatus(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "friend"},
		"gameCtl.enterOwnFarm":     map[string]any{"ok": false, "reason": "enter_failed"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_plant")

	if result.OK {
		t.Fatalf("friend farm should not be planted by own_plant: %#v", result)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, StatusFailed)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("friend farm should try ownership and enterOwnFarm only, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeOwnPlantReportsStatusFailure(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": errors.New("runtime link is not connected"),
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_plant")

	if result.OK {
		t.Fatalf("status failure should not be OK: %#v", result)
	}
	if result.Status != StatusRuntimeNotReady {
		t.Fatalf("status = %q, want %q", result.Status, StatusRuntimeNotReady)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("expected one status call, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeOwnPlantReportsAutoPlantFailure(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "stageKind": "empty", "interactable": true},
			},
		},
		"gameCtl.autoPlant": errors.New("autoPlant method not ready"),
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_plant")

	if result.OK {
		t.Fatalf("autoPlant failure should not be OK: %#v", result)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, StatusFailed)
	}
	if len(caller.calls) != 3 {
		t.Fatalf("expected ownership, status and autoPlant calls, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeOwnPlantReportsRuntimeNotOKResult(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "stageKind": "empty", "interactable": true},
			},
		},
		"gameCtl.autoPlant": map[string]any{"ok": false, "reason": "no_goods"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_plant")

	if result.OK {
		t.Fatalf("explicit autoPlant ok=false should not be OK: %#v", result)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, StatusFailed)
	}
}

func TestRuntimeFacadeOwnFertilizerRushesNearMatureOwnFarmLands(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(4), "stageKind": "growing", "hasPlant": true, "plantId": float64(20002), "matureInSec": float64(120)},
				map[string]any{"landId": "1", "stageKind": "growing", "hasPlant": true, "plantId": float64(20003), "matureInSec": float64(300)},
				map[string]any{"landId": float64(2), "stageKind": "growing", "hasPlant": true, "plantId": float64(20004), "matureInSec": float64(301)},
				map[string]any{"landId": float64(3), "stageKind": "mature", "hasPlant": true, "plantId": float64(20005), "matureInSec": float64(0)},
				map[string]any{"landId": float64(5), "stageKind": "growing", "hasPlant": false, "plantId": float64(20006), "matureInSec": float64(90)},
			},
		},
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{"autoFarmFertilizerEnabled": true})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if !result.OK {
		t.Fatalf("own_fertilizer should report OK on successful batch call: %#v", result)
	}
	if result.Status != StatusOK {
		t.Fatalf("status = %q, want %q", result.Status, StatusOK)
	}
	if result.TaskID != "own_fertilizer" {
		t.Fatalf("taskID = %q, want own_fertilizer", result.TaskID)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("runtime call count = %d, want 2: %#v", len(caller.calls), caller.calls)
	}
	batch := caller.calls[1]
	if batch.method != "gameCtl.fertilizeLandsBatch" {
		t.Fatalf("second method = %q, want gameCtl.fertilizeLandsBatch", batch.method)
	}
	wantArgs := []any{map[string]any{
		"landIds":                     []int{1, 4},
		"type":                        "organic",
		"mode":                        "organic",
		"dryRun":                      false,
		"cleanupUi":                   true,
		"linkedHarvestAfterFertilize": false,
		"rushThresholdSec":            300,
		"silent":                      true,
		"source":                      "farm_go_auto_fertilizer",
	}}
	if !reflect.DeepEqual(batch.args, wantArgs) {
		t.Fatalf("fertilizer args = %#v, want %#v", batch.args, wantArgs)
	}
}

func TestRuntimeFacadeDelayedAutomaticFertilizerRequiresRushScope(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		scopes     []any
		wantSerial bool
	}{
		{name: "rush selected", scopes: []any{"rush"}, wantSerial: true},
		{name: "planting selected", scopes: []any{"planting"}, wantSerial: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			caller := &fakeRuntimeCaller{responses: map[string]any{
				"gameCtl.getFarmStatus": map[string]any{
					"farmType": "own",
					"grids": []any{map[string]any{
						"landId": float64(1), "stageKind": "growing", "hasPlant": true,
						"plantId": float64(20002), "matureInSec": float64(120),
					}},
				},
				"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
			}}
			facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
				"autoFarmFertilizerEnabled":              true,
				"autoFarmFertilizerDelayedSubmitEnabled": true,
				"autoFarmFertilizerDelayedSubmitScopes":  testCase.scopes,
			})

			result := facade.RunTask(context.Background(), "own_fertilizer")

			if !result.OK {
				t.Fatalf("own_fertilizer should succeed: %#v", result)
			}
			payload := mapFromAny(caller.calls[1].args[0])
			gotSerial := payload["fertilizerSubmissionMode"] == "serial"
			if gotSerial != testCase.wantSerial {
				t.Fatalf("automatic fertilizer payload = %#v, want serial=%t", payload, testCase.wantSerial)
			}
		})
	}
}

func TestCollectFertilizerRushLandIDsUsesFourGridAnchor(t *testing.T) {
	status := map[string]any{
		"grids": []any{
			map[string]any{"landId": float64(1), "occupancyAnchorLandId": float64(5), "stageKind": "growing", "hasPlant": true, "plantId": float64(20416), "matureInSec": float64(60)},
			map[string]any{"landId": float64(2), "occupancyAnchorLandId": float64(5), "stageKind": "growing", "hasPlant": true, "plantId": float64(20416), "matureInSec": float64(60)},
			map[string]any{"landId": float64(5), "occupancyAnchorLandId": float64(5), "stageKind": "growing", "hasPlant": true, "plantId": float64(20416), "matureInSec": float64(60)},
			map[string]any{"landId": float64(6), "occupancyAnchorLandId": float64(5), "stageKind": "growing", "hasPlant": true, "plantId": float64(20416), "matureInSec": float64(60)},
			map[string]any{"landId": float64(9), "stageKind": "growing", "hasPlant": true, "plantId": float64(20002), "matureInSec": float64(60)},
		},
	}

	if got, want := collectFertilizerRushLandIDs(status, 60), []int{5, 9}; !reflect.DeepEqual(got, want) {
		t.Fatalf("rush fertilizer land IDs = %#v, want anchors %#v", got, want)
	}
}

func TestRuntimeFacadeOwnFertilizerDisabledRushModeSkips(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "stageKind": "growing", "hasPlant": true, "plantId": float64(20002), "matureInSec": float64(120)},
			},
		},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":  true,
		"autoFarmRushFertilizerMode": "none",
	})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("disabled rush mode should be an OK skip, got %#v", result)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("disabled rush mode called runtime: %#v", caller.calls)
	}
}

func TestRuntimeFacadeOwnFertilizerSmartPatrolRunsWhenRushDisabled(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{map[string]any{
				"landId": float64(5), "plantId": float64(1020003), "hasPlant": true,
				"stageKind": "growing", "phaseName": "大叶子",
			}},
		},
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":   true,
		"autoFarmPlantFertilizerMode": "smart_normal",
		"autoFarmRushFertilizerMode":  "none",
	})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if !result.OK {
		t.Fatalf("smart patrol should succeed: %#v", result)
	}
	wantMethods := []string{"gameCtl.getFarmStatus", "gameCtl.fertilizeLandsBatch"}
	if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", got, wantMethods)
	}
	payload := mapFromAny(caller.calls[1].args[0])
	if payload["type"] != "normal" || payload["mode"] != "normal" || !reflect.DeepEqual(payload["landIds"], []int{5}) {
		t.Fatalf("smart patrol payload = %#v", payload)
	}
}

func TestRuntimeFacadeOwnFertilizerSmartPatrolSkipsOtherPhasesAndUnknownCrops(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(1), "plantId": float64(1020003), "hasPlant": true, "stageKind": "growing", "phaseName": "小叶子"},
				map[string]any{"landId": float64(2), "plantId": float64(999999), "hasPlant": true, "stageKind": "growing", "phaseName": "大叶子"},
				map[string]any{"landId": float64(3), "plantId": float64(1020003), "hasPlant": true, "stageKind": "growing", "phaseName": "大叶子"},
			},
		},
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":   true,
		"autoFarmPlantFertilizerMode": "smart_organic",
		"autoFarmRushFertilizerMode":  "none",
	})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if !result.OK {
		t.Fatalf("smart patrol should succeed: %#v", result)
	}
	wantMethods := []string{"gameCtl.getFarmStatus", "gameCtl.fertilizeLandsBatch"}
	if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", got, wantMethods)
	}
	payload := mapFromAny(caller.calls[1].args[0])
	if payload["type"] != "organic" || !reflect.DeepEqual(payload["landIds"], []int{3}) {
		t.Fatalf("smart patrol payload = %#v", payload)
	}
}

func TestRuntimeFacadeOwnFertilizerSmartPatrolExcludesSmartLandFromRushBatch(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{map[string]any{
				"landId": float64(5), "plantId": float64(1020003), "hasPlant": true,
				"stageKind": "growing", "phaseName": "大叶子", "matureInSec": float64(30),
			}},
		},
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":          true,
		"autoFarmPlantFertilizerMode":        "smart_normal",
		"autoFarmRushFertilizerMode":         "organic",
		"autoFarmFertilizerRushThresholdSec": 60,
	})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if !result.OK {
		t.Fatalf("smart patrol should succeed: %#v", result)
	}
	wantMethods := []string{"gameCtl.getFarmStatus", "gameCtl.fertilizeLandsBatch"}
	if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", got, wantMethods)
	}
	payload := mapFromAny(caller.calls[1].args[0])
	if payload["type"] != "normal" || !reflect.DeepEqual(payload["landIds"], []int{5}) {
		t.Fatalf("deduplicated smart payload = %#v", payload)
	}
}

func TestRuntimeFacadeOwnFertilizerSkipsWhenMasterDisabled(t *testing.T) {
	caller := &fakeRuntimeCaller{}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":  false,
		"autoFarmRushFertilizerMode": "organic",
	})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if !result.OK || len(caller.calls) != 0 {
		t.Fatalf("disabled fertilizer master should skip rush fertilizer: result=%#v calls=%#v", result, caller.calls)
	}
}

func TestRuntimeFacadeOwnFertilizerUsesConfiguredModeAndThreshold(t *testing.T) {
	for _, mode := range []string{"normal", "organic"} {
		t.Run(mode, func(t *testing.T) {
			caller := &fakeRuntimeCaller{responses: map[string]any{
				"gameCtl.getFarmStatus": map[string]any{
					"farmType": "own",
					"grids": []any{
						map[string]any{"landId": float64(8), "stageKind": "growing", "hasPlant": true, "plantId": float64(20002), "matureInSec": float64(450)},
						map[string]any{"landId": float64(9), "stageKind": "growing", "hasPlant": true, "plantId": float64(20003), "matureInSec": float64(451)},
					},
				},
				"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
			}}
			facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
				"autoFarmFertilizerEnabled":          true,
				"autoFarmRushFertilizerMode":         mode,
				"autoFarmFertilizerRushThresholdSec": 450,
			})

			result := facade.RunTask(context.Background(), "own_fertilizer")

			if !result.OK || len(caller.calls) != 2 {
				t.Fatalf("configured fertilizer run = %#v, calls %#v", result, caller.calls)
			}
			payload := mapFromAny(caller.calls[1].args[0])
			if payload["type"] != mode || payload["mode"] != mode {
				t.Fatalf("fertilizer mode payload = %#v, want %q", payload, mode)
			}
			if got := intFromAny(payload["rushThresholdSec"]); got != 450 {
				t.Fatalf("rushThresholdSec = %d, want 450", got)
			}
			if got := normalizeUniquePositiveInts(sliceFromAny(payload["landIds"])); !reflect.DeepEqual(got, []int{8}) {
				t.Fatalf("landIds = %#v, want [8]", got)
			}
		})
	}
}

func TestRuntimeFacadeOwnFertilizerLinkedHarvestCleansDeadCropsAfterRescan(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": fakeRuntimeResponseSequence{
			map[string]any{
				"farmType": "own",
				"grids": []any{
					map[string]any{"landId": float64(4), "stageKind": "growing", "hasPlant": true, "plantId": float64(20002), "matureInSec": float64(120)},
					map[string]any{"landId": "1", "stageKind": "growing", "hasPlant": true, "plantId": float64(20003), "matureInSec": float64(300)},
				},
			},
			map[string]any{
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
		},
		"gameCtl.fertilizeLandsBatch": map[string]any{
			"ok": true,
			"linkedHarvest": map[string]any{
				"ok": true,
				"results": []any{
					map[string]any{"ok": true, "landId": float64(1)},
					map[string]any{"ok": false, "landId": float64(4)},
					map[string]any{"ok": true, "landId": float64(7)},
				},
			},
		},
		"gameCtl.shovelLandsBatch": map[string]any{"ok": true, "successCount": float64(5)},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":            true,
		"autoFarmFertilizerHarvestLinkEnabled": true,
	})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if !result.OK {
		t.Fatalf("linked fertilizer cleanup should report OK: %#v", result)
	}
	if result.ActionCount != 2 {
		t.Fatalf("action count = %d, want 2 confirmed linked harvests", result.ActionCount)
	}
	if len(caller.calls) != 4 {
		t.Fatalf("runtime call count = %d, want 4: %#v", len(caller.calls), caller.calls)
	}
	batch := caller.calls[1]
	if batch.method != "gameCtl.fertilizeLandsBatch" {
		t.Fatalf("second method = %q, want gameCtl.fertilizeLandsBatch", batch.method)
	}
	wantBatchArgs := []any{map[string]any{
		"landIds":                     []int{1, 4},
		"type":                        "organic",
		"mode":                        "organic",
		"dryRun":                      false,
		"cleanupUi":                   true,
		"linkedHarvestAfterFertilize": true,
		"rushThresholdSec":            300,
		"silent":                      true,
		"source":                      "farm_go_auto_fertilizer",
	}}
	if !reflect.DeepEqual(batch.args, wantBatchArgs) {
		t.Fatalf("fertilizer args = %#v, want %#v", batch.args, wantBatchArgs)
	}
	if caller.calls[2].method != "gameCtl.getFarmStatus" {
		t.Fatalf("third method = %q, want post-harvest getFarmStatus", caller.calls[2].method)
	}
	shovel := caller.calls[3]
	if shovel.method != "gameCtl.shovelLandsBatch" {
		t.Fatalf("fourth method = %q, want shovelLandsBatch", shovel.method)
	}
	wantShovelArgs := []any{map[string]any{
		"landIds":         []int{4, 7, 8, 9, 10},
		"onlyDead":        true,
		"silent":          true,
		"dryRun":          false,
		"waitAfterAction": 0,
		"betweenLandWait": 0,
		"source":          "farm_go_auto_fertilizer_dead_cleanup",
	}}
	if !reflect.DeepEqual(shovel.args, wantShovelArgs) {
		t.Fatalf("shovel args = %#v, want %#v", shovel.args, wantShovelArgs)
	}
}

func TestRuntimeFacadeOwnFertilizerLinkedHarvestFertilizesMultiSeasonAfterCleanup(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": fakeRuntimeResponseSequence{
			map[string]any{
				"farmType": "own",
				"grids": []any{
					map[string]any{"landId": float64(4), "stageKind": "growing", "hasPlant": true, "plantId": float64(20002), "matureInSec": float64(120)},
				},
			},
			map[string]any{
				"farmType": "own",
				"grids": []any{
					map[string]any{"landId": float64(2), "stageKind": "dead"},
					map[string]any{"landId": float64(4), "hasPlant": true, "stageKind": "growing", "isMultiSeason": true, "currentSeason": float64(2), "totalSeason": float64(3)},
				},
			},
		},
		"gameCtl.fertilizeLandsBatch": fakeRuntimeResponseSequence{
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
		"gameCtl.shovelLandsBatch": map[string]any{"ok": true, "successCount": float64(1)},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled":            true,
		"autoFarmFertilizerHarvestLinkEnabled": true,
		"autoFarmFertilizerMultiSeason":        true,
		"autoFarmPlantFertilizerMode":          "organic",
	})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if !result.OK {
		t.Fatalf("linked fertilizer should complete cleanup and multi-season fertilizer: %#v", result)
	}
	wantMethods := []string{
		"gameCtl.getFarmStatus",
		"gameCtl.fertilizeLandsBatch",
		"gameCtl.getFarmStatus",
		"gameCtl.shovelLandsBatch",
		"gameCtl.fertilizeLandsBatch",
	}
	if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", got, wantMethods)
	}
	wantArgs := []any{map[string]any{
		"landIds":                     []int{4},
		"type":                        "organic",
		"mode":                        "organic",
		"dryRun":                      false,
		"cleanupUi":                   true,
		"linkedHarvestAfterFertilize": false,
		"silent":                      true,
		"source":                      "farm_go_auto_fertilizer_multi_season",
	}}
	if got := caller.calls[4].args; !reflect.DeepEqual(got, wantArgs) {
		t.Fatalf("multi-season fertilizer args = %#v, want %#v", got, wantArgs)
	}
}

func TestCollectDeadLandIDsUsesRuntimeDeadSignals(t *testing.T) {
	got := collectDeadLandIDs(map[string]any{
		"workLandIds": map[string]any{
			"eraseDead": []any{float64(11), float64(4)},
		},
		"landIds": map[string]any{
			"eraseDead": []any{"12", float64(7)},
		},
		"grids": []any{
			map[string]any{"landId": float64(4), "stageKind": "dead"},
			map[string]any{"landId": "1", "isDead": true},
			map[string]any{"id": float64(7), "canEraseDead": true},
			map[string]any{"landId": float64(8), "needsEraseDead": true},
			map[string]any{"landId": float64(9), "needEraseDead": true},
			map[string]any{"landId": float64(13), "stageKind": "withered"},
			map[string]any{"landId": float64(10), "stageKind": "growing", "isDead": false},
			map[string]any{"landId": float64(4), "isDead": true},
		},
	})
	want := []int{1, 4, 7, 8, 9, 11, 12, 13}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dead land ids = %#v, want %#v", got, want)
	}
}

func TestRuntimeFacadeOwnCollectPollsAndCleansDeadCropsAppearingAfterHarvest(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": fakeRuntimeResponseSequence{
			map[string]any{
				"farmType": "own",
				"workLandIds": map[string]any{
					"collect": []any{float64(5)},
				},
			},
			map[string]any{
				"farmType": "own",
				"grids": []any{
					map[string]any{"landId": float64(5), "stageKind": "empty"},
				},
			},
			map[string]any{
				"farmType": "own",
				"landIds": map[string]any{
					"eraseDead": []any{float64(5)},
				},
				"grids": []any{
					map[string]any{"landId": float64(5), "stageKind": "withered"},
				},
			},
		},
		"gameCtl.harvestLandsBatchByProtocol": map[string]any{
			"ok": true,
			"results": []any{
				map[string]any{"ok": true, "landId": float64(5)},
			},
		},
		"gameCtl.shovelLandsBatch": map[string]any{"ok": true, "successCount": float64(1)},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "own_collect")

	if !result.OK {
		t.Fatalf("own_collect should poll for delayed dead crops: %#v", result)
	}
	wantMethods := []string{
		"gameCtl.getFarmStatus",
		"gameCtl.harvestLandsBatchByProtocol",
		"gameCtl.getFarmStatus",
		"gameCtl.getFarmStatus",
		"gameCtl.shovelLandsBatch",
	}
	if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
		t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
	}
	wantShovelArgs := []any{map[string]any{
		"landIds":         []int{5},
		"onlyDead":        true,
		"silent":          true,
		"dryRun":          false,
		"waitAfterAction": 0,
		"betweenLandWait": 0,
		"source":          "farm_go_auto_collect_dead_cleanup",
	}}
	if !reflect.DeepEqual(caller.calls[4].args, wantShovelArgs) {
		t.Fatalf("shovel args = %#v, want %#v", caller.calls[4].args, wantShovelArgs)
	}
	if result.Message != "一键收获 1 块，清理枯萎 1 块。" {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestRuntimeFacadeOwnFertilizerSkipsWhenNoRushTargets(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(7), "stageKind": "empty"},
				map[string]any{"landId": float64(8), "stageKind": "growing", "hasPlant": true, "plantId": float64(20002), "matureInSec": float64(600)},
			},
		},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{"autoFarmFertilizerEnabled": true})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("no rush targets should be an OK skip, got %#v", result)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("no-op fertilizer should not call batch runtime, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeOwnFertilizerRequiresOwnFarmStatus(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{"farmType": "friend"},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{"autoFarmFertilizerEnabled": true})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if result.OK {
		t.Fatalf("friend farm should not be fertilized by own_fertilizer: %#v", result)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, StatusFailed)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("friend farm should not call fertilizer batch, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeOwnFertilizerReportsStatusFailure(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": errors.New("runtime link is not connected"),
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{"autoFarmFertilizerEnabled": true})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if result.OK {
		t.Fatalf("status failure should not be OK: %#v", result)
	}
	if result.Status != StatusRuntimeNotReady {
		t.Fatalf("status = %q, want %q", result.Status, StatusRuntimeNotReady)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("expected one status call, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeOwnFertilizerReportsBatchFailure(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "stageKind": "growing", "hasPlant": true, "plantId": float64(20002), "matureInSec": float64(120)},
			},
		},
		"gameCtl.fertilizeLandsBatch": errors.New("fertilizer method not ready"),
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{"autoFarmFertilizerEnabled": true})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if result.OK {
		t.Fatalf("fertilizer failure should not be OK: %#v", result)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, StatusFailed)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("expected status and fertilizer calls, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeOwnFertilizerReportsRuntimeNotOKResult(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "stageKind": "growing", "hasPlant": true, "plantId": float64(20002), "matureInSec": float64(120)},
			},
		},
		"gameCtl.fertilizeLandsBatch": map[string]any{"ok": false, "reason": "out_of_stock"},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{"autoFarmFertilizerEnabled": true})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if result.OK {
		t.Fatalf("explicit fertilizer ok=false should not be OK: %#v", result)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, StatusFailed)
	}
}

func TestRuntimeFacadeOwnFertilizerTreatsAlreadyUsedBatchAsSkip(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "stageKind": "growing", "hasPlant": true, "plantId": float64(20002), "matureInSec": float64(120)},
			},
		},
		"gameCtl.fertilizeLandsBatch": map[string]any{
			"ok":     false,
			"action": "skipped",
			"reason": "same_fertilizer_type_already_used",
			"results": []any{map[string]any{
				"ok":     false,
				"action": "skipped",
				"reason": "same_fertilizer_type_already_used",
				"landId": float64(8),
			}},
		},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{"autoFarmFertilizerEnabled": true})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("already-used fertilizer should be an OK skip: %#v", result)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("expected status and fertilizer calls, got %#v", caller.calls)
	}
	if !strings.Contains(result.Message, "本季已施肥跳过 1 块") {
		t.Fatalf("skip message = %q", result.Message)
	}
}

func TestRuntimeFacadeLandUpgradeUpgradesHighestPriorityOwnFarmLand(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "couldUpgrade": true, "landType": "normal"},
				map[string]any{"landId": float64(3), "couldUpgrade": true, "landType": "black"},
				map[string]any{"landId": float64(1), "couldUpgrade": true, "landType": "black"},
				map[string]any{"landId": float64(2), "couldUpgrade": false, "landType": "gold"},
			},
		},
		"gameCtl.upgradeLandByProtocol": map[string]any{"ok": true, "success": true},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "land_upgrade")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("land_upgrade should report OK, got %#v", result)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("runtime call count = %d, want 2: %#v", len(caller.calls), caller.calls)
	}
	if caller.calls[1].method != "gameCtl.upgradeLandByProtocol" {
		t.Fatalf("second method = %q, want upgradeLandByProtocol", caller.calls[1].method)
	}
	wantArgs := []any{map[string]any{
		"landId": 1,
		"dryRun": false,
		"waitMs": 800,
		"silent": true,
		"source": "farm_go_auto_land_upgrade",
	}}
	if !reflect.DeepEqual(caller.calls[1].args, wantArgs) {
		t.Fatalf("upgrade args = %#v, want %#v", caller.calls[1].args, wantArgs)
	}
}

func TestRuntimeFacadeLandUpgradeSkipsWhenNoUpgradeableLand(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "couldUpgrade": false, "landType": "normal"},
			},
		},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "land_upgrade")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("no upgradeable lands should be an OK skip, got %#v", result)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("skip should not call upgrade runtime, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeLandUpgradeRequiresOwnFarmStatus(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{"farmType": "friend"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "land_upgrade")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("friend farm should fail land_upgrade, got %#v", result)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("friend farm should not call upgrade runtime, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeLandUpgradeReportsStatusFailure(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": errors.New("runtime link is not connected"),
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "land_upgrade")

	if result.OK || result.Status != StatusRuntimeNotReady {
		t.Fatalf("status failure should be runtime_not_ready, got %#v", result)
	}
}

func TestRuntimeFacadeLandUpgradeReportsRuntimeNotOKResult(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{
				map[string]any{"landId": float64(8), "couldUpgrade": true, "landType": "normal"},
			},
		},
		"gameCtl.upgradeLandByProtocol": map[string]any{"ok": false, "reason": "gold_insufficient"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "land_upgrade")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("explicit land upgrade ok=false should fail, got %#v", result)
	}
}
