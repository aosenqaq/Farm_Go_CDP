package automation

import (
	"Farm_Go/internal/farm/social"
	"Farm_Go/internal/farm/stealrules"
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"
)

func (r RuntimeFacade) runFriendSteal(ctx context.Context) ActionResult {
	const taskID = "friend_steal"
	if r.caller == nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "游戏运行时尚未连接，无法执行好友偷菜。",
		}
	}
	friendValue, err := r.caller.Call(ctx, "gameCtl.getFriendList", []any{friendListRefreshOptions("farm_go_auto_friend_steal")}, 30*time.Second)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "读取好友列表失败：" + err.Error(),
		}
	}
	friends := selectFriendsByWorkCountExcept(friendValue, "collect", r.recentFriendBlacklistSkipGIDs(ctx, time.Now()))
	if len(friends) == 0 {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "没有检测到可偷菜好友，本轮好友偷菜跳过。",
		}
	}
	rules, err := r.loadFriendRules(ctx)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "读取好友规则失败：" + err.Error(),
		}
	}
	friends = social.FilterFriendCandidatesByRules(friends, rules, "steal")
	if len(friends) == 0 {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "没有检测到可偷菜好友，本轮好友偷菜跳过。",
		}
	}

	friends = limitFriendStealCandidates(friends, r.config)
	var results []friendStealCandidateResult
	if r.runMode.IsSafe() {
		results = r.runFriendStealCandidatesSafe(ctx, friends)
	} else {
		if err := r.waitFriendStealRandomDelay(ctx); err != nil {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: "随机偷菜延迟已取消：" + err.Error(),
			}
		}
		results = r.runFriendStealCandidates(ctx, friends, friendStealConcurrency(r.config))
	}
	summary := friendBatchSummary{Attempted: len(results)}
	blacklistSkipCount := 0
	firstBlacklistGID := 0
	for _, result := range results {
		if result.ErrorMessage != "" {
			summary.Failed++
			if summary.FirstError == "" {
				summary.FirstError = result.ErrorMessage
			}
			continue
		}
		if result.BlacklistSkipped {
			r.saveFriendBlacklistSkipRecord(ctx, result.GID, result.Inspect, result.LandIDs)
			summary.Skipped++
			blacklistSkipCount++
			if firstBlacklistGID == 0 {
				firstBlacklistGID = result.GID
			}
			continue
		}
		if result.Skipped {
			summary.Skipped++
			continue
		}
		if !result.Success {
			summary.Skipped++
			continue
		}
		summary.Successful++
		summary.LandCount += len(result.LandIDs)
		r.saveFriendStealRecord(ctx, result.GID, result.Inspect, result.LandIDs, result.HarvestResult)
	}
	batchResult := summary.actionResult(taskID, "偷菜", "偷菜地块")
	if blacklistSkipCount > 0 {
		skipMessage := fmt.Sprintf("好友 %d 的可偷作物命中黑名单，本轮跳过并进入黑名单冷却。", firstBlacklistGID)
		if friendStealCropListMode(r.config) == "whitelist" {
			skipMessage = fmt.Sprintf("好友 %d 的可偷作物不符合白名单，本轮跳过并进入白名单冷却。", firstBlacklistGID)
		}
		if blacklistSkipCount > 1 {
			skipMessage = fmt.Sprintf("%d 个好友的可偷作物命中黑名单，本轮跳过并进入黑名单冷却。", blacklistSkipCount)
			if friendStealCropListMode(r.config) == "whitelist" {
				skipMessage = fmt.Sprintf("%d 个好友的可偷作物不符合白名单，本轮跳过并进入白名单冷却。", blacklistSkipCount)
			}
		}
		batchResult.Message += " " + skipMessage
	}
	return batchResult
}

func friendStealCropListMode(config map[string]any) string {
	if fmt.Sprint(firstExistingAny(config["autoFarmFriendStealPlantListMode"], "blacklist")) == "whitelist" {
		return "whitelist"
	}
	return "blacklist"
}

type friendStealCandidateResult struct {
	Index            int
	GID              int
	Inspect          map[string]any
	LandIDs          []int
	HarvestResult    any
	Success          bool
	Skipped          bool
	BlacklistSkipped bool
	ErrorMessage     string
}

type friendBatchSummary struct {
	Attempted  int
	Successful int
	Failed     int
	Skipped    int
	LandCount  int
	FirstError string
}

