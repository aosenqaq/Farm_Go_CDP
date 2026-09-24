package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"Farm_Go/internal/farm/automation"
	"Farm_Go/internal/farm/social"
	"Farm_Go/internal/lanaccess"
	"Farm_Go/internal/storage"
)

var errLANRPCInvalidArguments = errors.New("invalid LAN RPC arguments")

type lanRPCDispatcher struct {
	handlers map[string]func(context.Context, []json.RawMessage) (any, error)
}

func newLANRPCDispatcher(app *App) lanaccess.RPCDispatcher {
	return &lanRPCDispatcher{
		handlers: map[string]func(context.Context, []json.RawMessage) (any, error){
			"CurrentRuntimeAccount": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.CurrentRuntimeAccount(), nil
			},
			"IdentifyRuntimeAccount": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.IdentifyRuntimeAccount(), nil
			},
			"ConfirmRuntimeAccount": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[RuntimeAccount](args)
				if err != nil {
					return nil, err
				}
				return app.ConfirmRuntimeAccount(input), nil
			},
			"GetProgramNotice": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.GetProgramNotice(), nil
			},
			"FarmAccountStatus": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmAccountStatus(), nil
			},
			"FarmAutomationState": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmAutomationState(), nil
			},
			"FarmAutomationSchedulerState": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmAutomationSchedulerState(), nil
			},
			"SetFarmAutomationRunMode": func(_ context.Context, args []json.RawMessage) (any, error) {
				mode, err := decodeOne[string](args)
				if err != nil {
					return nil, err
				}
				state, err := app.SetFarmAutomationRunMode(mode)
				return state, err
			},
			"RunFarmAutomationTask": func(_ context.Context, args []json.RawMessage) (any, error) {
				taskID, err := decodeOne[string](args)
				if err != nil {
					return nil, err
				}
				return app.RunFarmAutomationTask(taskID), nil
			},
			"RunDueFarmAutomationTasks": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.RunDueFarmAutomationTasks(), nil
			},
			"StartFarmAutomationScheduler": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.StartFarmAutomationScheduler(), nil
			},
			"StopFarmAutomationScheduler": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.StopFarmAutomationScheduler(), nil
			},
			"SaveFarmAutomationState": func(_ context.Context, args []json.RawMessage) (any, error) {
				state, err := decodeOne[automation.State](args)
				if err != nil {
					return nil, err
				}
				saved, err := app.SaveFarmAutomationState(state)
				return saved, err
			},
			"FarmFeatureCatalog": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmFeatureCatalog(), nil
			},
			"FarmCropAnalytics": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmCropAnalytics(), nil
			},
			"FarmAtlasPreview": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmAtlasPreview(), nil
			},
			"FarmAtlasBuyLockedPreview": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[map[string]any](args)
				if err != nil {
					return nil, err
				}
				return app.FarmAtlasBuyLockedPreview(input), nil
			},
			"FarmAtlasBuyLockedCrops": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[map[string]any](args)
				if err != nil {
					return nil, err
				}
				return app.FarmAtlasBuyLockedCrops(input), nil
			},
			"FarmFertilizeLand": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[map[string]any](args)
				if err != nil {
					return nil, err
				}
				return app.FarmFertilizeLand(input), nil
			},
			"FarmLandRush": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[map[string]any](args)
				if err != nil {
					return nil, err
				}
				return app.FarmLandRush(input), nil
			},
			"FarmShovelLands": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[map[string]any](args)
				if err != nil {
					return nil, err
				}
				return app.FarmShovelLands(input), nil
			},
			"FarmBackpackSeedOptions": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[map[string]any](args)
				if err != nil {
					return nil, err
				}
				return app.FarmBackpackSeedOptions(input), nil
			},
			"FarmWarehouse": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmWarehouse(), nil
			},
			"FarmWarehouseRefresh": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmWarehouseRefresh(), nil
			},
			"FarmWarehouseSell": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[map[string]any](args)
				if err != nil {
					return nil, err
				}
				return app.FarmWarehouseSell(input), nil
			},
			"FarmWarehouseSellRecords": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[map[string]any](args)
				if err != nil {
					return nil, err
				}
				return app.FarmWarehouseSellRecords(input), nil
			},
			"FarmMysteryShopPurchaseRecords": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmMysteryShopPurchaseRecords(), nil
			},
			"WarehouseAutoSellSettings": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.WarehouseAutoSellSettings(), nil
			},
			"SaveWarehouseAutoSellSettings": func(_ context.Context, args []json.RawMessage) (any, error) {
				settings, err := decodeOne[storage.WarehouseAutoSellSettings](args)
				if err != nil {
					return nil, err
				}
				saved, err := app.SaveWarehouseAutoSellSettings(settings)
				return saved, err
			},
			"FarmSocialState": func(_ context.Context, args []json.RawMessage) (any, error) {
				refresh, err := decodeOne[bool](args)
				if err != nil {
					return nil, err
				}
				return app.FarmSocialState(refresh), nil
			},
			"FarmSocialAction": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[social.FriendActionRequest](args)
				if err != nil {
					return nil, err
				}
				return app.FarmSocialAction(input), nil
			},
			"FarmSocialRankings": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[social.RankingRequest](args)
				if err != nil {
					return nil, err
				}
				return app.FarmSocialRankings(input), nil
			},
			"FarmSocialRefreshVisitors": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmSocialRefreshVisitors(), nil
			},
			"FarmSocialRankingPreferences": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmSocialRankingPreferences(), nil
			},
			"SaveFarmSocialRankingPreferences": func(_ context.Context, args []json.RawMessage) (any, error) {
				accountKey, preferences, err := decodeTwo[string, social.RankingPreferences](args)
				if err != nil {
					return nil, err
				}
				saved, err := app.SaveFarmSocialRankingPreferences(accountKey, preferences)
				return saved, err
			},
			"FarmSocialDogGuardState": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmSocialDogGuardState(), nil
			},
			"FarmSocialDogGuardAction": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[social.DogGuardActionRequest](args)
				if err != nil {
					return nil, err
				}
				return app.FarmSocialDogGuardAction(input), nil
			},
			"FarmSocialProtocolBlockList": func(_ context.Context, args []json.RawMessage) (any, error) {
				refresh, err := decodeOne[bool](args)
				if err != nil {
					return nil, err
				}
				return app.FarmSocialProtocolBlockList(refresh), nil
			},
			"FarmSocialExport": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[social.ExportRequest](args)
				if err != nil {
					return nil, err
				}
				return app.FarmSocialExport(input), nil
			},
			"FarmSocialImport": func(_ context.Context, args []json.RawMessage) (any, error) {
				input, err := decodeOne[social.ImportExportPayload](args)
				if err != nil {
					return nil, err
				}
				return app.FarmSocialImport(input), nil
			},
			"FarmStealCropOptions": func(_ context.Context, args []json.RawMessage) (any, error) {
				if err := decodeNoArgs(args); err != nil {
					return nil, err
				}
				return app.FarmStealCropOptions(), nil
			},
		},
	}
}

