package social

import (
	"context"
	"fmt"
	"time"
)

type RuntimeCaller interface {
	Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error)
}

type Store interface {
	LoadFriendRules(ctx context.Context, accountKey string) (FriendRules, error)
	SaveFriendRules(ctx context.Context, accountKey string, rules FriendRules) error
}

type RankingStore interface {
	ListStealRecords(ctx context.Context, accountKey string, dateKeys []string) ([]StealRecord, error)
	ListVisitorRecords(ctx context.Context, accountKey string) ([]VisitorRecord, error)
}

type RankingPageStore interface {
	QueryRankingPage(ctx context.Context, accountKey string, query RankingQuery) (RankingPageData, error)
}

type RankingPreferencesStore interface {
	LoadRankingPreferences(ctx context.Context, accountKey string) (RankingPreferences, error)
	SaveRankingPreferences(ctx context.Context, accountKey string, preferences RankingPreferences) error
}

type Options struct {
	AccountKey       string
	AutomationConfig map[string]any
}

type Service struct {
	store            Store
	caller           RuntimeCaller
	accountKey       string
	automationConfig map[string]any
}

type StateRequest struct {
	Refresh bool `json:"refresh"`
}

type Summary struct {
	TotalFriends     int `json:"totalFriends"`
	StealableFriends int `json:"stealableFriends"`
	HelpableFriends  int `json:"helpableFriends"`
	MischiefFriends  int `json:"mischiefFriends"`
	Blacklisted      int `json:"blacklisted"`
	Whitelisted      int `json:"whitelisted"`
	MaskedBlocked    int `json:"maskedBlocked"`
	Protected        int `json:"protected"`
	DogGuardCount    int `json:"dogGuardCount"`
}

type State struct {
	OK      bool        `json:"ok"`
	Status  Status      `json:"status"`
	Message string      `json:"message"`
	Summary Summary     `json:"summary"`
	Rules   FriendRules `json:"rules"`
	Friends []FriendRow `json:"friends"`
}

