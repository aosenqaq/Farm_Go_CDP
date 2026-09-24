package automation

import (
	"fmt"
	"strings"
	"time"
)

type rewardScheduleConfig struct {
	modeKey string
	timeKey string
}

var rewardScheduleConfigs = map[string]rewardScheduleConfig{
	"reward_claim":            {modeKey: "autoRewardClaimScheduleMode", timeKey: "autoRewardClaimScheduleTime"},
	"svip_daily_gift":         {modeKey: "autoFarmSvipDailyGiftScheduleMode", timeKey: "autoFarmSvipDailyGiftScheduleTime"},
	"monthly_card_reward":     {modeKey: "autoFarmMonthlyCardRewardScheduleMode", timeKey: "autoFarmMonthlyCardRewardScheduleTime"},
	"mall_daily_fertilizer":   {modeKey: "autoFarmMallDailyFertilizerScheduleMode", timeKey: "autoFarmMallDailyFertilizerScheduleTime"},
	"share_reward":            {modeKey: "autoFarmShareRewardScheduleMode", timeKey: "autoFarmShareRewardScheduleTime"},
	"mail_reward":             {modeKey: "autoFarmMailRewardScheduleMode", timeKey: "autoFarmMailRewardScheduleTime"},
	"qian_xing_travel_reward": {modeKey: "autoFarmQianXingTravelRewardScheduleMode", timeKey: "autoFarmQianXingTravelRewardScheduleTime"},
	"xing_su_auto_light_up":   {modeKey: "autoFarmXingSuAutoLightUpScheduleMode", timeKey: "autoFarmXingSuAutoLightUpScheduleTime"},
	"limited_seed_draw":       {modeKey: "autoFarmLimitedSeedDrawScheduleMode", timeKey: "autoFarmLimitedSeedDrawScheduleTime"},
}

func rewardScheduleConfigKeys(taskID string) (string, string, bool) {
	schedule, ok := rewardScheduleConfigs[taskID]
	return schedule.modeKey, schedule.timeKey, ok
}

func rewardSpecifiedClock(config map[string]any, taskID string) (int, int, bool) {
	modeKey, timeKey, ok := rewardScheduleConfigKeys(taskID)
	if !ok || strings.TrimSpace(fmt.Sprint(config[modeKey])) != "daily_time" {
		return 0, 0, false
	}
	clockText := strings.TrimSpace(fmt.Sprint(config[timeKey]))
	if len(clockText) != 5 || clockText[2] != ':' ||
		clockText[0] < '0' || clockText[0] > '9' ||
		clockText[1] < '0' || clockText[1] > '9' ||
		clockText[3] < '0' || clockText[3] > '9' ||
		clockText[4] < '0' || clockText[4] > '9' {
		clockText = "08:00"
	}
	parsed, err := time.Parse("15:04", clockText)
	if err != nil {
		parsed, _ = time.Parse("15:04", "08:00")
	}
	return parsed.Hour(), parsed.Minute(), true
}

func nextRewardSpecifiedRun(config map[string]any, taskID string, now time.Time) (time.Time, bool) {
	hour, minute, ok := rewardSpecifiedClock(config, taskID)
	if !ok {
		return time.Time{}, false
	}
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !target.After(now) {
		nextDay := now.AddDate(0, 0, 1)
		target = time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), hour, minute, 0, 0, now.Location())
	}
	return target, true
}

func nextRewardSpecifiedRunAfterToday(config map[string]any, taskID string, now time.Time) (time.Time, bool) {
	hour, minute, ok := rewardSpecifiedClock(config, taskID)
	if !ok {
		return time.Time{}, false
	}
	nextDay := now.AddDate(0, 0, 1)
	return time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), hour, minute, 0, 0, now.Location()), true
}
