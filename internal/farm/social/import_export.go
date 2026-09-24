package social

import (
	"context"
	"strings"
	"time"
)

const ImportExportVersion = 1

type RecordWriter interface {
	SaveStealRecords(ctx context.Context, accountKey string, records []StealRecord) error
	SaveVisitorRecords(ctx context.Context, accountKey string, records []VisitorRecord) error
}

func (s *Service) Export(ctx context.Context, req ExportRequest) ImportExportPayload {
	groups, err := normalizeImportExportGroups(req.Groups)
	if err != nil {
		return ImportExportPayload{OK: false, Status: StatusFailed, Message: err.Error(), Version: ImportExportVersion, AccountKey: s.accountKey, Groups: []string{}}
	}
	payload := ImportExportPayload{
		OK:         true,
		Status:     StatusOK,
		Message:    "好友社交数据已导出。",
		Version:    ImportExportVersion,
		ExportedAt: time.Now().Format(time.RFC3339Nano),
		AccountKey: s.accountKey,
		Groups:     groups,
	}

	for _, group := range groups {
		switch group {
		case "rules":
			rules, err := s.loadRules(ctx)
			if err != nil {
				return failedExportPayload(payload, "读取好友规则失败："+err.Error())
			}
			payload.Rules = &rules
		case "friend_snapshot":
			if s.caller != nil {
				state := s.State(ctx, StateRequest{Refresh: req.RefreshFriendSnapshot})
				payload.FriendSnapshot = state.Friends
			} else {
				payload.FriendSnapshot = []FriendRow{}
			}
		case "steal_records":
			payload.StealRecords = []StealRecord{}
			if store, ok := s.store.(RankingStore); ok {
				records, err := store.ListStealRecords(ctx, s.accountKey, nil)
				if err != nil {
					return failedExportPayload(payload, "读取偷取记录失败："+err.Error())
				}
				payload.StealRecords = normalizeStealRecords(records)
			}
		case "visitor_records":
			payload.VisitorRecords = []VisitorRecord{}
			if store, ok := s.store.(RankingStore); ok {
				records, err := store.ListVisitorRecords(ctx, s.accountKey)
				if err != nil {
					return failedExportPayload(payload, "读取访客记录失败："+err.Error())
				}
				payload.VisitorRecords = normalizeVisitorRecords(records)
			}
		case "dog_guard":
			state := DogGuardState{Results: []DogGuardRow{}}
			if store, ok := s.store.(DogGuardStore); ok {
				cached, err := store.LoadDogGuardState(ctx, s.accountKey)
				if err != nil {
					return failedExportPayload(payload, "读取护主犬缓存失败："+err.Error())
				}
				state = normalizeDogGuardState(cached)
			}
			payload.DogGuard = &state
		}
	}
	return payload
}

func (s *Service) Import(ctx context.Context, payload ImportExportPayload) ActionResult {
	if payload.Version != ImportExportVersion {
		return ActionResult{OK: false, Status: StatusFailed, Message: "不支持的导入版本。"}
	}
	groups, err := normalizeImportExportGroups(payload.Groups)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: err.Error()}
	}
	if s.store == nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "本地存储尚未就绪，无法导入好友社交数据。"}
	}

	imported := []string{}
	for _, group := range groups {
		switch group {
		case "rules":
			if payload.Rules == nil {
				return ActionResult{OK: false, Status: StatusFailed, Message: "导入数据缺少好友规则。"}
			}
			if err := s.store.SaveFriendRules(ctx, s.accountKey, *payload.Rules); err != nil {
				return ActionResult{OK: false, Status: StatusFailed, Message: "导入好友规则失败：" + err.Error()}
			}
			imported = append(imported, group)
		case "friend_snapshot":
			imported = append(imported, group)
		case "steal_records":
			writer, ok := s.store.(RecordWriter)
			if !ok {
				return ActionResult{OK: false, Status: StatusFailed, Message: "本地存储不支持写入偷取记录。"}
			}
			if err := writer.SaveStealRecords(ctx, s.accountKey, payload.StealRecords); err != nil {
				return ActionResult{OK: false, Status: StatusFailed, Message: "导入偷取记录失败：" + err.Error()}
			}
			imported = append(imported, group)
		case "visitor_records":
			writer, ok := s.store.(RecordWriter)
			if !ok {
				return ActionResult{OK: false, Status: StatusFailed, Message: "本地存储不支持写入访客记录。"}
			}
			if err := writer.SaveVisitorRecords(ctx, s.accountKey, payload.VisitorRecords); err != nil {
				return ActionResult{OK: false, Status: StatusFailed, Message: "导入访客记录失败：" + err.Error()}
			}
			imported = append(imported, group)
		case "dog_guard":
			if payload.DogGuard == nil {
				return ActionResult{OK: false, Status: StatusFailed, Message: "导入数据缺少护主犬缓存。"}
			}
			store, ok := s.store.(DogGuardStore)
			if !ok {
				return ActionResult{OK: false, Status: StatusFailed, Message: "本地存储不支持写入护主犬缓存。"}
			}
			state := normalizeDogGuardState(*payload.DogGuard)
			if err := store.SaveDogGuardState(ctx, s.accountKey, state); err != nil {
				return ActionResult{OK: false, Status: StatusFailed, Message: "导入护主犬缓存失败：" + err.Error()}
			}
			imported = append(imported, group)
		}
	}
	return ActionResult{OK: true, Status: StatusOK, Message: "好友社交数据已导入。", Data: map[string]any{"groups": imported}}
}

func failedExportPayload(payload ImportExportPayload, message string) ImportExportPayload {
	payload.OK = false
	payload.Status = StatusFailed
	payload.Message = message
	return payload
}

func normalizeImportExportGroups(groups []string) ([]string, error) {
	if len(groups) == 0 {
		return []string{"rules", "friend_snapshot", "steal_records", "visitor_records", "dog_guard"}, nil
	}
	allowed := map[string]bool{
		"rules":           true,
		"friend_snapshot": true,
		"steal_records":   true,
		"visitor_records": true,
		"dog_guard":       true,
	}
	result := []string{}
	seen := map[string]bool{}
	for _, group := range groups {
		key := strings.ToLower(strings.TrimSpace(group))
		if !allowed[key] {
			return nil, importExportGroupError{group: group}
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, key)
		}
	}
	return result, nil
}

type importExportGroupError struct {
	group string
}

func (e importExportGroupError) Error() string {
	return "不支持的导入导出分组：" + e.group
}
