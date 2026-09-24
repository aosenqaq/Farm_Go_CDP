package automation

import (
	"Farm_Go/internal/farm"
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	ownPlantRuntimeBaseTimeout   = 60 * time.Second
	ownPlantRuntimeMaxTimeout    = 15 * time.Minute
	ownPlantRuntimeResponseGrace = time.Second
	ownPlantRandomDelayMaxMs     = 10_000
)

func (r RuntimeFacade) runOwnBase(ctx context.Context) ActionResult {
	const taskID = "own_base"
	if r.caller == nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "游戏运行时尚未连接，无法执行自家基础任务。",
		}
	}

	ownershipValue, err := r.caller.Call(ctx, "gameCtl.getFarmOwnership", []any{map[string]any{
		"allowWeakUi": true,
		"silent":      true,
	}}, 10*time.Second)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "游戏运行时尚未就绪：" + err.Error(),
		}
	}
	ownership := mapFromAny(ownershipValue)
	if farmType, _ := ownership["farmType"].(string); farmType != "" && farmType != "own" {
		enterResult, enterErr := r.caller.Call(ctx, "gameCtl.enterOwnFarm", []any{map[string]any{
			"waitMs":                100,
			"includeAfterOwnership": true,
			"silent":                true,
		}}, 20*time.Second)
		if enterErr != nil {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: "进入自家农场失败：" + enterErr.Error(),
			}
		}
		if failed, reason := runtimeResultFailed(enterResult); failed {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: "进入自家农场返回失败：" + reason,
			}
		}
	}

	value, err := r.caller.Call(ctx, "gameCtl.getFarmStatus", []any{farmStatusArgs()}, 15*time.Second)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "读取农场状态失败：" + err.Error(),
		}
	}
	status := mapFromAny(value)
	if farmType, _ := status["farmType"].(string); farmType != "" && farmType != "own" {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "当前不在自家农场，已阻止自家基础任务。",
		}
	}

	actions := 0
	careCount := collectCareWorkCount(status)
	if careCount > 0 {
		careResult, careErr := r.caller.Call(ctx, "gameCtl.triggerOneClickOperation", []any{"FARM_WORK", map[string]any{
			"silent":        true,
			"includeBefore": false,
			"includeAfter":  false,
			"source":        "farm_go_auto_base",
		}}, 30*time.Second)
		if careErr != nil {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: "自家基础照料调用失败：" + careErr.Error(),
			}
		}
		if failed, reason := runtimeResultFailed(careResult); failed {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: "自家基础照料返回失败：" + reason,
			}
		}
		actions++
	}

	if actions == 0 {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "没有检测到可执行的浇水、除草或杀虫任务，本轮一键务农跳过。",
		}
	}

	return ActionResult{
		OK:          true,
		Status:      StatusOK,
		TaskID:      taskID,
		Message:     fmt.Sprintf("一键务农已执行：照料 %d 项。", careCount),
		ActionCount: 1,
	}
}

func (r RuntimeFacade) runOwnCollect(ctx context.Context) ActionResult {
	const taskID = "own_collect"
	if r.caller == nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "游戏运行时尚未连接，无法执行自动收获。",
		}
	}

	value, err := r.caller.Call(ctx, "gameCtl.getFarmStatus", []any{farmStatusArgs()}, 15*time.Second)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "读取农场状态失败：" + err.Error(),
		}
	}
	status := mapFromAny(value)
	if farmType, _ := status["farmType"].(string); farmType != "" && farmType != "own" {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "当前不在自家农场，已阻止自动收获。",
		}
	}

	landIDs := collectHarvestLandIDs(status)
	preHarvestDeadLandIDs := collectDeadLandIDs(status)
	harvestCount := 0
	shovelCount := 0
	if len(landIDs) == 0 && len(preHarvestDeadLandIDs) == 0 {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "没有检测到可收获或枯萎土地，本轮自动收获跳过。",
		}
	}

	deadLandIDs := preHarvestDeadLandIDs
	if len(landIDs) > 0 {
		harvestResult, err := r.caller.Call(ctx, "gameCtl.harvestLandsBatchByProtocol", []any{landIDs, map[string]any{
			"silent":        true,
			"closeAfter":    true,
			"waitReplyMs":   600,
			"source":        "farm_go_auto_collect",
			"betweenLandMs": 120,
		}}, 45*time.Second)
		if err != nil {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: "自动收获调用失败：" + err.Error(),
			}
		}
		if failed, reason := runtimeResultFailed(harvestResult); failed {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: "自动收获返回失败：" + reason,
			}
		}
		harvestCount = runtimeSuccessfulOperationCount(harvestResult)
		successfulHarvestLandIDs := SuccessfulRuntimeLandIDs(harvestResult)

		// Harvest results can land slightly after the protocol returns. Rescan
		// once immediately, then briefly poll so newly withered crops are
		// cleared in this same auto-harvest round instead of waiting for the
		// next scheduler tick.
		postHarvestStatus, err := r.scanOwnFarmStatusAfterHarvest(ctx, taskID)
		if err != nil {
			return ActionResult{
				OK:      false,
				Status:  StatusRuntimeNotReady,
				TaskID:  taskID,
				Message: "收获后读取枯萎地块失败：" + err.Error(),
			}
		}
		deadLandIDs = mergeUniquePositiveInts(preHarvestDeadLandIDs, collectDeadLandIDs(postHarvestStatus))

		if len(deadLandIDs) > 0 {
			count, failed := r.shovelDeadOwnLands(ctx, taskID, deadLandIDs, "farm_go_auto_collect_dead_cleanup", "自动收获清理枯萎")
			if failed != nil {
				return *failed
			}
			shovelCount = count
		}

		if _, failed := r.fertilizeMultiSeasonAfterHarvest(ctx, taskID, postHarvestStatus, successfulHarvestLandIDs, "farm_go_auto_collect_multi_season"); failed != nil {
			return *failed
		}
		deadLandIDs = nil
	}

	if len(deadLandIDs) > 0 {
		count, failed := r.shovelDeadOwnLands(ctx, taskID, deadLandIDs, "farm_go_auto_collect_dead_cleanup", "自动收获清理枯萎")
		if failed != nil {
			return *failed
		}
		shovelCount = count
	}

	actionCount := 0
	if harvestCount > 0 {
		actionCount = 1
	}
	return ActionResult{
		OK:          true,
		Status:      StatusOK,
		TaskID:      taskID,
		Message:     fmt.Sprintf("一键收获 %d 块，清理枯萎 %d 块。", harvestCount, shovelCount),
		ActionCount: actionCount,
	}
}

