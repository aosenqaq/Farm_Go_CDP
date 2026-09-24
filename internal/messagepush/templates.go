package messagepush

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

var messageTypeDefinitions = []struct {
	typeValue MessageType
	label     string
}{
	{MessageTypeAbnormal, "异常告警"},
	{MessageTypeSuspected, "疑似异常"},
	{MessageTypeRecovery, "恢复通知"},
	{MessageTypeRestart, "重启通知"},
	{MessageTypeDaily, "每日报告"},
	{MessageTypeLogMonitor, "日志监控"},
}

var templateModesByChannel = map[string][]TemplateMode{
	"serverchan": {TemplateModeText, TemplateModeMarkdown},
	"pushplus":   {TemplateModeText, TemplateModeMarkdown},
	"qmsg":       {TemplateModeText},
	"wecom":      {TemplateModeText, TemplateModeMarkdown, TemplateModeCard, TemplateModeJSON},
	"dingtalk":   {TemplateModeText, TemplateModeMarkdown, TemplateModeCard, TemplateModeJSON},
	"feishu":     {TemplateModeText, TemplateModeMarkdown, TemplateModeCard, TemplateModeJSON},
	"telegram":   {TemplateModeText, TemplateModeMarkdown},
	"bark":       {TemplateModeText},
	"ntfy":       {TemplateModeText, TemplateModeMarkdown},
	"webhook":    {TemplateModeText, TemplateModeJSON},
}

func TemplateDefinitions() []TemplateDefinition {
	definitions := make([]TemplateDefinition, 0, len(messageTypeDefinitions))
	for _, definition := range messageTypeDefinitions {
		channelModes := make([]ChannelMode, 0, len(SupportedChannelTypes))
		for _, channelType := range SupportedChannelTypes {
			channelModes = append(channelModes, ChannelMode{
				Channel: channelType,
				Modes:   SupportedTemplateModes(channelType),
			})
		}
		definitions = append(definitions, TemplateDefinition{
			Type:         definition.typeValue,
			Label:        definition.label,
			ChannelModes: channelModes,
			Variables:    templateVariables(definition.typeValue),
		})
	}
	return definitions
}

