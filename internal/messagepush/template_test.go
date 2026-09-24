package messagepush

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderTemplateUsesAllowedVariables(t *testing.T) {
	rendered, err := RenderTemplate(Template{Mode: TemplateModeMarkdown, Content: "{{runtime.target}} {{error.message}}"}, TemplateContext{
		MessageType: MessageTypeAbnormal,
		Values: map[string]any{
			"runtime": map[string]any{"target": "qq_ws"},
			"error":   map[string]any{"message": "timeout"},
		},
	})
	if err != nil || rendered.Text != "qq_ws timeout" {
		t.Fatalf("rendered=%#v err=%v", rendered, err)
	}
}

func TestRenderTemplateRejectsUnknownAndMissingVariables(t *testing.T) {
	tests := []struct {
		name    string
		content string
		values  map[string]any
		want    string
	}{
		{"unknown", "{{process.env}}", nil, "未知变量"},
		{"missing", "{{error.message}}", map[string]any{"error": map[string]any{}}, "缺少变量"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := RenderTemplate(Template{Mode: TemplateModeText, Content: test.content}, TemplateContext{
				MessageType: MessageTypeAbnormal,
				Values:      test.values,
			})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestRenderTemplateRejectsInvalidJSONAndCardPayloads(t *testing.T) {
	tests := []Template{
		{Mode: TemplateModeJSON, Content: `{"message": "{{error.message}}"`},
		{Mode: TemplateModeCard, Content: `[]`},
	}
	for _, template := range tests {
		if _, err := RenderTemplate(template, TemplateContext{
			MessageType: MessageTypeAbnormal,
			Values:      map[string]any{"error": map[string]any{"message": "timeout"}},
		}); err == nil {
			t.Fatalf("expected invalid rendered payload for %#v", template)
		}
	}
}

func TestRenderTemplateEscapesJSONStringValues(t *testing.T) {
	rendered, err := RenderTemplate(Template{Mode: TemplateModeJSON, Content: `{"message":"{{error.message}}"}`}, TemplateContext{
		MessageType: MessageTypeAbnormal,
		Values:      map[string]any{"error": map[string]any{"message": `connection "lost"`}},
	})
	if err != nil || rendered.JSON["message"] != `connection "lost"` {
		t.Fatalf("rendered=%#v err=%v", rendered, err)
	}
}

func TestRenderTemplateEncodesUnquotedJSONValues(t *testing.T) {
	value := `ok","injected":true`
	rendered, err := RenderTemplate(Template{Mode: TemplateModeJSON, Content: `{"message":{{error.message}}}`}, TemplateContext{
		MessageType: MessageTypeAbnormal,
		Values:      map[string]any{"error": map[string]any{"message": value}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered.JSON["message"] != value {
		t.Fatalf("message=%#v", rendered.JSON["message"])
	}
	if _, ok := rendered.JSON["injected"]; ok {
		t.Fatalf("injected field=%#v", rendered.JSON)
	}
}

func TestWebhookJSONTemplateSendsRenderedObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Template-Test") != "yes" {
			t.Fatalf("missing configured header: %#v", r.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["kind"] != "abnormal" || body["message"] != "timeout" {
			t.Fatalf("body=%#v", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rendered, err := RenderTemplate(Template{Mode: TemplateModeJSON, Content: `{"kind":"{{event.type}}","message":"{{error.message}}"}`}, TemplateContext{
		MessageType: MessageTypeAbnormal,
		Values: map[string]any{
			"event": map[string]any{"type": "abnormal"},
			"error": map[string]any{"message": "timeout"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = sendRenderedByChannel(context.Background(), server.Client(), Config{
		Channels: Channels{WebhookURL: server.URL, WebhookMethod: http.MethodPost, WebhookHeaders: `{"X-Template-Test":"yes"}`},
	}, "webhook", rendered)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWecomMarkdownUsesMarkdownProviderEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		md, ok := body["markdown"].(map[string]any)
		if !ok || body["msgtype"] != "markdown" || md["content"] != "**connected**" {
			t.Fatalf("body=%#v", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := sendRenderedByChannel(context.Background(), server.Client(), Config{
		Channels: Channels{WecomWebhook: server.URL},
	}, "wecom", RenderedPayload{Kind: "test", Mode: TemplateModeMarkdown, Text: "**connected**"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderedPayloadRejectsUnsupportedChannelMode(t *testing.T) {
	err := sendRenderedByChannel(context.Background(), http.DefaultClient, Config{
		Channels: Channels{WebhookURL: "https://example.test/hook"},
	}, "webhook", RenderedPayload{Mode: TemplateModeCard, JSON: map[string]any{"msgtype": "card"}})
	if err == nil || !strings.Contains(err.Error(), "不支持") {
		t.Fatalf("err=%v", err)
	}
}

func TestFeishuRenderedPayloadRejectsProviderBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":19001,"msg":"invalid webhook"}`))
	}))
	defer server.Close()

	err := sendRenderedByChannel(context.Background(), server.Client(), Config{
		Channels: Channels{FeishuWebhook: server.URL},
	}, "feishu", RenderedPayload{Mode: TemplateModeText, Text: "test"})
	if err == nil || !strings.Contains(err.Error(), "invalid webhook") {
		t.Fatalf("err=%v", err)
	}
}

func TestLegacyConfigReceivesBuiltInTemplates(t *testing.T) {
	service := NewService(ServiceOptions{Store: rawStore{
		config: `{"enabled":true,"selectedChannels":["webhook"],"channels":{"webhookUrl":"https://example.test/hook"}}`,
		state:  `{}`,
	}})

	state, err := service.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := state.Config.Templates[MessageTypeAbnormal]["webhook"]
	if got.Mode != TemplateModeJSON || got.Content == "" {
		t.Fatalf("template=%#v", got)
	}
}

func TestBuiltInTemplatesCoverEveryMessageTypeAndChannel(t *testing.T) {
	config := NormalizeConfig(Config{})
	for _, definition := range TemplateDefinitions() {
		for _, channelType := range SupportedChannelTypes {
			template, ok := config.Templates[definition.Type][channelType]
			if !ok || template.Content == "" {
				t.Fatalf("missing template for %s/%s: %#v", definition.Type, channelType, template)
			}
			if !supportsTemplateMode(channelType, template.Mode) {
				t.Fatalf("unsupported default mode %q for %s/%s", template.Mode, definition.Type, channelType)
			}
		}
	}
}

func TestTemplateDefinitionsDescribeVariablesAndFeishuDefaultCard(t *testing.T) {
	definitions := TemplateDefinitions()
	if len(definitions) == 0 || len(definitions[0].Variables) == 0 {
		t.Fatalf("template variables are missing: %#v", definitions)
	}
	variable := definitions[0].Variables[0]
	if variable.Path == "" || variable.Label == "" || variable.Description == "" || variable.Example == "" {
		t.Fatalf("variable description is incomplete: %#v", variable)
	}

	template := DefaultTemplate(MessageTypeAbnormal, "feishu")
	for _, fragment := range []string{"wide_screen_mode", "{{event.type}}", "{{runtime.target}}", "{{event.time}}", "{{event.count}}"} {
		if !strings.Contains(template.Content, fragment) {
			t.Fatalf("feishu default card missing %q: %s", fragment, template.Content)
		}
	}
}

func TestDefaultTemplatesPrioritizeEventTimeAndUseDailyReportLayout(t *testing.T) {
	for _, messageType := range []MessageType{MessageTypeAbnormal, MessageTypeSuspected, MessageTypeRecovery, MessageTypeRestart, MessageTypeLogMonitor} {
		t.Run(string(messageType), func(t *testing.T) {
			template := DefaultTemplate(messageType, "feishu")
			timeIndex := strings.Index(template.Content, "**发生时间**")
			eventIndex := strings.Index(template.Content, "**事件**")
			if timeIndex < 0 || eventIndex < 0 || timeIndex > eventIndex {
				t.Fatalf("non-daily template must lead with time then event: %s", template.Content)
			}
		})
	}

	daily := DefaultTemplate(MessageTypeDaily, "feishu")
	for _, fragment := range []string{"Farm_Go 农场日报", "{{daily.date}}", "{{account.gid}}", "{{daily.gold}}", "{{daily.bean}}", "{{daily.warehouseEstimate}}", "{{daily.sellCount}}", "{{daily.runs}}"} {
		if !strings.Contains(daily.Content, fragment) {
			t.Fatalf("daily report template missing %q: %s", fragment, daily.Content)
		}
	}
}

func TestDefaultDailyTemplateUsesReportPeriodLabels(t *testing.T) {
	template := defaultDailyTemplate("feishu", TemplateModeCard)
	if !strings.Contains(template.Content, "**当日运行** {{daily.runs}}") {
		t.Fatalf("daily card should label historical activity by report period: %s", template.Content)
	}
	if strings.Contains(template.Content, "**今日运行**") {
		t.Fatalf("daily card still refers to historical activity as today: %s", template.Content)
	}
}

func TestTemplateDefinitionsNameAccountGIDAsRunningAccount(t *testing.T) {
	for _, definition := range TemplateDefinitions() {
		for _, variable := range definition.Variables {
			if variable.Path == "account.gid" && variable.Label != "运行账户 GID" {
				t.Fatalf("account GID label=%q", variable.Label)
			}
		}
	}
}

func TestNormalizeTemplatesUpgradesLegacyBuiltInDefault(t *testing.T) {
	legacy := Template{Enabled: true, Mode: TemplateModeCard, Content: `{"msg_type":"interactive","card":{"config":{"wide_screen_mode":true},"header":{"template":"orange","title":{"tag":"plain_text","content":"Farm_Go · 异常告警"}},"elements":[{"tag":"div","fields":[{"is_short":true,"text":{"tag":"lark_md","content":"**事件**\n{{event.type}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**运行目标**\n{{runtime.target}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**触发时间**\n{{event.time}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**累计次数**\n{{event.count}}"}}]},{"tag":"hr"},{"tag":"note","elements":[{"tag":"plain_text","content":"Farm_Go 自动消息推送"}]}]}}`}
	normalized := NormalizeTemplates(map[MessageType]map[string]Template{
		MessageTypeAbnormal: {"feishu": legacy},
	})
	got := normalized[MessageTypeAbnormal]["feishu"]
	if !strings.Contains(got.Content, "**发生时间**") || got.Content == legacy.Content {
		t.Fatalf("legacy default was not upgraded: %#v", got)
	}
}

func TestNormalizeTemplatesUpgradesPreviousDailyDefault(t *testing.T) {
	previous := Template{Enabled: true, Mode: TemplateModeCard, Content: `{"msg_type":"interactive","card":{"config":{"wide_screen_mode":true},"header":{"template":"green","title":{"tag":"plain_text","content":"Farm_Go 农场日报 · {{daily.date}}"}},"elements":[{"tag":"div","fields":[{"is_short":true,"text":{"tag":"lark_md","content":"**运行账户 GID**\n{{account.gid}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**统计日期**\n{{daily.date}}"}}]},{"tag":"hr"},{"tag":"div","text":{"tag":"lark_md","content":"**今日概览**\n{{daily.summary}}"}},{"tag":"note","elements":[{"tag":"plain_text","content":"Farm_Go 自动日报"}]}]}}`}
	normalized := NormalizeTemplates(map[MessageType]map[string]Template{
		MessageTypeDaily: {"feishu": previous},
	})
	if got := normalized[MessageTypeDaily]["feishu"]; !strings.Contains(got.Content, "{{daily.gold}}") || got.Content == previous.Content {
		t.Fatalf("previous daily default was not upgraded: %#v", got)
	}
}

func TestCardDefaultsUseProviderSpecificEnvelopes(t *testing.T) {
	templateContext := validationTemplateContext(MessageTypeDaily)
	tests := []struct {
		channel string
		assert  func(t *testing.T, body map[string]any)
	}{
		{"wecom", func(t *testing.T, body map[string]any) {
			if body["msgtype"] != "template_card" || body["template_card"] == nil {
				t.Fatalf("body=%#v", body)
			}
		}},
		{"dingtalk", func(t *testing.T, body map[string]any) {
			if body["msgtype"] != "actionCard" || body["actionCard"] == nil {
				t.Fatalf("body=%#v", body)
			}
		}},
		{"feishu", func(t *testing.T, body map[string]any) {
			if body["msg_type"] != "interactive" || body["card"] == nil {
				t.Fatalf("body=%#v", body)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.channel, func(t *testing.T) {
			template := DefaultTemplate(MessageTypeDaily, test.channel)
			if template.Mode != TemplateModeCard {
				t.Fatalf("template=%#v", template)
			}
			if err := ValidateTemplate(MessageTypeDaily, test.channel, template); err != nil {
				t.Fatal(err)
			}
			rendered, err := RenderTemplate(template, templateContext)
			if err != nil {
				t.Fatal(err)
			}
			test.assert(t, rendered.JSON)
		})
	}
}

func TestCardDefaultsSendProviderSpecificRequestBodies(t *testing.T) {
	templateContext := validationTemplateContext(MessageTypeDaily)
	for _, channel := range []string{"wecom", "dingtalk", "feishu"} {
		t.Run(channel, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			rendered, err := RenderTemplate(DefaultTemplate(MessageTypeDaily, channel), templateContext)
			if err != nil {
				t.Fatal(err)
			}
			config := Config{}
			switch channel {
			case "wecom":
				config.Channels.WecomWebhook = server.URL
			case "dingtalk":
				config.Channels.DingtalkWebhook = server.URL
			case "feishu":
				config.Channels.FeishuWebhook = server.URL
			}
			if err := sendRenderedByChannel(context.Background(), server.Client(), config, channel, rendered); err != nil {
				t.Fatal(err)
			}
			if body == nil {
				t.Fatal("provider did not receive a request body")
			}
			switch channel {
			case "wecom":
				if body["msgtype"] != "template_card" || body["template_card"] == nil {
					t.Fatalf("body=%#v", body)
				}
			case "dingtalk":
				if body["msgtype"] != "actionCard" || body["actionCard"] == nil {
					t.Fatalf("body=%#v", body)
				}
			case "feishu":
				if body["msg_type"] != "interactive" || body["card"] == nil {
					t.Fatalf("body=%#v", body)
				}
			}
		})
	}
}

func TestCardTemplatesRequireProviderEnvelope(t *testing.T) {
	for _, channel := range []string{"wecom", "dingtalk", "feishu"} {
		t.Run(channel, func(t *testing.T) {
			err := ValidateTemplate(MessageTypeDaily, channel, Template{Enabled: true, Mode: TemplateModeCard, Content: `{"title":"missing provider envelope"}`})
			if err == nil || !strings.Contains(err.Error(), "卡片") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestFeishuCardValidationRejectsMalformedTitleAndElementTags(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"title string", `{"msg_type":"interactive","card":{"header":{"title":"wrong"},"elements":[{"tag":"div"}]}}`},
		{"element missing tag", `{"msg_type":"interactive","card":{"header":{"title":{"tag":"plain_text","content":"title"}},"elements":[{}]}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(ServiceOptions{Store: NewMemoryStore()})
			config := Config{Templates: map[MessageType]map[string]Template{
				MessageTypeDaily: {
					"feishu": {Enabled: true, Mode: TemplateModeCard, Content: test.content},
				},
			}}
			if _, err := service.SaveConfig(context.Background(), config); err == nil || !strings.Contains(err.Error(), "飞书卡片") {
				t.Fatalf("err=%v", err)
			}

			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			config.Enabled = true
			config.SelectedChannels = []string{"feishu"}
			config.Channels.FeishuWebhook = server.URL
			if _, err := service.SendDailyNow(context.Background(), config); err == nil || !strings.Contains(err.Error(), "飞书卡片") {
				t.Fatalf("err=%v", err)
			}
			if attempts != 0 {
				t.Fatalf("invalid card made %d requests", attempts)
			}
		})
	}
}

func TestNormalizeTemplatesPreservesExplicitlyDisabledTemplate(t *testing.T) {
	config := NormalizeConfig(Config{Templates: map[MessageType]map[string]Template{
		MessageTypeDaily: {
			"webhook": {Enabled: false, Mode: TemplateModeText, Content: "do not replace"},
		},
	}})

	got := config.Templates[MessageTypeDaily]["webhook"]
	if got.Enabled || got.Mode != TemplateModeText || got.Content != "do not replace" {
		t.Fatalf("disabled template was clobbered: %#v", got)
	}
}

func TestNormalizeLogMonitorRulesTrimsAndPreservesBlankEnabledRules(t *testing.T) {
	got := NormalizeLogMonitorRules([]LogMonitorRule{
		{Enabled: true, Source: " runtime ", Type: " warning ", Keyword: " disk "},
		{Enabled: true, Source: " ", Type: " ", Keyword: " "},
	})
	if len(got) != 2 {
		t.Fatalf("rules=%#v", got)
	}
	if got[0].Source != "runtime" || got[0].Type != "warning" || got[0].Keyword != "disk" {
		t.Fatalf("rule=%#v", got[0])
	}
	if !got[1].Enabled || got[1].Source != "" || got[1].Type != "" || got[1].Keyword != "" {
		t.Fatalf("blank enabled rule=%#v", got[1])
	}
}

func TestTemplateRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name     string
		message  MessageType
		channel  string
		template Template
		want     string
	}{
		{"unsupported mode", MessageTypeLogMonitor, "qmsg", Template{Enabled: true, Mode: TemplateModeCard, Content: "{}"}, "不支持"},
		{"invalid JSON", MessageTypeAbnormal, "webhook", Template{Enabled: true, Mode: TemplateModeJSON, Content: "{"}, "JSON"},
		{"unknown variable", MessageTypeAbnormal, "webhook", Template{Enabled: true, Mode: TemplateModeJSON, Content: `{"message":"{{process.env}}"}`}, "未知变量"},
		{"unknown message type", MessageType("unknown"), "webhook", Template{Enabled: true, Mode: TemplateModeText, Content: "content"}, "消息类型"},
		{"unknown channel", MessageTypeDaily, "unknown", Template{Enabled: true, Mode: TemplateModeText, Content: "content"}, "渠道"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateTemplate(test.message, test.channel, test.template)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestSaveConfigAcceptsUnquotedNumericTemplatePlaceholder(t *testing.T) {
	service := NewService(ServiceOptions{Store: NewMemoryStore()})
	_, err := service.SaveConfig(context.Background(), Config{Templates: map[MessageType]map[string]Template{
		MessageTypeDaily: {
			"webhook": {Enabled: true, Mode: TemplateModeJSON, Content: `{"count":{{event.count}}}`},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSaveConfigRejectsInvalidTemplateBeforePersistence(t *testing.T) {
	store := &stateLoadFailingStore{config: `{"enabled":false}`}
	service := NewService(ServiceOptions{Store: store})

	_, err := service.SaveConfig(context.Background(), Config{Templates: map[MessageType]map[string]Template{
		MessageTypeDaily: {
			"webhook": {Enabled: true, Mode: TemplateModeJSON, Content: "{"},
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Fatalf("err=%v", err)
	}
	if store.saveCalls != 0 || store.config != `{"enabled":false}` {
		t.Fatalf("invalid config was persisted: calls=%d config=%s", store.saveCalls, store.config)
	}
}

func TestSaveConfigRejectsInvalidDisabledTemplatesBeforePersistence(t *testing.T) {
	tests := []struct {
		name     string
		channel  string
		template Template
		want     string
	}{
		{"unsupported qmsg card", "qmsg", Template{Enabled: false, Mode: TemplateModeCard, Content: "{}"}, "不支持"},
		{"malformed webhook JSON", "webhook", Template{Enabled: false, Mode: TemplateModeJSON, Content: "{"}, "JSON"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := NewMemoryStore()
			service := NewService(ServiceOptions{Store: store})
			_, err := service.SaveConfig(context.Background(), Config{Templates: map[MessageType]map[string]Template{
				MessageTypeDaily: {test.channel: test.template},
			}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v", err)
			}
			if len(store.Config.Templates) != 0 {
				t.Fatalf("invalid config was persisted: %#v", store.Config.Templates)
			}
		})
	}
}

func TestSaveConfigRejectsBlankEnabledLogMonitorRuleBeforePersistence(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(ServiceOptions{Store: store})

	_, err := service.SaveConfig(context.Background(), Config{LogMonitorRules: []LogMonitorRule{{Enabled: true}}})
	if err == nil || !strings.Contains(err.Error(), "日志监控规则") {
		t.Fatalf("err=%v", err)
	}
	if len(store.Config.Templates) != 0 {
		t.Fatalf("invalid config was persisted: %#v", store.Config.Templates)
	}
}

func TestSaveConfigRejectsUnknownMessageTypeWithEmptyChannelMap(t *testing.T) {
	tests := []struct {
		name     string
		channels map[string]Template
	}{
		{"nil map", nil},
		{"empty map", map[string]Template{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := NewMemoryStore()
			service := NewService(ServiceOptions{Store: store})
			_, err := service.SaveConfig(context.Background(), Config{Templates: map[MessageType]map[string]Template{
				MessageType("unknown"): test.channels,
			}})
			if err == nil || !strings.Contains(err.Error(), "消息类型") {
				t.Fatalf("err=%v", err)
			}
			if len(store.Config.Templates) != 0 {
				t.Fatalf("invalid config was persisted: %#v", store.Config.Templates)
			}
		})
	}
}

func TestSaveConfigRejectsCanonicalChannelKeyCollisions(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(ServiceOptions{Store: store})

	_, err := service.SaveConfig(context.Background(), Config{Templates: map[MessageType]map[string]Template{
		MessageTypeDaily: {
			"webhook":   {Enabled: true, Mode: TemplateModeJSON, Content: `{}`},
			" WEBHOOK ": {Enabled: true, Mode: TemplateModeJSON, Content: `{}`},
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "webhook") || !strings.Contains(err.Error(), " WEBHOOK ") {
		t.Fatalf("err=%v", err)
	}
	if len(store.Config.Templates) != 0 {
		t.Fatalf("invalid config was persisted: %#v", store.Config.Templates)
	}
}

func TestLoadConfigRejectsInvalidPersistedTemplateConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config string
		want   string
	}{
		{
			name:   "canonical channel collision",
			config: `{"templates":{"daily":{"webhook":{"enabled":true,"mode":"json","content":"{}"}," WEBHOOK ":{"enabled":true,"mode":"json","content":"{}"}}}}`,
			want:   "渠道键冲突",
		},
		{
			name:   "unknown message type with nil channels",
			config: `{"templates":{"unknown":null}}`,
			want:   "消息类型",
		},
		{
			name:   "unknown message type with empty channels",
			config: `{"templates":{"unknown":{}}}`,
			want:   "消息类型",
		},
		{
			name:   "blank enabled log monitor rule",
			config: `{"logMonitorRules":[{"enabled":true}]}`,
			want:   "日志监控规则",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(ServiceOptions{Store: rawStore{config: test.config, state: `{}`}})
			_, err := service.State(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestLoadStateInitializesRecentDedupe(t *testing.T) {
	service := NewService(ServiceOptions{})
	state, err := service.loadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.RecentDedupe == nil {
		t.Fatal("recent dedupe map is nil")
	}
}
