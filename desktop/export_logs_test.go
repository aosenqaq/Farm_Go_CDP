package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"Farm_Go/internal/eventbus"
)

func TestNormalizeRuntimeLogKind(t *testing.T) {
	kind, label, err := normalizeRuntimeLogKind("task")
	if err != nil || kind != runtimeLogKindTask || label != "任务运行日志" {
		t.Fatalf("task kind = %q %q %v", kind, label, err)
	}
	kind, label, err = normalizeRuntimeLogKind("system")
	if err != nil || kind != runtimeLogKindSystem || label != "系统日志" {
		t.Fatalf("system kind = %q %q %v", kind, label, err)
	}
	if _, _, err := normalizeRuntimeLogKind("other"); err == nil {
		t.Fatal("expected invalid kind error")
	}
}

func TestExportAutomationTaskLabelUsesQianXingTravelRewardName(t *testing.T) {
	if got := exportAutomationTaskLabel("qian_xing_travel_reward"); got != "千星游记奖励领取" {
		t.Fatalf("task label = %q, want qian xing label", got)
	}
}

func TestRuntimeLogDefaultFileName(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 4, 5, 0, time.Local)
	if got := runtimeLogDefaultFileName(runtimeLogKindTask, now); got != "任务运行日志_20260713_150405.txt" {
		t.Fatalf("task name = %q", got)
	}
	if got := runtimeLogDefaultFileName(runtimeLogKindSystem, now); got != "系统日志_20260713_150405.txt" {
		t.Fatalf("system name = %q", got)
	}
}

func TestFilterRuntimeLogEventsMatchesLogsViewTabs(t *testing.T) {
	events := []eventbus.Event{
		{Type: "task.start", Message: "automation task started", Source: "auto_farm", Data: map[string]any{"taskId": "own_plant"}},
		{Type: "task.done", Message: "没有检测到可种植空地，本轮自动种植跳过。", Source: "auto_farm", Data: map[string]any{"taskId": "own_plant", "ok": true, "status": "ok"}},
		{Type: "auto_farm.settings.save", Message: "自动农场设置已保存", Source: "auto_farm"},
		{Type: "runtime.status", Message: "wechat_cdp ready", Source: "wechat_cdp", Level: eventbus.LevelInfo},
	}

	taskEvents := filterRuntimeLogEvents(runtimeLogKindTask, events)
	if len(taskEvents) != 1 || taskEvents[0].Type != "task.done" {
		t.Fatalf("task events = %#v", taskEvents)
	}

	systemEvents := filterRuntimeLogEvents(runtimeLogKindSystem, events)
	if len(systemEvents) != 2 {
		t.Fatalf("system events = %#v", systemEvents)
	}
	if systemEvents[0].Type != "auto_farm.settings.save" || systemEvents[1].Type != "runtime.status" {
		t.Fatalf("system events = %#v", systemEvents)
	}
}

func TestFormatRuntimeLogExportTaskContent(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 4, 5, 0, time.Local)
	events := []eventbus.Event{
		{
			Timestamp: now.Add(-time.Minute),
			Level:     eventbus.LevelInfo,
			Source:    "auto_farm",
			Type:      "task.done",
			Message:   "没有检测到可种植空地，本轮自动种植跳过。",
			Data:      map[string]any{"taskId": "own_plant", "ok": true},
		},
		{
			Timestamp: now.Add(-30 * time.Second),
			Level:     eventbus.LevelInfo,
			Source:    "wechat_cdp",
			Type:      "runtime.status",
			Message:   "wechat_cdp ready",
		},
	}

	content := formatRuntimeLogExport(runtimeLogKindTask, events, now)
	if !strings.Contains(content, "任务运行日志") {
		t.Fatalf("missing title: %s", content)
	}
	if !strings.Contains(content, "自动种植") {
		t.Fatalf("missing task label: %s", content)
	}
	if !strings.Contains(content, "没有检测到可种植空地，本轮自动种植跳过。") {
		t.Fatalf("missing task result: %s", content)
	}
	if strings.Contains(content, "wechat_cdp ready") {
		t.Fatalf("system event leaked into task export: %s", content)
	}
}

func TestFormatRuntimeLogExportSystemContent(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 4, 5, 0, time.Local)
	events := []eventbus.Event{
		{
			Timestamp: now.Add(-time.Minute),
			Level:     eventbus.LevelInfo,
			Source:    "auto_farm",
			Type:      "task.done",
			Message:   "自动种植完成",
			Data:      map[string]any{"taskId": "own_plant"},
		},
		{
			Timestamp: now.Add(-30 * time.Second),
			Level:     eventbus.LevelInfo,
			Source:    "wechat_cdp",
			Type:      "runtime.status",
			Message:   "wechat_cdp ready",
		},
	}

	content := formatRuntimeLogExport(runtimeLogKindSystem, events, now)
	if !strings.Contains(content, "系统日志") {
		t.Fatalf("missing title: %s", content)
	}
	if !strings.Contains(content, "微信 CDP") {
		t.Fatalf("missing source label: %s", content)
	}
	if !strings.Contains(content, "wechat_cdp ready") {
		t.Fatalf("missing system message: %s", content)
	}
	if strings.Contains(content, "自动种植完成") {
		t.Fatalf("task event leaked into system export: %s", content)
	}
}

func TestSaveRuntimeLogExportWritesFile(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 4, 5, 0, time.Local)
	content := formatRuntimeLogExport(runtimeLogKindTask, nil, now)
	path := runtimeLogExportPath(t.TempDir(), runtimeLogKindTask, now)

	result, err := saveTextFileToPath(path, content, "txt")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), "暂无日志") {
		t.Fatalf("content = %q", string(raw))
	}
	if filepath.Base(result.Path) != "任务运行日志_20260713_150405.txt" {
		t.Fatalf("basename = %q", filepath.Base(result.Path))
	}
}
