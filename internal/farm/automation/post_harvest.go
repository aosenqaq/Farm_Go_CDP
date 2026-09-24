package automation

import (
	"sort"
	"strings"
)

func SuccessfulRuntimeLandIDs(value any) []int {
	result := mapFromAny(value)
	seen := map[int]bool{}
	for _, item := range sliceFromAny(result["results"]) {
		entry := mapFromAny(item)
		if !boolFromAny(entry["ok"]) || strings.EqualFold(strings.TrimSpace(stringConfigAny(entry["action"])), "skipped") {
			continue
		}
		if landID := intFromAny(firstExistingAny(entry["landId"], entry["land_id"])); landID > 0 {
			seen[landID] = true
		}
	}

	ids := make([]int, 0, len(seen))
	for landID := range seen {
		ids = append(ids, landID)
	}
	sort.Ints(ids)
	return ids
}

func MultiSeasonContinuationLandIDs(status map[string]any, harvestedLandIDs []int) []int {
	harvested := map[int]bool{}
	for _, landID := range harvestedLandIDs {
		if landID > 0 {
			harvested[landID] = true
		}
	}

	anchors := map[int]bool{}
	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		landID := intFromAny(firstExistingAny(grid["landId"], grid["id"]))
		anchorID := intFromAny(firstExistingAny(grid["occupancyAnchorLandId"], grid["landId"], grid["id"]))
		if anchorID <= 0 || (!harvested[landID] && !harvested[anchorID]) {
			continue
		}
		if !boolFromAny(grid["hasPlant"]) || !strings.EqualFold(strings.TrimSpace(stringConfigAny(grid["stageKind"])), "growing") || !boolFromAny(grid["isMultiSeason"]) {
			continue
		}
		if intFromAny(grid["totalSeason"]) <= 1 || intFromAny(grid["currentSeason"]) <= 1 {
			continue
		}
		anchors[anchorID] = true
	}

	ids := make([]int, 0, len(anchors))
	for landID := range anchors {
		ids = append(ids, landID)
	}
	sort.Ints(ids)
	return ids
}

func MultiSeasonFertilizerMode(config map[string]any) string {
	if !boolFromAny(config["autoFarmFertilizerMultiSeason"]) {
		return ""
	}
	return plantFertilizerMode(config)
}
