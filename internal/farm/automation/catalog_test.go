package automation

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestStateContainsApprovedFeatureGroups(t *testing.T) {
	state := DefaultState()
	want := []string{"own_base", "planting", "fertilizer", "friends", "rewards", "mystery_shop"}
	if len(state.FeatureGroups) != len(want) {
		t.Fatalf("feature group count = %d, want %d", len(state.FeatureGroups), len(want))
	}
	for i, id := range want {
		if state.FeatureGroups[i].ID != id {
			t.Fatalf("feature group[%d] = %q, want %q", i, state.FeatureGroups[i].ID, id)
		}
	}
}

func TestDefaultConfigIncludesFriendQuietHoursModeAndScopes(t *testing.T) {
	config := DefaultConfig()
	if config[friendQuietHoursModeConfigKey] != friendQuietHoursModeSleep {
		t.Fatalf("quiet-hours mode = %#v, want sleep", config[friendQuietHoursModeConfigKey])
	}
	if !reflect.DeepEqual(config[friendQuietHoursScopesConfigKey], []string{"steal", "help"}) {
		t.Fatalf("quiet-hours scopes = %#v, want steal/help", config[friendQuietHoursScopesConfigKey])
	}
	merged := MergeConfigWithDefaults(map[string]any{friendQuietHoursScopesConfigKey: []string{}})
	if scopes, ok := merged[friendQuietHoursScopesConfigKey].([]string); !ok || len(scopes) != 0 {
		t.Fatalf("explicit empty scopes were replaced: %#v", merged[friendQuietHoursScopesConfigKey])
	}
}

func TestDefaultConfigUsesThePointCouponCurrencyID(t *testing.T) {
	config := DefaultConfig()
	if !reflect.DeepEqual(config["autoFarmMysteryShopCurrencyIds"], []int{1001, 1002}) {
		t.Fatalf("mystery shop default currencies = %#v, want gold and point coupons", config["autoFarmMysteryShopCurrencyIds"])
	}
}

func TestMergeConfigWithDefaultsMigratesLegacyMysteryShopPointCouponID(t *testing.T) {
	config := MergeConfigWithDefaults(map[string]any{
		"autoFarmMysteryShopCurrencyIds": []any{float64(1001), float64(1003), float64(1005)},
	})
	if !reflect.DeepEqual(config["autoFarmMysteryShopCurrencyIds"], []any{float64(1001), float64(1002), float64(1005)}) {
		t.Fatalf("mystery shop currencies = %#v, want migrated point coupon ID", config["autoFarmMysteryShopCurrencyIds"])
	}
}

func TestMergeConfigWithDefaultsPreservesCustomMysteryShopCurrencyIDs(t *testing.T) {
	custom := []any{float64(7001), float64(7002)}
	config := MergeConfigWithDefaults(map[string]any{
		"autoFarmMysteryShopCurrencyIds": custom,
	})
	if !reflect.DeepEqual(config["autoFarmMysteryShopCurrencyIds"], custom) {
		t.Fatalf("custom mystery shop currencies = %#v, want %#v", config["autoFarmMysteryShopCurrencyIds"], custom)
	}
}

func TestMergeConfigWithDefaultsDisablesRetiredHeFengTasks(t *testing.T) {
	config := MergeConfigWithDefaults(map[string]any{
		"autoFarmHeFengTravelRewardEnabled":  true,
		"autoFarmLimitedSeedDrawEnabled":     true,
		"autoFarmLimitedSeedDrawPaidEnabled": true,
	})

	for _, key := range []string{
		"autoFarmHeFengTravelRewardEnabled",
		"autoFarmLimitedSeedDrawEnabled",
		"autoFarmLimitedSeedDrawPaidEnabled",
	} {
		if got := config[key]; got != false {
			t.Fatalf("config[%s] = %#v, want false", key, got)
		}
	}
}

func TestDefaultConfigExcludesFriendGoldenBugSettings(t *testing.T) {
	config := DefaultConfig()
	for _, key := range []string{
		"autoFarmFriendGoldenBugEnabled",
		"autoFarmFriendGoldenBugSpecifiedEnabled",
		"autoFarmFriendGoldenBugFriendGids",
	} {
		if _, exists := config[key]; exists {
			t.Fatalf("retired golden-bug setting remains: %s", key)
		}
	}
}