func (r RuntimeFacade) scanOwnFarmStatusAfterHarvest(ctx context.Context, taskID string) (map[string]any, error) {
	_ = taskID
	// Harvest protocol returns as soon as requests are dispatched; dead state
	// often appears a moment later. One settle + rescan is required, and one
	// short retry covers delayed wither transitions without waiting for the
	// next scheduler round.
	if err := sleepWithContext(ctx, 300*time.Millisecond); err != nil {
		return nil, err
	}

	args := farmStatusArgs()
	args["forceRefresh"] = true
	statusValue, err := r.caller.Call(ctx, "gameCtl.getFarmStatus", []any{args}, 15*time.Second)
	if err != nil {
		return nil, err
	}
	status := mapFromAny(statusValue)
	if len(collectDeadLandIDs(status)) > 0 {
		return status, nil
	}

	if err := sleepWithContext(ctx, 350*time.Millisecond); err != nil {
		return nil, err
	}
	statusValue, err = r.caller.Call(ctx, "gameCtl.getFarmStatus", []any{args}, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return mapFromAny(statusValue), nil
}

func sleepWithContext(ctx context.Context, wait time.Duration) error {
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (r RuntimeFacade) shovelDeadOwnLands(ctx context.Context, taskID string, landIDs []int, source string, label string) (int, *ActionResult) {
	shovelResult, shovelErr := r.caller.Call(ctx, "gameCtl.shovelLandsBatch", []any{map[string]any{
		"landIds":         landIDs,
		"onlyDead":        true,
		"silent":          true,
		"dryRun":          false,
		"waitAfterAction": 0,
		"betweenLandWait": 0,
		"source":          source,
	}}, 45*time.Second)
	if shovelErr != nil {
		return 0, &ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: label + "调用失败：" + shovelErr.Error(),
		}
	}
	if failed, reason := runtimeResultFailed(shovelResult); failed {
		return 0, &ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: label + "返回失败：" + reason,
		}
	}
	return runtimeSuccessfulOperationCount(shovelResult), nil
}

func (r RuntimeFacade) fertilizeMultiSeasonAfterHarvest(ctx context.Context, taskID string, status map[string]any, harvestedLandIDs []int, source string) (int, *ActionResult) {
	mode, smart := plantFertilizerStrategy(r.config)
	if !boolFromAny(r.config["autoFarmFertilizerMultiSeason"]) {
		mode = ""
	}
	landIDs := MultiSeasonContinuationLandIDs(status, harvestedLandIDs)
	if smart && len(landIDs) > 0 {
		index, err := farm.LoadSmartFertilizerCropIndex(farm.DefaultGameConfigRoot())
		if err != nil {
			return 0, nil
		}
		landIDs = smartFertilizerFallbackLandIDs(status, landIDs, index)
	}
	if mode == "" || len(landIDs) == 0 {
		return 0, nil
	}

	payload := ApplyFertilizerSubmissionRuntimeArgs(map[string]any{
		"landIds":                     landIDs,
		"type":                        mode,
		"mode":                        mode,
		"dryRun":                      false,
		"cleanupUi":                   true,
		"linkedHarvestAfterFertilize": false,
		"silent":                      true,
		"source":                      source,
	}, r.config, "planting")
	value, err := r.caller.Call(ctx, "gameCtl.fertilizeLandsBatch", []any{payload}, 45*time.Second)
	if err != nil {
		return 0, &ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "多季补肥调用失败：" + err.Error(),
		}
	}
	if failed, reason := runtimeResultFailed(value); failed {
		return 0, &ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "多季补肥返回失败：" + reason,
		}
	}
	return runtimeSuccessfulOperationCount(value), nil
}