func (summary friendBatchSummary) actionResult(taskID string, actionLabel string, landLabel string) ActionResult {
	message := fmt.Sprintf(
		"好友%s批次完成：已执行 %d 个好友，成功 %d，失败 %d，跳过 %d，%s %d。",
		actionLabel,
		summary.Attempted,
		summary.Successful,
		summary.Failed,
		summary.Skipped,
		landLabel,
		summary.LandCount,
	)
	result := ActionResult{
		OK:                summary.Failed == 0 || summary.Successful > 0,
		Status:            StatusOK,
		TaskID:            taskID,
		Message:           message,
		ActionCount:       summary.Successful,
		AttemptedFriends:  summary.Attempted,
		SuccessfulFriends: summary.Successful,
		FailedFriends:     summary.Failed,
		SkippedFriends:    summary.Skipped,
	}
	if summary.Failed > 0 && summary.Successful == 0 {
		result.OK = false
		result.Status = StatusFailed
		if summary.FirstError != "" {
			result.Message += " 首个失败：" + summary.FirstError + "。"
		}
	}
	return result
}

func friendListRefreshOptions(source string) map[string]any {
	return map[string]any{
		"refresh":            true,
		"waitRefresh":        true,
		"refreshPlantStatus": true,
		"includeSelf":        false,
		"sort":               true,
		"silent":             true,
		"source":             source,
	}
}

func (r RuntimeFacade) runFriendStealCandidates(ctx context.Context, friends []map[string]any, concurrency int) []friendStealCandidateResult {
	if len(friends) == 0 {
		return nil
	}
	if concurrency <= 1 || len(friends) == 1 {
		results := make([]friendStealCandidateResult, 0, len(friends))
		for index, friend := range friends {
			result := r.runFriendStealCandidate(ctx, index, friend)
			results = append(results, result)
			if result.Success || result.BlacklistSkipped {
				break
			}
		}
		return results
	}
	if concurrency > len(friends) {
		concurrency = len(friends)
	}
	type friendStealJob struct {
		index  int
		friend map[string]any
	}
	jobs := make(chan friendStealJob)
	results := make(chan friendStealCandidateResult, len(friends))
	var waitGroup sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for job := range jobs {
				results <- r.runFriendStealCandidate(ctx, job.index, job.friend)
			}
		}()
	}
	for index, friend := range friends {
		jobs <- friendStealJob{index: index, friend: friend}
	}
	close(jobs)
	waitGroup.Wait()
	close(results)

	collected := make([]friendStealCandidateResult, 0, len(friends))
	for result := range results {
		collected = append(collected, result)
	}
	sort.SliceStable(collected, func(i, j int) bool {
		return collected[i].Index < collected[j].Index
	})
	return collected
}

func (r RuntimeFacade) runFriendStealCandidatesSafe(ctx context.Context, friends []map[string]any) []friendStealCandidateResult {
	results := make([]friendStealCandidateResult, 0, len(friends))
	for index, friend := range friends {
		if err := ctx.Err(); err != nil {
			results = append(results, friendStealCandidateResult{Index: index, ErrorMessage: "安全模式好友偷菜已取消：" + err.Error()})
			break
		}
		result := r.runFriendStealCandidateSafe(ctx, index, friend)
		results = append(results, result)
		if index == len(friends)-1 {
			continue
		}
		if err := r.waitSafeFriendGap(ctx); err != nil {
			results[len(results)-1].ErrorMessage = "安全模式好友间隔已取消：" + err.Error()
			break
		}
	}
	return results
}

func (r RuntimeFacade) runFriendStealCandidate(ctx context.Context, index int, friend map[string]any) friendStealCandidateResult {
	gid := intFromAny(firstExistingAny(friend["gid"], friend["uin"], friend["id"]))
	result := friendStealCandidateResult{Index: index, GID: gid}
	inspectValue, err := r.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"silent":       true,
		"includeLands": true,
		"leaveAfter":   false,
		"source":       "farm_go_auto_friend_steal",
	}}, 45*time.Second)
	if err != nil {
		result.ErrorMessage = "协议进入好友农场失败：" + err.Error()
		return result
	}
	if failed, reason := runtimeResultFailed(inspectValue); failed {
		result.ErrorMessage = "协议进入好友农场返回失败：" + reason
		return result
	}
	inspect := mapFromAny(inspectValue)
	result.Inspect = inspect
	landIDs := collectProtocolWorkLandIDs(inspect, "collect")
	if len(landIDs) == 0 {
		result.Skipped = true
		return result
	}
	decision := stealrules.ApplyCropRules(r.config, inspect, landIDs)
	if decision.SkipFarm {
		result.BlacklistSkipped = true
		result.LandIDs = landIDs
		return result
	}
	landIDs = decision.LandIDs
	if len(landIDs) == 0 {
		result.Skipped = true
		return result
	}
	harvestResult, err := r.caller.Call(ctx, "gameCtl.friendHarvestLandsByProtocol", []any{map[string]any{
		"hostGid":     gid,
		"landIds":     landIDs,
		"isAll":       false,
		"silent":      true,
		"waitReplyMs": 1200,
		"source":      "farm_go_auto_friend_steal",
	}}, 45*time.Second)
	if err != nil {
		result.ErrorMessage = "好友偷菜调用失败：" + err.Error()
		return result
	}
	if failed, reason := runtimeResultFailed(harvestResult); failed {
		result.ErrorMessage = "好友偷菜返回失败：" + reason
		return result
	}
	result.LandIDs = landIDs
	result.HarvestResult = harvestResult
	result.Success = true
	return result
}

