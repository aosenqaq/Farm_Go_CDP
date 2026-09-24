package social

import (
	"Farm_Go/internal/farm/stealrules"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Service) Action(ctx context.Context, req FriendActionRequest) ActionResult {
	action := strings.TrimSpace(req.Action)
	target := strings.TrimSpace(req.Target)
	if action == "" {
		return ActionResult{OK: false, Status: StatusFailed, Message: "好友操作不能为空。"}
	}

	switch action {
	case "blacklist_toggle":
		return s.toggleLocalRule(ctx, target, "blacklist")
	case "whitelist_toggle":
		return s.toggleLocalRule(ctx, target, "whitelist")
	case "blacklist_remove_batch", "whitelist_remove_batch":
		return s.removeLocalRuleBatch(ctx, action, req.Targets)
	case "unblock_batch", "unblock_friend_batch":
		return s.unblockFriendsByProtocol(ctx, action, req.Targets, req.DryRun)
	case "blacklist_clear", "whitelist_clear":
		return s.clearLocalRuleList(ctx, action)
	case "blacklist_cleanup", "cleanup_invalid_rules":
		return s.cleanupInvalidLocalRules(ctx)
	case "rules_save":
		return s.saveRules(ctx, req.Rules)
	}

	gid := PositiveInt(target)
	if gid <= 0 {
		if action == "god_rank" {
			return s.readGodRank(ctx)
		}
		return ActionResult{OK: false, Status: StatusFailed, Message: "好友 gid 不能为空。"}
	}
	if isProtectedAction(action) && IsProtectedFriendGID(gid) {
		return ActionResult{OK: true, Status: StatusSkipped, Message: "该好友在保护名单中，已跳过操作。", Data: map[string]any{
			"action":  action,
			"gid":     gid,
			"skipped": true,
		}}
	}
	if s.caller == nil {
		return ActionResult{OK: false, Status: StatusRuntimeNotReady, Message: "游戏运行时尚未连接，无法执行好友操作。"}
	}
	if isProtectedAction(action) {
		if blocked := s.guardFriendActionByRules(ctx, gid, action); blocked != nil {
			return *blocked
		}
	}

	switch action {
	case "enter":
		return s.enterFriendFarm(ctx, gid, "farm_go_social_enter", 0)
	case "block", "block_friend":
		return s.callRuntime(ctx, "gameCtl.blockFriendByProtocol", []any{map[string]any{
			"friendGid": gid,
			"dryRun":    req.DryRun,
			"silent":    true,
		}}, 30*time.Second, "已提交协议拉黑请求。", "协议拉黑失败：")
	case "unblock", "unblock_friend":
		return s.callRuntime(ctx, "gameCtl.unblockFriendByProtocol", []any{map[string]any{
			"friendGid": gid,
			"dryRun":    req.DryRun,
			"silent":    true,
		}}, 30*time.Second, "已提交协议解除拉黑请求。", "协议解除拉黑失败：")
	case "steal":
		return s.runSteal(ctx, gid)
	case "help":
		return s.runHelp(ctx, gid)
	case "mischief":
		return s.runMischief(ctx, gid)
	case "view_qq":
		return s.viewQQFriend(ctx, gid)
	default:
		return ActionResult{OK: false, Status: StatusUnsupported, Message: "未知好友操作：" + action}
	}
}

func (s *Service) ProtocolBlockList(ctx context.Context, dryRun bool) ProtocolBlockList {
	if s.caller == nil {
		return ProtocolBlockList{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			Message: "游戏运行时尚未连接，无法读取协议拉黑好友。",
			Friends: []FriendRow{},
		}
	}
	value, err := s.caller.Call(ctx, "gameCtl.getFriendBlockListByProtocol", []any{map[string]any{
		"dryRun": dryRun,
		"debug":  false,
		"silent": true,
	}}, 30*time.Second)
	if err != nil {
		return ProtocolBlockList{OK: false, Status: StatusFailed, Message: "读取协议拉黑好友失败：" + err.Error(), Friends: []FriendRow{}}
	}
	return ProtocolBlockList{
		OK:      true,
		Status:  StatusOK,
		Message: "协议拉黑好友已读取。",
		Friends: enrichFriendRows(value, FriendRules{}),
		Raw:     value,
	}
}