func (r RuntimeFacade) runOwnPlant(ctx context.Context) ActionResult {
	const taskID = "own_plant"
	if r.caller == nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "游戏运行时尚未连接，无法执行自动种植。",
		}
	}

	if len(ownPlantModeCandidates(r.config)) == 0 && r.config != nil {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "自动种植策略未启用，本轮自动种植跳过。",
		}
	}

	if failed := r.ensureOwnFarmForPlant(ctx); failed != nil {
		return *failed
	}

	emptyLandIDs, knownEmptyLands, failed := r.loadOwnPlantEmptyLandIDs(ctx)
	if failed != nil {
		return *failed
	}
	if knownEmptyLands && len(emptyLandIDs) == 0 {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "没有检测到可种植空地，本轮自动种植跳过。",
		}
	}

	plantPlans, failed := r.resolveOwnPlantPlans(ctx, emptyLandIDs)
	if failed != nil {
		return *failed
	}
	submittedPlans := 0
	skippedFourGridPlans := 0
	reclaimedLandIDs := []int{}
	for _, plantOpts := range plantPlans {
		if boolConfigAny(plantOpts["waitForFourGridFallback"]) && len(reclaimedLandIDs) == 0 {
			continue
		}
		if len(reclaimedLandIDs) > 0 && intFromAny(plantOpts["plantSize"]) < 2 {
			landIDs, _ := plantOpts["emptyLandIds"].([]int)
			plantOpts["emptyLandIds"] = appendUniqueLandIDs(reclaimedLandIDs, landIDs)
			reclaimedLandIDs = nil
		}
		plantTimeout, deadlineAtMs := ownPlantRuntimeTiming(plantOpts, time.Now())
		plantOpts["plantExecutionDeadlineAtMs"] = deadlineAtMs
		plantResult, err := r.caller.Call(ctx, "gameCtl.autoPlant", []any{plantOpts}, plantTimeout)
		if err != nil {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: "自动种植调用失败：" + err.Error(),
			}
		}
		if result := mapFromAny(plantResult); len(result) > 0 && result["ok"] == false {
			reason := autoPlantFailureReason(result)
			if reason == "no_complete_multi_land_group" {
				skippedFourGridPlans++
				landIDs, _ := plantOpts["emptyLandIds"].([]int)
				reclaimedLandIDs = appendUniqueLandIDs(reclaimedLandIDs, landIDs)
				continue
			}
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: fmt.Sprintf("自动种植返回失败：%v", firstExistingAny(reason, result["message"], "unknown")),
			}
		}
		submittedPlans++
		if fertilizerMode, smart := plantFertilizerStrategy(r.config); fertilizerMode != "" {
			landIDs := plantFertilizerLandIDs(plantOpts, plantResult)
			if smart && !smartPlantFertilizerFallsBackForSeed(intFromAny(plantOpts["seedId"])) {
				landIDs = nil
			}
			if len(landIDs) > 0 {
				payload := ApplyFertilizerSubmissionRuntimeArgs(map[string]any{
					"landIds":                     landIDs,
					"type":                        fertilizerMode,
					"mode":                        fertilizerMode,
					"dryRun":                      false,
					"cleanupUi":                   true,
					"linkedHarvestAfterFertilize": false,
					"silent":                      true,
					"source":                      "farm_go_auto_plant_fertilizer",
				}, r.config, "planting")
				fertilizerResult, err := r.caller.Call(ctx, "gameCtl.fertilizeLandsBatch", []any{payload}, 45*time.Second)
				if err != nil {
					return ActionResult{
						OK:      false,
						Status:  StatusFailed,
						TaskID:  taskID,
						Message: "种植后施肥调用失败：" + err.Error(),
					}
				}
				if result := mapFromAny(fertilizerResult); len(result) > 0 && result["ok"] == false {
					return ActionResult{
						OK:      false,
						Status:  StatusFailed,
						TaskID:  taskID,
						Message: fmt.Sprintf("种植后施肥返回失败：%v", firstExistingAny(result["reason"], result["message"], "unknown")),
					}
				}
			}
		}
	}
	if submittedPlans == 0 && skippedFourGridPlans > 0 {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "没有可组成 2x2 的四格空地，本轮跳过四格种植。",
		}
	}

	return ActionResult{
		OK:      true,
		Status:  StatusOK,
		TaskID:  taskID,
		Message: fmt.Sprintf("已提交 %d 轮自动种植请求。", submittedPlans),
	}
}

func plantFertilizerMode(config map[string]any) string {
	mode, _ := plantFertilizerStrategy(config)
	return mode
}

func plantFertilizerStrategy(config map[string]any) (string, bool) {
	if !fertilizerMasterEnabled(config) {
		return "", false
	}
	switch strings.ToLower(strings.TrimSpace(stringConfigAny(config["autoFarmPlantFertilizerMode"]))) {
	case "normal":
		return "normal", false
	case "organic":
		return "organic", false
	case "smart_normal":
		return "normal", true
	case "smart_organic":
		return "organic", true
	default:
		return "", false
	}
}

func smartPlantFertilizerFallsBackForSeed(seedID int) bool {
	if seedID <= 0 {
		return false
	}
	index, err := farm.LoadSmartFertilizerCropIndex(farm.DefaultGameConfigRoot())
	return err == nil && index.Classify(0, seedID) == farm.SmartFertilizerCropFallback
}

func smartFertilizerFallbackLandIDs(status map[string]any, landIDs []int, index farm.SmartFertilizerCropIndex) []int {
	classByLandID := map[int]farm.SmartFertilizerCropClass{}
	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		landID := intFromAny(firstExistingAny(grid["occupancyAnchorLandId"], grid["landId"], grid["id"]))
		if landID <= 0 {
			continue
		}
		plantID := intFromAny(firstExistingAny(grid["plantId"], grid["plant_id"]))
		seedID := intFromAny(firstExistingAny(grid["seedId"], grid["seed_id"]))
		classByLandID[landID] = index.Classify(plantID, seedID)
	}

	filtered := make([]int, 0, len(landIDs))
	for _, landID := range landIDs {
		if classByLandID[landID] == farm.SmartFertilizerCropFallback {
			filtered = append(filtered, landID)
		}
	}
	return filtered
}

