package stealrules

import (
	"fmt"
	"strconv"
)

type CropDecision struct {
	LandIDs  []int
	SkipFarm bool
}

func ApplyCropRules(config map[string]any, inspect map[string]any, landIDs []int) CropDecision {
	kept := append([]int(nil), landIDs...)
	if !boolFromAny(config["autoFarmFriendStealPlantBlacklistEnabled"]) {
		return CropDecision{LandIDs: kept}
	}
	mode := fmt.Sprint(firstExistingAny(config["autoFarmFriendStealPlantListMode"], "blacklist"))
	if mode != "blacklist" && mode != "whitelist" {
		return CropDecision{LandIDs: kept}
	}
	listKey := "autoFarmFriendStealPlantBlacklist"
	if mode == "whitelist" {
		listKey = "autoFarmFriendStealPlantWhitelist"
	}
	listed := normalizeUniquePositiveInts(sliceFromAny(config[listKey]))
	if len(listed) == 0 {
		if mode == "whitelist" {
			return CropDecision{SkipFarm: true}
		}
		return CropDecision{LandIDs: kept}
	}

	listedPlantIDs := map[int]bool{}
	for _, id := range listed {
		listedPlantIDs[id] = true
	}
	plantIDsByLandID := plantIDsByLandID(inspect)
	filtered := make([]int, 0, len(landIDs))
	blocked := 0
	for _, landID := range landIDs {
		plantID := plantIDsByLandID[landID]
		isBlocked := plantID > 0 && listedPlantIDs[plantID]
		if mode == "whitelist" {
			isBlocked = plantID > 0 && !listedPlantIDs[plantID]
		}
		if isBlocked {
			blocked++
			continue
		}
		filtered = append(filtered, landID)
	}
	if blocked == 0 {
		return CropDecision{LandIDs: kept}
	}

	strategy := intFromAny(firstExistingAny(config["autoFarmFriendStealPlantBlacklistStrategy"], 1))
	if strategy != 2 {
		return CropDecision{SkipFarm: true}
	}
	if len(filtered) == 0 {
		return CropDecision{SkipFarm: true}
	}
	return CropDecision{LandIDs: filtered}
}

func plantIDsByLandID(inspect map[string]any) map[int]int {
	result := map[int]int{}
	for _, item := range sliceFromAny(inspect["lands"]) {
		land := mapFromAny(item)
		if len(land) == 0 {
			continue
		}
		landID := intFromAny(firstExistingAny(land["landId"], land["landID"], land["id"]))
		if landID <= 0 {
			continue
		}
		plant := mapFromAny(firstExistingAny(land["plantInfo"], land["plant"], land["crop"]))
		plantID := intFromAny(firstExistingAny(
			land["plantId"],
			land["plantID"],
			land["cropId"],
			land["cropID"],
			plant["id"],
			plant["plantId"],
			plant["cropId"],
		))
		if plantID > 0 {
			result[landID] = plantID
		}
	}
	return result
}

func firstExistingAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func boolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(typed)
		return err == nil && parsed
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func mapFromAny(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return nil
}

func sliceFromAny(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []int:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	case []float64:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	case []string:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}

func normalizeUniquePositiveInts(values []any) []int {
	seen := map[int]bool{}
	result := []int{}
	for _, value := range values {
		id := intFromAny(value)
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}
