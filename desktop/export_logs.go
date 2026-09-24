package desktop

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"Farm_Go/internal/eventbus"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	runtimeLogKindTask   = "task"
	runtimeLogKindSystem = "system"
)

func (a *App) ExportRuntimeLogs(kind string) (SaveTextFileResult, error) {

	normalizedKind, label, err := normalizeRuntimeLogKind(kind)
	if err != nil {
		return SaveTextFileResult{}, err
	}

	now := time.Now()
	events := a.runStatisticsEvents(a.accountKey())
	content := formatRuntimeLogExport(normalizedKind, events, now)
	defaultName := runtimeLogDefaultFileName(normalizedKind, now)

	path, err := wailsruntime.SaveFileDialog(a.contextOrBackground(), wailsruntime.SaveDialogOptions{
		Title:                "导出" + label,
		DefaultFilename:      safeDefaultFileName(defaultName),
		CanCreateDirectories: true,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Text Files", Pattern: "*.txt"},
		},
	})
	if err != nil {
		return SaveTextFileResult{}, err
	}
	if strings.TrimSpace(path) == "" {
		return SaveTextFileResult{}, errExportCanceled
	}
	return saveTextFileToPath(path, content, "txt")
}

func normalizeRuntimeLogKind(kind string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case runtimeLogKindTask, "task_logs", "任务", "任务运行日志", "任务日志":
		return runtimeLogKindTask, "任务运行日志", nil
	case runtimeLogKindSystem, "system_logs", "系统", "系统日志":
		return runtimeLogKindSystem, "系统日志", nil
	default:
		return "", "", exportFileError("日志类型无效，仅支持 task 或 system")
	}
}

func runtimeLogDefaultFileName(kind string, now time.Time) string {
	stamp := now.Format("20060102_150405")
	switch kind {
	case runtimeLogKindTask:
		return fmt.Sprintf("任务运行日志_%s.txt", stamp)
	default:
		return fmt.Sprintf("系统日志_%s.txt", stamp)
	}
}

func formatRuntimeLogExport(kind string, events []eventbus.Event, exportedAt time.Time) string {
	var builder strings.Builder
	label := "系统日志"
	if kind == runtimeLogKindTask {
		label = "任务运行日志"
	}
	builder.WriteString(fmt.Sprintf("# %s\n", label))
	builder.WriteString(fmt.Sprintf("# 导出时间: %s\n", exportedAt.Format("2006-01-02 15:04:05")))
	builder.WriteString("# 范围: 本次运行\n")
	builder.WriteString("\n")

	selected := filterRuntimeLogEvents(kind, events)
	if kind == runtimeLogKindTask {
		builder.WriteString("时间\t级别\t任务\t运行结果\t数据\n")
		for _, event := range selected {
			builder.WriteString(strings.Join([]string{
				formatExportEventTime(event.Timestamp),
				exportEventLevel(event),
				exportWorkbenchTaskName(event),
				exportWorkbenchTaskResult(event),
				compactExportEventData(event.Data),
			}, "\t"))
			builder.WriteByte('\n')
		}
	} else {
		builder.WriteString("时间\t级别\t来源\t类型\t消息\t数据\n")
		for _, event := range selected {
			builder.WriteString(strings.Join([]string{
				formatExportEventTime(event.Timestamp),
				exportEventLevel(event),
				exportTargetLabel(event.Source),
				strings.TrimSpace(event.Type),
				sanitizeExportField(event.Message),
				compactExportEventData(event.Data),
			}, "\t"))
			builder.WriteByte('\n')
		}
	}

	if len(selected) == 0 {
		builder.WriteString("# 暂无日志\n")
	}
	return builder.String()
}

func filterRuntimeLogEvents(kind string, events []eventbus.Event) []eventbus.Event {
	selected := make([]eventbus.Event, 0, len(events))
	for _, event := range events {
		if kind == runtimeLogKindTask {
			if isExportTaskResultEvent(event) {
				selected = append(selected, event)
			}
			continue
		}
		if !isExportTaskResultEvent(event) && !isExportTaskLifecycleNoise(event) {
			selected = append(selected, event)
		}
	}
	return selected
}

