package automation

import (
	"sort"
	"strconv"
	"time"
)

type ActionStatus string

const (
	StatusOK              ActionStatus = "ok"
	StatusNotMigrated     ActionStatus = "not_migrated"
	StatusRuntimeReady    ActionStatus = "runtime_ready"
	StatusRuntimeNotReady ActionStatus = "runtime_not_ready"
	StatusRuntimeBusy     ActionStatus = "busy"
	StatusFailed          ActionStatus = "failed"
)

type ActionResult struct {
	OK                bool         `json:"ok"`
	Status            ActionStatus `json:"status"`
	TaskID            string       `json:"taskId,omitempty"`
	Message           string       `json:"message"`
	ActionCount       int          `json:"actionCount,omitempty"`
	AttemptedFriends  int          `json:"attemptedFriends,omitempty"`
	SuccessfulFriends int          `json:"successfulFriends,omitempty"`
	FailedFriends     int          `json:"failedFriends,omitempty"`
	SkippedFriends    int          `json:"skippedFriends,omitempty"`
}

type FeatureGroup struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Summary     string   `json:"summary"`
	Enabled     bool     `json:"enabled"`
	SettingKeys []string `json:"settingKeys"`
}

type SchedulerTask struct {
	ID                string `json:"id"`
	Label             string `json:"label"`
	EnabledConfigKey  string `json:"enabledConfigKey,omitempty"`
	IntervalConfigKey string `json:"intervalConfigKey,omitempty"`
	Priority          int    `json:"priority"`
	IntervalSec       int    `json:"intervalSec"`
	Enabled           bool   `json:"enabled"`
	DailyDoneToday    bool   `json:"dailyDoneToday,omitempty"`
	NextRunAt         string `json:"nextRunAt,omitempty"`
	LastStartedAt     string `json:"lastStartedAt,omitempty"`
	LastFinishedAt    string `json:"lastFinishedAt,omitempty"`
	LastSuccessAt     string `json:"lastSuccessAt,omitempty"`
	LastError         string `json:"lastError,omitempty"`
	LastResultSummary string `json:"lastResultSummary,omitempty"`
}

type SchedulerState struct {
	Enabled       bool            `json:"enabled"`
	MinGapMs      int             `json:"minGapMs"`
	RunningTaskID string          `json:"runningTaskId,omitempty"`
	Tasks         []SchedulerTask `json:"tasks"`
}

type State struct {
	Running       bool           `json:"running"`
	RunMode       RunMode        `json:"runMode"`
	Summary       map[string]int `json:"summary"`
	FeatureGroups []FeatureGroup `json:"featureGroups"`
	Scheduler     SchedulerState `json:"scheduler"`
	Config        map[string]any `json:"config"`
}

type Settings struct {
	SchedulerEnabled  bool           `json:"schedulerEnabled"`
	SchedulerMinGapMs int            `json:"schedulerMinGapMs"`
	RunMode           RunMode        `json:"runMode"`
	Tasks             []TaskSettings `json:"tasks"`
	Config            map[string]any `json:"config"`
}

type TaskSettings struct {
	ID          string `json:"id"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`
	IntervalSec int    `json:"intervalSec"`
}

type taskDef struct {
	id          string
	label       string
	priority    int
	intervalSec int
	enabled     bool
}

var featureGroupSwitchConfigKeys = map[string]string{
	"own_base":     "autoFarmFeatureGroupEnabled.own_base",
	"planting":     "autoFarmFeatureGroupEnabled.planting",
	"fertilizer":   "autoFarmFeatureGroupEnabled.fertilizer",
	"friends":      "autoFarmFeatureGroupEnabled.friends",
	"rewards":      "autoFarmFeatureGroupEnabled.rewards",
	"mystery_shop": "autoFarmFeatureGroupEnabled.mystery_shop",
}

var legacyFeatureGroupSwitchConfigKeys = map[string]string{
	"own_base": "autoFarmBasicTasksEnabled",
}