func TestStateUsesOptimizedFeatureGroupLabels(t *testing.T) {
	state := DefaultState()
	want := map[string]string{
		"own_base":   "基础任务",
		"planting":   "自动种植",
		"fertilizer": "自动施肥",
		"friends":    "好友互动",
		"rewards":    "自动领取奖励",
	}
	for _, group := range state.FeatureGroups {
		if label, ok := want[group.ID]; ok && group.Label != label {
			t.Fatalf("feature group %s label = %q, want %q", group.ID, group.Label, label)
		}
	}
}

func TestStateFeatureGroupSettingKeysDoNotMutateCatalog(t *testing.T) {
	state := DefaultState()
	for index := range state.FeatureGroups {
		if state.FeatureGroups[index].ID == "own_base" {
			state.FeatureGroups[index].SettingKeys[0] = "mutated"
		}
	}

	next := DefaultState()
	for _, group := range next.FeatureGroups {
		if group.ID == "own_base" {
			if group.SettingKeys[0] != "autoFarmOneClickEnabled" {
				t.Fatalf("own_base setting keys were mutated through returned state: %#v", group.SettingKeys)
			}
			return
		}
	}
	t.Fatal("own_base feature group not found")
}

func TestStateDoesNotExposeRuntimeAsFeatureGroup(t *testing.T) {
	state := DefaultState()
	for _, group := range state.FeatureGroups {
		if group.ID == "runtime" || group.Label == "任务全局运行设置" {
			t.Fatalf("runtime settings should live in scheduler center, got feature group %#v", group)
		}
	}
}

func TestFertilizerFeatureGroupHidesFertilizerFillFromFrontend(t *testing.T) {
	state := DefaultState()
	for _, group := range state.FeatureGroups {
		if group.ID != "fertilizer" {
			continue
		}
		if strings.Contains(group.Summary, "填充肥料") {
			t.Fatalf("fertilizer summary exposes fertilizer fill: %q", group.Summary)
		}
		if slices.Contains(group.SettingKeys, "autoFarmFertilizerFillEnabled") {
			t.Fatalf("fertilizer setting keys expose fertilizer fill: %#v", group.SettingKeys)
		}
		return
	}
	t.Fatal("fertilizer feature group not found")
}

func TestFriendsFeatureGroupHidesFriendListsFromSummary(t *testing.T) {
	state := DefaultState()
	for _, group := range state.FeatureGroups {
		if group.ID != "friends" {
			continue
		}
		if strings.Contains(group.Summary, "黑白名单") {
			t.Fatalf("friends summary exposes friend lists: %q", group.Summary)
		}
		return
	}
	t.Fatal("friends feature group not found")
}

func TestSettingsRoundTripPreservesFeatureGroupSwitches(t *testing.T) {
	state := DefaultState()
	for index := range state.FeatureGroups {
		if state.FeatureGroups[index].ID == "own_base" {
			state.FeatureGroups[index].Enabled = false
		}
		if state.FeatureGroups[index].ID == "friends" {
			state.FeatureGroups[index].Enabled = true
		}
	}

	next := StateFromSettings(SettingsFromState(state))
	got := map[string]bool{}
	for _, group := range next.FeatureGroups {
		got[group.ID] = group.Enabled
	}

	if got["own_base"] {
		t.Fatalf("own_base feature group switch was not preserved: %#v", got)
	}
	if !got["friends"] {
		t.Fatalf("friends feature group switch was not preserved: %#v", got)
	}
}

func TestStateContainsReferenceSchedulerTasksSortedByPriority(t *testing.T) {
	state := DefaultState()
	if len(state.Scheduler.Tasks) != 19 {
		t.Fatalf("scheduler task count = %d, want 19", len(state.Scheduler.Tasks))
	}
	if state.Scheduler.Tasks[0].ID != "own_base" || state.Scheduler.Tasks[0].Priority != 100 {
		t.Fatalf("first task = %#v, want own_base priority 100", state.Scheduler.Tasks[0])
	}
	if state.Scheduler.Tasks[len(state.Scheduler.Tasks)-1].ID != "friend_mischief" {
		t.Fatalf("last task = %#v, want friend_mischief", state.Scheduler.Tasks[len(state.Scheduler.Tasks)-1])
	}
}

func TestStateDoesNotExposeRetiredSchedulerTasks(t *testing.T) {
	state := DefaultState()
	for _, taskID := range []string{"he_feng_travel_reward", "limited_seed_draw"} {
		if task := findSchedulerTaskForTest(state.Scheduler.Tasks, taskID); task != nil {
			t.Fatalf("retired task %s should not be exposed: %#v", taskID, task)
		}
	}

	task := findSchedulerTaskForTest(state.Scheduler.Tasks, "mystery_shop_auto_buy")
	if task == nil || task.Enabled {
		t.Fatalf("non-retired disabled task should remain visible: %#v", task)
	}
}