func plantFertilizerLandIDs(plantOpts map[string]any, plantResult any) []int {
	original := normalizeUniquePositiveInts(sliceFromAny(plantOpts["emptyLandIds"]))
	if intFromAny(plantOpts["plantSize"]) < 2 {
		return original
	}

	decision := mapFromAny(mapFromAny(plantResult)["fourGridPlantDecision"])
	anchors := make([]any, 0, len(sliceFromAny(decision["groups"])))
	for _, rawGroup := range sliceFromAny(decision["groups"]) {
		for _, rawLandID := range sliceFromAny(rawGroup) {
			if landID := intFromAny(rawLandID); landID > 0 {
				anchors = append(anchors, landID)
				break
			}
		}
	}
	if normalized := normalizeUniquePositiveInts(anchors); len(normalized) > 0 {
		return normalized
	}
	return original
}

func fertilizerMasterEnabled(config map[string]any) bool {
	return boolConfigAny(config["autoFarmFertilizerEnabled"])
}

func (r RuntimeFacade) loadOwnPlantEmptyLandIDs(ctx context.Context) ([]int, bool, *ActionResult) {
	const taskID = "own_plant"
	value, err := r.caller.Call(ctx, "gameCtl.getFarmStatus", []any{farmStatusArgs()}, 15*time.Second)
	if err != nil {
		return nil, false, &ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "读取农场状态失败：" + err.Error(),
		}
	}
	status := mapFromAny(value)
	if farmType, _ := status["farmType"].(string); farmType != "" && farmType != "own" {
		return nil, false, &ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "当前不在自家农场，已阻止自动种植。",
		}
	}
	return collectPlantingEmptyLandIDs(
		status,
		boolConfigAny(r.config["autoFarmFourGridPlantEnabled"]),
	), true, nil
}

func (r RuntimeFacade) ensureOwnFarmForPlant(ctx context.Context) *ActionResult {
	const taskID = "own_plant"
	ownershipValue, err := r.caller.Call(ctx, "gameCtl.getFarmOwnership", []any{map[string]any{
		"allowWeakUi": true,
		"silent":      true,
	}}, 10*time.Second)
	if err != nil {
		return &ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "游戏运行时尚未就绪：" + err.Error(),
		}
	}
	ownership := mapFromAny(ownershipValue)
	if farmType, _ := ownership["farmType"].(string); farmType == "" || farmType == "own" {
		return nil
	}
	enterResult, enterErr := r.caller.Call(ctx, "gameCtl.enterOwnFarm", []any{map[string]any{
		"waitMs":                100,
		"includeAfterOwnership": true,
		"silent":                true,
	}}, 20*time.Second)
	if enterErr != nil {
		return &ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "进入自家农场失败：" + enterErr.Error(),
		}
	}
	if failed, reason := runtimeResultFailed(enterResult); failed {
		return &ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "进入自家农场返回失败：" + reason,
		}
	}
	return nil
}

func (r RuntimeFacade) resolveOwnPlantOptions(ctx context.Context, landIDs []int) (map[string]any, *ActionResult) {
	plans, failed := r.resolveOwnPlantPlans(ctx, landIDs)
	if failed != nil {
		return nil, failed
	}
	if len(plans) == 0 {
		return nil, &ActionResult{OK: false, Status: StatusFailed, TaskID: "own_plant", Message: "自动种植策略解析失败：没有可用的自动种植策略"}
	}
	return plans[0], nil
}

func (r RuntimeFacade) resolveOwnPlantPlans(ctx context.Context, landIDs []int) ([]map[string]any, *ActionResult) {
	const taskID = "own_plant"
	candidates := ownPlantModeCandidates(r.config)
	if len(candidates) == 0 {
		if r.config != nil {
			return nil, &ActionResult{
				OK:      true,
				Status:  StatusOK,
				TaskID:  taskID,
				Message: "自动种植策略未启用，本轮自动种植跳过。",
			}
		}
		const defaultSeedID = 20002
		payload := r.baseOwnPlantOptions("specified_seed", landIDs)
		payload["seedId"] = defaultSeedID
		payload["autoPlantSeedId"] = defaultSeedID
		return []map[string]any{payload}, nil
	}

	var lastReason string
	remaining := append([]int(nil), landIDs...)
	plans := []map[string]any{}
	for _, mode := range candidates {
		if len(remaining) == 0 {
			break
		}
		if mode == "backpack_first" {
			backpackPlans, leftover, reason := r.resolveBackpackFirstPlantPlans(ctx, remaining)
			if len(backpackPlans) > 0 {
				plans = append(plans, backpackPlans...)
				remaining = leftover
				continue
			}
			if reason != "" {
				lastReason = reason
			}
			continue
		}
		payload, reason := r.resolveOwnPlantOptionsForMode(ctx, mode, remaining)
		if payload != nil {
			plans = append(plans, payload)
			remaining = nil
			break
		}
		if reason != "" {
			lastReason = reason
		}
	}
	if len(plans) > 0 {
		return plans, nil
	}
	if lastReason == "" {
		lastReason = "没有可用的自动种植策略"
	}
	return nil, &ActionResult{
		OK:      false,
		Status:  StatusFailed,
		TaskID:  taskID,
		Message: "自动种植策略解析失败：" + lastReason,
	}
}