const (
	fertilizerFeatureGroupConfigKey = "autoFarmFeatureGroupEnabled.fertilizer"
	fertilizerTaskConfigKey         = "autoFarmFertilizerEnabled"
	fertilizerTaskID                = "own_fertilizer"
)

var schedulerTaskConfigKeys = map[string]string{
	"own_base":                "autoFarmOneClickEnabled",
	"land_upgrade":            "autoFarmLandUpgradeEnabled",
	"own_collect":             "autoFarmOwnCollectEnabled",
	"own_plant":               "autoFarmPlantEnabled",
	"fertilizer_fill":         "autoFarmFertilizerFillEnabled",
	"own_fertilizer":          "autoFarmFertilizerEnabled",
	"friend_steal":            "autoFarmFriendEnabled",
	"friend_help":             "autoFarmFriendHelpEnabled",
	"friend_mischief":         "autoFarmFriendMischiefEnabled",
	"reward_claim":            "autoRewardClaimEnabled",
	"svip_daily_gift":         "autoFarmSvipDailyGiftEnabled",
	"monthly_card_reward":     "autoFarmMonthlyCardRewardEnabled",
	"mall_daily_fertilizer":   "autoFarmMallDailyFertilizerEnabled",
	"share_reward":            "autoFarmShareRewardEnabled",
	"mail_reward":             "autoFarmMailRewardEnabled",
	"qian_xing_travel_reward": "autoFarmQianXingTravelRewardEnabled",
	"xing_su_auto_light_up":   "autoFarmXingSuAutoLightUpEnabled",
	"limited_seed_draw":       "autoFarmLimitedSeedDrawEnabled",
	"mystery_shop_auto_buy":   "autoFarmMysteryShopAutoBuyEnabled",
	"auto_warehouse_sell":     "autoWarehouseSellEnabled",
}

var schedulerTaskIntervalConfigKeys = map[string]string{
	"own_base":                "autoFarmOwnBaseIntervalSec",
	"land_upgrade":            "autoFarmLandUpgradeIntervalSec",
	"own_collect":             "autoFarmOwnCollectIntervalSec",
	"own_plant":               "autoFarmPlantIntervalSec",
	"fertilizer_fill":         "autoFarmFertilizerFillIntervalSec",
	"own_fertilizer":          "autoFarmFertilizerIntervalSec",
	"friend_steal":            "autoFarmFriendStealIntervalSec",
	"friend_help":             "autoFarmFriendHelpIntervalSec",
	"friend_mischief":         "autoFarmFriendMischiefIntervalSec",
	"reward_claim":            "autoRewardClaimIntervalSec",
	"svip_daily_gift":         "autoFarmSvipDailyGiftIntervalSec",
	"monthly_card_reward":     "autoFarmMonthlyCardRewardIntervalSec",
	"mall_daily_fertilizer":   "autoFarmMallDailyFertilizerIntervalSec",
	"share_reward":            "autoFarmShareRewardIntervalSec",
	"mail_reward":             "autoFarmMailRewardIntervalSec",
	"qian_xing_travel_reward": "autoFarmQianXingTravelRewardIntervalSec",
	"xing_su_auto_light_up":   "autoFarmXingSuAutoLightUpIntervalSec",
	"limited_seed_draw":       "autoFarmLimitedSeedDrawIntervalSec",
	"mystery_shop_auto_buy":   "autoFarmMysteryShopAutoBuyIntervalSec",
	"auto_warehouse_sell":     "autoWarehouseSellIntervalSec",
}

var featureGroupTaskIDs = map[string][]string{
	"own_base":     {"own_base", "own_collect", "land_upgrade"},
	"planting":     {"own_plant"},
	"fertilizer":   {"fertilizer_fill", "own_fertilizer"},
	"friends":      {"friend_steal", "friend_help", "friend_mischief"},
	"rewards":      {"reward_claim", "svip_daily_gift", "monthly_card_reward", "mall_daily_fertilizer", "share_reward", "mail_reward", "qian_xing_travel_reward", "xing_su_auto_light_up", "limited_seed_draw"},
	"mystery_shop": {"mystery_shop_auto_buy"},
}