func (s *Service) clearLocalRuleList(ctx context.Context, action string) ActionResult {
	if s.store == nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "本地存储尚未就绪，无法保存名单。"}
	}
	rules, err := s.loadRules(ctx)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "读取好友规则失败：" + err.Error()}
	}
	removed := 0
	if action == "blacklist_clear" {
		removed = len(rules.Blacklist)
		rules.Blacklist = []string{}
	} else {
		removed = len(rules.Whitelist)
		rules.Whitelist = []string{}
	}
	if err := s.store.SaveFriendRules(ctx, s.accountKey, rules); err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "保存好友规则失败：" + err.Error()}
	}
	return ActionResult{OK: true, Status: StatusOK, Message: "名单已更新。", Data: map[string]any{
		"action":       action,
		"removedCount": removed,
		"rules":        rules,
	}}
}

func (s *Service) removeLocalRuleBatch(ctx context.Context, action string, targets []string) ActionResult {
	if s.store == nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "本地存储尚未就绪，无法保存名单。"}
	}
	rules, err := s.loadRules(ctx)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "读取好友规则失败：" + err.Error()}
	}
	configKey := "blacklist"
	items := rules.Blacklist
	if action == "whitelist_remove_batch" {
		configKey = "whitelist"
		items = rules.Whitelist
	}
	kept, removed, missing := removeFriendRuleBatch(items, targets)
	if configKey == "blacklist" {
		rules.Blacklist = kept
	} else {
		rules.Whitelist = kept
	}
	if err := s.store.SaveFriendRules(ctx, s.accountKey, rules); err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "保存好友规则失败：" + err.Error()}
	}
	return ActionResult{OK: true, Status: StatusOK, Message: "名单已更新。", Data: map[string]any{
		"action":       action,
		"configKey":    configKey,
		"targets":      NormalizeStringList(targets),
		"removed":      removed,
		"missing":      missing,
		"removedCount": len(removed),
		"missingCount": len(missing),
		"changed":      len(removed) > 0,
		"rules":        rules,
	}}
}

func (s *Service) saveRules(ctx context.Context, next *FriendRules) ActionResult {
	if s.store == nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "本地存储尚未就绪，无法保存名单。"}
	}
	if next == nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "好友规则不能为空。"}
	}
	rules := NormalizeFriendRules(*next)
	if err := s.store.SaveFriendRules(ctx, s.accountKey, rules); err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "保存好友规则失败：" + err.Error()}
	}
	return ActionResult{OK: true, Status: StatusOK, Message: "好友规则已保存。", Data: map[string]any{
		"action":  "rules_save",
		"changed": true,
		"rules":   rules,
	}}
}

func (s *Service) cleanupInvalidLocalRules(ctx context.Context) ActionResult {
	if s.store == nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "本地存储尚未就绪，无法清理名单。"}
	}
	if s.caller == nil {
		return ActionResult{OK: false, Status: StatusRuntimeNotReady, Message: "游戏运行时尚未连接，无法清理无效黑白名单。"}
	}
	rules, err := s.loadRules(ctx)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "读取好友规则失败：" + err.Error()}
	}
	value, err := s.caller.Call(ctx, "gameCtl.getFriendList", []any{map[string]any{
		"refresh":     true,
		"sort":        true,
		"includeSelf": false,
		"waitRefresh": true,
		"silent":      true,
	}}, 30*time.Second)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "读取好友列表失败：" + err.Error()}
	}
	current := map[int]bool{}
	for _, friend := range enrichFriendRows(value, FriendRules{}) {
		current[friend.GID] = true
	}
	blacklist, removedBlacklist := cleanupInvalidNumericRules(rules.Blacklist, current)
	whitelist, removedWhitelist := cleanupInvalidNumericRules(rules.Whitelist, current)
	rules.Blacklist = blacklist
	rules.Whitelist = whitelist
	if err := s.store.SaveFriendRules(ctx, s.accountKey, rules); err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "保存好友规则失败：" + err.Error()}
	}
	return ActionResult{OK: true, Status: StatusOK, Message: "无效名单已清理。", Data: map[string]any{
		"removed": map[string]any{
			"blacklist":      removedBlacklist,
			"whitelist":      removedWhitelist,
			"blacklistCount": len(removedBlacklist),
			"whitelistCount": len(removedWhitelist),
		},
		"friendGidCount": len(current),
		"rules":          rules,
	}}
}