func (r RuntimeFacade) resolveOwnPlantOptionsForMode(ctx context.Context, mode string, landIDs []int) (map[string]any, string) {
	switch mode {
	case "backpack_first":
		return r.resolveBackpackFirstPlantOptions(ctx, landIDs)
	case "specified_seed":
		seedID := intFromAny(r.config["autoFarmPlantSeedId"])
		if seedID <= 0 {
			return nil, "指定种子策略缺少种子 ID"
		}
		payload := r.baseOwnPlantOptions(mode, landIDs)
		payload["seedId"] = seedID
		payload["autoPlantSeedId"] = seedID
		return payload, ""
	case "highest_level", "max_exp", "max_fert_exp", "max_profit", "max_fert_profit":
		return r.resolveAnalyticPlantOptions(ctx, mode, landIDs)
	case "buy_highest", "buy_lowest":
		return r.baseOwnPlantOptions(mode, landIDs), ""
	default:
		return nil, "未知策略 " + mode
	}
}

func (r RuntimeFacade) resolveBackpackFirstPlantOptions(ctx context.Context, landIDs []int) (map[string]any, string) {
	plans, _, reason := r.resolveBackpackFirstPlantPlans(ctx, landIDs)
	if len(plans) == 0 {
		return nil, reason
	}
	return plans[0], ""
}

func (r RuntimeFacade) resolveBackpackFirstPlantPlans(ctx context.Context, landIDs []int) ([]map[string]any, []int, string) {
	value, err := r.caller.Call(ctx, "gameCtl.getSeedList", []any{map[string]any{
		"sortMode":     3,
		"silent":       true,
		"protocolOnly": true,
	}}, 8*time.Second)
	if err != nil {
		return nil, landIDs, "读取背包种子失败：" + err.Error()
	}
	options := farm.BuildBackpackSeedOptions(sliceFromAny(value), farm.BackpackSeedOptionsRequest{
		SelectedSeedIDs:      normalizeUniquePositiveInts(sliceFromAny(r.config["autoFarmPlantBackpackSeedPriority"])),
		DisabledSeedIDs:      normalizeUniquePositiveInts(sliceFromAny(r.config["autoFarmPlantBackpackSeedDisabled"])),
		ForcePriority:        boolConfigAny(r.config["autoFarmPlantBackpackForcePriority"]),
		FourGridPlantEnabled: boolConfigAny(r.config["autoFarmFourGridPlantEnabled"]),
	})
	if !options.OK {
		return nil, landIDs, firstNonEmptyText(options.Error, "背包种子列表不可用")
	}
	remaining := append([]int(nil), landIDs...)
	reservedFourGridLandIDs := []int{}
	plans := []map[string]any{}
	for _, option := range options.List {
		if option.SeedID <= 0 || option.Disabled || !option.Plantable || option.BackpackCount <= 0 {
			continue
		}
		if len(remaining) == 0 && len(reservedFourGridLandIDs) == 0 {
			break
		}
		landCount := option.BackpackCount
		if option.PlantSize >= 2 {
			landCount *= option.PlantSize * option.PlantSize
		}
		if landCount > len(remaining) {
			landCount = len(remaining)
		}
		batchLandIDs := append([]int(nil), remaining[:landCount]...)
		remaining = remaining[landCount:]
		payload := r.baseOwnPlantOptions("backpack_first", batchLandIDs)
		payload["seedId"] = option.SeedID
		payload["autoPlantSeedId"] = option.SeedID
		payload["seedName"] = option.Name
		payload["backpackCount"] = option.BackpackCount
		payload["runtimeSeedId"] = option.SeedID
		payload["itemId"] = option.SeedID
		payload["plantSize"] = option.PlantSize
		if option.PlantSize >= 2 {
			reservedFourGridLandIDs = appendUniqueLandIDs(reservedFourGridLandIDs, batchLandIDs)
		} else if len(batchLandIDs) == 0 && len(reservedFourGridLandIDs) > 0 {
			payload["waitForFourGridFallback"] = true
		}
		plans = append(plans, payload)
	}
	if len(plans) > 0 {
		return plans, remaining, ""
	}
	return nil, landIDs, "背包没有未禁用的可用种子"
}

func (r RuntimeFacade) resolveAnalyticPlantOptions(ctx context.Context, mode string, landIDs []int) (map[string]any, string) {
	var profile map[string]any
	if value, err := r.caller.Call(ctx, "gameCtl.getPlayerProfile", []any{map[string]any{"silent": true}}, 8*time.Second); err == nil {
		profile = mapFromAny(value)
	}

	seedValue, seedErr := r.caller.Call(ctx, "gameCtl.getSeedList", []any{map[string]any{
		"sortMode":     3,
		"silent":       true,
		"protocolOnly": true,
	}}, 8*time.Second)
	if seedErr != nil {
		return nil, "读取背包种子失败：" + seedErr.Error()
	}
	seedList := sliceFromAny(seedValue)

	_, _ = r.caller.Call(ctx, "gameCtl.requestShopData", []any{2}, 8*time.Second)
	levelInfo := farm.NormalizeAnalyticsLevelRequest(intFromAny(r.config["autoFarmPlantMaxLevel"]), profile)
	var shopList []any
	if value, err := r.caller.Call(ctx, "gameCtl.getShopSeedList", []any{map[string]any{
		"sortByLevel": true,
		"silent":      true,
		"playerLevel": levelInfo.EffectiveMaxLevel,
		"n":           true,
	}}, 8*time.Second); err == nil {
		shopList = sliceFromAny(value)
	}

	analytics, err := farm.LoadCropAnalyticsForLevel(farm.DefaultGameConfigRoot(), farm.CropAnalyticsOptions{
		Sort:              analyticSortKey(mode),
		RequestedMaxLevel: levelInfo.RequestedMaxLevel,
		Profile:           profile,
		SeedList:          seedList,
		ShopList:          shopList,
	})
	if err != nil {
		return nil, err.Error()
	}
	for _, card := range analytics.Recommendations {
		if card.Value != mode || card.CurrentRecommended == nil {
			continue
		}
		if card.CurrentSource != "backpack" && card.CurrentSource != "shop" {
			return nil, "策略 " + mode + " 当前没有背包或商店可用种子"
		}
		seedID := card.CurrentRecommended.SeedID
		if seedID <= 0 {
			return nil, "策略 " + mode + " 未解析到种子"
		}
		payload := r.baseOwnPlantOptions(mode, landIDs)
		payload["seedId"] = seedID
		payload["autoPlantSeedId"] = seedID
		payload["seedName"] = card.CurrentRecommended.Name
		payload["plantId"] = card.CurrentRecommended.ID
		payload["plantSize"] = card.CurrentRecommended.PlantSize
		payload["backpackCount"] = seedCountFromRuntimeList(seedList, seedID)
		if shop := seedRuntimeItem(shopList, seedID); shop != nil {
			if goodsID := intFromAny(firstExistingAny(shop["goodsId"], shop["goods_id"])); goodsID > 0 {
				payload["shopGoodsId"] = goodsID
			}
			if price := intFromAny(shop["price"]); price > 0 {
				payload["shopPrice"] = price
			}
			if priceID := intFromAny(firstExistingAny(shop["priceId"], shop["price_id"])); priceID > 0 {
				payload["shopPriceId"] = priceID
			}
		}
		return payload, ""
	}
	return nil, "策略 " + mode + " 未产生推荐作物"
}