var featureGroups = []FeatureGroup{
	{ID: "own_base", Label: "基础任务", Summary: "一键务农、自动收获、除草、浇水、杀虫。", Enabled: true, SettingKeys: []string{"autoFarmOneClickEnabled", "autoFarmOwnCollectEnabled"}},
	{ID: "planting", Label: "自动种植", Summary: "主/副策略、背包种子选择、随机延迟。", Enabled: true, SettingKeys: []string{"autoFarmPlantPrimaryMode", "autoFarmPlantSecondaryMode"}},
	{ID: "fertilizer", Label: "自动施肥", Summary: "自动施肥、催熟联动、自动购买。", Enabled: false, SettingKeys: []string{"autoFarmFertilizerEnabled"}},
	{ID: "friends", Label: "好友互动", Summary: "偷菜、帮忙、捣乱、冷却与静默时间。", Enabled: false, SettingKeys: []string{"autoFarmFriendEnabled", "autoFarmFriendHelpEnabled"}},
	{ID: "rewards", Label: "自动领取奖励", Summary: "任务奖励、礼包、月卡、邮件、抽奖。", Enabled: true, SettingKeys: []string{"autoRewardClaimEnabled", "autoFarmSvipDailyGiftEnabled"}},
	{ID: "mystery_shop", Label: "神秘商店自动购买", Summary: "货币类型、折扣阈值和手动读取购买。", Enabled: false, SettingKeys: []string{"autoFarmMysteryShopAutoBuyEnabled"}},
}

var schedulerTaskDefs = []taskDef{
	{id: "own_base", label: "一键务农", priority: 100, intervalSec: 60, enabled: true},
	{id: "land_upgrade", label: "土地自动升级", priority: 99, intervalSec: 60, enabled: true},
	{id: "reward_claim", label: "自动领取任务奖励", priority: 98, intervalSec: 3600, enabled: true},
	{id: "svip_daily_gift", label: "SVIP每日礼包", priority: 97, intervalSec: 43200, enabled: true},
	{id: "monthly_card_reward", label: "月卡奖励", priority: 96, intervalSec: 43200, enabled: true},
	{id: "mall_daily_fertilizer", label: "商城每日肥料", priority: 95, intervalSec: 43200, enabled: true},
	{id: "share_reward", label: "自动领取分享奖励", priority: 94, intervalSec: 43200, enabled: true},
	{id: "mail_reward", label: "自动领取邮件奖励", priority: 93, intervalSec: 43200, enabled: true},
	{id: "qian_xing_travel_reward", label: "千星游记奖励领取", priority: 92, intervalSec: 7200, enabled: false},
	{id: "xing_su_auto_light_up", label: "自动点亮星宿", priority: 91, intervalSec: 7200, enabled: false},
	{id: "limited_seed_draw", label: "荷风游记抽奖", priority: 92, intervalSec: 43200, enabled: false},
	{id: "mystery_shop_auto_buy", label: "神秘商店自动购买", priority: 91, intervalSec: 43200, enabled: false},
	{id: "own_collect", label: "自动收获", priority: 91, intervalSec: 60, enabled: true},
	{id: "own_plant", label: "自动种植", priority: 90, intervalSec: 10, enabled: true},
	{id: "fertilizer_fill", label: "自动填充化肥", priority: 86, intervalSec: 43200, enabled: false},
	{id: "own_fertilizer", label: "自动施肥", priority: 85, intervalSec: 30, enabled: false},
	{id: "friend_steal", label: "好友偷菜", priority: 70, intervalSec: 90, enabled: false},
	{id: "friend_help", label: "好友帮忙", priority: 65, intervalSec: 90, enabled: false},
	{id: "auto_warehouse_sell", label: "仓库自动出售", priority: 64, intervalSec: 3600, enabled: false},
	{id: "friend_mischief", label: "好友捣乱", priority: 60, intervalSec: 90, enabled: false},
}

func DefaultState() State {
	return StateFromSettings(DefaultSettings())
}

