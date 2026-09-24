package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"Farm_Go/internal/lanaccess"
)

func TestLANRPCDispatchesWhitelistedNoArgumentMethod(t *testing.T) {
	dispatcher := newLANRPCDispatcher(newAuthorizedTestApp(t))

	if _, err := dispatcher.Dispatch(context.Background(), "FarmFeatureCatalog", nil); err != nil {
		t.Fatalf("FarmFeatureCatalog dispatch failed: %v", err)
	}
}

func TestLANRPCDecodesWarehouseSellRecordOptions(t *testing.T) {
	dispatcher := newLANRPCDispatcher(newAuthorizedTestApp(t))

	if _, err := dispatcher.Dispatch(context.Background(), "FarmWarehouseSellRecords", []json.RawMessage{json.RawMessage(`{"date":"2026-07-22","limit":20}`)}); err != nil {
		t.Fatalf("FarmWarehouseSellRecords dispatch failed: %v", err)
	}
}

func TestLANRPCDispatchesMysteryShopPurchaseRecords(t *testing.T) {
	dispatcher := newLANRPCDispatcher(newAuthorizedTestApp(t))

	if _, err := dispatcher.Dispatch(context.Background(), "FarmMysteryShopPurchaseRecords", nil); err != nil {
		t.Fatalf("FarmMysteryShopPurchaseRecords dispatch failed: %v", err)
	}
}

func TestLANRPCRegistersOnlyExplicitlyAllowedMethods(t *testing.T) {
	dispatcher := newLANRPCDispatcher(newAuthorizedTestApp(t))
	invalidArgument := []json.RawMessage{json.RawMessage(`1`)}
	methods := []string{
		"CurrentRuntimeAccount",
		"IdentifyRuntimeAccount",
		"ConfirmRuntimeAccount",
		"GetProgramNotice",
		"FarmAccountStatus",
		"FarmAutomationState",
		"FarmAutomationSchedulerState",
		"SetFarmAutomationRunMode",
		"RunFarmAutomationTask",
		"RunDueFarmAutomationTasks",
		"StartFarmAutomationScheduler",
		"StopFarmAutomationScheduler",
		"SaveFarmAutomationState",
		"FarmFeatureCatalog",
		"FarmCropAnalytics",
		"FarmAtlasPreview",
		"FarmAtlasBuyLockedPreview",
		"FarmAtlasBuyLockedCrops",
		"FarmFertilizeLand",
		"FarmLandRush",
		"FarmShovelLands",
		"FarmBackpackSeedOptions",
		"FarmWarehouse",
		"FarmWarehouseRefresh",
		"FarmWarehouseSell",
		"FarmWarehouseSellRecords",
		"FarmMysteryShopPurchaseRecords",
		"WarehouseAutoSellSettings",
		"SaveWarehouseAutoSellSettings",
		"FarmSocialState",
		"FarmSocialAction",
		"FarmSocialRankings",
		"FarmSocialRefreshVisitors",
		"FarmSocialRankingPreferences",
		"SaveFarmSocialRankingPreferences",
		"FarmSocialDogGuardState",
		"FarmSocialDogGuardAction",
		"FarmSocialProtocolBlockList",
		"FarmSocialExport",
		"FarmSocialImport",
		"FarmStealCropOptions",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			_, err := dispatcher.Dispatch(context.Background(), method, invalidArgument)
			if errors.Is(err, lanaccess.ErrMethodNotAllowed) {
				t.Fatalf("%s was not registered", method)
			}
		})
	}
}

func TestLANRPCRejectsInvalidArguments(t *testing.T) {
	dispatcher := newLANRPCDispatcher(newAuthorizedTestApp(t))
	tests := []struct {
		name   string
		method string
		args   []json.RawMessage
	}{
		{name: "extra no-argument value", method: "FarmFeatureCatalog", args: []json.RawMessage{json.RawMessage(`1`)}},
		{name: "wrong boolean type", method: "FarmSocialState", args: []json.RawMessage{json.RawMessage(`"true"`)}},
		{name: "wrong object type", method: "FarmWarehouseSell", args: []json.RawMessage{json.RawMessage(`[]`)}},
		{name: "extra two-argument value", method: "SaveFarmSocialRankingPreferences", args: []json.RawMessage{json.RawMessage(`"account"`), json.RawMessage(`{}`), json.RawMessage(`false`)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := dispatcher.Dispatch(context.Background(), test.method, test.args)
			if err == nil {
				t.Fatal("invalid arguments were accepted")
			}
			if errors.Is(err, lanaccess.ErrMethodNotAllowed) {
				t.Fatalf("%s was rejected as an unknown method: %v", test.method, err)
			}
		})
	}
}

func TestLANRPCRejectsDesktopOnlyAndUnknownMethods(t *testing.T) {
	dispatcher := newLANRPCDispatcher(newAuthorizedTestApp(t))
	methods := []string{
		"ExitApplication",
		"MinimizeToTray",
		"SaveTextFile",
		"InstallQQDebugPatch",
		"LaunchHostProcess",
		"RestartHostProcess",
		"UnlistedMethod",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			_, err := dispatcher.Dispatch(context.Background(), method, nil)
			if !errors.Is(err, lanaccess.ErrMethodNotAllowed) {
				t.Fatalf("error = %v, want lanaccess.ErrMethodNotAllowed", err)
			}
		})
	}
}