func (r RuntimeFacade) runFriendStealCandidateSafe(ctx context.Context, index int, friend map[string]any) friendStealCandidateResult {
	gid := intFromAny(firstExistingAny(friend["gid"], friend["uin"], friend["id"]))
	result := friendStealCandidateResult{Index: index, GID: gid}
	inspectValue, err := r.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"silent":       true,
		"includeLands": true,
		"leaveAfter":   false,
		"source":       "farm_go_auto_friend_steal",
	}}, 45*time.Second)
	if err != nil {
		result.ErrorMessage = "协议进入好友农场失败：" + err.Error()
		return result
	}
	if failed, reason := runtimeResultFailed(inspectValue); failed {
		result.ErrorMessage = "协议进入好友农场返回失败：" + reason
		return result
	}
	inspect := mapFromAny(inspectValue)
	result.Inspect = inspect
	landIDs := collectProtocolWorkLandIDs(inspect, "collect")
	if len(landIDs) == 0 {
		result.Skipped = true
		return result
	}
	decision := stealrules.ApplyCropRules(r.config, inspect, landIDs)
	if decision.SkipFarm {
		result.BlacklistSkipped = true
		result.LandIDs = landIDs
		return result
	}
	landIDs = decision.LandIDs
	if len(landIDs) == 0 {
		result.Skipped = true
		return result
	}
	result.LandIDs = landIDs
	harvestResult, err := r.caller.Call(ctx, "gameCtl.friendHarvestLandsByProtocol", []any{map[string]any{
		"hostGid":     gid,
		"landIds":     landIDs,
		"isAll":       false,
		"silent":      true,
		"waitReplyMs": 1200,
		"source":      "farm_go_auto_friend_steal",
	}}, 45*time.Second)
	if err != nil {
		result.ErrorMessage = "好友偷菜调用失败：" + err.Error()
		return result
	}
	if failed, reason := runtimeResultFailed(harvestResult); failed {
		result.ErrorMessage = "好友偷菜返回失败：" + reason
		return result
	}
	result.HarvestResult = harvestResult
	result.Success = true
	return result
}

func friendStealConcurrency(config map[string]any) int {
	if config == nil {
		return 1
	}
	value, exists := config["autoFarmFriendStealConcurrency"]
	if !exists {
		return 1
	}
	concurrency := intFromAny(value)
	if concurrency <= 0 {
		return 1
	}
	return concurrency
}

const (
	friendStealRandomDelayEnabledKey = "autoFarmFriendStealRandomDelayEnabled"
	friendStealRandomDelayMinMsKey   = "autoFarmFriendStealRandomDelayMinMs"
	friendStealRandomDelayMaxMsKey   = "autoFarmFriendStealRandomDelayMaxMs"
	friendStealRandomDelayMinDefault = 1000
	friendStealRandomDelayMaxDefault = 5000
)

type friendStealRandomDelaySettings struct {
	enabled bool
	minMs   int
	maxMs   int
}

func friendStealRandomDelaySettingsFromConfig(config map[string]any) friendStealRandomDelaySettings {
	settings := friendStealRandomDelaySettings{
		minMs: friendStealRandomDelayMinDefault,
		maxMs: friendStealRandomDelayMaxDefault,
	}
	if config == nil {
		return settings
	}
	settings.enabled = boolConfigAny(config[friendStealRandomDelayEnabledKey])
	if value, ok := config[friendStealRandomDelayMinMsKey]; ok {
		settings.minMs = intFromAny(value)
	}
	if value, ok := config[friendStealRandomDelayMaxMsKey]; ok {
		settings.maxMs = intFromAny(value)
	}
	if settings.minMs < 0 {
		settings.minMs = 0
	}
	if settings.maxMs < 0 {
		settings.maxMs = 0
	}
	if settings.maxMs < settings.minMs {
		settings.maxMs = settings.minMs
	}
	return settings
}

