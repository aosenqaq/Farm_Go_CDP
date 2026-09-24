package messagepush

import (
	"context"
	"os"
	"testing"
)

func TestManualFeishuTemplateVersions(t *testing.T) {
	webhook := os.Getenv("FEISHU_WEBHOOK")
	if webhook == "" {
		t.Skip("set FEISHU_WEBHOOK to send the three review templates")
	}

	versions := []struct {
		name     string
		template Template
	}{
		{
			name:     "compact_alert_card",
			template: Template{Enabled: true, Mode: TemplateModeCard, Content: `{"msg_type":"interactive","card":{"config":{"wide_screen_mode":true},"header":{"template":"red","title":{"tag":"plain_text","content":"Farm_Go · 异常告警 V1"}},"elements":[{"tag":"div","fields":[{"is_short":true,"text":{"tag":"lark_md","content":"**事件**\\n{{event.type}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**运行目标**\\n{{runtime.target}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**触发时间**\\n{{event.time}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**累计次数**\\n{{event.count}}"}}]},{"tag":"note","elements":[{"tag":"plain_text","content":"V1 紧凑告警卡"}]}]}}`},
		},
		{
			name:     "diagnostic_detail_card",
			template: Template{Enabled: true, Mode: TemplateModeCard, Content: `{"msg_type":"interactive","card":{"config":{"wide_screen_mode":true},"header":{"template":"orange","title":{"tag":"plain_text","content":"Farm_Go · 异常诊断 V2"}},"elements":[{"tag":"div","fields":[{"is_short":false,"text":{"tag":"lark_md","content":"**事件** {{event.type}}\\n**目标** {{runtime.target}}"}},{"is_short":false,"text":{"tag":"lark_md","content":"**错误摘要** {{error.message}}\\n**失败阶段** {{error.stage}}"}}]},{"tag":"hr"},{"tag":"note","elements":[{"tag":"plain_text","content":"V2 诊断详情卡：请结合运行日志排查"}]}]}}`},
		},
		{
			name:     "markdown_information_card",
			template: Template{Enabled: true, Mode: TemplateModeMarkdown, Content: "**Farm_Go · 异常告警 V3**\\n\\n> 事件：{{event.type}}\\n\\n- 运行目标：{{runtime.target}}\\n- 触发时间：{{event.time}}\\n- 错误摘要：{{error.message}}\\n- 累计次数：{{event.count}}\\n\\n请查看运行日志确认处理结果。"},
		},
	}

	store := NewMemoryStore()
	service := NewService(ServiceOptions{Store: store})
	for _, version := range versions {
		t.Run(version.name, func(t *testing.T) {
			config := NormalizeConfig(Config{
				Enabled:          true,
				SelectedChannels: []string{"feishu"},
				Channels:         Channels{FeishuWebhook: webhook},
				Templates: map[MessageType]map[string]Template{
					MessageTypeAbnormal: {"feishu": version.template},
				},
			})
			preview, err := service.PreviewTemplate(config, MessageTypeAbnormal, "feishu")
			if err != nil || !preview.OK {
				t.Fatalf("preview failed: %#v, %v", preview, err)
			}
			result, err := service.SendTemplateTest(context.Background(), config, MessageTypeAbnormal, "feishu")
			if err != nil || !result.OK {
				t.Fatalf("send failed: %#v, %v", result, err)
			}
		})
	}
}

func TestManualFeishuNonDailyRuntimeEvents(t *testing.T) {
	webhook := os.Getenv("FEISHU_WEBHOOK")
	if webhook == "" {
		t.Skip("set FEISHU_WEBHOOK to send the five non-daily runtime templates")
	}

	store := NewMemoryStore()
	service := NewService(ServiceOptions{Store: store})
	config := NormalizeConfig(Config{
		Enabled:           true,
		AbnormalEnabled:   true,
		SuspectedEnabled:  true,
		RecoveryEnabled:   true,
		RestartEnabled:    true,
		LogMonitorEnabled: true,
		SelectedChannels:  []string{"feishu"},
		Channels:          Channels{FeishuWebhook: webhook},
		LogMonitorRules:   []LogMonitorRule{{Enabled: true, Source: "manual_verification"}},
	})
	if _, err := service.SaveConfig(context.Background(), config); err != nil {
		t.Fatalf("save verification config: %v", err)
	}

	events := []MessageEvent{
		{Type: MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "manual verification timeout"},
		{Type: MessageTypeSuspected, RuntimeTarget: "qq_ws", Error: "manual verification delay", Values: map[string]any{"monitor": map[string]any{"streak": 2, "threshold": 3}}},
		{Type: MessageTypeRecovery, RuntimeTarget: "qq_ws", RecoveryVia: "guardian"},
		{Type: MessageTypeRestart, RuntimeTarget: "qq_ws", Trigger: "manual_verification", Values: map[string]any{"restart": map[string]any{"trigger": "manual_verification", "reason": "manual verification", "ok": true}}},
		{Type: MessageTypeLogMonitor, RuntimeTarget: "qq_ws", RuntimeLog: &RuntimeLog{Level: "error", Source: "manual_verification", Type: "manual.feishu.verify", Message: "manual verification runtime log"}},
	}
	for _, event := range events {
		t.Run(string(event.Type), func(t *testing.T) {
			result, err := service.Dispatch(context.Background(), event)
			if err != nil || !result.Sent {
				t.Fatalf("dispatch %s failed: result=%#v err=%v", event.Type, result, err)
			}
		})
	}
}