func DefaultSettings() Settings {
	tasks := make([]TaskSettings, 0, len(schedulerTaskDefs))
	for _, def := range schedulerTaskDefs {
		tasks = append(tasks, TaskSettings{
			ID:          def.id,
			Enabled:     def.enabled,
			Priority:    def.priority,
			IntervalSec: def.intervalSec,
		})
	}
	return Settings{
		SchedulerEnabled:  true,
		SchedulerMinGapMs: 350,
		RunMode:           RunModeGod,
		Tasks:             tasks,
		Config:            DefaultConfig(),
	}
}

func StateFromSettings(settings Settings) State {
	settings.Config = migrateLegacyFeatureGroupSwitches(settings.Config)
	explicitConfig := cloneConfig(settings.Config)
	fertilizerEnabled, hasFertilizerEnabled := resolveFertilizerMasterSwitch(settings)
	settings = mergeSettingsWithDefaults(settings)
	if hasFertilizerEnabled {
		settings = applyFertilizerMasterSwitch(settings, fertilizerEnabled)
		explicitConfig[fertilizerFeatureGroupConfigKey] = fertilizerEnabled
		explicitConfig[fertilizerTaskConfigKey] = fertilizerEnabled
	}
	config := MergeConfigWithDefaults(settings.Config)
	taskSettings := map[string]TaskSettings{}
	for _, task := range settings.Tasks {
		taskSettings[task.ID] = task
	}

	tasks := make([]SchedulerTask, 0, len(schedulerTaskDefs))
	now := time.Now()
	for _, def := range schedulerTaskDefs {
		if isRetiredAutomationSwitch(schedulerTaskConfigKeys[def.id]) {
			continue
		}
		task := taskSettings[def.id]
		intervalSec := task.IntervalSec
		if intervalSec <= 0 {
			intervalSec = def.intervalSec
		}
		if configInterval, ok := schedulerTaskIntervalFromConfig(settings.Config, def.id, false); ok {
			intervalSec = configInterval
		}
		if key := schedulerTaskIntervalConfigKeys[def.id]; key != "" {
			config[key] = intervalSec
		}
		schedulerTask := SchedulerTask{
			ID:                def.id,
			Label:             def.label,
			EnabledConfigKey:  schedulerTaskConfigKeys[def.id],
			IntervalConfigKey: schedulerTaskIntervalConfigKeys[def.id],
			Priority:          task.Priority,
			IntervalSec:       intervalSec,
			Enabled:           task.Enabled,
		}
		enabled, ok, disabledByGroup := schedulerTaskEnabledFromConfig(config, explicitConfig, def.id, task.Enabled)
		if ok {
			schedulerTask.Enabled = enabled
		}
		if key := schedulerTaskConfigKeys[def.id]; key != "" && !disabledByGroup {
			config[key] = schedulerTask.Enabled
		}
		applyDailyOnceTaskState(&schedulerTask, config, now)
		tasks = append(tasks, schedulerTask)
	}
	order := map[string]int{}
	for index, def := range schedulerTaskDefs {
		order[def.id] = index
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Priority != tasks[j].Priority {
			return tasks[i].Priority > tasks[j].Priority
		}
		return order[tasks[i].ID] < order[tasks[j].ID]
	})

	taskEnabled := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		taskEnabled[task.ID] = task.Enabled
	}
	groups := make([]FeatureGroup, len(featureGroups))
	for index, group := range featureGroups {
		key := featureGroupSwitchConfigKeys[group.ID]
		explicitGroupValue, hasExplicitGroup := explicitConfig[key]
		switch {
		case key == "":
		case hasExplicitGroup:
			group.Enabled = boolConfig(explicitGroupValue, group.Enabled)
			config[key] = group.Enabled
		default:
			group.Enabled = false
			for _, taskID := range featureGroupTaskIDs[group.ID] {
				if taskEnabled[taskID] {
					group.Enabled = true
					break
				}
			}
			config[key] = group.Enabled
		}
		group.SettingKeys = append([]string(nil), group.SettingKeys...)
		groups[index] = group
	}

	enabled := 0
	for _, task := range tasks {
		if task.Enabled {
			enabled++
		}
	}

	return State{
		Running:       false,
		RunMode:       settings.RunMode,
		Summary:       map[string]int{"enabledTasks": enabled, "totalTasks": len(tasks), "todayHarvest": 0},
		FeatureGroups: groups,
		Config:        config,
		Scheduler: SchedulerState{
			Enabled:  settings.SchedulerEnabled,
			MinGapMs: settings.SchedulerMinGapMs,
			Tasks:    tasks,
		},
	}
}