func TestDefaultStateExposesQianXingTravelReward(t *testing.T) {
	state := DefaultState()
	task := findSchedulerTaskForTest(state.Scheduler.Tasks, "qian_xing_travel_reward")
	if task == nil {
		t.Fatal("qian xing travel reward task not found")
	}
	if task.Label != "千星游记奖励领取" || task.Enabled || task.IntervalSec != 7200 {
		t.Fatalf("unexpected qian xing task: %#v", task)
	}
	if task.EnabledConfigKey != "autoFarmQianXingTravelRewardEnabled" || task.IntervalConfigKey != "autoFarmQianXingTravelRewardIntervalSec" {
		t.Fatalf("unexpected qian xing task config keys: %#v", task)
	}
	for key, want := range map[string]any{
		"autoFarmQianXingTravelRewardEnabled":      false,
		"autoFarmQianXingTravelRewardIntervalSec":  7200,
		"autoFarmQianXingTravelRewardIntervalMin":  120,
		"autoFarmQianXingTravelRewardScheduleMode": "interval",
		"autoFarmQianXingTravelRewardScheduleTime": "08:00",
	} {
		if got := state.Config[key]; got != want {
			t.Fatalf("config[%s] = %#v, want %#v", key, got, want)
		}
	}
}

func TestDefaultStateExposesXingSuAutoLightUp(t *testing.T) {
	state := DefaultState()
	task := findSchedulerTaskForTest(state.Scheduler.Tasks, "xing_su_auto_light_up")
	if task == nil {
		t.Fatal("xing su auto light up task not found")
	}
	if task.Label != "自动点亮星宿" || task.Enabled || task.IntervalSec != 7200 {
		t.Fatalf("unexpected xing su task: %#v", task)
	}
	if task.EnabledConfigKey != "autoFarmXingSuAutoLightUpEnabled" || task.IntervalConfigKey != "autoFarmXingSuAutoLightUpIntervalSec" {
		t.Fatalf("unexpected xing su task config keys: %#v", task)
	}
	for key, want := range map[string]any{
		"autoFarmXingSuAutoLightUpEnabled":      false,
		"autoFarmXingSuAutoLightUpIntervalSec":  7200,
		"autoFarmXingSuAutoLightUpIntervalMin":  120,
		"autoFarmXingSuAutoLightUpScheduleMode": "interval",
		"autoFarmXingSuAutoLightUpScheduleTime": "08:00",
	} {
		if got := state.Config[key]; got != want {
			t.Fatalf("config[%s] = %#v, want %#v", key, got, want)
		}
	}
}

func TestDefaultStateContainsWarehouseAutoSellSchedulerTask(t *testing.T) {
	task := findSchedulerTaskForTest(DefaultState().Scheduler.Tasks, "auto_warehouse_sell")
	if task == nil {
		t.Fatal("auto_warehouse_sell task not found")
	}
	if task.Label != "仓库自动出售" || task.Enabled || task.IntervalSec != 3600 {
		t.Fatalf("unexpected auto warehouse sell task: %#v", task)
	}
	if task.EnabledConfigKey != "autoWarehouseSellEnabled" || task.IntervalConfigKey != "autoWarehouseSellIntervalSec" {
		t.Fatalf("unexpected auto warehouse sell config keys: %#v", task)
	}
}

func TestDefaultConfigMatchesReferenceIntervals(t *testing.T) {
	state := DefaultState()
	got := map[string]int{}
	for _, task := range state.Scheduler.Tasks {
		got[task.ID] = task.IntervalSec
	}
	checks := map[string]int{
		"own_base":              60,
		"own_collect":           60,
		"own_plant":             10,
		"own_fertilizer":        30,
		"friend_steal":          90,
		"auto_warehouse_sell":   3600,
		"reward_claim":          3600,
		"mystery_shop_auto_buy": 43200,
	}
	for id, want := range checks {
		if got[id] != want {
			t.Fatalf("%s interval = %d, want %d", id, got[id], want)
		}
	}
}