func (r RuntimeFacade) waitFriendStealRandomDelay(ctx context.Context) error {
	settings := friendStealRandomDelaySettingsFromConfig(r.config)
	if !settings.enabled {
		return nil
	}
	waitMs := settings.minMs
	if settings.maxMs > settings.minMs {
		waitMs += rand.Intn(settings.maxMs - settings.minMs + 1)
	}
	return sleepWithContext(ctx, time.Duration(waitMs)*time.Millisecond)
}

func (r RuntimeFacade) waitSafeFriendGap(ctx context.Context) error {
	return r.waitSafeExecutionGap(ctx, 100, 1000)
}

func (r RuntimeFacade) waitSafeLandGap(ctx context.Context) error {
	return r.waitSafeExecutionGap(ctx, 100, 200)
}

func (r RuntimeFacade) waitSafeExecutionGap(ctx context.Context, minimumMs int, maximumMs int) error {
	if !r.runMode.IsSafe() || maximumMs < minimumMs {
		return nil
	}
	span := maximumMs - minimumMs + 1
	intn := r.intn
	if intn == nil {
		intn = rand.Intn
	}
	offset := intn(span)
	if offset < 0 {
		offset = 0
	}
	if offset >= span {
		offset = span - 1
	}
	wait := r.wait
	if wait == nil {
		wait = sleepWithContext
	}
	return wait(ctx, time.Duration(minimumMs+offset)*time.Millisecond)
}

func limitFriendStealCandidates(friends []map[string]any, config map[string]any) []map[string]any {
	maxFriends := intFromAny(config["autoFarmFriendStealMaxFriends"])
	if maxFriends <= 0 || maxFriends >= len(friends) {
		return friends
	}
	return friends[:maxFriends]
}

func (r RuntimeFacade) recentFriendBlacklistSkipGIDs(ctx context.Context, now time.Time) map[int]bool {
	reader, ok := r.stealRecordWriter.(StealRecordReader)
	if !ok {
		return nil
	}
	cooldownMin := intFromAny(firstExistingAny(r.config["autoFarmFriendBlacklistCooldownMin"], 10))
	if cooldownMin <= 0 {
		return nil
	}
	accountKey := r.accountKey
	if accountKey == "" {
		accountKey = "default"
	}
	records, err := reader.ListStealRecords(ctx, accountKey, nil)
	if err != nil {
		return nil
	}
	cutoff := now.Add(-time.Duration(cooldownMin) * time.Minute)
	excluded := map[int]bool{}
	for _, record := range records {
		if record.GID <= 0 || record.Action != "steal_blacklist_skip" || !recentStealRecordOccurredAfter(record, cutoff) {
			continue
		}
		excluded[record.GID] = true
	}
	return excluded
}

func recentStealRecordOccurredAfter(record social.StealRecord, cutoff time.Time) bool {
	occurredAt, err := time.Parse(time.RFC3339Nano, record.OccurredAt)
	if err != nil {
		return false
	}
	return occurredAt.After(cutoff)
}

func (r RuntimeFacade) saveFriendStealRecord(ctx context.Context, gid int, status map[string]any, landIDs []int, harvestValue any) {
	if r.stealRecordWriter == nil {
		return
	}
	accountKey := r.accountKey
	if accountKey == "" {
		accountKey = "default"
	}
	record := social.BuildStealRecord(gid, status, landIDs, harvestValue, social.StealRecordOptions{
		Action: r.friendStealRecordAction(),
	})
	_ = r.stealRecordWriter.SaveStealRecords(ctx, accountKey, []social.StealRecord{record})
}

func (r RuntimeFacade) saveFriendBlacklistSkipRecord(ctx context.Context, gid int, status map[string]any, landIDs []int) {
	if r.stealRecordWriter == nil {
		return
	}
	accountKey := r.accountKey
	if accountKey == "" {
		accountKey = "default"
	}
	friend := mapFromAny(firstExistingAny(status["friend"], status["friendInfo"], status["owner"], status["user"], status["basic"]))
	displayName := fmt.Sprint(firstExistingAny(friend["displayName"], friend["remark"], friend["name"], status["displayName"], status["name"], gid))
	occurredAt := time.Now().Format(time.RFC3339Nano)
	record := social.StealRecord{
		ID:          fmt.Sprintf("%d:%s:%s", gid, occurredAt, "steal_blacklist_skip"),
		GID:         gid,
		DisplayName: displayName,
		Action:      "steal_blacklist_skip",
		OccurredAt:  occurredAt,
		LandIDs:     landIDs,
		Raw: map[string]any{
			"ok":      true,
			"skipped": true,
			"reason":  "steal_crop_blacklist",
		},
	}
	_ = r.stealRecordWriter.SaveStealRecords(ctx, accountKey, []social.StealRecord{record})
}