func (r RuntimeFacade) baseOwnPlantOptions(mode string, landIDs []int) map[string]any {
	payload := map[string]any{
		"mode":             mode,
		"protocolOnly":     true,
		"n":                true,
		"silent":           true,
		"waitAfterPlantMs": 300,
	}
	if len(landIDs) > 0 {
		payload["emptyLandIds"] = landIDs
	}
	payload["fourGridPlantEnabled"] = boolConfigAny(r.config["autoFarmFourGridPlantEnabled"])
	payload["autoPlantBackpackSeedPriority"] = normalizeUniquePositiveInts(sliceFromAny(r.config["autoFarmPlantBackpackSeedPriority"]))
	payload["autoPlantBackpackSeedDisabled"] = normalizeUniquePositiveInts(sliceFromAny(r.config["autoFarmPlantBackpackSeedDisabled"]))
	payload["autoPlantBackpackForcePriority"] = boolConfigAny(r.config["autoFarmPlantBackpackForcePriority"])
	payload["autoPlantRandomizedEnabled"] = boolConfigAny(r.config["autoFarmPlantRandomizedEnabled"])
	payload["autoPlantRandomOrderEnabled"] = boolConfigAny(r.config["autoFarmPlantRandomOrderEnabled"])
	payload["autoPlantRandomizedDelayMinMs"] = intFromAny(r.config["autoFarmPlantRandomizedDelayMinMs"])
	payload["autoPlantRandomizedDelayMaxMs"] = intFromAny(r.config["autoFarmPlantRandomizedDelayMaxMs"])
	return payload
}

func ownPlantRuntimeTiming(plantOpts map[string]any, now time.Time) (time.Duration, int64) {
	timeout := ownPlantRuntimeBaseTimeout
	landIDs := normalizeUniquePositiveInts(sliceFromAny(plantOpts["emptyLandIds"]))
	if !boolConfigAny(plantOpts["autoPlantRandomizedEnabled"]) || len(landIDs) < 2 {
		return timeout, ownPlantExecutionDeadlineAtMs(now, timeout)
	}

	maxDelayMs := intFromAny(plantOpts["autoPlantRandomizedDelayMaxMs"])
	if maxDelayMs < 0 {
		maxDelayMs = 0
	}
	if maxDelayMs > ownPlantRandomDelayMaxMs {
		maxDelayMs = ownPlantRandomDelayMaxMs
	}
	if maxDelayMs == 0 {
		return timeout, ownPlantExecutionDeadlineAtMs(now, timeout)
	}

	maxExtra := ownPlantRuntimeMaxTimeout - ownPlantRuntimeBaseTimeout
	maxIntervals := int64(maxExtra / (time.Duration(maxDelayMs) * time.Millisecond))
	intervals := int64(len(landIDs) - 1)
	if intervals >= maxIntervals {
		timeout = ownPlantRuntimeMaxTimeout
	} else {
		timeout += time.Duration(intervals*int64(maxDelayMs)) * time.Millisecond
	}
	return timeout, ownPlantExecutionDeadlineAtMs(now, timeout)
}

func ownPlantExecutionDeadlineAtMs(now time.Time, timeout time.Duration) int64 {
	executionTimeout := timeout - ownPlantRuntimeResponseGrace
	if executionTimeout < 0 {
		executionTimeout = 0
	}
	return now.Add(executionTimeout).UnixMilli()
}

func ownPlantModeCandidates(config map[string]any) []string {
	primary := firstNonEmptyText(
		stringConfigAny(config["autoFarmPlantPrimaryMode"]),
		stringConfigAny(config["autoFarmPlantMode"]),
	)
	secondary := stringConfigAny(config["autoFarmPlantSecondaryMode"])
	seen := map[string]bool{}
	result := []string{}
	for _, mode := range []string{primary, secondary} {
		mode = strings.TrimSpace(mode)
		if mode == "" || mode == "none" || seen[mode] {
			continue
		}
		seen[mode] = true
		result = append(result, mode)
	}
	return result
}