func (s *Service) readGodRank(ctx context.Context) ActionResult {
	if s.caller == nil {
		return ActionResult{OK: false, Status: StatusRuntimeNotReady, Message: "游戏运行时尚未连接，无法读取封神榜。"}
	}
	return s.callRuntime(ctx, "gameCtl.getGodRankList", []any{map[string]any{
		"silent": true,
	}}, 30*time.Second, "封神榜已读取。", "读取封神榜失败：")
}

func (s *Service) toggleLocalRule(ctx context.Context, target string, listName string) ActionResult {
	if target == "" {
		return ActionResult{OK: false, Status: StatusFailed, Message: "名单项不能为空。"}
	}
	if s.store == nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "本地存储尚未就绪，无法保存名单。"}
	}
	rules, err := s.loadRules(ctx)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "读取好友规则失败：" + err.Error()}
	}
	changed := false
	if listName == "blacklist" {
		rules.Blacklist, changed = toggleStringItem(rules.Blacklist, target)
	} else {
		rules.Whitelist, changed = toggleStringItem(rules.Whitelist, target)
	}
	rules = NormalizeFriendRules(rules)
	if err := s.store.SaveFriendRules(ctx, s.accountKey, rules); err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "保存好友规则失败：" + err.Error()}
	}
	return ActionResult{OK: true, Status: StatusOK, Message: "好友规则已保存。", Data: map[string]any{
		"changed": changed,
		"rules":   rules,
	}}
}

func (s *Service) callRuntime(ctx context.Context, method string, args []any, timeout time.Duration, successText string, errorPrefix string) ActionResult {
	value, err := s.caller.Call(ctx, method, args, timeout)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: errorPrefix + err.Error()}
	}
	if failed, reason := runtimeResultFailed(value); failed {
		return ActionResult{OK: false, Status: StatusFailed, Message: errorPrefix + reason, Data: value}
	}
	return ActionResult{OK: true, Status: StatusOK, Message: successText, Data: value}
}

const qqFriendVerificationMessage = "来自QQ农场"

func (s *Service) viewQQFriend(ctx context.Context, gid int) ActionResult {
	value, err := s.caller.Call(ctx, "gameCtl.addFriendByGidDiagnostic", []any{map[string]any{
		"hostGid":   gid,
		"verifyMsg": qqFriendVerificationMessage,
		"silent":    true,
	}}, 30*time.Second)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "打开 QQ 好友对话框失败：" + err.Error()}
	}
	result := mapFromAny(value)
	if result["ok"] != true || result["invoked"] != true {
		return ActionResult{OK: false, Status: StatusFailed, Message: viewQQFailureMessage(fmt.Sprint(result["reason"]))}
	}
	return ActionResult{
		OK: true, Status: StatusOK, Message: "已请求打开 QQ 原生好友对话框。",
		Data: map[string]any{"action": "view_qq", "gid": gid, "invoked": true},
	}
}

func viewQQFailureMessage(reason string) string {
	switch reason {
	case "visit_query_failed":
		return "无法读取该好友的 QQ 信息。"
	case "reply_gid_mismatch":
		return "QQ 目标校验失败，已阻止打开对话框。"
	case "reply_open_id_missing":
		return "QQ 目标信息不完整，已阻止打开对话框。"
	case "qq_add_friend_api_unavailable":
		return "当前 QQ 客户端不支持打开好友对话框。"
	default:
		return "打开 QQ 好友对话框失败。"
	}
}