func templateVariables(messageType MessageType) []TemplateVariable {
	variables := []TemplateVariable{
		{Path: "event.type", Label: "事件类型", Description: "触发本条消息的业务类型。", Example: string(messageType)},
		{Path: "event.time", Label: "发生时间", Description: "事件被 Farm_Go 记录的北京时间。", Example: "2026-07-12 09:30:00"},
		{Path: "event.count", Label: "累计次数", Description: "五分钟去重窗口内合并的同类事件数量。", Example: "3"},
		{Path: "account.key", Label: "账号作用域", Description: "当前推送配置所属账号的内部标识。", Example: "gid:10001"},
		{Path: "account.gid", Label: "运行账户 GID", Description: "当前运行账户已确认的 GID。", Example: "10001"},
		{Path: "runtime.target", Label: "运行目标", Description: "发生事件的运行链路或宿主目标。", Example: "qq_ws"},
		{Path: "payload.title", Label: "消息标题", Description: "当前测试或日报消息的显示标题。", Example: "农场资产日报"},
	}
	appendVariables := func(values ...TemplateVariable) []TemplateVariable { return append(variables, values...) }
	switch messageType {
	case MessageTypeAbnormal:
		return appendVariables(
			TemplateVariable{Path: "error.message", Label: "错误信息", Description: "本次异常的原始错误摘要。", Example: "context deadline exceeded"},
			TemplateVariable{Path: "error.stage", Label: "失败阶段", Description: "异常发生的恢复或连接阶段。", Example: "network_reconnect"},
			TemplateVariable{Path: "restart.reason", Label: "重启原因", Description: "守护进程发起重启时记录的原因。", Example: "timeout"},
		)
	case MessageTypeSuspected:
		return appendVariables(
			TemplateVariable{Path: "error.message", Label: "错误信息", Description: "触发疑似异常的错误摘要。", Example: "runtime not connected"},
			TemplateVariable{Path: "monitor.streak", Label: "连续次数", Description: "连续检测到异常的次数。", Example: "2"},
			TemplateVariable{Path: "monitor.threshold", Label: "告警阈值", Description: "进入自动恢复前的异常次数阈值。", Example: "3"},
		)
	case MessageTypeRecovery:
		return appendVariables(
			TemplateVariable{Path: "recovery.duration", Label: "恢复耗时", Description: "异常到恢复健康状态的持续时间。", Example: "45s"},
			TemplateVariable{Path: "recovery.via", Label: "恢复方式", Description: "完成恢复的守护或重连路径。", Example: "guardian"},
			TemplateVariable{Path: "previous.error", Label: "此前错误", Description: "恢复前最后记录的异常摘要。", Example: "timeout"},
		)
	case MessageTypeRestart:
		return appendVariables(
			TemplateVariable{Path: "restart.trigger", Label: "触发方式", Description: "手动、定时或自动重启。", Example: "scheduled"},
			TemplateVariable{Path: "restart.reason", Label: "重启原因", Description: "本次重启的原因。", Example: "manual restart"},
			TemplateVariable{Path: "restart.ok", Label: "重启结果", Description: "重启是否成功。", Example: "true"},
			TemplateVariable{Path: "restart.error", Label: "重启错误", Description: "失败时的错误信息。", Example: "host process not found"},
		)
	case MessageTypeDaily:
		return appendVariables(
			TemplateVariable{Path: "daily.name", Label: "账户昵称", Description: "游戏运行时返回的当前账户昵称。", Example: "Dpo.L"},
			TemplateVariable{Path: "daily.level", Label: "农场等级", Description: "当前账户的农场等级。", Example: "118"},
			TemplateVariable{Path: "daily.date", Label: "日报日期", Description: "本次日报对应的北京日期。", Example: "2026-07-12"},
			TemplateVariable{Path: "daily.gold", Label: "当前金币", Description: "游戏运行时读取的当前金币余额。", Example: "3891552777"},
			TemplateVariable{Path: "daily.bean", Label: "当前金豆", Description: "游戏运行时读取的当前金豆余额。", Example: "455521"},
			TemplateVariable{Path: "daily.warehouseEstimate", Label: "仓库可售估值", Description: "当前仓库可出售物品按售价计算的总估值。", Example: "115998840"},
			TemplateVariable{Path: "daily.warehouseSellableCount", Label: "可售物品数量", Description: "当前仓库中可出售物品的总数量。", Example: "330"},
			TemplateVariable{Path: "daily.sellCount", Label: "当日出售次数", Description: "今天已记录的仓库出售次数。", Example: "24"},
			TemplateVariable{Path: "daily.sellAmount", Label: "当日出售金额", Description: "今天出售记录累计获得的金币。", Example: "10336"},
			TemplateVariable{Path: "daily.runs", Label: "当日运行次数", Description: "今天成功完成的自动化任务总次数。", Example: "15399"},
			TemplateVariable{Path: "daily.collect", Label: "收获次数", Description: "今天成功执行的收获次数。", Example: "130"},
			TemplateVariable{Path: "daily.water", Label: "浇水次数", Description: "今天成功执行的浇水次数。", Example: "4"},
			TemplateVariable{Path: "daily.steal", Label: "偷菜次数", Description: "今天成功执行的偷菜次数。", Example: "38"},
			TemplateVariable{Path: "daily.help", Label: "帮忙次数", Description: "今天成功执行的帮忙次数。", Example: "27"},
			TemplateVariable{Path: "daily.mischiefGrass", Label: "除草次数", Description: "今天成功执行的除草次数。", Example: "3"},
			TemplateVariable{Path: "daily.mischiefBug", Label: "除虫次数", Description: "今天成功执行的除虫次数。", Example: "2"},
			TemplateVariable{Path: "daily.summary", Label: "日报摘要", Description: "由当前账户真实数据汇总生成的摘要正文。", Example: "金币 3891552777，金豆 455521。"},
		)
	case MessageTypeLogMonitor:
		return appendVariables(
			TemplateVariable{Path: "log.level", Label: "日志级别", Description: "命中规则的运行日志等级。", Example: "error"},
			TemplateVariable{Path: "log.source", Label: "日志来源", Description: "产生该日志的模块。", Example: "guardian"},
			TemplateVariable{Path: "log.type", Label: "事件类型", Description: "运行事件的分类标识。", Example: "guardian.network.failed"},
			TemplateVariable{Path: "log.message", Label: "日志内容", Description: "命中规则的日志消息。", Example: "network reconnect failed"},
			TemplateVariable{Path: "log.data", Label: "日志数据", Description: "附带的结构化运行数据。", Example: "{\"error\":\"timeout\"}"},
		)
	}
	return variables
}