func SettingsFromState(state State) Settings {
	config := cloneConfig(state.Config)
	for _, group := range state.FeatureGroups {
		if key := featureGroupSwitchConfigKeys[group.ID]; key != "" {
			config[key] = group.Enabled
		}
	}
	settings := Settings{
		SchedulerEnabled:  state.Scheduler.Enabled,
		SchedulerMinGapMs: state.Scheduler.MinGapMs,
		RunMode:           NormalizeRunMode(state.RunMode),
		Tasks:             make([]TaskSettings, 0, len(state.Scheduler.Tasks)),
		Config:            config,
	}
	for _, task := range state.Scheduler.Tasks {
		enabled := schedulerTaskEnabledForSave(config, task)
		intervalSec := normalizedIntervalSec(task.IntervalSec)
		intervalKey := schedulerTaskIntervalConfigKeys[task.ID]
		if configInterval, ok := schedulerTaskIntervalFromConfig(config, task.ID, task.IntervalConfigKey == intervalKey); ok {
			intervalSec = configInterval
		}
		if key := schedulerTaskIntervalConfigKeys[task.ID]; key != "" {
			config[key] = intervalSec
		}
		settings.Tasks = append(settings.Tasks, TaskSettings{
			ID:          task.ID,
			Enabled:     enabled,
			Priority:    task.Priority,
			IntervalSec: intervalSec,
		})
	}
	if fertilizerEnabled, ok := resolveFertilizerMasterSwitch(settings); ok {
		settings = applyFertilizerMasterSwitch(settings, fertilizerEnabled)
	}
	return mergeSettingsWithDefaults(settings)
}

func resolveFertilizerMasterSwitch(settings Settings) (bool, bool) {
	if value, ok := settings.Config[fertilizerFeatureGroupConfigKey]; ok {
		return boolConfig(value, false), true
	}
	if value, ok := settings.Config[fertilizerTaskConfigKey]; ok {
		return boolConfig(value, false), true
	}
	for _, task := range settings.Tasks {
		if task.ID == fertilizerTaskID {
			return task.Enabled, true
		}
	}
	return false, false
}

func applyFertilizerMasterSwitch(settings Settings, enabled bool) Settings {
	settings.Config = cloneConfig(settings.Config)
	settings.Config[fertilizerFeatureGroupConfigKey] = enabled
	settings.Config[fertilizerTaskConfigKey] = enabled
	for index := range settings.Tasks {
		if settings.Tasks[index].ID == fertilizerTaskID {
			settings.Tasks[index].Enabled = enabled
		}
	}
	return settings
}

func schedulerTaskEnabledForSave(config map[string]any, task SchedulerTask) bool {
	enabled := task.Enabled
	if key := schedulerTaskConfigKeys[task.ID]; key != "" {
		if value, ok := config[key]; ok {
			enabled = boolConfig(value, enabled)
		}
		config[key] = enabled
	}
	return enabled
}

func schedulerTaskEnabledFromConfig(config, explicitConfig map[string]any, taskID string, fallback bool) (enabled bool, found bool, disabledByGroup bool) {
	taskKey := schedulerTaskConfigKeys[taskID]
	for groupID, taskIDs := range featureGroupTaskIDs {
		if !stringSliceContains(taskIDs, taskID) {
			continue
		}
		groupKey := featureGroupSwitchConfigKeys[groupID]
		if groupKey == "" {
			continue
		}
		if _, ok := explicitConfig[groupKey]; ok && !boolConfig(config[groupKey], true) {
			return false, true, true
		}
	}
	if taskKey != "" {
		if _, ok := explicitConfig[taskKey]; ok {
			return boolConfig(config[taskKey], fallback), true, false
		}
	}
	return false, false, false
}

