package automation

import (
	"Farm_Go/internal/farm"
	"Farm_Go/internal/farm/social"
	"fmt"
	"sort"
	"strings"
)

func farmStatusArgs() map[string]any {
	return map[string]any{
		"includeGrids":          true,
		"includeLandIds":        true,
		"includeRawGrid":        true,
		"includeRawLandRuntime": true,
		"silent":                true,
	}
}

func collectHarvestLandIDs(status map[string]any) []int {
	seen := map[int]bool{}
	add := func(value any) {
		id := intFromAny(value)
		if id > 0 {
			seen[id] = true
		}
	}

	if workLandIDs := mapFromAny(status["workLandIds"]); len(workLandIDs) > 0 {
		for _, item := range sliceFromAny(workLandIDs["collect"]) {
			add(item)
		}
	}
	if landIDs := mapFromAny(status["landIds"]); len(landIDs) > 0 {
		for _, item := range sliceFromAny(landIDs["collect"]) {
			add(item)
		}
	}

	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		if len(grid) == 0 {
			continue
		}
		if boolFromAny(grid["canHarvest"]) || boolFromAny(grid["canCollect"]) {
			add(firstExistingAny(grid["landId"], grid["id"]))
		}
	}

	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func collectProtocolWorkLandIDs(status map[string]any, keys ...string) []int {
	seen := map[int]bool{}
	workLandIDs := mapFromAny(status["workLandIds"])
	for _, key := range keys {
		for _, item := range sliceFromAny(workLandIDs[key]) {
			id := intFromAny(item)
			if id > 0 {
				seen[id] = true
			}
		}
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func collectEmptyLandIDs(status map[string]any) []int {
	seen := map[int]bool{}
	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		if len(grid) == 0 {
			continue
		}
		stageKind, _ := grid["stageKind"].(string)
		if stageKind != "empty" || !boolFromAny(grid["interactable"]) {
			continue
		}
		id := intFromAny(firstExistingAny(grid["landId"], grid["id"]))
		if id > 0 {
			seen[id] = true
		}
	}

	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func collectPlantingEmptyLandIDs(status map[string]any, prioritizeFourGrid bool) []int {
	ids := collectEmptyLandIDs(status)
	if !prioritizeFourGrid || len(ids) < 4 {
		return ids
	}

	positionsByLandID := map[int][2]int{}
	landIDsByPosition := map[[2]int]int{}
	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		stageKind, _ := grid["stageKind"].(string)
		if stageKind != "empty" || !boolFromAny(grid["interactable"]) {
			continue
		}
		landID := intFromAny(firstExistingAny(grid["landId"], grid["id"]))
		position := mapFromAny(grid["gridPos"])
		xValue, hasX := position["x"]
		yValue, hasY := position["y"]
		if landID <= 0 || !hasX || !hasY {
			continue
		}
		positionKey := [2]int{intFromAny(xValue), intFromAny(yValue)}
		positionsByLandID[landID] = positionKey
		landIDsByPosition[positionKey] = landID
	}

	used := map[int]bool{}
	ordered := make([]int, 0, len(ids))
	for _, landID := range ids {
		position, ok := positionsByLandID[landID]
		if !ok || used[landID] {
			continue
		}
		for _, offset := range [][2]int{{0, -1}, {-1, -1}, {0, 0}, {-1, 0}} {
			minX, minY := position[0]+offset[0], position[1]+offset[1]
			group := []int{
				landIDsByPosition[[2]int{minX, minY}],
				landIDsByPosition[[2]int{minX + 1, minY}],
				landIDsByPosition[[2]int{minX, minY + 1}],
				landIDsByPosition[[2]int{minX + 1, minY + 1}],
			}
			if !completeUnusedLandGroup(group, used) {
				continue
			}
			sort.Ints(group)
			for _, groupedLandID := range group {
				used[groupedLandID] = true
				ordered = append(ordered, groupedLandID)
			}
			break
		}
	}
	for _, landID := range ids {
		if !used[landID] {
			ordered = append(ordered, landID)
		}
	}
	return ordered
}

func completeUnusedLandGroup(group []int, used map[int]bool) bool {
	seen := map[int]bool{}
	for _, landID := range group {
		if landID <= 0 || used[landID] || seen[landID] {
			return false
		}
		seen[landID] = true
	}
	return len(seen) == 4
}

func collectFertilizerRushLandIDs(status map[string]any, rushThresholdSec int) []int {
	seen := map[int]bool{}
	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		if !isFertilizerRushCandidate(grid, rushThresholdSec) {
			continue
		}
		id := intFromAny(firstExistingAny(grid["occupancyAnchorLandId"], grid["landId"], grid["id"]))
		if id > 0 {
			seen[id] = true
		}
	}

	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func collectSmartFertilizerLandIDs(status map[string]any, index farm.SmartFertilizerCropIndex) []int {
	seen := map[int]bool{}
	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		if !boolFromAny(grid["hasPlant"]) || !strings.EqualFold(strings.TrimSpace(stringConfigAny(grid["stageKind"])), "growing") || strings.TrimSpace(stringConfigAny(grid["phaseName"])) != "大叶子" {
			continue
		}
		plantID := intFromAny(firstExistingAny(grid["plantId"], grid["plant_id"]))
		seedID := intFromAny(firstExistingAny(grid["seedId"], grid["seed_id"]))
		if index.Classify(plantID, seedID) != farm.SmartFertilizerCropSmart {
			continue
		}
		if landID := intFromAny(firstExistingAny(grid["occupancyAnchorLandId"], grid["landId"], grid["id"])); landID > 0 {
			seen[landID] = true
		}
	}

	landIDs := make([]int, 0, len(seen))
	for landID := range seen {
		landIDs = append(landIDs, landID)
	}
	sort.Ints(landIDs)
	return landIDs
}