func TestBasicTaskDefaultsUseSixtySecondInterval(t *testing.T) {
	state := DefaultState()
	intervals := map[string]int{}
	for _, task := range state.Scheduler.Tasks {
		intervals[task.ID] = task.IntervalSec
	}
	for _, taskID := range []string{"own_base", "own_collect", "land_upgrade"} {
		if got := intervals[taskID]; got != 60 {
			t.Fatalf("%s default interval = %d, want 60", taskID, got)
		}
	}

	config := DefaultConfig()
	for _, key := range []string{
		"autoFarmOwnBaseIntervalSec",
		"autoFarmOwnCollectIntervalSec",
		"autoFarmLandUpgradeIntervalSec",
	} {
		if got := config[key]; got != 60 {
			t.Fatalf("config[%s] = %#v, want 60", key, got)
		}
	}
}

func TestDefaultStateIncludesReferenceDetailedConfig(t *testing.T) {
	state := DefaultState()
	checks := map[string]any{
		"autoFarmBasicTasksEnabled":             true,
		"autoFarmOneClickEnabled":               true,
		"autoFarmPlantPrimaryMode":              "none",
		"autoFarmPlantRandomizedDelayMinMs":     100,
		"autoFarmFertilizerFillIntervalSec":     43200,
		"autoFarmMysteryShopDiscountThreshold":  0,
		"autoFarmFriendStealConcurrency":        3,
		"autoFarmFriendStealRandomDelayEnabled": false,
		"autoFarmFriendStealRandomDelayMinMs":   1000,
		"autoFarmFriendStealRandomDelayMaxMs":   5000,
		"autoFarmFriendStealMaxFriends":         0,
		"autoFarmFriendHelpGuardDogOnly":        false,
		"autoFarmFriendHelpDailyLimit":          30,
		"autoFarmFriendMischiefMaxFriends":      200,
		"autoFarmRpcTimeoutMs":                  90000,
		"autoRewardClaimScheduleMode":           "interval",
		"autoRewardClaimIntervalMin":            1,
		"autoFarmLimitedSeedDrawScheduleMode":   "interval",
		"autoFarmLimitedSeedDrawIntervalMin":    720,
		"autoFarmLimitedSeedDrawScheduleTime":   "08:00",
	}
	for key, want := range checks {
		if got := state.Config[key]; got != want {
			t.Fatalf("config[%s] = %#v, want %#v", key, got, want)
		}
	}
	if _, ok := state.Config["autoRewardClaimIntervalHour"]; ok {
		t.Fatal("autoRewardClaimIntervalHour should not remain in default automation config")
	}
	if _, ok := state.Config["autoFarmEnterWaitMs"]; ok {
		t.Fatal("autoFarmEnterWaitMs should not remain in default automation config")
	}
	if _, ok := state.Config["autoFarmActionWaitMs"]; ok {
		t.Fatal("autoFarmActionWaitMs should not remain in default automation config")
	}
}

func TestStateUsesFeatureConfigForSchedulerTaskEnabledState(t *testing.T) {
	state := StateFromSettings(Settings{
		SchedulerEnabled: true,
		Tasks: []TaskSettings{
			{ID: "friend_steal", Enabled: false, Priority: 70, IntervalSec: 90},
			{ID: "own_fertilizer", Enabled: false, Priority: 85, IntervalSec: 30},
		},
		Config: map[string]any{
			"autoFarmFriendEnabled":     true,
			"autoFarmFertilizerEnabled": true,
		},
	})

	if task := findSchedulerTaskForTest(state.Scheduler.Tasks, "friend_steal"); task == nil || !task.Enabled {
		t.Fatalf("friend_steal should follow autoFarmFriendEnabled: %#v", task)
	}
	if task := findSchedulerTaskForTest(state.Scheduler.Tasks, "own_fertilizer"); task == nil || !task.Enabled {
		t.Fatalf("own_fertilizer should follow autoFarmFertilizerEnabled: %#v", task)
	}
}

func TestStateFromSettingsUsesExplicitTaskConfigWithEnabledFeatureGroup(t *testing.T) {
	state := StateFromSettings(Settings{
		SchedulerEnabled: true,
		Tasks: []TaskSettings{
			{ID: "friend_steal", Enabled: true, Priority: 70, IntervalSec: 90},
		},
		Config: map[string]any{
			"autoFarmFeatureGroupEnabled.friends": true,
			"autoFarmFriendEnabled":               false,
		},
	})

	if task := findSchedulerTaskForTest(state.Scheduler.Tasks, "friend_steal"); task == nil || task.Enabled {
		t.Fatalf("friend_steal should follow explicit autoFarmFriendEnabled=false: %#v", task)
	}
}

