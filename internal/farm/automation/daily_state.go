package automation

import (
	"fmt"
	"strings"
	"time"
)

const (
	FriendMischiefDailyDoneDateConfigKey = "autoFarmFriendMischiefDailyDoneDate"
	FriendHelpDailyCountDateConfigKey    = "autoFarmFriendHelpDailyCountDate"
	FriendHelpDailyCountConfigKey        = "autoFarmFriendHelpDailyCount"
)

var dailyOnceTaskDoneDateConfigKeys = map[string]string{
	"svip_daily_gift":       "autoFarmSvipDailyGiftDailyDoneDate",
	"monthly_card_reward":   "autoFarmMonthlyCardRewardDailyDoneDate",
	"mall_daily_fertilizer": "autoFarmMallDailyFertilizerDailyDoneDate",
	"share_reward":          "autoFarmShareRewardDailyDoneDate",
	"limited_seed_draw":     "autoFarmLimitedSeedDrawDailyDoneDate",
}

var dailyOnceTaskLabels = map[string]string{
	"svip_daily_gift":       "SVIP每日礼包",
	"monthly_card_reward":   "月卡奖励",
	"mall_daily_fertilizer": "商城每日肥料",
	"share_reward":          "自动领取分享奖励",
	"limited_seed_draw":     "荷风游记抽奖",
}

func FriendMischiefDailyDone(config map[string]any, now time.Time) bool {
	if len(config) == 0 {
		return false
	}
	date := strings.TrimSpace(fmt.Sprint(config[FriendMischiefDailyDoneDateConfigKey]))
	return date != "" && date == automationDate(now)
}

func MarkFriendMischiefDailyDone(config map[string]any, now time.Time) map[string]any {
	next := cloneConfig(config)
	next[FriendMischiefDailyDoneDateConfigKey] = automationDate(now)
	return next
}

func FriendHelpDailyRemaining(config map[string]any, now time.Time) (int, bool) {
	limit := intFromAny(config["autoFarmFriendHelpDailyLimit"])
	if limit <= 0 {
		return 0, false
	}
	count := 0
	if strings.TrimSpace(fmt.Sprint(config[FriendHelpDailyCountDateConfigKey])) == automationDate(now) {
		count = intFromAny(config[FriendHelpDailyCountConfigKey])
	}
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	return remaining, true
}

func MarkFriendHelpDailySuccesses(config map[string]any, now time.Time, successCount int) map[string]any {
	next := cloneConfig(config)
	date := automationDate(now)
	count := 0
	if strings.TrimSpace(fmt.Sprint(next[FriendHelpDailyCountDateConfigKey])) == date {
		count = intFromAny(next[FriendHelpDailyCountConfigKey])
	}
	if successCount < 0 {
		successCount = 0
	}
	next[FriendHelpDailyCountDateConfigKey] = date
	next[FriendHelpDailyCountConfigKey] = count + successCount
	return next
}

func DailyOnceTaskDoneDateConfigKey(taskID string) string {
	return dailyOnceTaskDoneDateConfigKeys[taskID]
}

func IsDailyOnceTask(taskID string) bool {
	return DailyOnceTaskDoneDateConfigKey(taskID) != ""
}

func DailyOnceTaskDone(config map[string]any, taskID string, now time.Time) bool {
	key := DailyOnceTaskDoneDateConfigKey(taskID)
	if key == "" || len(config) == 0 {
		return false
	}
	date := strings.TrimSpace(fmt.Sprint(config[key]))
	return date != "" && date == automationDate(now)
}

func MarkDailyOnceTaskDone(config map[string]any, taskID string, now time.Time) map[string]any {
	next := cloneConfig(config)
	if key := DailyOnceTaskDoneDateConfigKey(taskID); key != "" {
		next[key] = automationDate(now)
	}
	return next
}

func MergeRuntimeDailyState(incoming map[string]any, current map[string]any) map[string]any {
	merged := cloneConfig(incoming)
	keys := make([]string, 0, len(dailyOnceTaskDoneDateConfigKeys)+3)
	keys = append(keys, FriendMischiefDailyDoneDateConfigKey, FriendHelpDailyCountDateConfigKey, FriendHelpDailyCountConfigKey)
	for _, key := range dailyOnceTaskDoneDateConfigKeys {
		keys = append(keys, key)
	}
	for _, key := range keys {
		if value, ok := current[key]; ok {
			merged[key] = value
		}
	}
	return merged
}

func DailyOnceTaskSkipResult(taskID string) ActionResult {
	label := dailyOnceTaskLabels[taskID]
	if label == "" {
		label = taskID
	}
	return ActionResult{
		OK:      true,
		Status:  StatusOK,
		TaskID:  taskID,
		Message: fmt.Sprintf("%s今日已完成，本轮直接跳过。", label),
	}
}

func IsDailyOnceTaskDoneResult(result ActionResult) bool {
	return result.OK && result.Status == StatusOK && IsDailyOnceTask(result.TaskID)
}

func automationTaskDoneToday(config map[string]any, taskID string, now time.Time) bool {
	if taskID == "friend_mischief" {
		return FriendMischiefDailyDone(config, now)
	}
	return DailyOnceTaskDone(config, taskID, now)
}

func applyDailyOnceTaskState(task *SchedulerTask, config map[string]any, now time.Time) {
	if task == nil || !automationTaskDoneToday(config, task.ID, now) {
		return
	}
	task.DailyDoneToday = true
	if nextRunAt, ok := nextRewardSpecifiedRunAfterToday(config, task.ID, now); ok {
		task.NextRunAt = nextRunAt.Format(time.RFC3339)
		return
	}
	task.NextRunAt = nextAutomationDayStart(now).Format(time.RFC3339)
}

func IsFriendMischiefDailyLimitResult(result ActionResult) bool {
	return result.TaskID == "friend_mischief" && IsFriendMischiefDailyLimitText(result.Message)
}

func IsFriendMischiefDailyLimitText(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	compact := strings.ReplaceAll(text, " ", "")
	hasDay := strings.Contains(compact, "今日") || strings.Contains(compact, "今天") || strings.Contains(compact, "当日") || strings.Contains(compact, "本日")
	hasLimit := strings.Contains(compact, "上限") ||
		strings.Contains(compact, "已达") ||
		strings.Contains(compact, "达到") ||
		strings.Contains(compact, "达到了") ||
		strings.Contains(compact, "次数用完") ||
		strings.Contains(compact, "没有捣乱次数")
	// The friend-mischief protocol also returns the context-specific text
	// "操作次数已达上限" without an explicit "today" marker. Only accept
	// count-limit wording here so unrelated limits (inventory, level, etc.)
	// are not incorrectly persisted as the daily mischief completion state.
	hasMischiefCountContext := strings.Contains(compact, "操作次数") ||
		strings.Contains(compact, "捣乱次数") ||
		strings.Contains(compact, "恶作剧次数") ||
		strings.Contains(compact, "次数用完") ||
		strings.Contains(compact, "没有捣乱次数")
	return (hasDay && hasLimit) || (hasMischiefCountContext && hasLimit)
}

func automationDate(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	return now.Format("2006-01-02")
}

func nextAutomationDayStart(now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	year, month, day := now.Date()
	return time.Date(year, month, day+1, 0, 0, 0, 0, now.Location())
}