func removeLandIDs(landIDs []int, excludedLandIDs []int) []int {
	excluded := map[int]bool{}
	for _, landID := range excludedLandIDs {
		if landID > 0 {
			excluded[landID] = true
		}
	}
	result := make([]int, 0, len(landIDs))
	for _, landID := range landIDs {
		if landID > 0 && !excluded[landID] {
			result = append(result, landID)
		}
	}
	return result
}

func collectDeadLandIDs(status map[string]any) []int {
	seen := map[int]bool{}
	add := func(value any) {
		id := intFromAny(value)
		if id > 0 {
			seen[id] = true
		}
	}

	if workLandIDs := mapFromAny(status["workLandIds"]); len(workLandIDs) > 0 {
		for _, item := range sliceFromAny(workLandIDs["eraseDead"]) {
			add(item)
		}
	}
	if landIDs := mapFromAny(status["landIds"]); len(landIDs) > 0 {
		for _, item := range sliceFromAny(landIDs["eraseDead"]) {
			add(item)
		}
	}

	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		if !isDeadCropGrid(grid) {
			continue
		}
		add(firstExistingAny(grid["landId"], grid["id"]))
	}

	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func mergeUniquePositiveInts(parts ...[]int) []int {
	seen := map[int]bool{}
	for _, part := range parts {
		for _, id := range part {
			if id > 0 {
				seen[id] = true
			}
		}
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func collectCareWorkCount(status map[string]any) int {
	total := 0
	if workLandIDs := mapFromAny(status["workLandIds"]); len(workLandIDs) > 0 {
		for _, key := range []string{"water", "eraseGrass", "killBug"} {
			total += len(normalizeUniquePositiveInts(sliceFromAny(workLandIDs[key])))
		}
	}
	if landIDs := mapFromAny(status["landIds"]); len(landIDs) > 0 {
		for _, key := range []string{"water", "eraseGrass", "killBug"} {
			total += len(normalizeUniquePositiveInts(sliceFromAny(landIDs[key])))
		}
	}
	if total == 0 {
		if workCounts := mapFromAny(status["workCounts"]); len(workCounts) > 0 {
			for _, key := range []string{"water", "eraseGrass", "killBug"} {
				total += intFromAny(workCounts[key])
			}
		}
	}
	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		if len(grid) == 0 {
			continue
		}
		if boolFromAny(firstExistingAny(grid["needsWater"], grid["needWater"])) {
			total++
		}
		if boolFromAny(firstExistingAny(grid["needsEraseGrass"], grid["needWeed"], grid["needsWeed"])) {
			total++
		}
		if boolFromAny(firstExistingAny(grid["needsKillBug"], grid["needBug"])) {
			total++
		}
		if gridHasGoldenBug(grid) {
			total++
		}
	}
	return total
}

func gridHasGoldenBug(grid map[string]any) bool {
	if boolFromAny(firstExistingAny(grid["hasGoldenBug"], grid["needGoldenBug"], grid["needsGoldenBug"])) {
		return true
	}
	for _, item := range sliceFromAny(grid["socialItemIds"]) {
		if intFromAny(item) == 301101 {
			return true
		}
	}
	return false
}

func collectMischiefLandIDs(status map[string]any) []int {
	seen := map[int]bool{}
	add := func(value any) {
		id := intFromAny(value)
		if id > 0 {
			seen[id] = true
		}
	}
	if workLandIDs := mapFromAny(status["workLandIds"]); len(workLandIDs) > 0 {
		for _, item := range sliceFromAny(workLandIDs["mischief"]) {
			add(item)
		}
	}
	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		if isFriendMischiefCandidateGrid(grid) {
			add(firstExistingAny(grid["landId"], grid["id"]))
		}
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func isFriendMischiefCandidateGrid(grid map[string]any) bool {
	if len(grid) == 0 {
		return false
	}
	if boolFromAny(grid["canMischief"]) {
		return true
	}
	hasPlant := boolFromAny(grid["hasPlant"])
	if !hasPlant && intFromAny(firstExistingAny(grid["plantId"], grid["cropId"])) <= 0 {
		return false
	}
	stageKind := strings.ToLower(strings.TrimSpace(fmt.Sprint(firstExistingAny(grid["stageKind"], ""))))
	if stageKind != "" && stageKind != "growing" {
		return false
	}
	if boolFromAny(firstExistingAny(grid["isMature"], grid["canCollect"], grid["canHarvest"], grid["canSteal"])) {
		return false
	}
	if value, exists := grid["matureInSec"]; exists && intFromAny(value) <= 0 {
		return false
	}
	return true
}

type landUpgradeCandidate struct {
	landID   int
	priority int
}

func collectLandUpgradeCandidates(status map[string]any) []landUpgradeCandidate {
	candidates := []landUpgradeCandidate{}
	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		if len(grid) == 0 || !boolFromAny(grid["couldUpgrade"]) {
			continue
		}
		landID := intFromAny(firstExistingAny(grid["landId"], grid["id"]))
		if landID <= 0 {
			continue
		}
		priority := landUpgradePriority(fmt.Sprint(firstExistingAny(grid["landType"], grid["landTypeLabel"], grid["landBadge"], "")))
		if priority <= 0 {
			continue
		}
		candidates = append(candidates, landUpgradeCandidate{landID: landID, priority: priority})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority > candidates[j].priority
		}
		return candidates[i].landID < candidates[j].landID
	})
	return candidates
}