func analyticSortKey(mode string) string {
	switch mode {
	case "highest_level":
		return "level"
	case "max_fert_exp":
		return "fert_exp"
	case "max_profit":
		return "profit"
	case "max_fert_profit":
		return "fert_profit"
	default:
		return "exp"
	}
}

func seedCountFromRuntimeList(items []any, seedID int) int {
	if item := seedRuntimeItem(items, seedID); item != nil {
		return intFromAny(firstExistingAny(item["count"], item["stock"], item["inventory"], item["quantity"]))
	}
	return 0
}

func seedRuntimeItem(items []any, seedID int) map[string]any {
	for _, raw := range items {
		item := mapFromAny(raw)
		if firstPositiveRuntimeInt(item["seedId"], item["itemId"], item["id"], item["seed_id"]) == seedID {
			return item
		}
	}
	return nil
}

func firstPositiveRuntimeInt(values ...any) int {
	for _, value := range values {
		if result := intFromAny(value); result > 0 {
			return result
		}
	}
	return 0
}

func boolConfigAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "on", "yes":
			return true
		}
		return false
	case int:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func stringConfigAny(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func autoPlantFailureReason(result map[string]any) string {
	return firstNonEmptyText(
		stringConfigAny(result["reason"]),
		stringConfigAny(mapFromAny(result["plantResult"])["reason"]),
		stringConfigAny(mapFromAny(result["fourGridPlantDecision"])["reason"]),
	)
}

func appendUniqueLandIDs(prefix []int, suffix []int) []int {
	seen := map[int]bool{}
	result := make([]int, 0, len(prefix)+len(suffix))
	for _, landIDs := range [][]int{prefix, suffix} {
		for _, landID := range landIDs {
			if landID <= 0 || seen[landID] {
				continue
			}
			seen[landID] = true
			result = append(result, landID)
		}
	}
	return result
}

func ownPlantSuccessMessage(value any) string {
	result := mapFromAny(value)
	if len(result) == 0 {
		return "自动种植请求已提交。"
	}
	if action, _ := result["action"].(string); action == "no_empty_lands" {
		return "没有检测到可种植空地，本轮自动种植跳过。"
	}
	if boolFromAny(result["skipped"]) {
		return fmt.Sprintf("自动种植本轮跳过：%v", firstExistingAny(result["reason"], "skipped"))
	}
	count := intFromAny(firstExistingAny(result["plantedCount"], result["emptyCount"], result["successCount"]))
	if count > 0 {
		return fmt.Sprintf("已提交 %d 块土地的自动种植请求。", count)
	}
	return runtimeSuccessMessage(value, "自动种植请求已提交。")
}

func (r RuntimeFacade) runOwnFertilizer(ctx context.Context) ActionResult {
	const taskID = "own_fertilizer"
	if !fertilizerMasterEnabled(r.config) {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "自动施肥总开关已关闭，本轮自动施肥跳过。",
		}
	}
	smartMode, smartEnabled := plantFertilizerStrategy(r.config)
	rushMode := strings.ToLower(strings.TrimSpace(stringConfigAny(r.config["autoFarmRushFertilizerMode"])))
	switch rushMode {
	case "none":
		rushMode = ""
	case "normal", "organic":
	default:
		rushMode = "organic"
	}
	if !smartEnabled && rushMode == "" {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "催熟策略已关闭，本轮自动施肥跳过。",
		}
	}
	rushThresholdSec := intFromAny(r.config["autoFarmFertilizerRushThresholdSec"])
	if rushThresholdSec <= 0 {
		rushThresholdSec = 300
	}
	if r.caller == nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "游戏运行时尚未连接，无法执行自动施肥。",
		}
	}

	value, err := r.caller.Call(ctx, "gameCtl.getFarmStatus", []any{farmStatusArgs()}, 15*time.Second)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "读取农场状态失败：" + err.Error(),
		}
	}
	status := mapFromAny(value)
	if farmType, _ := status["farmType"].(string); farmType != "" && farmType != "own" {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "当前不在自家农场，已阻止自动施肥。",
		}
	}

	smartLandIDs := []int(nil)
	if smartEnabled {
		if index, loadErr := farm.LoadSmartFertilizerCropIndex(farm.DefaultGameConfigRoot()); loadErr == nil {
			smartLandIDs = collectSmartFertilizerLandIDs(status, index)
		}
	}
	rushLandIDs := []int(nil)
	if rushMode != "" {
		rushLandIDs = removeLandIDs(collectFertilizerRushLandIDs(status, rushThresholdSec), smartLandIDs)
	}
	if len(smartLandIDs) == 0 && len(rushLandIDs) == 0 {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "没有检测到可施肥土地，本轮自动施肥跳过。",
		}
	}

	linkedHarvest := boolFromAny(r.config["autoFarmFertilizerHarvestLinkEnabled"])
	type fertilizerBatch struct {
		landIDs       []int
		mode          string
		linkedHarvest bool
	}
	batches := make([]fertilizerBatch, 0, 2)
	if len(smartLandIDs) > 0 {
		if rushMode == smartMode {
			batches = append(batches, fertilizerBatch{
				landIDs:       appendUniqueLandIDs(smartLandIDs, rushLandIDs),
				mode:          smartMode,
				linkedHarvest: linkedHarvest && len(rushLandIDs) > 0,
			})
			rushLandIDs = nil
		} else {
			batches = append(batches, fertilizerBatch{landIDs: smartLandIDs, mode: smartMode})
		}
	}
	if len(rushLandIDs) > 0 {
		batches = append(batches, fertilizerBatch{landIDs: rushLandIDs, mode: rushMode, linkedHarvest: linkedHarvest})
	}

	totalLandCount := 0
	skippedLandCount := 0
	linkedHarvestResult := map[string]any{}
	linkedHarvestCount := 0
	hasLinkedHarvestBatch := false
	for _, batch := range batches {
		totalLandCount += len(batch.landIDs)
		payload := ApplyFertilizerSubmissionRuntimeArgs(map[string]any{
			"landIds":                     batch.landIDs,
			"type":                        batch.mode,
			"mode":                        batch.mode,
			"dryRun":                      false,
			"cleanupUi":                   true,
			"linkedHarvestAfterFertilize": batch.linkedHarvest,
			"rushThresholdSec":            rushThresholdSec,
			"silent":                      true,
			"source":                      "farm_go_auto_fertilizer",
		}, r.config, "rush")
		fertilizerResult, err := r.caller.Call(ctx, "gameCtl.fertilizeLandsBatch", []any{payload}, 45*time.Second)
		if err != nil {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: "自动施肥调用失败：" + err.Error(),
			}
		}
		result := mapFromAny(fertilizerResult)
		if skippedCount := sameFertilizerTypeAlreadyUsedCount(fertilizerResult, len(batch.landIDs)); skippedCount > 0 {
			skippedLandCount += skippedCount
		}
		if isSameFertilizerTypeAlreadyUsed(fertilizerResult) {
			continue
		}
		if len(result) > 0 && result["ok"] == false {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: fmt.Sprintf("自动施肥返回失败：%v", firstExistingAny(result["reason"], result["message"], "unknown")),
			}
		}
		if batch.linkedHarvest {
			hasLinkedHarvestBatch = true
			linkedHarvestResult = mapFromAny(result["linkedHarvest"])
			linkedHarvestCount += runtimeSuccessfulOperationCount(linkedHarvestResult)
		}
	}
	skipPrefix := ""
	if skippedLandCount > 0 {
		skipPrefix = fmt.Sprintf("本季已施肥跳过 %d 块，", skippedLandCount)
	}

	if !hasLinkedHarvestBatch {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: fmt.Sprintf("%s已提交 %d 块土地的自动施肥请求。", skipPrefix, totalLandCount-skippedLandCount),
		}
	}

	deadStatusValue, err := r.caller.Call(ctx, "gameCtl.getFarmStatus", []any{farmStatusArgs()}, 15*time.Second)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "收获后读取枯萎地块失败：" + err.Error(),
		}
	}
	postHarvestStatus := mapFromAny(deadStatusValue)
	deadLandIDs := collectDeadLandIDs(postHarvestStatus)
	if len(deadLandIDs) > 0 {
		shovelResult, shovelErr := r.caller.Call(ctx, "gameCtl.shovelLandsBatch", []any{map[string]any{
			"landIds":         deadLandIDs,
			"onlyDead":        true,
			"silent":          true,
			"dryRun":          false,
			"waitAfterAction": 0,
			"betweenLandWait": 0,
			"source":          "farm_go_auto_fertilizer_dead_cleanup",
		}}, 45*time.Second)
		if shovelErr != nil {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: "收获后铲除枯萎调用失败：" + shovelErr.Error(),
			}
		}
		if failed, reason := runtimeResultFailed(shovelResult); failed {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: "收获后铲除枯萎返回失败：" + reason,
			}
		}
	}
	if _, failed := r.fertilizeMultiSeasonAfterHarvest(ctx, taskID, postHarvestStatus, SuccessfulRuntimeLandIDs(linkedHarvestResult), "farm_go_auto_fertilizer_multi_season"); failed != nil {
		return *failed
	}

	return ActionResult{
		OK:          true,
		Status:      StatusOK,
		TaskID:      taskID,
		Message:     fmt.Sprintf("%s已提交 %d 块土地的自动施肥请求，清理枯萎 %d 块。", skipPrefix, totalLandCount-skippedLandCount, len(deadLandIDs)),
		ActionCount: linkedHarvestCount,
	}
}