func (r RuntimeFacade) friendStealRecordAction() string {
	trigger := fmt.Sprint(firstExistingAny(r.config["automationTrigger"], r.config["trigger"], "auto"))
	if trigger == "auto" {
		return "steal_auto"
	}
	return "steal_scheduler_manual"
}

const friendHelpConcurrency = 3

type friendHelpCandidateResult struct {
	Index        int
	GID          int
	LandCount    int
	Success      bool
	Skipped      bool
	ErrorMessage string
}

func (r RuntimeFacade) runFriendHelp(ctx context.Context) ActionResult {
	const taskID = "friend_help"
	dailyRemaining := 0
	dailyLimited := false
	if fmt.Sprint(r.config["automationTrigger"]) == "auto" {
		dailyRemaining, dailyLimited = FriendHelpDailyRemaining(r.config, time.Now())
		if dailyLimited && dailyRemaining == 0 {
			return ActionResult{
				OK:      true,
				Status:  StatusOK,
				TaskID:  taskID,
				Message: "本账号今日自动好友帮忙已达到每日上限，本轮直接跳过。",
			}
		}
	}
	if r.caller == nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "游戏运行时尚未连接，无法执行好友帮忙。",
		}
	}
	friendValue, err := r.caller.Call(ctx, "gameCtl.getFriendList", []any{friendListRefreshOptions("farm_go_auto_friend_help")}, 30*time.Second)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "读取好友列表失败：" + err.Error(),
		}
	}
	friends := selectFriendsByWorkCount(friendValue, "help")
	if len(friends) == 0 {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "没有检测到可帮忙好友，本轮好友帮忙跳过。",
		}
	}
	guardDogOnly := boolFromAny(r.config["autoFarmFriendHelpGuardDogOnly"])
	if guardDogOnly {
		guardGIDs, err := r.loadGuardDogGIDs(ctx)
		if err != nil {
			return ActionResult{
				OK:      false,
				Status:  StatusFailed,
				TaskID:  taskID,
				Message: "读取护主犬好友缓存失败：" + err.Error(),
			}
		}
		if len(guardGIDs) == 0 {
			return ActionResult{
				OK:      true,
				Status:  StatusOK,
				TaskID:  taskID,
				Message: "请先读取护主犬好友，当前缓存中没有可用的护主犬好友记录，本轮好友帮忙跳过。",
			}
		}
		friends = filterGuardDogHelpCandidates(friends, guardGIDs)
		if len(friends) == 0 {
			return ActionResult{
				OK:      true,
				Status:  StatusOK,
				TaskID:  taskID,
				Message: "护主犬好友中没有检测到可帮忙对象，本轮好友帮忙跳过。",
			}
		}
	}
	rules, err := r.loadFriendRules(ctx)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "读取好友规则失败：" + err.Error(),
		}
	}
	friends = social.FilterFriendCandidatesByRules(friends, rules, "help")
	if len(friends) == 0 {
		message := "没有检测到可帮忙好友，本轮好友帮忙跳过。"
		if guardDogOnly {
			message = "护主犬好友中没有检测到可帮忙对象，本轮好友帮忙跳过。"
		}
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: message,
		}
	}
	friends = limitFriendHelpCandidates(friends, r.config, dailyRemaining, dailyLimited)
	var results []friendHelpCandidateResult
	if r.runMode.IsSafe() {
		results = r.runFriendHelpCandidatesSafe(ctx, friends)
	} else {
		results = r.runFriendHelpCandidates(ctx, friends)
	}
	summary := friendBatchSummary{Attempted: len(results)}
	for _, result := range results {
		if result.Success {
			summary.Successful++
			summary.LandCount += result.LandCount
		}
		if result.Skipped {
			summary.Skipped++
		}
		if result.ErrorMessage != "" {
			summary.Failed++
			if summary.FirstError == "" {
				summary.FirstError = result.ErrorMessage
			}
		}
	}
	return summary.actionResult(taskID, "帮忙", "帮忙项")
}