func (s *Service) unblockFriendsByProtocol(ctx context.Context, action string, targets []string, dryRun bool) ActionResult {
	gids := make([]int, 0, len(targets))
	seen := map[int]bool{}
	for _, target := range NormalizeStringList(targets) {
		gid := PositiveInt(target)
		if gid <= 0 || seen[gid] {
			continue
		}
		seen[gid] = true
		gids = append(gids, gid)
	}
	if len(gids) == 0 {
		return ActionResult{OK: false, Status: StatusFailed, Message: "请选择要解除协议拉黑的好友。"}
	}
	if s.caller == nil {
		return ActionResult{OK: false, Status: StatusRuntimeNotReady, Message: "游戏运行时尚未连接，无法解除协议拉黑。"}
	}

	results := make([]map[string]any, 0, len(gids))
	successCount := 0
	failureCount := 0
	for _, gid := range gids {
		result := s.callRuntime(ctx, "gameCtl.unblockFriendByProtocol", []any{map[string]any{
			"friendGid": gid,
			"dryRun":    dryRun,
			"silent":    true,
		}}, 30*time.Second, "已提交协议解除拉黑请求。", "协议解除拉黑失败：")
		entry := map[string]any{"gid": gid, "ok": result.OK, "status": result.Status, "message": result.Message}
		if result.Data != nil {
			entry["data"] = result.Data
		}
		results = append(results, entry)
		if result.OK {
			successCount++
		} else {
			failureCount++
		}
	}

	ok := failureCount == 0
	status := StatusOK
	if !ok {
		status = StatusFailed
	}
	return ActionResult{
		OK:      ok,
		Status:  status,
		Message: fmt.Sprintf("协议批量解除完成：成功 %d 个，失败 %d 个。", successCount, failureCount),
		Data: map[string]any{
			"action":       action,
			"targets":      gids,
			"results":      results,
			"successCount": successCount,
			"failureCount": failureCount,
		},
	}
}

func (s *Service) enterFriendFarm(ctx context.Context, gid int, source string, waitMs int) ActionResult {
	return s.callRuntime(ctx, "gameCtl.enterFriendFarm", []any{map[string]any{"gid": gid}, map[string]any{
		"waitMs":                waitMs,
		"includeAfterOwnership": true,
		"silent":                true,
		"source":                source,
	}}, 30*time.Second, "已进入好友农场。", "进入好友农场失败：")
}

func (s *Service) runSteal(ctx context.Context, gid int) ActionResult {
	statusValue, err := s.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"silent":       true,
		"includeLands": true,
		"leaveAfter":   false,
		"source":       "farm_go_social_steal",
	}}, 45*time.Second)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "协议进入好友农场失败：" + err.Error()}
	}
	if failed, reason := runtimeResultFailed(statusValue); failed {
		return ActionResult{OK: false, Status: StatusFailed, Message: "协议进入好友农场返回失败：" + reason, Data: statusValue}
	}
	status := mapFromAny(statusValue)
	landIDs := collectProtocolWorkLandIDs(status, "collect")
	if len(landIDs) == 0 {
		return ActionResult{OK: true, Status: StatusOK, Message: "没有检测到可偷菜地块，本次跳过。"}
	}
	decision := stealrules.ApplyCropRules(s.automationConfig, status, landIDs)
	if decision.SkipFarm {
		s.saveManualStealBlacklistSkipRecord(ctx, gid, status, landIDs)
		if friendStealCropListMode(s.automationConfig) == "whitelist" {
			return ActionResult{OK: true, Status: StatusOK, Message: fmt.Sprintf("好友 %d 的可偷作物不符合白名单，本次跳过并进入白名单冷却。", gid)}
		}
		return ActionResult{OK: true, Status: StatusOK, Message: fmt.Sprintf("好友 %d 的可偷作物命中黑名单，本次跳过并进入黑名单冷却。", gid)}
	}
	landIDs = decision.LandIDs
	result := s.callRuntime(ctx, "gameCtl.friendHarvestLandsByProtocol", []any{map[string]any{
		"hostGid":     gid,
		"landIds":     landIDs,
		"isAll":       false,
		"silent":      true,
		"waitReplyMs": 1200,
		"source":      "farm_go_social_steal",
	}}, 45*time.Second, fmt.Sprintf("已提交好友 %d 的 %d 块土地偷菜请求。", gid, len(landIDs)), "好友偷菜失败：")
	if !result.OK {
		return result
	}
	s.saveManualStealRecord(ctx, gid, status, landIDs, result.Data)
	return result
}