func NewService(store Store, caller RuntimeCaller, opts Options) *Service {
	accountKey := opts.AccountKey
	if accountKey == "" {
		accountKey = "default"
	}
	return &Service{store: store, caller: caller, accountKey: accountKey, automationConfig: cloneAnyMap(opts.AutomationConfig)}
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func (s *Service) State(ctx context.Context, req StateRequest) State {
	rules, err := s.loadRules(ctx)
	if err != nil {
		return State{
			OK:      false,
			Status:  StatusFailed,
			Message: "读取好友规则失败：" + err.Error(),
			Rules:   rules,
			Friends: []FriendRow{},
		}
	}
	if s.caller == nil {
		return State{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			Message: "游戏运行时尚未连接，无法读取好友列表。",
			Rules:   rules,
			Friends: []FriendRow{},
		}
	}

	value, err := s.caller.Call(ctx, "gameCtl.getFriendList", []any{map[string]any{
		"refresh":     req.Refresh,
		"sort":        true,
		"includeSelf": false,
		"waitRefresh": req.Refresh,
		"silent":      true,
	}}, 30*time.Second)
	if err != nil {
		return State{
			OK:      false,
			Status:  StatusFailed,
			Message: "读取好友列表失败：" + err.Error(),
			Rules:   rules,
			Friends: []FriendRow{},
		}
	}

	friends := enrichFriendRows(value, rules)
	if store, ok := s.store.(DogGuardStore); ok {
		dogGuardState, err := store.LoadDogGuardState(ctx, s.accountKey)
		if err != nil {
			return State{
				OK:      false,
				Status:  StatusFailed,
				Message: "读取护主犬缓存失败：" + err.Error(),
				Rules:   rules,
				Friends: []FriendRow{},
			}
		}
		applyDogGuardState(friends, dogGuardState)
	}
	return State{
		OK:      true,
		Status:  StatusOK,
		Message: "好友列表已更新。",
		Summary: summarizeFriends(friends),
		Rules:   rules,
		Friends: friends,
	}
}

func (s *Service) RefreshVisitors(ctx context.Context) ActionResult {
	if s.caller == nil {
		return ActionResult{Status: StatusRuntimeNotReady, Message: "游戏运行时尚未连接，无法刷新访客记录。"}
	}
	value, err := s.caller.Call(ctx, "gameCtl.getVisitorRecords", []any{map[string]any{
		"silent":  true,
		"debug":   true,
		"refresh": true,
	}}, 30*time.Second)
	if err != nil {
		return ActionResult{Status: StatusFailed, Message: "刷新访客记录失败：" + err.Error()}
	}
	refreshed := visitorRecordsFromRuntime(value)
	if len(refreshed) > 0 {
		writer, ok := s.store.(RecordWriter)
		if !ok {
			return ActionResult{Status: StatusFailed, Message: "保存访客记录失败：存储不支持访客记录写入。"}
		}
		if err := writer.SaveVisitorRecords(ctx, s.accountKey, refreshed); err != nil {
			return ActionResult{Status: StatusFailed, Message: "保存访客记录失败：" + err.Error()}
		}
	}
	return ActionResult{OK: true, Status: StatusOK, Message: "访客记录已刷新。"}
}

func (s *Service) RankingPreferences(ctx context.Context) RankingPreferences {
	preferences, err := s.loadRankingPreferences(ctx)
	if err != nil {
		return NormalizeRankingPreferences(RankingPreferences{})
	}
	return preferences
}

func (s *Service) SaveRankingPreferences(ctx context.Context, preferences RankingPreferences) (RankingPreferences, error) {
	preferences = NormalizeRankingPreferences(preferences)
	if store, ok := s.store.(RankingPreferencesStore); ok {
		if err := store.SaveRankingPreferences(ctx, s.accountKey, preferences); err != nil {
			return RankingPreferences{}, err
		}
	}
	return preferences, nil
}

func NormalizeRankingPreferences(preferences RankingPreferences) RankingPreferences {
	if preferences.StolenByMeViewMode != "ranking" {
		preferences.StolenByMeViewMode = "timeline"
	}
	if preferences.StolenFromMeViewMode != "ranking" {
		preferences.StolenFromMeViewMode = "timeline"
	}
	return preferences
}

func (s *Service) loadRules(ctx context.Context) (FriendRules, error) {
	if s.store == nil {
		return NormalizeFriendRules(FriendRules{MaskedBlacklist: true, MaskedMaxLevel: 1}), nil
	}
	rules, err := s.store.LoadFriendRules(ctx, s.accountKey)
	if err != nil {
		return FriendRules{}, err
	}
	return NormalizeFriendRules(rules), nil
}

func (s *Service) loadRankingPreferences(ctx context.Context) (RankingPreferences, error) {
	if s.store == nil {
		return NormalizeRankingPreferences(RankingPreferences{}), nil
	}
	store, ok := s.store.(RankingPreferencesStore)
	if !ok {
		return NormalizeRankingPreferences(RankingPreferences{}), nil
	}
	preferences, err := store.LoadRankingPreferences(ctx, s.accountKey)
	if err != nil {
		return RankingPreferences{}, err
	}
	return NormalizeRankingPreferences(preferences), nil
}

func enrichFriendRows(value any, rules FriendRules) []FriendRow {
	items := friendListFromAny(value)
	rows := make([]FriendRow, 0, len(items))
	for _, item := range items {
		row, ok := friendRowFromRaw(mapFromAny(item))
		if !ok {
			continue
		}
		row.Whitelisted = ruleListMatchesFriend(rules.Whitelist, row)
		row.Blacklisted = ruleListMatchesFriend(rules.Blacklist, row)
		row.MaskedBlocked = rules.MaskedBlacklist && row.Level > 0 && row.Level <= rules.MaskedMaxLevel
		rows = append(rows, row)
	}
	return rows
}

func friendRowFromRaw(raw map[string]any) (FriendRow, bool) {
	if len(raw) == 0 {
		return FriendRow{}, false
	}
	gid := PositiveInt(firstExistingAny(raw["gid"], raw["uin"], raw["uid"], raw["id"], raw["playerId"]))
	if gid <= 0 || IsProtectedFriendGID(gid) {
		return FriendRow{}, false
	}
	workCounts := workCountsFromAny(raw["workCounts"])
	level := PositiveInt(raw["level"])
	return FriendRow{
		GID:         gid,
		Name:        firstText(raw["name"], raw["nick"], raw["nickname"]),
		DisplayName: firstText(raw["displayName"], raw["remark"], raw["name"], raw["nick"], raw["nickname"], fmt.Sprint(gid)),
		Remark:      firstText(raw["remark"]),
		AvatarURL:   firstText(raw["avatarUrl"], raw["avatar"]),
		Level:       level,
		WorkCounts:  workCounts,
		Stealable:   workCounts["collect"] > 0,
		Helpable: workCounts["help"] > 0 ||
			workCounts["water"] > 0 ||
			workCounts["eraseGrass"] > 0 ||
			workCounts["killBug"] > 0,
		Mischiefable: workCounts["mischief"] > 0,
		Protected:    false,
		Raw:          raw,
	}, true
}

func FriendAllowedByRules(rules FriendRules, scope string, friend FriendRow) bool {
	rules = NormalizeFriendRules(rules)
	blacklistApplies := rules.BlacklistEnabled && scopeListContains(rules.BlacklistScopes, scope)
	whitelistApplies := rules.WhitelistEnabled && scopeListContains(rules.WhitelistScopes, scope)
	if blacklistApplies && (ruleListMatchesFriend(rules.Blacklist, friend) ||
		(rules.MaskedBlacklist && friend.Level > 0 && friend.Level <= rules.MaskedMaxLevel)) {
		return false
	}
	return !whitelistApplies || ruleListMatchesFriend(rules.Whitelist, friend)
}

func FilterFriendCandidatesByRules(candidates []map[string]any, rules FriendRules, scope string) []map[string]any {
	filtered := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		friend, ok := friendRowFromRaw(candidate)
		if ok && FriendAllowedByRules(rules, scope, friend) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func applyDogGuardState(friends []FriendRow, state DogGuardState) {
	guardGIDs := make(map[int]bool, len(state.Results))
	for _, row := range state.Results {
		if row.GID > 0 && row.HasGuardDog {
			guardGIDs[row.GID] = true
		}
	}
	for index := range friends {
		friends[index].HasGuardDog = guardGIDs[friends[index].GID]
	}
}

func friendListFromAny(value any) []any {
	if list := sliceFromAny(value); len(list) > 0 {
		return list
	}
	payload := mapFromAny(value)
	if len(payload) == 0 {
		return nil
	}
	return sliceFromAny(payload["list"])
}

func workCountsFromAny(value any) map[string]int {
	source := mapFromAny(value)
	result := map[string]int{}
	for _, key := range []string{"collect", "help", "water", "eraseGrass", "killBug", "mischief"} {
		result[key] = PositiveInt(source[key])
	}
	return result
}

func ruleListMatchesFriend(items []string, friend FriendRow) bool {
	if len(items) == 0 {
		return false
	}
	candidates := map[string]bool{
		fmt.Sprint(friend.GID): true,
		friend.DisplayName:     true,
		friend.Name:            true,
		friend.Remark:          true,
	}
	for _, item := range items {
		if candidates[item] {
			return true
		}
	}
	return false
}

func summarizeFriends(friends []FriendRow) Summary {
	var summary Summary
	summary.TotalFriends = len(friends)
	for _, friend := range friends {
		if friend.Stealable {
			summary.StealableFriends++
		}
		if friend.Helpable {
			summary.HelpableFriends++
		}
		if friend.Mischiefable {
			summary.MischiefFriends++
		}
		if friend.Blacklisted {
			summary.Blacklisted++
		}
		if friend.Whitelisted {
			summary.Whitelisted++
		}
		if friend.MaskedBlocked {
			summary.MaskedBlocked++
		}
		if friend.Protected {
			summary.Protected++
		}
		if friend.HasGuardDog {
			summary.DogGuardCount++
		}
	}
	return summary
}

func firstText(values ...any) string {
	for _, value := range values {
		text := fmt.Sprint(value)
		if text != "" && text != "<nil>" {
			return text
		}
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
	case []map[string]any:
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

func visitorRecordsFromRuntime(value any) []VisitorRecord {
	payload := mapFromAny(value)
	var items []any
	if len(payload) > 0 {
		items = sliceFromAny(firstExistingAny(payload["records"], payload["list"], payload["data"]))
	}
	if len(items) == 0 {
		items = sliceFromAny(value)
	}
	records := make([]VisitorRecord, 0, len(items))
	for _, item := range items {
		raw := mapFromAny(item)
		if len(raw) == 0 {
			continue
		}
		playerID := PositiveInt(firstExistingAny(raw["playerId"], raw["gid"], raw["uid"]))
		timeValue := positiveInt64(raw["time"])
		record := VisitorRecord{
			ID:               firstText(raw["id"]),
			PlayerID:         playerID,
			Name:             firstText(raw["name"]),
			DisplayName:      firstText(raw["displayName"], raw["name"], raw["nick"], raw["nickname"], raw["remark"]),
			AvatarURL:        firstText(raw["avatarUrl"], raw["avatar"]),
			ActionType:       PositiveInt(raw["actionType"]),
			Time:             timeValue,
			StealItemID:      PositiveInt(raw["stealItemId"]),
			StealItemName:    firstText(raw["stealItemName"]),
			StealItemNum:     PositiveInt(raw["stealItemNum"]),
			Count:            PositiveInt(raw["count"]),
			Level:            PositiveInt(raw["level"]),
			AuthorizedStatus: PositiveInt(firstExistingAny(raw["authorized_status"], raw["authorizedStatus"])),
			HostType:         PositiveInt(firstExistingAny(raw["host_type"], raw["hostType"])),
			ActionLabel:      firstText(raw["actionLabel"]),
		}
		if record.DisplayName == "" && playerID > 0 {
			record.DisplayName = fmt.Sprint(playerID)
		}
		if record.ID == "" {
			record.ID = fmt.Sprintf("%d:%d:%d:%d:%d:%d", record.PlayerID, record.Time, record.ActionType, record.StealItemID, record.StealItemNum, record.Count)
		}
		if record.PlayerID > 0 || record.Time > 0 {
			records = append(records, record)
		}
	}
	return normalizeVisitorRecords(records)
}

func positiveInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		if typed > 0 {
			return typed
		}
	case int:
		if typed > 0 {
			return int64(typed)
		}
	case float64:
		if typed > 0 {
			return int64(typed)
		}
	case string:
		parsed := PositiveInt(typed)
		if parsed > 0 {
			return int64(parsed)
		}
	}
	return 0
}