func limitFriendHelpCandidates(friends []map[string]any, config map[string]any, dailyRemaining int, dailyLimited bool) []map[string]any {
	limit := len(friends)
	if maxFriends := intFromAny(config["autoFarmFriendHelpMaxFriends"]); maxFriends > 0 && maxFriends < limit {
		limit = maxFriends
	}
	if dailyLimited && dailyRemaining < limit {
		limit = dailyRemaining
	}
	return friends[:limit]
}

func filterGuardDogHelpCandidates(friends []map[string]any, guardGIDs map[int]bool) []map[string]any {
	filtered := make([]map[string]any, 0, len(friends))
	for _, friend := range friends {
		gid := intFromAny(firstExistingAny(friend["gid"], friend["uin"], friend["id"]))
		if gid > 0 && guardGIDs[gid] {
			filtered = append(filtered, friend)
		}
	}
	return filtered
}

func (r RuntimeFacade) loadGuardDogGIDs(ctx context.Context) (map[int]bool, error) {
	if r.dogGuardStateReader == nil {
		return nil, fmt.Errorf("未配置护主犬好友缓存读取器")
	}
	accountKey := r.accountKey
	if accountKey == "" {
		accountKey = "default"
	}
	state, err := r.dogGuardStateReader.LoadDogGuardState(ctx, accountKey)
	if err != nil {
		return nil, err
	}
	guardGIDs := map[int]bool{}
	for _, row := range state.Results {
		if row.GID > 0 && row.HasGuardDog {
			guardGIDs[row.GID] = true
		}
	}
	return guardGIDs, nil
}

func (r RuntimeFacade) runFriendHelpCandidates(ctx context.Context, friends []map[string]any) []friendHelpCandidateResult {
	if len(friends) == 0 {
		return nil
	}
	workerCount := friendHelpConcurrency
	if workerCount > len(friends) {
		workerCount = len(friends)
	}
	type friendHelpJob struct {
		index  int
		friend map[string]any
	}
	jobs := make(chan friendHelpJob)
	results := make(chan friendHelpCandidateResult, len(friends))
	var waitGroup sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					if ctx.Err() != nil {
						return
					}
					results <- r.runFriendHelpCandidate(ctx, job.index, job.friend)
				}
			}
		}()
	}
dispatching:
	for index, friend := range friends {
		if ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
			break dispatching
		case jobs <- friendHelpJob{index: index, friend: friend}:
		}
	}
	close(jobs)
	waitGroup.Wait()
	close(results)

	collected := make([]friendHelpCandidateResult, 0, len(friends))
	for result := range results {
		collected = append(collected, result)
	}
	sort.SliceStable(collected, func(i, j int) bool {
		return collected[i].Index < collected[j].Index
	})
	return collected
}

func (r RuntimeFacade) runFriendHelpCandidatesSafe(ctx context.Context, friends []map[string]any) []friendHelpCandidateResult {
	results := make([]friendHelpCandidateResult, 0, len(friends))
	for index, friend := range friends {
		if err := ctx.Err(); err != nil {
			results = append(results, friendHelpCandidateResult{Index: index, ErrorMessage: "安全模式好友帮忙已取消：" + err.Error()})
			break
		}
		result := r.runFriendHelpCandidate(ctx, index, friend)
		results = append(results, result)
		if index == len(friends)-1 {
			continue
		}
		if err := r.waitSafeFriendGap(ctx); err != nil {
			results[len(results)-1].ErrorMessage = "安全模式好友间隔已取消：" + err.Error()
			break
		}
	}
	return results
}