func (s *Service) saveManualStealRecord(ctx context.Context, gid int, status map[string]any, landIDs []int, harvestValue any) {
	writer, ok := s.store.(RecordWriter)
	if !ok {
		return
	}
	record := BuildStealRecord(gid, status, landIDs, harvestValue, StealRecordOptions{Action: "steal"})
	_ = writer.SaveStealRecords(ctx, s.accountKey, []StealRecord{record})
}

func (s *Service) saveManualStealBlacklistSkipRecord(ctx context.Context, gid int, status map[string]any, landIDs []int) {
	writer, ok := s.store.(RecordWriter)
	if !ok {
		return
	}
	friend := mapFromAny(firstExistingAny(status["friend"], status["friendInfo"], status["owner"], status["user"], status["basic"]))
	displayName := fmt.Sprint(firstExistingAny(friend["displayName"], friend["remark"], friend["name"], status["displayName"], status["name"], gid))
	occurredAt := time.Now().Format(time.RFC3339Nano)
	record := StealRecord{
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
	_ = writer.SaveStealRecords(ctx, s.accountKey, []StealRecord{record})
}

func friendStealCropListMode(config map[string]any) string {
	if fmt.Sprint(firstExistingAny(config["autoFarmFriendStealPlantListMode"], "blacklist")) == "whitelist" {
		return "whitelist"
	}
	return "blacklist"
}

func (s *Service) runHelp(ctx context.Context, gid int) ActionResult {
	statusValue, err := s.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"silent":       true,
		"includeLands": false,
		"leaveAfter":   false,
		"source":       "farm_go_social_help",
	}}, 45*time.Second)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "协议进入好友农场失败：" + err.Error()}
	}
	if failed, reason := runtimeResultFailed(statusValue); failed {
		return ActionResult{OK: false, Status: StatusFailed, Message: "协议进入好友农场返回失败：" + reason, Data: statusValue}
	}
	status := mapFromAny(statusValue)
	landIDs := collectProtocolWorkLandIDs(status, "farming", "water", "eraseGrass", "killBug")
	if len(landIDs) == 0 {
		return ActionResult{OK: true, Status: StatusOK, Message: "没有检测到可帮忙地块，本次跳过。"}
	}
	return s.callRuntime(ctx, "gameCtl.friendFarmingByProtocol", []any{map[string]any{
		"hostGid":     gid,
		"landIds":     landIDs,
		"source":      0,
		"silent":      true,
		"waitReplyMs": 1200,
	}}, 45*time.Second, fmt.Sprintf("已提交好友 %d 的 %d 项帮忙请求。", gid, len(landIDs)), "好友帮忙失败：")
}

func (s *Service) runMischief(ctx context.Context, gid int) ActionResult {
	statusValue, err := s.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"silent":       true,
		"includeLands": false,
		"leaveAfter":   false,
		"source":       "farm_go_social_mischief",
	}}, 45*time.Second)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "协议进入好友农场失败：" + err.Error()}
	}
	if failed, reason := runtimeResultFailed(statusValue); failed {
		return ActionResult{OK: false, Status: StatusFailed, Message: "协议进入好友农场返回失败：" + reason, Data: statusValue}
	}
	status := mapFromAny(statusValue)
	bugLandIDs := collectProtocolWorkLandIDs(status, "bug")
	grassLandIDs := collectProtocolWorkLandIDs(status, "grass")
	if len(bugLandIDs) == 0 && len(grassLandIDs) == 0 {
		return ActionResult{OK: true, Status: StatusOK, Message: "没有检测到可捣乱地块，本次跳过。"}
	}
	return s.callRuntime(ctx, "gameCtl.friendMischiefLandsBatch", []any{map[string]any{
		"hostGid":      gid,
		"bugLandIds":   bugLandIDs,
		"grassLandIds": grassLandIDs,
		"dryRun":       false,
		"silent":       true,
		"source":       "farm_go_social_mischief",
	}}, 45*time.Second, fmt.Sprintf("已提交好友 %d 的 %d 块土地捣乱请求。", gid, len(bugLandIDs)+len(grassLandIDs)), "好友捣乱失败：")
}