func SupportedTemplateModes(channelType string) []TemplateMode {
	modes := templateModesByChannel[strings.ToLower(strings.TrimSpace(channelType))]
	return append([]TemplateMode(nil), modes...)
}

func DefaultTemplate(messageType MessageType, channelType string) Template {
	mode := TemplateModeText
	if channelType = strings.ToLower(strings.TrimSpace(channelType)); channelType == "webhook" {
		mode = TemplateModeJSON
	} else if supportsTemplateMode(channelType, TemplateModeCard) {
		mode = TemplateModeCard
	} else if supportsTemplateMode(channelType, TemplateModeMarkdown) {
		mode = TemplateModeMarkdown
	}

	label := messageTypeLabel(messageType)
	if messageType == MessageTypeDaily {
		return defaultDailyTemplate(channelType, mode)
	}

	content := "【" + label + "】\n发生时间：{{event.time}}\n事件：{{event.type}}\n运行目标：{{runtime.target}}\n累计次数：{{event.count}}"
	switch mode {
	case TemplateModeMarkdown:
		content = "**Farm_Go · " + label + "**\n\n> 发生时间：{{event.time}}\n> 事件：{{event.type}}\n\n运行目标：{{runtime.target}}\n\n累计次数：{{event.count}}"
	case TemplateModeCard:
		switch channelType {
		case "wecom":
			content = `{"msgtype":"template_card","template_card":{"card_type":"text_notice","main_title":{"title":"` + label + `","desc":"发生时间：{{event.time}}"},"emphasis_content":{"title":"事件：{{event.type}}","desc":"运行目标：{{runtime.target}}"}}}`
		case "dingtalk":
			content = `{"msgtype":"actionCard","actionCard":{"title":"` + label + `","text":"发生时间：{{event.time}}\n事件：{{event.type}}\n运行目标：{{runtime.target}}\n累计次数：{{event.count}}","singleTitle":"查看详情","singleURL":"https://www.dingtalk.com/"}}`
		case "feishu":
			content = `{"msg_type":"interactive","card":{"config":{"wide_screen_mode":true},"header":{"template":"` + eventTemplateColor(messageType) + `","title":{"tag":"plain_text","content":"Farm_Go · ` + label + `"}},"elements":[{"tag":"div","fields":[{"is_short":true,"text":{"tag":"lark_md","content":"**发生时间**\n{{event.time}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**事件**\n{{event.type}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**运行目标**\n{{runtime.target}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**累计次数**\n{{event.count}}"}}]},{"tag":"note","elements":[{"tag":"plain_text","content":"Farm_Go 自动消息推送"}]}]}}`
		}
	case TemplateModeJSON:
		content = `{"title":"` + label + `","occurredAt":"{{event.time}}","event":"{{event.type}}","target":"{{runtime.target}}","count":{{event.count}}}`
	}
	return Template{Enabled: true, Mode: mode, Content: content}
}