func TestStateFromSettingsPreservesSubfeaturePreferenceWhenGroupDisabled(t *testing.T) {
	tests := []struct {
		name     string
		groupID  string
		groupKey string
		taskID   string
		taskKey  string
		priority int
		interval int
	}{
		{
			name:     "friends",
			groupID:  "friends",
			groupKey: "autoFarmFeatureGroupEnabled.friends",
			taskID:   "friend_steal",
			taskKey:  "autoFarmFriendEnabled",
			priority: 70,
			interval: 90,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := StateFromSettings(Settings{
				SchedulerEnabled: true,
				Tasks: []TaskSettings{
					{ID: test.taskID, Enabled: true, Priority: test.priority, IntervalSec: test.interval},
				},
				Config: map[string]any{
					test.groupKey: false,
					test.taskKey:  true,
				},
			})

			if task := findSchedulerTaskForTest(state.Scheduler.Tasks, test.taskID); task == nil || task.Enabled {
				t.Fatalf("disabled %s feature group should disable %s at runtime: %#v", test.groupID, test.taskID, task)
			}
			if state.Config[test.taskKey] != true {
				t.Fatalf("%s preference = %#v, want true", test.taskKey, state.Config[test.taskKey])
			}

			for index := range state.FeatureGroups {
				if state.FeatureGroups[index].ID == test.groupID {
					state.FeatureGroups[index].Enabled = true
				}
			}
			restored := StateFromSettings(SettingsFromState(state))
			if task := findSchedulerTaskForTest(restored.Scheduler.Tasks, test.taskID); task == nil || !task.Enabled {
				t.Fatalf("re-enabled %s feature group should restore %s preference: %#v", test.groupID, test.taskID, task)
			}
			if restored.Config[test.taskKey] != true {
				t.Fatalf("%s preference after re-enable = %#v, want true", test.taskKey, restored.Config[test.taskKey])
			}
		})
	}
}

func TestStateFromSettingsPreservesOldTaskEnabledWithoutExplicitConfig(t *testing.T) {
	state := StateFromSettings(Settings{
		SchedulerEnabled: true,
		Tasks: []TaskSettings{
			{ID: "own_fertilizer", Enabled: true, Priority: 85, IntervalSec: 30},
		},
	})

	if task := findSchedulerTaskForTest(state.Scheduler.Tasks, "own_fertilizer"); task == nil || !task.Enabled {
		t.Fatalf("task-only own_fertilizer enabled state should be preserved: %#v", task)
	}
}

func TestStateFromSettingsCanonicalizesLegacySharedTaskEnabledState(t *testing.T) {
	tests := []struct {
		name      string
		task      TaskSettings
		configKey string
		featureID string
	}{
		{
			name:      "fertilizer",
			task:      TaskSettings{ID: "own_fertilizer", Enabled: true, Priority: 85, IntervalSec: 30},
			configKey: "autoFarmFertilizerEnabled",
			featureID: "fertilizer",
		},
		{
			name:      "mystery shop",
			task:      TaskSettings{ID: "mystery_shop_auto_buy", Enabled: true, Priority: 91, IntervalSec: 43200},
			configKey: "autoFarmMysteryShopAutoBuyEnabled",
			featureID: "mystery_shop",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := StateFromSettings(Settings{
				SchedulerEnabled: true,
				Tasks:            []TaskSettings{test.task},
			})

			assertCanonicalAutomationEnabledState(t, state, test.task.ID, test.configKey, test.featureID)
			state = StateFromSettings(SettingsFromState(state))
			assertCanonicalAutomationEnabledState(t, state, test.task.ID, test.configKey, test.featureID)
		})
	}
}

func TestStateFromSettingsCanonicalizesLegacyIndependentFeatureGroup(t *testing.T) {
	state := StateFromSettings(Settings{
		SchedulerEnabled: true,
		Tasks: []TaskSettings{
			{ID: "friend_steal", Enabled: true, Priority: 70, IntervalSec: 90},
		},
	})

	assertCanonicalAutomationEnabledState(t, state, "friend_steal", "autoFarmFriendEnabled", "friends")
	if state.Config["autoFarmFeatureGroupEnabled.friends"] != true {
		t.Fatalf("autoFarmFeatureGroupEnabled.friends = %#v, want true", state.Config["autoFarmFeatureGroupEnabled.friends"])
	}
	state = StateFromSettings(SettingsFromState(state))
	assertCanonicalAutomationEnabledState(t, state, "friend_steal", "autoFarmFriendEnabled", "friends")
	if state.Config["autoFarmFeatureGroupEnabled.friends"] != true {
		t.Fatalf("autoFarmFeatureGroupEnabled.friends after round trip = %#v, want true", state.Config["autoFarmFeatureGroupEnabled.friends"])
	}
}