func isProtectedAction(action string) bool {
	return action == "steal" || action == "help" || action == "mischief"
}

func friendActionScope(action string) string {
	switch action {
	case "steal":
		return "steal"
	case "help":
		return "help"
	case "mischief":
		return "mischief"
	default:
		return ""
	}
}

func (s *Service) guardFriendActionByRules(ctx context.Context, gid int, action string) *ActionResult {
	scope := friendActionScope(action)
	if scope == "" {
		return nil
	}
	rules, err := s.loadRules(ctx)
	if err != nil {
		result := ActionResult{OK: false, Status: StatusFailed, Message: "读取好友规则失败：" + err.Error()}
		return &result
	}
	blacklistApplies := rules.BlacklistEnabled && scopeListContains(rules.BlacklistScopes, scope)
	whitelistApplies := rules.WhitelistEnabled && scopeListContains(rules.WhitelistScopes, scope)
	if !blacklistApplies && !whitelistApplies {
		return nil
	}
	friend := FriendRow{GID: gid, DisplayName: fmt.Sprint(gid), WorkCounts: map[string]int{}}
	if runtimeFriend, ok, err := s.lookupFriendForRules(ctx, gid, rules); err != nil {
		result := ActionResult{OK: false, Status: StatusFailed, Message: "读取好友规则匹配列表失败：" + err.Error()}
		return &result
	} else if ok {
		friend = runtimeFriend
	}
	if !FriendAllowedByRules(rules, scope, friend) {
		blacklisted := ruleListMatchesFriend(rules.Blacklist, friend) ||
			(rules.MaskedBlacklist && friend.Level > 0 && friend.Level <= rules.MaskedMaxLevel)
		reason, message := "whitelist", "该好友不在白名单规则内，已跳过操作。"
		if blacklistApplies && blacklisted {
			reason, message = "blacklist", "该好友命中黑名单规则，已跳过操作。"
		}
		result := ActionResult{OK: true, Status: StatusSkipped, Message: message, Data: map[string]any{
			"action":  action,
			"gid":     gid,
			"scope":   scope,
			"skipped": true,
			"reason":  reason,
		}}
		return &result
	}
	return nil
}

func (s *Service) lookupFriendForRules(ctx context.Context, gid int, rules FriendRules) (FriendRow, bool, error) {
	if s.caller == nil {
		return FriendRow{}, false, nil
	}
	value, err := s.caller.Call(ctx, "gameCtl.getFriendList", []any{map[string]any{
		"refresh":     false,
		"sort":        true,
		"includeSelf": false,
		"waitRefresh": false,
		"silent":      true,
	}}, 30*time.Second)
	if err != nil {
		return FriendRow{}, false, err
	}
	for _, friend := range enrichFriendRows(value, rules) {
		if friend.GID == gid {
			return friend, true, nil
		}
	}
	return FriendRow{}, false, nil
}

func scopeListContains(scopes []string, scope string) bool {
	for _, item := range NormalizeScopeList(scopes) {
		if item == scope {
			return true
		}
	}
	return false
}

func (s *Service) attachReturnHome(ctx context.Context, result ActionResult, source string) ActionResult {
	returnHome, err := s.caller.Call(ctx, "gameCtl.enterOwnFarm", []any{map[string]any{
		"silent": true,
		"source": source,
	}}, 30*time.Second)
	payload := map[string]any{
		"action":     result.Data,
		"returnHome": returnHome,
	}
	if err != nil {
		payload["returnHome"] = map[string]any{"ok": false, "error": err.Error()}
		result.Message += " 返回自家农场失败：" + err.Error()
	}
	result.Data = payload
	return result
}