func defaultDailyTemplate(channelType string, mode TemplateMode) Template {
	content := "【Farm_Go 农场日报 · {{daily.date}}】\n{{daily.name}} · Lv.{{daily.level}} · GID {{account.gid}}\n\n金币 {{daily.gold}} · 金豆 {{daily.bean}}\n出售 {{daily.sellCount}} 件 · 获得 {{daily.sellAmount}} 金币\n仓库可售估值 {{daily.warehouseEstimate}} 金币 · {{daily.warehouseSellableCount}} 件\n\n运行 {{daily.runs}} · 收获 {{daily.collect}} · 浇水 {{daily.water}} · 偷菜 {{daily.steal}} · 帮忙 {{daily.help}} · 除草 {{daily.mischiefGrass}} · 除虫 {{daily.mischiefBug}}"
	switch mode {
	case TemplateModeMarkdown:
		content = "# Farm_Go 农场日报 · {{daily.date}}\n\n**{{daily.name}}** · Lv.{{daily.level}} · GID {{account.gid}}\n\n---\n\n**金币** {{daily.gold}}    **金豆** {{daily.bean}}\n\n出售 {{daily.sellCount}} 件，获得 {{daily.sellAmount}} 金币\n\n仓库可售估值 {{daily.warehouseEstimate}} 金币，共 {{daily.warehouseSellableCount}} 件\n\n---\n\n运行 {{daily.runs}} · 收获 {{daily.collect}} · 浇水 {{daily.water}} · 偷菜 {{daily.steal}} · 帮忙 {{daily.help}} · 除草 {{daily.mischiefGrass}} · 除虫 {{daily.mischiefBug}}"
	case TemplateModeCard:
		switch channelType {
		case "wecom":
			content = `{"msgtype":"template_card","template_card":{"card_type":"text_notice","main_title":{"title":"Farm_Go 农场日报 · {{daily.date}}","desc":"{{daily.name}} · Lv.{{daily.level}} · GID {{account.gid}}"},"emphasis_content":{"title":"金币 {{daily.gold}} · 金豆 {{daily.bean}}","desc":"出售 {{daily.sellCount}} 件 · 获得 {{daily.sellAmount}} 金币"},"sub_title_text":"仓库可售估值 {{daily.warehouseEstimate}} 金币（{{daily.warehouseSellableCount}} 件）\n运行 {{daily.runs}} · 收获 {{daily.collect}} · 浇水 {{daily.water}} · 偷菜 {{daily.steal}} · 帮忙 {{daily.help}}"}}`
		case "dingtalk":
			content = `{"msgtype":"actionCard","actionCard":{"title":"Farm_Go 农场日报 · {{daily.date}}","text":"{{daily.name}} · Lv.{{daily.level}} · GID {{account.gid}}\n\n金币 {{daily.gold}} · 金豆 {{daily.bean}}\n出售 {{daily.sellCount}} 件 · 获得 {{daily.sellAmount}} 金币\n仓库可售估值 {{daily.warehouseEstimate}} 金币 · {{daily.warehouseSellableCount}} 件\n\n运行 {{daily.runs}} · 收获 {{daily.collect}} · 浇水 {{daily.water}} · 偷菜 {{daily.steal}} · 帮忙 {{daily.help}}","singleTitle":"查看日报","singleURL":"https://www.dingtalk.com/"}}`
		case "feishu":
			content = `{"msg_type":"interactive","card":{"config":{"wide_screen_mode":true},"header":{"template":"green","title":{"tag":"plain_text","content":"Farm_Go 农场日报 · {{daily.date}}"}},"elements":[{"tag":"div","text":{"tag":"lark_md","content":"**{{daily.name}}** · Lv.{{daily.level}} · GID {{account.gid}}"}},{"tag":"hr"},{"tag":"div","fields":[{"is_short":true,"text":{"tag":"lark_md","content":"**金币**\n{{daily.gold}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**金豆**\n{{daily.bean}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**出售**\n{{daily.sellCount}} 件 · {{daily.sellAmount}} 金币"}},{"is_short":true,"text":{"tag":"lark_md","content":"**仓库可售估值**\n{{daily.warehouseEstimate}} 金币 · {{daily.warehouseSellableCount}} 件"}}]},{"tag":"hr"},{"tag":"div","text":{"tag":"lark_md","content":"**当日运行** {{daily.runs}}\n收获 {{daily.collect}} · 浇水 {{daily.water}} · 偷菜 {{daily.steal}} · 帮忙 {{daily.help}} · 除草 {{daily.mischiefGrass}} · 除虫 {{daily.mischiefBug}}"}},{"tag":"note","elements":[{"tag":"plain_text","content":"数据来自当前运行账户、仓库快照和当日出售记录"}]}]}}`
		}
	case TemplateModeJSON:
		content = `{"title":"Farm_Go 农场日报","date":"{{daily.date}}","account":{"gid":"{{account.gid}}","name":"{{daily.name}}","level":{{daily.level}}},"assets":{"gold":{{daily.gold}},"bean":{{daily.bean}}},"sales":{"count":{{daily.sellCount}},"amount":{{daily.sellAmount}}},"warehouse":{"estimate":{{daily.warehouseEstimate}},"sellableCount":{{daily.warehouseSellableCount}}},"activity":{"runs":{{daily.runs}},"collect":{{daily.collect}},"water":{{daily.water}},"steal":{{daily.steal}},"help":{{daily.help}},"mischiefGrass":{{daily.mischiefGrass}},"mischiefBug":{{daily.mischiefBug}}}}`
	}
	return Template{Enabled: true, Mode: mode, Content: content}
}