func TestSettingsFromStateUsesFeatureConfigForSchedulerTaskEnabledState(t *testing.T) {
	state := DefaultState()
	state.Config["autoFarmFriendEnabled"] = true

	settings := SettingsFromState(state)

	if task := findTaskSettingsForTest(settings.Tasks, "friend_steal"); task == nil || !task.Enabled {
		t.Fatalf("friend_steal should follow autoFarmFriendEnabled=true: %#v", task)
	}
}

func TestSettingsRoundTripUsesFeatureConfigAsSchedulerTaskEnabledSource(t *testing.T) {
	state := DefaultState()
	state.Config["autoFarmFriendEnabled"] = true
	for index := range state.FeatureGroups {
		if state.FeatureGroups[index].ID == "friends" {
			state.FeatureGroups[index].Enabled = true
		}
	}
	for index := range state.Scheduler.Tasks {
		if state.Scheduler.Tasks[index].ID == "friend_steal" {
			state.Scheduler.Tasks[index].Enabled = true
		}
	}

	state = StateFromSettings(SettingsFromState(state))
	if task := findSchedulerTaskForTest(state.Scheduler.Tasks, "friend_steal"); task == nil || !task.Enabled {
		t.Fatalf("friend_steal should remain enabled after the first round trip: %#v", task)
	}
	state.Config["autoFarmFriendEnabled"] = false
	state = StateFromSettings(SettingsFromState(state))

	if state.Config["autoFarmFriendEnabled"] != false {
		t.Fatalf("autoFarmFriendEnabled = %#v, want false", state.Config["autoFarmFriendEnabled"])
	}
	if task := findSchedulerTaskForTest(state.Scheduler.Tasks, "friend_steal"); task == nil || task.Enabled {
		t.Fatalf("friend_steal should follow autoFarmFriendEnabled=false: %#v", task)
	}
}

func TestStateFromSettingsUsesFertilizerGroupAsMasterSwitch(t *testing.T) {
	state := StateFromSettings(Settings{
		SchedulerEnabled: true,
		Tasks: []TaskSettings{
			{ID: "own_fertilizer", Enabled: false, Priority: 85, IntervalSec: 30},
		},
		Config: map[string]any{
			"autoFarmFeatureGroupEnabled.fertilizer": true,
			"autoFarmFertilizerEnabled":              false,
		},
	})

	if state.Config["autoFarmFertilizerEnabled"] != true {
		t.Fatalf("autoFarmFertilizerEnabled = %#v, want true", state.Config["autoFarmFertilizerEnabled"])
	}
	if task := findSchedulerTaskForTest(state.Scheduler.Tasks, "own_fertilizer"); task == nil || !task.Enabled {
		t.Fatalf("own_fertilizer should follow fertilizer group master switch: %#v", task)
	}
}

func TestStateFromSettingsFallsBackToLegacyFertilizerSwitch(t *testing.T) {
	state := StateFromSettings(Settings{
		SchedulerEnabled: true,
		Tasks: []TaskSettings{
			{ID: "own_fertilizer", Enabled: false, Priority: 85, IntervalSec: 30},
		},
		Config: map[string]any{
			"autoFarmFertilizerEnabled": true,
		},
	})

	foundEnabledGroup := false
	for _, group := range state.FeatureGroups {
		if group.ID == "fertilizer" && group.Enabled {
			foundEnabledGroup = true
		}
	}
	if !foundEnabledGroup {
		t.Fatalf("fertilizer group should follow legacy switch: %#v", state.FeatureGroups)
	}
	if task := findSchedulerTaskForTest(state.Scheduler.Tasks, "own_fertilizer"); task == nil || !task.Enabled {
		t.Fatalf("own_fertilizer should follow legacy switch: %#v", task)
	}
}

func TestSettingsFromStateWritesConsistentFertilizerMasterSwitch(t *testing.T) {
	state := DefaultState()
	state.Config["autoFarmFertilizerEnabled"] = false
	for index := range state.FeatureGroups {
		if state.FeatureGroups[index].ID == "fertilizer" {
			state.FeatureGroups[index].Enabled = true
		}
	}
	for index := range state.Scheduler.Tasks {
		if state.Scheduler.Tasks[index].ID == "own_fertilizer" {
			state.Scheduler.Tasks[index].Enabled = false
		}
	}

	settings := SettingsFromState(state)
	if settings.Config["autoFarmFeatureGroupEnabled.fertilizer"] != true || settings.Config["autoFarmFertilizerEnabled"] != true {
		t.Fatalf("fertilizer master config is inconsistent: %#v", settings.Config)
	}
	if task := findTaskSettingsForTest(settings.Tasks, "own_fertilizer"); task == nil || !task.Enabled {
		t.Fatalf("persisted own_fertilizer should be enabled: %#v", task)
	}
}