func (r RuntimeFacade) runFriendHelpCandidate(ctx context.Context, index int, friend map[string]any) friendHelpCandidateResult {
	gid := intFromAny(firstExistingAny(friend["gid"], friend["uin"], friend["id"]))
	result := friendHelpCandidateResult{Index: index, GID: gid}
	if err := ctx.Err(); err != nil {
		result.ErrorMessage = "好友帮忙已取消：" + err.Error()
		return result
	}
	inspectValue, err := r.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"silent":       true,
		"includeLands": false,
		"leaveAfter":   false,
		"source":       "farm_go_auto_friend_help",
	}}, 45*time.Second)
	if err != nil {
		result.ErrorMessage = "协议进入好友农场失败：" + err.Error()
		return result
	}
	if err := ctx.Err(); err != nil {
		result.ErrorMessage = "好友帮忙已取消：" + err.Error()
		return result
	}
	if failed, reason := runtimeResultFailed(inspectValue); failed {
		result.ErrorMessage = "协议进入好友农场返回失败：" + reason
		return result
	}
	landIDs := collectProtocolWorkLandIDs(mapFromAny(inspectValue), "farming", "water", "eraseGrass", "killBug")
	if len(landIDs) == 0 {
		result.Skipped = true
		return result
	}
	helpResult, err := r.caller.Call(ctx, "gameCtl.friendFarmingByProtocol", []any{map[string]any{
		"hostGid":     gid,
		"landIds":     landIDs,
		"source":      0,
		"silent":      true,
		"waitReplyMs": 1200,
	}}, 45*time.Second)
	if err != nil {
		result.ErrorMessage = "好友帮忙调用失败：" + err.Error()
		return result
	}
	if failed, reason := runtimeResultFailed(helpResult); failed {
		result.ErrorMessage = "好友帮忙返回失败：" + reason
		return result
	}
	result.LandCount = len(landIDs)
	result.Success = true
	return result
}

func (r RuntimeFacade) runFriendMischief(ctx context.Context) ActionResult {
	const taskID = "friend_mischief"
	if FriendMischiefDailyDone(r.config, time.Now()) {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "本账号今日自动捣乱已完成，本轮直接跳过。",
		}
	}
	if r.caller == nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "游戏运行时尚未连接，无法执行好友捣乱。",
		}
	}
	friendValue, err := r.caller.Call(ctx, "gameCtl.getFriendList", []any{friendListRefreshOptions("farm_go_auto_friend_mischief")}, 30*time.Second)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "读取好友列表失败：" + err.Error(),
		}
	}
	friends := selectFriendsByWorkCount(friendValue, "mischief")
	if len(friends) == 0 {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "没有检测到可捣乱好友，本轮好友捣乱跳过。",
		}
	}
	rules, err := r.loadFriendRules(ctx)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "读取好友规则失败：" + err.Error(),
		}
	}
	friends = social.FilterFriendCandidatesByRules(friends, rules, "mischief")
	if len(friends) == 0 {
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: "没有检测到可捣乱好友，本轮好友捣乱跳过。",
		}
	}
	if r.runMode.IsSafe() {
		return r.runFriendMischiefSafe(ctx, friends)
	}
	summary := friendBatchSummary{}
	for _, friend := range friends {
		gid := intFromAny(firstExistingAny(friend["gid"], friend["uin"], friend["id"]))
		summary.Attempted++
		inspectValue, err := r.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
			"hostGid":      gid,
			"silent":       true,
			"includeLands": false,
			"leaveAfter":   false,
			"source":       "farm_go_auto_friend_mischief",
		}}, 45*time.Second)
		if err != nil {
			summary.Failed++
			if summary.FirstError == "" {
				summary.FirstError = "协议进入好友农场失败：" + err.Error()
			}
			continue
		}
		if failed, reason := runtimeResultFailed(inspectValue); failed {
			summary.Failed++
			if summary.FirstError == "" {
				summary.FirstError = "协议进入好友农场返回失败：" + reason
			}
			continue
		}
		inspect := mapFromAny(inspectValue)
		bugLandIDs := collectProtocolWorkLandIDs(inspect, "bug")
		grassLandIDs := collectProtocolWorkLandIDs(inspect, "grass")
		if len(bugLandIDs) == 0 && len(grassLandIDs) == 0 {
			summary.Skipped++
			continue
		}
		mischiefResult, err := r.caller.Call(ctx, "gameCtl.friendMischiefLandsBatch", []any{map[string]any{
			"hostGid":      gid,
			"bugLandIds":   bugLandIDs,
			"grassLandIds": grassLandIDs,
			"dryRun":       false,
			"silent":       true,
			"source":       "farm_go_auto_friend_mischief",
		}}, 45*time.Second)
		if err != nil {
			summary.Failed++
			if summary.FirstError == "" {
				summary.FirstError = "好友捣乱调用失败：" + err.Error()
			}
			continue
		}
		if failed, reason := runtimeResultFailed(mischiefResult); failed {
			if IsFriendMischiefDailyLimitText(reason) {
				summary.Skipped++
				result := summary.actionResult(taskID, "捣乱", "捣乱地块")
				result.OK = true
				result.Status = StatusOK
				result.Message = reason + "，已停止本账号今日自动捣乱；" + result.Message
				return result
			}
			summary.Failed++
			if summary.FirstError == "" {
				summary.FirstError = "好友捣乱返回失败：" + reason
			}
			continue
		}
		summary.Successful++
		summary.LandCount += len(bugLandIDs) + len(grassLandIDs)
		break
	}
	return summary.actionResult(taskID, "捣乱", "捣乱地块")
}