func eventTemplateColor(messageType MessageType) string {
	switch messageType {
	case MessageTypeAbnormal:
		return "red"
	case MessageTypeSuspected:
		return "orange"
	case MessageTypeRecovery:
		return "green"
	case MessageTypeLogMonitor:
		return "grey"
	default:
		return "blue"
	}
}

func legacyDefaultTemplate(messageType MessageType, channelType string) Template {
	mode := TemplateModeText
	if channelType = strings.ToLower(strings.TrimSpace(channelType)); channelType == "webhook" {
		mode = TemplateModeJSON
	} else if supportsTemplateMode(channelType, TemplateModeCard) {
		mode = TemplateModeCard
	} else if supportsTemplateMode(channelType, TemplateModeMarkdown) {
		mode = TemplateModeMarkdown
	}

	label := messageTypeLabel(messageType)
	content := "[" + label + "] {{event.type}}\n时间：{{event.time}}\n目标：{{runtime.target}}"
	switch mode {
	case TemplateModeMarkdown:
		content = "**[" + label + "] {{event.type}}**\n\n时间：{{event.time}}\n\n目标：{{runtime.target}}"
	case TemplateModeCard:
		switch channelType {
		case "wecom":
			content = `{"msgtype":"template_card","template_card":{"card_type":"text_notice","main_title":{"title":"` + label + `","desc":"{{event.type}}"},"emphasis_content":{"title":"{{runtime.target}}","desc":"{{event.time}}"}}}`
		case "dingtalk":
			content = `{"msgtype":"actionCard","actionCard":{"title":"` + label + `","text":"{{event.type}}\n{{event.time}}\n{{runtime.target}}","singleTitle":"查看详情","singleURL":"https://www.dingtalk.com/"}}`
		case "feishu":
			content = `{"msg_type":"interactive","card":{"config":{"wide_screen_mode":true},"header":{"template":"orange","title":{"tag":"plain_text","content":"Farm_Go · ` + label + `"}},"elements":[{"tag":"div","fields":[{"is_short":true,"text":{"tag":"lark_md","content":"**事件**\n{{event.type}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**运行目标**\n{{runtime.target}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**触发时间**\n{{event.time}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**累计次数**\n{{event.count}}"}}]},{"tag":"hr"},{"tag":"note","elements":[{"tag":"plain_text","content":"Farm_Go 自动消息推送"}]}]}}`
		}
	case TemplateModeJSON:
		content = `{"title":"` + label + `","content":"{{event.type}}\n{{event.time}}\n{{runtime.target}}"}`
	}
	return Template{Enabled: true, Mode: mode, Content: content}
}