func TestStateFromSettingsUsesDetailedIntervalConfigForSchedulerTasks(t *testing.T) {
	state := StateFromSettings(Settings{
		SchedulerEnabled: true,
		Tasks: []TaskSettings{
			{ID: "friend_steal", Enabled: true, Priority: 70, IntervalSec: 90},
		},
		Config: map[string]any{
			"autoFarmFriendStealIntervalSec": 5,
		},
	})

	task := findSchedulerTaskForTest(state.Scheduler.Tasks, "friend_steal")
	if task == nil || task.IntervalSec != 5 {
		t.Fatalf("friend_steal interval = %#v, want detailed config interval 5", task)
	}
	if state.Config["autoFarmFriendStealIntervalSec"] != 5 {
		t.Fatalf("config interval = %#v, want 5", state.Config["autoFarmFriendStealIntervalSec"])
	}
}

func TestSettingsFromStateWritesSchedulerTaskIntervalToDetailedConfig(t *testing.T) {
	state := DefaultState()
	for index := range state.Scheduler.Tasks {
		if state.Scheduler.Tasks[index].ID == "friend_steal" {
			state.Scheduler.Tasks[index].IntervalSec = 7
			state.Scheduler.Tasks[index].IntervalConfigKey = ""
		}
	}

	settings := SettingsFromState(state)

	if settings.Config["autoFarmFriendStealIntervalSec"] != 7 {
		t.Fatalf("autoFarmFriendStealIntervalSec = %#v, want 7", settings.Config["autoFarmFriendStealIntervalSec"])
	}
	task := findTaskSettingsForTest(settings.Tasks, "friend_steal")
	if task == nil || task.IntervalSec != 7 {
		t.Fatalf("friend_steal task interval = %#v, want 7", task)
	}
}

func TestSettingsFromStatePreservesLegacyTaskIntervalWhenConfigContainsJSONDefault(t *testing.T) {
	state := DefaultState()
	state.Config["autoFarmOwnCollectIntervalSec"] = float64(60)
	for index := range state.Scheduler.Tasks {
		if state.Scheduler.Tasks[index].ID == "own_collect" {
			state.Scheduler.Tasks[index].IntervalSec = 120
			state.Scheduler.Tasks[index].IntervalConfigKey = ""
		}
	}

	settings := SettingsFromState(state)

	task := findTaskSettingsForTest(settings.Tasks, "own_collect")
	if task == nil || task.IntervalSec != 120 {
		t.Fatalf("own_collect task interval = %#v, want 120", task)
	}
	if got := intConfig(settings.Config["autoFarmOwnCollectIntervalSec"], 0); got != 120 {
		t.Fatalf("autoFarmOwnCollectIntervalSec = %d, want 120", got)
	}
}

func TestSettingsFromStateUsesCanonicalDefaultIntervalToResetStaleTask(t *testing.T) {
	state := DefaultState()
	state.Config["autoFarmOwnCollectIntervalSec"] = float64(60)
	for index := range state.Scheduler.Tasks {
		if state.Scheduler.Tasks[index].ID == "own_collect" {
			state.Scheduler.Tasks[index].IntervalSec = 120
		}
	}

	settings := SettingsFromState(state)

	task := findTaskSettingsForTest(settings.Tasks, "own_collect")
	if task == nil || task.IntervalSec != 60 {
		t.Fatalf("own_collect task interval = %#v, want canonical default 60", task)
	}
	if got := intConfig(settings.Config["autoFarmOwnCollectIntervalSec"], 0); got != 60 {
		t.Fatalf("autoFarmOwnCollectIntervalSec = %d, want 60", got)
	}
}

func TestSchedulerTaskExposesCanonicalConfigKeys(t *testing.T) {
	state := DefaultState()
	for _, task := range state.Scheduler.Tasks {
		if got, want := task.EnabledConfigKey, schedulerTaskConfigKeys[task.ID]; got != want {
			t.Errorf("%s enabled config key = %q, want %q", task.ID, got, want)
		}
		if got, want := task.IntervalConfigKey, schedulerTaskIntervalConfigKeys[task.ID]; got != want {
			t.Errorf("%s interval config key = %q, want %q", task.ID, got, want)
		}
	}
}