func landUpgradePriority(value string) int {
	switch normalizeLandUpgradeType(value) {
	case "gold":
		return 4
	case "black":
		return 3
	case "red":
		return 2
	case "normal":
		return 1
	default:
		return 0
	}
}

func normalizeLandUpgradeType(value string) string {
	switch value {
	case "purpleGold", "purplegold", "紫金土地":
		return "purpleGold"
	case "gold", "黄金土地", "金土地":
		return "gold"
	case "black", "黑土地":
		return "black"
	case "red", "红土地":
		return "red"
	case "normal", "普通土地", "default":
		return "normal"
	default:
		return ""
	}
}

func mysteryShopCardCount(value any) int {
	payload := mapFromAny(value)
	if count := intFromAny(firstExistingAny(payload["cardCount"], payload["count"])); count > 0 {
		return count
	}
	if cards := sliceFromAny(payload["cards"]); len(cards) > 0 {
		return len(cards)
	}
	return len(sliceFromAny(payload["items"]))
}

func selectMysteryShopBuyCard(value any, config map[string]any) map[string]any {
	payload := mapFromAny(value)
	for _, item := range sliceFromAny(payload["cards"]) {
		card := mapFromAny(item)
		if mysteryShopCardMatchesConfig(card, config) {
			return card
		}
	}
	for _, item := range sliceFromAny(payload["items"]) {
		card := mapFromAny(item)
		if mysteryShopCardMatchesConfig(card, config) {
			return card
		}
	}
	return nil
}

func mysteryShopCardMatchesConfig(card map[string]any, config map[string]any) bool {
	if mysteryShopGoodsID(card) <= 0 {
		return false
	}
	if len(config) == 0 {
		return true
	}
	currencyIDs := normalizeUniquePositiveInts(sliceFromAny(config["autoFarmMysteryShopCurrencyIds"]))
	if len(currencyIDs) > 0 {
		cardCurrencyID := intFromAny(firstExistingAny(card["currencyId"], card["currency_id"]))
		if cardCurrencyID <= 0 || !intSliceContains(currencyIDs, cardCurrencyID) {
			return false
		}
	}
	threshold := intFromAny(config["autoFarmMysteryShopDiscountThreshold"])
	if threshold > 0 {
		discount := intFromAny(firstExistingAny(card["discount"], card["discountRatio"], card["discount_ratio"]))
		if discount <= 0 || discount > threshold {
			return false
		}
	}
	return true
}

func mysteryShopGoodsID(card map[string]any) int {
	return intFromAny(firstExistingAny(card["goodsId"], card["goods_id"]))
}

