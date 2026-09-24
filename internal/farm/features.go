package farm

type FeatureStatus string

const (
	StatusMigrated FeatureStatus = "migrated"
	StatusPlanned  FeatureStatus = "planned"
)

type FeatureCatalog struct {
	Groups []FeatureGroup `json:"groups"`
}

type FeatureGroup struct {
	ID       string        `json:"id"`
	Label    string        `json:"label"`
	Summary  string        `json:"summary"`
	Features []FarmFeature `json:"features"`
}

type FarmFeature struct {
	ID        string        `json:"id"`
	Label     string        `json:"label"`
	Summary   string        `json:"summary"`
	Status    FeatureStatus `json:"status"`
	Reference string        `json:"reference"`
}

func Catalog() FeatureCatalog {
	return FeatureCatalog{Groups: []FeatureGroup{
		{
			ID:      "workspace",
			Label:   "工作台",
			Summary: "总览、快捷任务和最近运行状态",
			Features: []FarmFeature{
				{ID: "overview", Label: "总览", Summary: "Farm_Go 运行状态与最近事件", Status: StatusMigrated, Reference: "Farm_Go OverviewView"},
			},
		},
		{
			ID:      "automation",
			Label:   "农场自动化",
			Summary: "自动农场、调度、消息推送",
			Features: []FarmFeature{
				{ID: "auto_farm", Label: "自动农场", Summary: "待迁移 auto-farm-manager / executor", Status: StatusPlanned, Reference: "core/src/auto-farm-manager.js"},
				{ID: "scheduler", Label: "调度中心", Summary: "待迁移自动任务间隔和运行队列", Status: StatusPlanned, Reference: "core/src/auto-farm-manager.js"},
				{ID: "message_push", Label: "消息推送", Summary: "待迁移消息推送状态和测试发送", Status: StatusPlanned, Reference: "core/src/message-push-manager.js"},
			},
		},
		{
			ID:      "assets",
			Label:   "资产与土地",
			Summary: "作物、土地、仓库、图鉴",
			Features: []FarmFeature{
				{ID: "crop_analytics", Label: "作物分析", Summary: "待迁移种植收益和种子策略分析", Status: StatusPlanned, Reference: "core/src/plant-analytics.js"},
				{ID: "lands", Label: "土地详情", Summary: "待迁移土地状态查看和一键催熟动作", Status: StatusPlanned, Reference: "core/src/auto-farm-executor.js"},
				{ID: "warehouse", Label: "仓库", Summary: "待迁移仓库刷新和出售", Status: StatusPlanned, Reference: "core/src/warehouse-sell-record-store.js"},
				{ID: "atlas", Label: "图鉴", Summary: "待迁移图鉴刷新和解锁购买", Status: StatusPlanned, Reference: "core/src/plant-analytics.js"},
			},
		},
		{
			ID:      "social",
			Label:   "好友社交",
			Summary: "好友、排行榜、访客记录",
			Features: []FarmFeature{
				{ID: "friends", Label: "好友", Summary: "待迁移好友操作、帮助、捣乱和黑白名单", Status: StatusPlanned, Reference: "core/src/friend-*.js"},
				{ID: "rankings", Label: "排行榜", Summary: "待迁移偷取排行和访客记录", Status: StatusPlanned, Reference: "core/src/friend-steal-ranking-store.js"},
			},
		},
		{
			ID:      "system",
			Label:   "账户与系统",
			Summary: "账户、守护、日志、设置",
			Features: []FarmFeature{
				{ID: "account", Label: "账户状态", Summary: "账户资产、肥料容器、等级进度和运行统计已接入真实运行时", Status: StatusMigrated, Reference: "gameCtl.getPlayerProfile / gameCtl.getFertilizerContainerStatus"},
				{ID: "guard", Label: "守护服务", Summary: "进程守护已迁入 Farm_Go", Status: StatusMigrated, Reference: "internal/runtime/guard"},
				{ID: "logs", Label: "日志中心", Summary: "运行事件日志已迁入 Farm_Go", Status: StatusMigrated, Reference: "internal/storage/events.go"},
				{ID: "settings", Label: "系统设置", Summary: "运行链路设置已迁入 Farm_Go", Status: StatusMigrated, Reference: "internal/storage/settings.go"},
			},
		},
	}}
}