func (d *lanRPCDispatcher) Dispatch(ctx context.Context, method string, args []json.RawMessage) (any, error) {
	handler, ok := d.handlers[method]
	if !ok {
		return nil, lanaccess.ErrMethodNotAllowed
	}
	return handler(ctx, args)
}

func decodeNoArgs(args []json.RawMessage) error {
	if len(args) != 0 {
		return fmt.Errorf("%w: expected no arguments, got %d", errLANRPCInvalidArguments, len(args))
	}
	return nil
}

func decodeOne[T any](args []json.RawMessage) (T, error) {
	var zero T
	if len(args) != 1 {
		return zero, fmt.Errorf("%w: expected one argument, got %d", errLANRPCInvalidArguments, len(args))
	}
	return decodeLANRPCValue[T](args[0])
}

func decodeTwo[A, B any](args []json.RawMessage) (A, B, error) {
	var zeroA A
	var zeroB B
	if len(args) != 2 {
		return zeroA, zeroB, fmt.Errorf("%w: expected two arguments, got %d", errLANRPCInvalidArguments, len(args))
	}
	first, err := decodeLANRPCValue[A](args[0])
	if err != nil {
		return zeroA, zeroB, err
	}
	second, err := decodeLANRPCValue[B](args[1])
	if err != nil {
		return zeroA, zeroB, err
	}
	return first, second, nil
}

func decodeLANRPCValue[T any](raw json.RawMessage) (T, error) {
	var value T
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return value, fmt.Errorf("%w: argument must be a non-null JSON value", errLANRPCInvalidArguments)
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return value, fmt.Errorf("%w: %v", errLANRPCInvalidArguments, err)
	}
	return value, nil
}

var _ lanaccess.RPCDispatcher = (*lanRPCDispatcher)(nil)