func socialFarmStatusArgs() map[string]any {
	return map[string]any{
		"includeGrids":          true,
		"includeLandIds":        true,
		"includeRawGrid":        true,
		"includeRawLandRuntime": true,
		"silent":                true,
	}
}

func toggleStringItem(items []string, target string) ([]string, bool) {
	normalized := NormalizeStringList(items)
	for index, item := range normalized {
		if item == target {
			return append(normalized[:index], normalized[index+1:]...), true
		}
	}
	return append(normalized, target), true
}

func cleanupInvalidNumericRules(items []string, currentGIDs map[int]bool) ([]string, []string) {
	kept := []string{}
	removed := []string{}
	for _, item := range NormalizeStringList(items) {
		gid := PositiveInt(item)
		if gid > 0 && !currentGIDs[gid] {
			removed = append(removed, item)
			continue
		}
		kept = append(kept, item)
	}
	return kept, removed
}

func removeFriendRuleBatch(items []string, targets []string) ([]string, []string, []string) {
	source := NormalizeStringList(items)
	selected := map[string]bool{}
	for _, target := range NormalizeStringList(targets) {
		selected[target] = true
	}
	kept := []string{}
	removed := []string{}
	for _, item := range source {
		if selected[item] {
			removed = append(removed, item)
			continue
		}
		kept = append(kept, item)
	}
	missing := []string{}
	for target := range selected {
		found := false
		for _, item := range removed {
			if item == target {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, target)
		}
	}
	sort.Strings(missing)
	return kept, removed, missing
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

func collectProtocolWorkLandIDs(status map[string]any, keys ...string) []int {
	seen := map[int]bool{}
	workLandIDs := mapFromAny(status["workLandIds"])
	for _, key := range keys {
		for _, item := range sliceFromAny(workLandIDs[key]) {
			id := PositiveInt(item)
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

func collectWorkLandIDs(value any, key string) []int {
	status := mapFromAny(value)
	seen := map[int]bool{}
	add := func(value any) {
		id := PositiveInt(value)
		if id > 0 {
			seen[id] = true
		}
	}
	workLandIDs := mapFromAny(status["workLandIds"])
	for _, rawID := range sliceFromAny(workLandIDs[key]) {
		add(rawID)
	}
	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		if len(grid) == 0 {
			continue
		}
		switch key {
		case "collect":
			if boolFromAny(firstExistingAny(grid["canHarvest"], grid["canCollect"], grid["stealable"], grid["collectable"])) {
				add(firstExistingAny(grid["landId"], grid["id"]))
			}
		case "mischief":
			if isFriendMischiefCandidateGrid(grid) {
				add(firstExistingAny(grid["landId"], grid["id"]))
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

func isFriendMischiefCandidateGrid(grid map[string]any) bool {
	if len(grid) == 0 {
		return false
	}
	if boolFromAny(grid["canMischief"]) {
		return true
	}
	hasPlant := boolFromAny(grid["hasPlant"])
	if !hasPlant && PositiveInt(firstExistingAny(grid["plantId"], grid["cropId"])) <= 0 {
		return false
	}
	stageKind := strings.ToLower(strings.TrimSpace(fmt.Sprint(firstExistingAny(grid["stageKind"], ""))))
	if stageKind != "" && stageKind != "growing" {
		return false
	}
	if boolFromAny(firstExistingAny(grid["isMature"], grid["canCollect"], grid["canHarvest"], grid["canSteal"])) {
		return false
	}
	if value, exists := grid["matureInSec"]; exists && PositiveInt(value) <= 0 {
		return false
	}
	return true
}

func collectCareWorkCount(status map[string]any) int {
	total := 0
	workLandIDs := mapFromAny(status["workLandIds"])
	for _, key := range []string{"water", "eraseGrass", "killBug"} {
		total += len(uniquePositiveInts(sliceFromAny(workLandIDs[key])))
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
	}
	return total
}

func uniquePositiveInts(values []any) []int {
	seen := map[int]bool{}
	result := []int{}
	for _, value := range values {
		id := PositiveInt(value)
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func boolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case string:
		return typed == "true" || typed == "1"
	default:
		return false
	}
}
