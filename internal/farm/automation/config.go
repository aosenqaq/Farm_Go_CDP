package automation

var retiredAutomationSwitches = []string{
	"autoFarmHeFengTravelRewardEnabled",
	"autoFarmLimitedSeedDrawEnabled",
	"autoFarmLimitedSeedDrawPaidEnabled",
}

func isRetiredAutomationSwitch(key string) bool {
	for _, retiredKey := range retiredAutomationSwitches {
		if key == retiredKey {
			return true
		}
	}
	return false
}

func DefaultConfig() map[string]any {
	return map[string]any{
		"autoFarmBasicTasksEnabled":                 true,
		"autoFarmOneClickEnabled":                   true,
		"autoFarmOwnEnabled":                        true,
		"autoFarmOwnCollectEnabled":                 true,
		"autoFarmOwnCollectOnlyWhenOwnFarm":         true,
		"autoFarmOwnEraseGrassEnabled":              true,
		"autoFarmOwnWaterEnabled":                   true,
		"autoFarmOwnKillBugEnabled":                 true,
		"autoFarmLandUpgradeEnabled":                false,
		"autoFarmFriendEnabled":                     false,
		"autoFarmFriendHelpEnabled":                 false,
		"autoFarmFriendHelpGuardDogOnly":            false,
		"autoFarmFriendMischiefEnabled":             false,
		"autoFarmFriendHelpDailyLimit":              30,
		"autoFarmOwnIntervalSec":                    30,
		"autoFarmOwnBaseIntervalSec":                60,
		"autoFarmOwnCollectIntervalSec":             60,
		"autoFarmLandUpgradeIntervalSec":            60,
		"autoFarmPlantIntervalSec":                  30,
		"autoFarmFertilizerFillEnabled":             false,
		"autoFarmFertilizerFillIntervalSec":         43200,
		"autoFarmFriendStealIntervalSec":            90,
		"autoFarmFriendHelpIntervalSec":             90,
		"autoFarmFriendMischiefIntervalSec":         90,
		"autoRewardClaimEnabled":                    false,
		"autoRewardClaimIntervalSec":                30,
		"autoRewardClaimIntervalMin":                1,
		"autoRewardClaimScheduleMode":               "interval",
		"autoRewardClaimScheduleTime":               "08:00",
		"autoFarmSvipDailyGiftEnabled":              false,
		"autoFarmSvipDailyGiftIntervalSec":          43200,
		"autoFarmSvipDailyGiftIntervalMin":          720,
		"autoFarmSvipDailyGiftScheduleMode":         "interval",
		"autoFarmSvipDailyGiftScheduleTime":         "08:00",
		"autoFarmMonthlyCardRewardEnabled":          false,
		"autoFarmMonthlyCardRewardIntervalSec":      43200,
		"autoFarmMonthlyCardRewardIntervalMin":      720,
		"autoFarmMonthlyCardRewardScheduleMode":     "interval",
		"autoFarmMonthlyCardRewardScheduleTime":     "08:00",
		"autoFarmMallDailyFertilizerEnabled":        false,
		"autoFarmMallDailyFertilizerIntervalSec":    43200,
		"autoFarmMallDailyFertilizerIntervalMin":    720,
		"autoFarmMallDailyFertilizerScheduleMode":   "interval",
		"autoFarmMallDailyFertilizerScheduleTime":   "08:00",
		"autoFarmShareRewardEnabled":                false,
		"autoFarmShareRewardIntervalSec":            43200,
		"autoFarmShareRewardIntervalMin":            720,
		"autoFarmShareRewardScheduleMode":           "interval",
		"autoFarmShareRewardScheduleTime":           "08:00",
		"autoFarmMailRewardEnabled":                 false,
		"autoFarmMailRewardIntervalSec":             43200,
		"autoFarmMailRewardIntervalMin":             720,
		"autoFarmMailRewardScheduleMode":            "interval",
		"autoFarmMailRewardScheduleTime":            "08:00",
		"autoFarmQianXingTravelRewardEnabled":       false,
		"autoFarmQianXingTravelRewardIntervalSec":   7200,
		"autoFarmQianXingTravelRewardIntervalMin":   120,
		"autoFarmQianXingTravelRewardScheduleMode":  "interval",
		"autoFarmQianXingTravelRewardScheduleTime":  "08:00",
		"autoFarmXingSuAutoLightUpEnabled":          false,
		"autoFarmXingSuAutoLightUpIntervalSec":      7200,
		"autoFarmXingSuAutoLightUpIntervalMin":      120,
		"autoFarmXingSuAutoLightUpScheduleMode":     "interval",
		"autoFarmXingSuAutoLightUpScheduleTime":     "08:00",
		"autoFarmLimitedSeedDrawEnabled":            false,
		"autoFarmLimitedSeedDrawPaidEnabled":        false,
		"autoFarmLimitedSeedDrawIntervalSec":        43200,
		"autoFarmLimitedSeedDrawIntervalMin":        720,
		"autoFarmLimitedSeedDrawScheduleMode":       "interval",
		"autoFarmLimitedSeedDrawScheduleTime":       "08:00",
		"autoFarmMysteryShopAutoBuyEnabled":         false,
		"autoFarmMysteryShopAutoBuyIntervalSec":     43200,
		"autoFarmMysteryShopTargetSeedIds":          []int{},
		"autoFarmMysteryShopCurrencyIds":            []int{1001, 1002},
		"autoFarmMysteryShopDiscountThreshold":      0,
		"autoFarmFriendStealConcurrency":            3,
		"autoFarmFriendStealRandomDelayEnabled":     false,
		"autoFarmFriendStealRandomDelayMinMs":       1000,
		"autoFarmFriendStealRandomDelayMaxMs":       5000,
		"autoFarmFriendStealMaxFriends":             0,
		"autoFarmFriendHelpMaxFriends":              5,
		"autoFarmFriendMischiefMaxFriends":          200,
		"autoFarmFriendBlacklistCooldownMin":        10,
		"autoFarmRpcTimeoutMs":                      90000,
		"autoFarmPlantMode":                         "none",
		"autoFarmPlantPrimaryMode":                  "none",
		"autoFarmPlantSecondaryMode":                "none",
		"autoFarmPlantSeedId":                       0,
		"autoFarmFourGridPlantEnabled":              false,
		"autoFarmPlantBackpackSeedPriority":         []int{},
		"autoFarmPlantBackpackSeedDisabled":         []int{},
		"autoFarmPlantBackpackForcePriority":        false,
		"autoFarmPlantRandomizedEnabled":            false,
		"autoFarmPlantRandomOrderEnabled":           false,
		"autoFarmPlantRandomizedDelayMinMs":         100,
		"autoFarmPlantRandomizedDelayMaxMs":         500,
		"autoFarmPlantMaxLevel":                     0,
		"autoFarmFertilizerEnabled":                 false,
		"autoFarmFertilizerMode":                    "none",
		"autoFarmPlantFertilizerMode":               "none",
		"autoFarmRushFertilizerMode":                "none",
		"autoFarmFertilizerMultiSeason":             false,
		"autoFarmFertilizerHarvestLinkEnabled":      false,
		"autoFarmFertilizerContinuousRushEnabled":   false,
		"autoFarmFertilizerAutoBuyEnabled":          false,
		"autoFarmFertilizerAutoBuyType":             "inorganic",
		"autoFarmFertilizerAutoBuyMaxCount":         1,
		"autoFarmFertilizerLandTypes":               []string{"purpleGold", "gold", "black", "red", "normal"},
		"autoFarmFertilizerIntervalSec":             30,
		"autoFarmFertilizerInterLandIntervalSec":    0,
		"autoFarmFertilizerRushThresholdSec":        300,
		"autoFarmFertilizerMaxLandsPerRun":          0,
		"autoFarmFertilizerBatchChunkSize":          0,
		"autoFarmFertilizeBatchCallTimeoutMs":       180000,
		"autoFarmFertilizeSingleCallTimeoutMs":      90000,
		friendQuietHoursEnabledConfigKey:            false,
		friendQuietHoursStartConfigKey:              "23:00",
		friendQuietHoursEndConfigKey:                "07:00",
		friendQuietHoursModeConfigKey:               friendQuietHoursModeSleep,
		friendQuietHoursScopesConfigKey:             []string{"steal", "help"},
		"autoFarmFriendWhitelistEnabled":            false,
		"autoFarmFriendWhitelistScopes":             []string{},
		"autoFarmFriendWhitelist":                   []string{},
		"autoFarmFriendBlacklistEnabled":            false,
		"autoFarmFriendBlacklistScopes":             []string{},
		"autoFarmFriendBlacklist":                   []string{},
		"autoFarmFriendMaskedBlacklistEnabled":      true,
		"autoFarmFriendMaskedBlacklistMaxLevel":     1,
		"autoFarmFriendStealPlantBlacklistEnabled":  false,
		"autoFarmFriendStealPlantBlacklistStrategy": 1,
		"autoFarmFriendStealPlantListMode":          "blacklist",
		"autoFarmFriendStealPlantBlacklist":         []int{},
		"autoFarmFriendStealPlantWhitelist":         []int{},
		"autoFarmFriendMischiefGrassLandIds":        []int{},
		"autoFarmFriendMischiefBugLandIds":          []int{},
		"autoFarmAutoStartEnabled":                  false,
		"warehouseAutoRefreshEnabled":               true,
		"warehouseRefreshIntervalMin":               5,
		"warehouseRefreshOnlyOnAutoSell":            false,
		"autoWarehouseSellEnabled":                  false,
		"autoWarehouseSellIntervalMinute":           60,
		"autoWarehouseSellIntervalSec":              3600,
		"autoWarehouseSellCategories":               []string{"fruit", "mutation", "seed", "tool"},
	}
}