func migrateLegacyFeatureGroupSwitches(config map[string]any) map[string]any {
	migrated := cloneConfig(config)
	for groupID, legacyKey := range legacyFeatureGroupSwitchConfigKeys {
		groupKey := featureGroupSwitchConfigKeys[groupID]
		if groupKey == "" {
			continue
		}
		if _, ok := migrated[groupKey]; ok {
			continue
		}
		if value, ok := migrated[legacyKey]; ok {
			migrated[groupKey] = value
		}
	}
	return migrated
}

func schedulerTaskIntervalFromConfig(config map[string]any, taskID string, canonical bool) (int, bool) {
	key := schedulerTaskIntervalConfigKeys[taskID]
	if key == "" {
		return 0, false
	}
	value, ok := config[key]
	if !ok {
		return 0, false
	}
	rawIntervalSec := intConfig(value, 0)
	if rawIntervalSec <= 0 {
		return 0, false
	}
	intervalSec := normalizedIntervalSec(rawIntervalSec)
	if canonical {
		return intervalSec, true
	}
	defaultValue, hasDefault := DefaultConfig()[key]
	if hasDefault && intervalSec == normalizedIntervalSec(intConfig(defaultValue, 0)) {
		return 0, false
	}
	for _, def := range schedulerTaskDefs {
		if def.id == taskID && intervalSec == normalizedIntervalSec(def.intervalSec) {
			return 0, false
		}
	}
	return intervalSec, true
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func normalizedIntervalSec(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func intConfig(value any, fallback int) int {
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
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return parsed
		}
		return fallback
	default:
		return fallback
	}
}

func boolConfig(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true" || typed == "1" || typed == "on" || typed == "yes"
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return fallback
	}
}

func mergeSettingsWithDefaults(settings Settings) Settings {
	defaults := DefaultSettings()
	settings.RunMode = NormalizeRunMode(settings.RunMode)
	if settings.SchedulerMinGapMs <= 0 {
		settings.SchedulerMinGapMs = defaults.SchedulerMinGapMs
	}
	if len(settings.Tasks) == 0 {
		settings.Tasks = defaults.Tasks
		settings.Config = MergeConfigWithDefaults(settings.Config)
		return normalizeRetiredAutomationTasks(settings)
	}

	overrides := map[string]TaskSettings{}
	for _, task := range settings.Tasks {
		if task.ID == "" {
			continue
		}
		overrides[task.ID] = task
	}
	tasks := make([]TaskSettings, 0, len(defaults.Tasks))
	for _, task := range defaults.Tasks {
		if override, ok := overrides[task.ID]; ok {
			if override.Priority <= 0 {
				override.Priority = task.Priority
			}
			if override.IntervalSec <= 0 {
				override.IntervalSec = task.IntervalSec
			}
			task = override
		}
		tasks = append(tasks, task)
	}
	settings.Tasks = tasks
	settings.Config = MergeConfigWithDefaults(settings.Config)
	return normalizeRetiredAutomationTasks(settings)
}

func normalizeRetiredAutomationTasks(settings Settings) Settings {
	for index := range settings.Tasks {
		if isRetiredAutomationSwitch(schedulerTaskConfigKeys[settings.Tasks[index].ID]) {
			settings.Tasks[index].Enabled = false
		}
	}
	return settings
}

func RunTask(taskID string) ActionResult {
	if taskID == "" {
		return ActionResult{OK: false, Status: StatusFailed, Message: "任务 ID 不能为空"}
	}
	return ActionResult{
		OK:      false,
		Status:  StatusNotMigrated,
		TaskID:  taskID,
		Message: "该自动化任务的真实运行时脚本尚未迁移，本阶段只提供配置和调度 UI。",
	}
}