func TestSettingsFromStateUsesAllDetailedSchedulerConfigValuesAsCanonicalSource(t *testing.T) {
	for index, def := range schedulerTaskDefs {
		if isRetiredAutomationSwitch(schedulerTaskConfigKeys[def.id]) {
			continue
		}
		t.Run(def.id, func(t *testing.T) {
			state := DefaultState()
			task := findSchedulerTaskForTest(state.Scheduler.Tasks, def.id)
			if task == nil {
				t.Fatalf("task %s not found", def.id)
			}
			if key := schedulerTaskConfigKeys[def.id]; key != "" {
				state.Config[key] = !task.Enabled
				if def.id == fertilizerTaskID {
					for groupIndex := range state.FeatureGroups {
						if state.FeatureGroups[groupIndex].ID == "fertilizer" {
							state.FeatureGroups[groupIndex].Enabled = !task.Enabled
						}
					}
				}
			}
			wantInterval := task.IntervalSec + 137 + index
			if key := schedulerTaskIntervalConfigKeys[def.id]; key != "" {
				state.Config[key] = wantInterval
			}

			settings := SettingsFromState(state)
			savedTask := findTaskSettingsForTest(settings.Tasks, def.id)
			if savedTask == nil {
				t.Fatalf("saved task %s not found", def.id)
			}
			if key := schedulerTaskConfigKeys[def.id]; key != "" {
				wantEnabled := boolConfig(state.Config[key], task.Enabled)
				if savedTask.Enabled != wantEnabled || settings.Config[key] != wantEnabled {
					t.Errorf("enabled canonicalization = task %v config %#v, want %v", savedTask.Enabled, settings.Config[key], wantEnabled)
				}
			}
			if key := schedulerTaskIntervalConfigKeys[def.id]; key != "" {
				if savedTask.IntervalSec != wantInterval || settings.Config[key] != wantInterval {
					t.Errorf("interval canonicalization = task %d config %#v, want %d", savedTask.IntervalSec, settings.Config[key], wantInterval)
				}
			}
		})
	}
}

func findSchedulerTaskForTest(tasks []SchedulerTask, id string) *SchedulerTask {
	for index := range tasks {
		if tasks[index].ID == id {
			return &tasks[index]
		}
	}
	return nil
}

func findTaskSettingsForTest(tasks []TaskSettings, id string) *TaskSettings {
	for index := range tasks {
		if tasks[index].ID == id {
			return &tasks[index]
		}
	}
	return nil
}

func assertCanonicalAutomationEnabledState(t *testing.T, state State, taskID, configKey, featureID string) {
	t.Helper()
	if task := findSchedulerTaskForTest(state.Scheduler.Tasks, taskID); task == nil || !task.Enabled {
		t.Fatalf("%s should be enabled: %#v", taskID, task)
	}
	if state.Config[configKey] != true {
		t.Fatalf("config[%s] = %#v, want true", configKey, state.Config[configKey])
	}
	for _, group := range state.FeatureGroups {
		if group.ID == featureID {
			if !group.Enabled {
				t.Fatalf("feature group %s should be enabled: %#v", featureID, group)
			}
			return
		}
	}
	t.Fatalf("feature group %s not found", featureID)
}

func TestStateFromSettingsMergesDetailedConfigDefaults(t *testing.T) {
	state := StateFromSettings(Settings{
		Config: map[string]any{
			"autoFarmOneClickEnabled": false,
			"autoFarmPlantSeedId":     100123,
		},
	})

	if state.Config["autoFarmOneClickEnabled"] != false {
		t.Fatalf("override was not preserved: %#v", state.Config)
	}
	if state.Config["autoFarmPlantSeedId"] != 100123 {
		t.Fatalf("seed override was not preserved: %#v", state.Config)
	}
	if state.Config["autoFarmOwnCollectEnabled"] != true {
		t.Fatalf("default config was not merged: %#v", state.Config)
	}
}

func TestRunTaskReturnsNotMigrated(t *testing.T) {
	result := RunTask("friend_steal")
	if result.OK {
		t.Fatalf("RunTask returned OK for phase 1: %#v", result)
	}
	if result.Status != StatusNotMigrated {
		t.Fatalf("status = %q, want %q", result.Status, StatusNotMigrated)
	}
	if result.TaskID != "friend_steal" {
		t.Fatalf("taskID = %q, want friend_steal", result.TaskID)
	}
}