func (r RuntimeFacade) runLandUpgrade(ctx context.Context) ActionResult {
	const taskID = "land_upgrade"
	if r.caller == nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "游戏运行时尚未连接，无法执行土地自动升级。",
		}
	}

	value, err := r.caller.Call(ctx, "gameCtl.getFarmStatus", []any{farmStatusArgs()}, 15*time.Second)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "读取农场状态失败：" + err.Error(),
		}
	}
	status := mapFromAny(value)
	if farmType, _ := status["farmType"].(string); farmType != "" && farmType != "own" {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "当前不在自家农场，已阻止土地自动升级。",
		}
	}

	candidates := collectLandUpgradeCandidates(status)
	if len(candidates) == 0 {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "当前没有可升级土地，本轮土地自动升级跳过。",
		}
	}
	target := candidates[0]
	upgradeResult, err := r.caller.Call(ctx, "gameCtl.upgradeLandByProtocol", []any{map[string]any{
		"landId": target.landID,
		"dryRun": false,
		"waitMs": 800,
		"silent": true,
		"source": "farm_go_auto_land_upgrade",
	}}, 30*time.Second)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "土地自动升级调用失败：" + err.Error(),
		}
	}
	if failed, reason := runtimeResultFailed(upgradeResult); failed {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "土地自动升级返回失败：" + reason,
		}
	}

	return ActionResult{
		OK:      true,
		Status:  StatusOK,
		TaskID:  taskID,
		Message: fmt.Sprintf("已提交 %d 号土地自动升级请求。", target.landID),
	}
}