func previousDailyDefaultTemplate(channelType string) Template {
	mode := legacyDefaultTemplate(MessageTypeDaily, channelType).Mode
	content := "【Farm_Go 农场日报 · {{daily.date}}】\n运行账户 GID：{{account.gid}}\n统计日期：{{daily.date}}\n\n{{daily.summary}}"
	switch mode {
	case TemplateModeMarkdown:
		content = "# Farm_Go 农场日报 · {{daily.date}}\n\n**运行账户 GID**  {{account.gid}}\n\n统计日期：{{daily.date}}\n\n---\n\n## 今日概览\n\n{{daily.summary}}"
	case TemplateModeCard:
		switch channelType {
		case "wecom":
			content = `{"msgtype":"template_card","template_card":{"card_type":"text_notice","main_title":{"title":"Farm_Go 农场日报","desc":"统计日期：{{daily.date}}"},"emphasis_content":{"title":"运行账户 GID","desc":"{{account.gid}}"},"sub_title_text":"{{daily.summary}}"}}`
		case "dingtalk":
			content = `{"msgtype":"actionCard","actionCard":{"title":"Farm_Go 农场日报 · {{daily.date}}","text":"运行账户 GID：{{account.gid}}\n统计日期：{{daily.date}}\n\n{{daily.summary}}","singleTitle":"查看日报","singleURL":"https://www.dingtalk.com/"}}`
		case "feishu":
			content = `{"msg_type":"interactive","card":{"config":{"wide_screen_mode":true},"header":{"template":"green","title":{"tag":"plain_text","content":"Farm_Go 农场日报 · {{daily.date}}"}},"elements":[{"tag":"div","fields":[{"is_short":true,"text":{"tag":"lark_md","content":"**运行账户 GID**\n{{account.gid}}"}},{"is_short":true,"text":{"tag":"lark_md","content":"**统计日期**\n{{daily.date}}"}}]},{"tag":"hr"},{"tag":"div","text":{"tag":"lark_md","content":"**今日概览**\n{{daily.summary}}"}},{"tag":"note","elements":[{"tag":"plain_text","content":"Farm_Go 自动日报"}]}]}}`
		}
	case TemplateModeJSON:
		content = `{"title":"Farm_Go 农场日报","date":"{{daily.date}}","accountGID":"{{account.gid}}","summary":"{{daily.summary}}"}`
	}
	return Template{Enabled: true, Mode: mode, Content: content}
}

func isLegacyDefaultTemplate(messageType MessageType, channelType string, template Template) bool {
	candidates := []Template{legacyDefaultTemplate(messageType, channelType)}
	if messageType == MessageTypeDaily {
		candidates = append(candidates, previousDailyDefaultTemplate(channelType))
	}
	for _, candidate := range candidates {
		if template.Enabled == candidate.Enabled && template.Mode == candidate.Mode && strings.TrimSpace(template.Content) == strings.TrimSpace(candidate.Content) {
			return true
		}
	}
	return false
}

func NormalizeTemplates(templates map[MessageType]map[string]Template) map[MessageType]map[string]Template {
	normalized := make(map[MessageType]map[string]Template, len(messageTypeDefinitions))
	for _, definition := range messageTypeDefinitions {
		byChannel := make(map[string]Template, len(SupportedChannelTypes))
		for _, channelType := range SupportedChannelTypes {
			byChannel[channelType] = DefaultTemplate(definition.typeValue, channelType)
		}
		normalized[definition.typeValue] = byChannel
	}
	for messageType, byChannel := range templates {
		if normalized[messageType] == nil {
			normalized[messageType] = map[string]Template{}
		}
		for channelType, template := range byChannel {
			channelType = strings.ToLower(strings.TrimSpace(channelType))
			if isLegacyDefaultTemplate(messageType, channelType, template) {
				template = DefaultTemplate(messageType, channelType)
			}
			template.Mode = TemplateMode(strings.ToLower(strings.TrimSpace(string(template.Mode))))
			template.Content = strings.TrimSpace(template.Content)
			normalized[messageType][channelType] = template
		}
	}
	return normalized
}

func NormalizeLogMonitorRules(rules []LogMonitorRule) []LogMonitorRule {
	normalized := make([]LogMonitorRule, 0, len(rules))
	for _, rule := range rules {
		rule.Source = strings.TrimSpace(rule.Source)
		rule.Type = strings.TrimSpace(rule.Type)
		rule.Keyword = strings.TrimSpace(rule.Keyword)
		if !rule.Enabled && rule.Source == "" && rule.Type == "" && rule.Keyword == "" {
			continue
		}
		normalized = append(normalized, rule)
	}
	return normalized
}