func intSliceContains(values []int, needle int) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func selectFriendByWorkCount(value any, key string) map[string]any {
	friends := selectFriendsByWorkCount(value, key)
	if len(friends) == 0 {
		return nil
	}
	return friends[0]
}

func selectFriendByWorkCountExcept(value any, key string, excludedGIDs map[int]bool) map[string]any {
	friends := selectFriendsByWorkCountExcept(value, key, excludedGIDs)
	if len(friends) == 0 {
		return nil
	}
	return friends[0]
}

func selectFriendsByWorkCount(value any, key string) []map[string]any {
	return selectFriendsByWorkCountExcept(value, key, nil)
}

func selectFriendsByWorkCountExcept(value any, key string, excludedGIDs map[int]bool) []map[string]any {
	items := sliceFromAny(value)
	if len(items) == 0 {
		payload := mapFromAny(value)
		items = sliceFromAny(payload["list"])
	}
	friends := make([]map[string]any, 0, len(items))
	for _, item := range items {
		friend := mapFromAny(item)
		if len(friend) == 0 {
			continue
		}
		gid := intFromAny(firstExistingAny(friend["gid"], friend["uin"], friend["id"]))
		if gid <= 0 {
			continue
		}
		if social.IsProtectedFriendGID(gid) {
			continue
		}
		if excludedGIDs[gid] {
			continue
		}
		workCounts := mapFromAny(friend["workCounts"])
		if friendHasWorkCount(workCounts, key) {
			friends = append(friends, friend)
		}
	}
	return friends
}

func friendHasWorkCount(workCounts map[string]any, key string) bool {
	if intFromAny(workCounts[key]) > 0 {
		return true
	}
	if key == "mischief" {
		_, hasExplicitMischiefCount := workCounts["mischief"]
		return !hasExplicitMischiefCount
	}
	if key != "help" {
		return false
	}
	for _, careKey := range []string{"water", "eraseGrass", "killBug"} {
		if intFromAny(workCounts[careKey]) > 0 {
			return true
		}
	}
	return false
}

func isFertilizerRushCandidate(grid map[string]any, rushThresholdSec int) bool {
	if len(grid) == 0 {
		return false
	}
	stageKind, _ := grid["stageKind"].(string)
	if stageKind != "growing" {
		return false
	}
	if boolFromAny(grid["isDead"]) || boolFromAny(grid["canHarvest"]) || boolFromAny(grid["canCollect"]) {
		return false
	}
	if value, exists := grid["hasPlant"]; exists && !boolFromAny(value) {
		return false
	}
	if intFromAny(firstExistingAny(grid["plantId"], grid["cropId"])) <= 0 {
		return false
	}
	matureInSec := intFromAny(grid["matureInSec"])
	return matureInSec > 5 && matureInSec <= rushThresholdSec
}

func isDeadCropGrid(grid map[string]any) bool {
	if len(grid) == 0 {
		return false
	}
	stageKind := strings.ToLower(strings.TrimSpace(fmt.Sprint(firstExistingAny(grid["stageKind"], ""))))
	if stageKind == "dead" || stageKind == "withered" {
		return true
	}
	for _, key := range []string{"isDead", "canEraseDead", "needsEraseDead", "needEraseDead"} {
		if boolFromAny(grid[key]) {
			return true
		}
	}
	return false
}

func runtimeResultFailed(value any) (bool, string) {
	result := mapFromAny(value)
	if len(result) == 0 {
		return false, ""
	}
	if result["ok"] == false || result["success"] == false {
		reason := fmt.Sprint(firstExistingAny(result["reason"], result["message"], result["error"], result["failureText"], result["callbackError"], "unknown"))
		if detail := runtimeFailureDetail(result); detail != "" {
			if reason == "" || reason == "unknown" || reason == "dispatch_failed" {
				reason = detail
			} else if !strings.Contains(reason, detail) {
				reason += ": " + detail
			}
		}
		return true, reason
	}
	return false, ""
}

func isSameFertilizerTypeAlreadyUsed(value any) bool {
	result := mapFromAny(value)
	entries := sliceFromAny(result["results"])
	if len(entries) == 0 {
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(result["reason"])), "same_fertilizer_type_already_used") &&
			strings.EqualFold(strings.TrimSpace(fmt.Sprint(result["action"])), "skipped")
	}
	for _, item := range entries {
		entry := mapFromAny(item)
		if !strings.EqualFold(strings.TrimSpace(fmt.Sprint(entry["reason"])), "same_fertilizer_type_already_used") {
			return false
		}
	}
	return true
}