type friendMischiefLandTarget struct {
	bug    bool
	landID int
}

func (r RuntimeFacade) runFriendMischiefSafe(ctx context.Context, friends []map[string]any) ActionResult {
	const taskID = "friend_mischief"
	summary := friendBatchSummary{}
	for friendIndex, friend := range friends {
		if err := ctx.Err(); err != nil {
			summary.Failed++
			if summary.FirstError == "" {
				summary.FirstError = "安全模式好友捣乱已取消：" + err.Error()
			}
			break
		}
		gid := intFromAny(firstExistingAny(friend["gid"], friend["uin"], friend["id"]))
		summary.Attempted++
		inspectValue, err := r.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
			"hostGid":      gid,
			"silent":       true,
			"includeLands": false,
			"leaveAfter":   false,
			"source":       "farm_go_auto_friend_mischief",
		}}, 45*time.Second)
		if err != nil {
			summary.Failed++
			if summary.FirstError == "" {
				summary.FirstError = "协议进入好友农场失败：" + err.Error()
			}
		} else if failed, reason := runtimeResultFailed(inspectValue); failed {
			summary.Failed++
			if summary.FirstError == "" {
				summary.FirstError = "协议进入好友农场返回失败：" + reason
			}
		} else {
			inspect := mapFromAny(inspectValue)
			targets := friendMischiefTargets(inspect)
			if len(targets) == 0 {
				summary.Skipped++
			} else {
				friendFailed := false
				for targetIndex, target := range targets {
					bugLandIDs := []int{}
					grassLandIDs := []int{}
					if target.bug {
						bugLandIDs = []int{target.landID}
					} else {
						grassLandIDs = []int{target.landID}
					}
					mischiefResult, callErr := r.caller.Call(ctx, "gameCtl.friendMischiefLandsBatch", []any{map[string]any{
						"hostGid":      gid,
						"bugLandIds":   bugLandIDs,
						"grassLandIds": grassLandIDs,
						"dryRun":       false,
						"silent":       true,
						"source":       "farm_go_auto_friend_mischief",
					}}, 45*time.Second)
					if callErr != nil {
						summary.Failed++
						if summary.FirstError == "" {
							summary.FirstError = "好友捣乱调用失败：" + callErr.Error()
						}
						friendFailed = true
						break
					}
					if failed, reason := runtimeResultFailed(mischiefResult); failed {
						if IsFriendMischiefDailyLimitText(reason) {
							summary.Skipped++
							result := summary.actionResult(taskID, "捣乱", "捣乱地块")
							result.OK = true
							result.Status = StatusOK
							result.Message = reason + "，已停止本账号今日自动捣乱；" + result.Message
							return result
						}
						summary.Failed++
						if summary.FirstError == "" {
							summary.FirstError = "好友捣乱返回失败：" + reason
						}
						friendFailed = true
						break
					}
					summary.LandCount++
					if targetIndex < len(targets)-1 {
						if waitErr := r.waitSafeLandGap(ctx); waitErr != nil {
							summary.Failed++
							if summary.FirstError == "" {
								summary.FirstError = "安全模式捣乱地块间隔已取消：" + waitErr.Error()
							}
							friendFailed = true
							break
						}
					}
				}
				if !friendFailed {
					summary.Successful++
				}
			}
		}
		if friendIndex == len(friends)-1 {
			continue
		}
		if err := r.waitSafeFriendGap(ctx); err != nil {
			summary.Failed++
			if summary.FirstError == "" {
				summary.FirstError = "安全模式好友间隔已取消：" + err.Error()
			}
			break
		}
	}
	return summary.actionResult(taskID, "捣乱", "捣乱地块")
}

func friendMischiefTargets(inspect map[string]any) []friendMischiefLandTarget {
	bugLandIDs := collectProtocolWorkLandIDs(inspect, "bug")
	grassLandIDs := collectProtocolWorkLandIDs(inspect, "grass")
	targets := make([]friendMischiefLandTarget, 0, len(bugLandIDs)+len(grassLandIDs))
	for _, landID := range bugLandIDs {
		targets = append(targets, friendMischiefLandTarget{bug: true, landID: landID})
	}
	for _, landID := range grassLandIDs {
		targets = append(targets, friendMischiefLandTarget{landID: landID})
	}
	return targets
}