func MergeConfigWithDefaults(config map[string]any) map[string]any {
	merged := cloneConfig(DefaultConfig())
	for key, value := range config {
		if key == "" || value == nil {
			continue
		}
		if key == "autoFarmMysteryShopCurrencyIds" {
			merged[key] = normalizeMysteryShopCurrencyIDs(value)
			continue
		}
		merged[key] = value
	}
	for _, key := range retiredAutomationSwitches {
		merged[key] = false
	}
	return merged
}

func normalizeMysteryShopCurrencyIDs(value any) any {
	switch typed := value.(type) {
	case []int:
		for index, id := range typed {
			if id != 1003 {
				continue
			}
			normalized := append([]int(nil), typed...)
			normalized[index] = 1002
			return normalized
		}
	case []float64:
		for index, id := range typed {
			if id != 1003 {
				continue
			}
			normalized := append([]float64(nil), typed...)
			normalized[index] = 1002
			return normalized
		}
	case []any:
		for index, raw := range typed {
			switch id := raw.(type) {
			case int:
				if id != 1003 {
					continue
				}
				normalized := append([]any(nil), typed...)
				normalized[index] = 1002
				return normalized
			case float64:
				if id != 1003 {
					continue
				}
				normalized := append([]any(nil), typed...)
				normalized[index] = float64(1002)
				return normalized
			}
		}
	}
	return value
}

func cloneConfig(config map[string]any) map[string]any {
	next := make(map[string]any, len(config))
	for key, value := range config {
		next[key] = value
	}
	return next
}