func sameFertilizerTypeAlreadyUsedCount(value any, fallback int) int {
	count := 0
	for _, item := range sliceFromAny(mapFromAny(value)["results"]) {
		if strings.EqualFold(strings.TrimSpace(fmt.Sprint(mapFromAny(item)["reason"])), "same_fertilizer_type_already_used") {
			count++
		}
	}
	if count == 0 && isSameFertilizerTypeAlreadyUsed(value) {
		return fallback
	}
	return count
}

func runtimeSuccessfulOperationCount(value any) int {
	result := mapFromAny(value)
	for _, key := range []string{"successCount", "claimedCount", "boughtCount"} {
		if _, exists := result[key]; exists {
			count := intFromAny(result[key])
			if count > 0 {
				return count
			}
			return 0
		}
	}

	seenLandIDs := map[int]bool{}
	successesWithoutLandID := 0
	for _, item := range sliceFromAny(result["results"]) {
		entry := mapFromAny(item)
		if len(entry) == 0 || entry["ok"] == false || strings.EqualFold(strings.TrimSpace(fmt.Sprint(entry["action"])), "skipped") {
			continue
		}
		if landID := intFromAny(firstExistingAny(entry["landId"], entry["land_id"])); landID > 0 {
			seenLandIDs[landID] = true
		} else {
			successesWithoutLandID++
		}
	}
	return len(seenLandIDs) + successesWithoutLandID
}

func runtimeFailureDetail(result map[string]any) string {
	for _, collectionKey := range []string{"results", "dispatches"} {
		for _, item := range sliceFromAny(result[collectionKey]) {
			entry := mapFromAny(item)
			for _, key := range []string{"error", "message", "reason", "failureText", "callbackError"} {
				text := strings.TrimSpace(fmt.Sprint(entry[key]))
				if text != "" && text != "<nil>" && text != "dispatch_failed" {
					return text
				}
			}
		}
	}
	return ""
}

func runtimeSuccessMessage(value any, fallback string) string {
	result := mapFromAny(value)
	if len(result) == 0 {
		return fallback
	}
	action := strings.TrimSpace(fmt.Sprint(firstExistingAny(result["action"], "")))
	reason := strings.TrimSpace(fmt.Sprint(firstExistingAny(result["reason"], "")))
	if message := runtimeNoClaimableRewardMessage(action, reason); message != "" {
		return message
	}
	if boolFromAny(result["skipped"]) {
		if message := runtimeNoClaimableRewardMessage(action, reason); message != "" {
			return message
		}
		return fmt.Sprintf("%s本轮跳过：%v", fallback, firstExistingAny(result["reason"], "skipped"))
	}
	if count := intFromAny(firstExistingAny(result["totalFilledCount"], result["claimedCount"], result["boughtCount"], result["successCount"], result["itemUpdateCount"])); count > 0 {
		return fmt.Sprintf("%s本次处理 %d 项。", fallback, count)
	}
	return fallback
}

func runtimeNoClaimableRewardMessage(action string, reason string) string {
	normalized := strings.ToLower(strings.TrimSpace(reason))
	if normalized == "" {
		return ""
	}
	noClaimable := normalized == "already_claimed" ||
		strings.HasPrefix(normalized, "no_claimable") ||
		strings.Contains(normalized, "already claimed") ||
		strings.Contains(reason, "已经领取") ||
		strings.Contains(reason, "已领取") ||
		strings.Contains(reason, "限购次数已用完") ||
		strings.Contains(reason, "次数已用完")
	if !noClaimable {
		return ""
	}
	switch action {
	case "svip_daily_gift":
		return "SVIP每日礼包：今日已领取或无可领取奖励。"
	case "monthly_card_reward":
		return "月卡奖励：今日已领取或无可领取奖励。"
	case "claim_mall_daily_fertilizer":
		return "商城每日肥料：今日已领取或无可领取奖励。"
	case "claim_share_reward":
		return "分享奖励：今日已领取或无可领取奖励。"
	case "claim_mail_rewards":
		return "邮件奖励：今日已领取或无可领取奖励。"
	}
	if normalized == "no_claimable_task_reward" {
		return "自动领取任务奖励：无可领取奖励。"
	}
	return ""
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	if typed, ok := value.(map[any]any); ok {
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if text, ok := key.(string); ok {
				result[text] = item
			}
		}
		return result
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
	default:
		return nil
	}
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
	typed, _ := value.(bool)
	return typed
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
		var result int
		if _, err := fmt.Sscanf(typed, "%d", &result); err == nil {
			return result
		}
		return 0
	default:
		return 0
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