func isExportTaskLifecycleNoise(event eventbus.Event) bool {
	eventType := strings.ToLower(strings.TrimSpace(event.Type))
	if eventType == "task.start" {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(event.Message))
	return message == "automation task started" || message == "automation task completed"
}

func isExportWorkbenchTaskEvent(event eventbus.Event) bool {
	eventType := strings.ToLower(strings.TrimSpace(event.Type))
	if eventType == "task.done" || eventType == "task.failed" {
		return true
	}
	if isExportTaskLifecycleNoise(event) {
		return false
	}
	if strings.HasPrefix(eventType, "auto_farm.settings") || strings.Contains(eventType, "settings.") {
		return false
	}
	taskID := automationTaskIDFromEvent(event)
	if taskID == "" {
		return false
	}
	source := strings.ToLower(strings.TrimSpace(event.Source))
	return source == "auto_farm" || source == "task" || strings.HasPrefix(source, "farm_")
}

func isExportTaskResultEvent(event eventbus.Event) bool {
	return isExportWorkbenchTaskEvent(event)
}

var exportAutomationTaskLabels = map[string]string{
	"own_base":                "一键务农",
	"land_upgrade":            "土地自动升级",
	"reward_claim":            "自动领取任务奖励",
	"svip_daily_gift":         "SVIP每日礼包",
	"monthly_card_reward":     "月卡奖励",
	"mall_daily_fertilizer":   "商城每日肥料",
	"share_reward":            "自动领取分享奖励",
	"mail_reward":             "自动领取邮件奖励",
	"qian_xing_travel_reward": "千星游记奖励领取",
	"he_feng_travel_reward":   "限时活动/荷风游记奖励领取",
	"limited_seed_draw":       "荷风游记抽奖",
	"mystery_shop_auto_buy":   "神秘商店自动购买",
	"mystery_shop_read":       "神秘商店查看",
	"own_collect":             "自动收获",
	"own_plant":               "自动种植",
	"fertilizer_fill":         "自动填充化肥",
	"own_fertilizer":          "自动施肥",
	"friend_steal":            "好友偷菜",
	"friend_help":             "好友帮忙",
	"friend_mischief":         "好友捣乱",
	"auto_warehouse_sell":     "仓库自动出售",
	"warehouse_sell":          "仓库出售",
}

func exportAutomationTaskLabel(taskID string) string {
	id := strings.TrimSpace(taskID)
	if id == "" {
		return ""
	}
	if label, ok := exportAutomationTaskLabels[id]; ok {
		return label
	}
	return id
}

func exportWorkbenchTaskName(event eventbus.Event) string {
	taskID := automationTaskIDFromEvent(event)
	if label := exportAutomationTaskLabel(taskID); label != "" {
		return label
	}
	return "自动化任务"
}

func exportWorkbenchTaskResult(event eventbus.Event) string {
	message := strings.TrimSpace(event.Message)
	if message != "" {
		return sanitizeExportField(message)
	}
	eventType := strings.ToLower(strings.TrimSpace(event.Type))
	if eventType == "task.failed" {
		return "任务执行失败"
	}
	if eventType == "task.done" {
		return "任务执行完成"
	}
	return "任务已执行"
}

func exportEventLevel(event eventbus.Event) string {
	level := strings.TrimSpace(string(event.Level))
	if level == "" {
		return "info"
	}
	return level
}

func exportTargetLabel(target string) string {
	switch strings.TrimSpace(target) {
	case "qq_ws":
		return "QQ WS"
	case "wechat_cdp":
		return "微信 CDP"
	case "yyb_cdp":
		return "应用宝 CDP"
	case "":
		return "-"
	default:
		return target
	}
}

func formatExportEventTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Local().Format("2006-01-02 15:04:05")
}

func compactExportEventData(data map[string]any) string {
	if len(data) == 0 {
		return "-"
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return "-"
	}
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" || text == "{}" {
		return "-"
	}
	return sanitizeExportField(text)
}

func sanitizeExportField(value string) string {
	replacer := strings.NewReplacer("\t", " ", "\r", " ", "\n", " ")
	return strings.TrimSpace(replacer.Replace(value))
}

func runtimeLogExportPath(dir string, kind string, now time.Time) string {
	return filepath.Join(dir, runtimeLogDefaultFileName(kind, now))
}