func ValidateTemplate(messageType MessageType, channelType string, template Template) error {
	if !isSupportedMessageType(messageType) {
		return fmt.Errorf("未知消息类型: %s", messageType)
	}
	channelType = strings.ToLower(strings.TrimSpace(channelType))
	if !isSupportedChannel(channelType) {
		return fmt.Errorf("未知推送渠道: %s", channelType)
	}
	if !supportsTemplateMode(channelType, template.Mode) {
		return fmt.Errorf("渠道 %s 不支持 %s 模板", channelType, template.Mode)
	}
	if strings.TrimSpace(template.Content) == "" {
		return fmt.Errorf("模板内容不能为空")
	}
	rendered, err := RenderTemplate(template, validationTemplateContext(messageType))
	if err != nil {
		if template.Mode == TemplateModeJSON {
			return fmt.Errorf("JSON 模板无效: %w", err)
		}
		if template.Mode == TemplateModeCard {
			return fmt.Errorf("卡片模板无效: %w", err)
		}
		return err
	}
	if template.Mode == TemplateModeCard {
		if err := validateCardEnvelope(channelType, rendered.JSON); err != nil {
			return err
		}
	}
	return nil
}

func validationTemplateContext(messageType MessageType) TemplateContext {
	return sampleTemplateContext(messageType, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
}

func sampleTemplateContext(messageType MessageType, now time.Time) TemplateContext {
	if now.IsZero() {
		now = time.Now()
	}
	values := map[string]any{
		"event":   map[string]any{"type": string(messageType), "time": formatBeijingTime(now), "count": 1},
		"account": map[string]any{"key": "account", "gid": "gid"},
		"payload": map[string]any{"title": "title"},
		"runtime": map[string]any{"target": "Farm_Go"},
	}
	switch messageType {
	case MessageTypeAbnormal:
		values["error"] = map[string]any{"message": "error", "stage": "stage"}
		values["restart"] = map[string]any{"reason": "reason"}
	case MessageTypeSuspected:
		values["error"] = map[string]any{"message": "error"}
		values["monitor"] = map[string]any{"streak": 1, "threshold": 2}
	case MessageTypeRecovery:
		values["recovery"] = map[string]any{"duration": "1m", "via": "guardian"}
		values["previous"] = map[string]any{"error": "error"}
	case MessageTypeRestart:
		values["restart"] = map[string]any{"trigger": "manual", "reason": "reason", "ok": true, "error": ""}
	case MessageTypeDaily:
		values["daily"] = map[string]any{
			"date":                   formatBeijingDate(now),
			"summary":                "summary",
			"name":                   "Farm_Go",
			"level":                  118,
			"gold":                   3891552777,
			"bean":                   455521,
			"warehouseEstimate":      115998840,
			"warehouseSellableCount": 330,
			"sellCount":              24,
			"sellAmount":             10336,
			"runs":                   15399,
			"collect":                130,
			"water":                  4,
			"steal":                  38,
			"help":                   27,
			"mischiefGrass":          3,
			"mischiefBug":            2,
		}
	case MessageTypeLogMonitor:
		values["log"] = map[string]any{"level": "warn", "source": "runtime", "type": "event", "message": "message", "data": map[string]any{"key": "value"}}
	}
	return TemplateContext{MessageType: messageType, Values: values}
}

func validateCardEnvelope(channelType string, body map[string]any) error {
	switch channelType {
	case "wecom":
		card, ok := body["template_card"].(map[string]any)
		if body["msgtype"] != "template_card" || !ok || !hasString(card, "card_type") || !hasNestedString(card, "main_title", "title") {
			return fmt.Errorf("企业微信卡片模板必须是 template_card 信封")
		}
	case "dingtalk":
		card, ok := body["actionCard"].(map[string]any)
		if body["msgtype"] != "actionCard" || !ok || !hasString(card, "title") || !hasString(card, "text") || !hasString(card, "singleTitle") || !hasString(card, "singleURL") {
			return fmt.Errorf("钉钉卡片模板必须是 actionCard 信封")
		}
	case "feishu":
		card, ok := body["card"].(map[string]any)
		elements, elementsOK := card["elements"].([]any)
		if body["msg_type"] != "interactive" || !ok || !hasFeishuCardTitle(card) || !hasFeishuCardElementTags(elements, elementsOK) {
			return fmt.Errorf("飞书卡片模板必须是 interactive 信封")
		}
	default:
		return fmt.Errorf("渠道 %s 不支持卡片模板", channelType)
	}
	return nil
}

func hasFeishuCardTitle(card map[string]any) bool {
	header, ok := card["header"].(map[string]any)
	if !ok {
		return false
	}
	title, ok := header["title"].(map[string]any)
	return ok && title["tag"] == "plain_text" && hasString(title, "content")
}

func hasFeishuCardElementTags(elements []any, elementsOK bool) bool {
	if !elementsOK || len(elements) == 0 {
		return false
	}
	for _, element := range elements {
		values, ok := element.(map[string]any)
		if !ok || !hasString(values, "tag") {
			return false
		}
	}
	return true
}

func hasString(values map[string]any, key string) bool {
	value, ok := values[key].(string)
	return ok && strings.TrimSpace(value) != ""
}

func hasNestedString(values map[string]any, key string, nestedKey string) bool {
	nested, ok := values[key].(map[string]any)
	if !ok {
		return false
	}
	if nestedKey == "title" {
		if title, ok := nested[nestedKey].(map[string]any); ok {
			return hasString(title, "content")
		}
	}
	return hasString(nested, nestedKey)
}

func ValidateConfigTemplates(config Config) error {
	for messageType, byChannel := range config.Templates {
		if !isSupportedMessageType(messageType) {
			return fmt.Errorf("未知消息类型: %s", messageType)
		}
		for channelType, template := range byChannel {
			if err := ValidateTemplate(messageType, channelType, template); err != nil {
				return fmt.Errorf("模板 %s/%s: %w", messageType, channelType, err)
			}
		}
	}
	for _, rule := range config.LogMonitorRules {
		if rule.Enabled && strings.TrimSpace(rule.Source) == "" && strings.TrimSpace(rule.Type) == "" && strings.TrimSpace(rule.Keyword) == "" {
			return fmt.Errorf("日志监控规则至少需要一个匹配条件")
		}
	}
	return nil
}

func validateTemplateChannelKeyCollisions(templates map[MessageType]map[string]Template) error {
	messageTypes := make([]string, 0, len(templates))
	byType := make(map[string]map[string]Template, len(templates))
	for messageType, byChannel := range templates {
		key := string(messageType)
		messageTypes = append(messageTypes, key)
		byType[key] = byChannel
	}
	sort.Strings(messageTypes)
	for _, messageType := range messageTypes {
		byChannel := byType[messageType]
		channels := make([]string, 0, len(byChannel))
		for channelType := range byChannel {
			channels = append(channels, channelType)
		}
		sort.Strings(channels)
		canonicalKeys := make(map[string]string, len(channels))
		for _, channelType := range channels {
			canonical := strings.ToLower(strings.TrimSpace(channelType))
			if previous, exists := canonicalKeys[canonical]; exists {
				return fmt.Errorf("模板 %s 的渠道键冲突: %q 和 %q 都规范化为 %q", messageType, previous, channelType, canonical)
			}
			canonicalKeys[canonical] = channelType
		}
	}
	return nil
}

func supportsTemplateMode(channelType string, mode TemplateMode) bool {
	for _, supported := range templateModesByChannel[channelType] {
		if mode == supported {
			return true
		}
	}
	return false
}

func isSupportedMessageType(messageType MessageType) bool {
	for _, definition := range messageTypeDefinitions {
		if messageType == definition.typeValue {
			return true
		}
	}
	return false
}

func messageTypeLabel(messageType MessageType) string {
	for _, definition := range messageTypeDefinitions {
		if messageType == definition.typeValue {
			return definition.label
		}
	}
	return string(messageType)
}
