package automation

import (
	"testing"
	"time"
)

func TestRewardScheduleConfigKeysCoversRewardTasks(t *testing.T) {
	want := map[string][2]string{
		"reward_claim":            {"autoRewardClaimScheduleMode", "autoRewardClaimScheduleTime"},
		"svip_daily_gift":         {"autoFarmSvipDailyGiftScheduleMode", "autoFarmSvipDailyGiftScheduleTime"},
		"monthly_card_reward":     {"autoFarmMonthlyCardRewardScheduleMode", "autoFarmMonthlyCardRewardScheduleTime"},
		"mall_daily_fertilizer":   {"autoFarmMallDailyFertilizerScheduleMode", "autoFarmMallDailyFertilizerScheduleTime"},
		"share_reward":            {"autoFarmShareRewardScheduleMode", "autoFarmShareRewardScheduleTime"},
		"mail_reward":             {"autoFarmMailRewardScheduleMode", "autoFarmMailRewardScheduleTime"},
		"qian_xing_travel_reward": {"autoFarmQianXingTravelRewardScheduleMode", "autoFarmQianXingTravelRewardScheduleTime"},
		"xing_su_auto_light_up":   {"autoFarmXingSuAutoLightUpScheduleMode", "autoFarmXingSuAutoLightUpScheduleTime"},
		"limited_seed_draw":       {"autoFarmLimitedSeedDrawScheduleMode", "autoFarmLimitedSeedDrawScheduleTime"},
	}
	if len(rewardScheduleConfigs) != len(want) {
		t.Fatalf("rewardScheduleConfigs has %d entries; want exactly %d", len(rewardScheduleConfigs), len(want))
	}
	for taskID, keys := range want {
		modeKey, timeKey, ok := rewardScheduleConfigKeys(taskID)
		if !ok || modeKey != keys[0] || timeKey != keys[1] {
			t.Fatalf("rewardScheduleConfigKeys(%q) = %q, %q, %v; want %q, %q, true", taskID, modeKey, timeKey, ok, keys[0], keys[1])
		}
	}
	if _, _, ok := rewardScheduleConfigKeys("own_base"); ok {
		t.Fatal("own_base must not be treated as a reward specified-time task")
	}
}

func TestNextRewardSpecifiedRunUsesNextLocalOccurrence(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	tests := []struct {
		name         string
		now          time.Time
		scheduleTime string
		want         time.Time
	}{
		{"upcoming today", time.Date(2026, 7, 10, 8, 15, 0, 0, location), "09:30", time.Date(2026, 7, 10, 9, 30, 0, 0, location)},
		{"elapsed tomorrow", time.Date(2026, 7, 10, 10, 0, 0, 0, location), "09:30", time.Date(2026, 7, 11, 9, 30, 0, 0, location)},
		{"equal minute tomorrow", time.Date(2026, 7, 10, 9, 30, 0, 0, location), "09:30", time.Date(2026, 7, 11, 9, 30, 0, 0, location)},
		{"invalid falls back", time.Date(2026, 7, 10, 7, 0, 0, 0, location), "invalid", time.Date(2026, 7, 10, 8, 0, 0, 0, location)},
		{"empty falls back", time.Date(2026, 7, 10, 7, 0, 0, 0, location), "", time.Date(2026, 7, 10, 8, 0, 0, 0, location)},
		{"non-padded falls back", time.Date(2026, 7, 10, 7, 0, 0, 0, location), "9:30", time.Date(2026, 7, 10, 8, 0, 0, 0, location)},
		{"out of range falls back", time.Date(2026, 7, 10, 7, 0, 0, 0, location), "24:00", time.Date(2026, 7, 10, 8, 0, 0, 0, location)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := map[string]any{
				"autoFarmMailRewardScheduleMode": " daily_time ",
				"autoFarmMailRewardScheduleTime": tt.scheduleTime,
			}
			got, ok := nextRewardSpecifiedRun(config, "mail_reward", tt.now)
			if !ok || !got.Equal(tt.want) || got.Location() != location {
				t.Fatalf("nextRewardSpecifiedRun() = %v, %v; want %v, true", got, ok, tt.want)
			}
		})
	}
}

func TestNextRewardSpecifiedRunReconstructsTomorrowAfterSpringForwardGap(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 8, 4, 0, 0, 0, location)
	config := map[string]any{
		"autoFarmMailRewardScheduleMode": "daily_time",
		"autoFarmMailRewardScheduleTime": "02:30",
	}
	want := time.Date(2026, 3, 9, 2, 30, 0, 0, location)

	got, ok := nextRewardSpecifiedRun(config, "mail_reward", now)

	if !ok || !got.Equal(want) {
		t.Fatalf("nextRewardSpecifiedRun() = %v, %v; want %v, true", got, ok, want)
	}
}

func TestNextRewardSpecifiedRunRejectsIntervalAndUnsupportedTasks(t *testing.T) {
	now := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	config := map[string]any{
		"autoFarmMailRewardScheduleMode": "interval",
		"autoFarmMailRewardScheduleTime": "09:30",
	}
	if _, ok := nextRewardSpecifiedRun(config, "mail_reward", now); ok {
		t.Fatal("interval mode must use existing scheduler behavior")
	}
	if _, ok := nextRewardSpecifiedRun(config, "own_base", now); ok {
		t.Fatal("unsupported task must use existing scheduler behavior")
	}
}

func TestNextRewardSpecifiedRunAfterTodayUsesNextCalendarDay(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 10, 8, 15, 0, 0, location)
	config := map[string]any{
		"autoFarmMailRewardScheduleMode": "daily_time",
		"autoFarmMailRewardScheduleTime": "09:30",
	}
	want := time.Date(2026, 7, 11, 9, 30, 0, 0, location)

	got, ok := nextRewardSpecifiedRunAfterToday(config, "mail_reward", now)

	if !ok || !got.Equal(want) || got.Location() != location {
		t.Fatalf("nextRewardSpecifiedRunAfterToday() = %v, %v; want %v, true", got, ok, want)
	}
}
