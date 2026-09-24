package messagepush

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type TemplateContext struct {
	MessageType MessageType
	Values      map[string]any
}

type RenderedPayload struct {
	Kind    string
	Mode    TemplateMode
	Text    string
	JSON    map[string]any
	Context TemplateContext
}

var templatePathPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$`)

var commonTemplatePaths = map[string]struct{}{
	"event.type":     {},
	"event.time":     {},
	"event.count":    {},
	"account.key":    {},
	"account.gid":    {},
	"payload.title":  {},
	"runtime.target": {},
}

var templatePathsByMessageType = map[MessageType]map[string]struct{}{
	MessageTypeAbnormal:  {"error.message": {}, "error.stage": {}, "restart.reason": {}},
	MessageTypeSuspected: {"error.message": {}, "monitor.streak": {}, "monitor.threshold": {}},
	MessageTypeRecovery:  {"recovery.duration": {}, "recovery.via": {}, "previous.error": {}},
	MessageTypeRestart:   {"restart.trigger": {}, "restart.reason": {}, "restart.ok": {}, "restart.error": {}},
	MessageTypeDaily: {
		"daily.date": {}, "daily.summary": {}, "daily.name": {}, "daily.level": {},
		"daily.gold": {}, "daily.bean": {}, "daily.warehouseEstimate": {}, "daily.warehouseSellableCount": {},
		"daily.sellCount": {}, "daily.sellAmount": {}, "daily.runs": {}, "daily.collect": {},
		"daily.water": {}, "daily.steal": {}, "daily.help": {}, "daily.mischiefGrass": {}, "daily.mischiefBug": {},
	},
	MessageTypeLogMonitor: {"log.level": {}, "log.source": {}, "log.type": {}, "log.message": {}, "log.data": {}},
}

func RenderTemplate(template Template, context TemplateContext) (RenderedPayload, error) {
	if !isSupportedMessageType(context.MessageType) {
		return RenderedPayload{}, fmt.Errorf("未知消息类型: %s", context.MessageType)
	}
	rendered, err := renderTemplateContent(template.Content, context, template.Mode == TemplateModeCard || template.Mode == TemplateModeJSON)
	if err != nil {
		return RenderedPayload{}, err
	}
	payload := RenderedPayload{
		Kind:    string(context.MessageType),
		Mode:    template.Mode,
		Text:    rendered,
		Context: context,
	}
	if template.Mode != TemplateModeCard && template.Mode != TemplateModeJSON {
		return payload, nil
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(rendered), &body); err != nil {
		return RenderedPayload{}, fmt.Errorf("%s 模板渲染后不是有效 JSON 对象: %w", template.Mode, err)
	}
	if body == nil {
		return RenderedPayload{}, fmt.Errorf("%s 模板渲染后必须是 JSON 对象", template.Mode)
	}
	payload.JSON = body
	payload.Text = ""
	return payload, nil
}

func renderTemplateContent(content string, context TemplateContext, escapeJSONStringValues bool) (string, error) {
	var result strings.Builder
	for len(content) > 0 {
		start := strings.Index(content, "{{")
		if start < 0 {
			if !escapeJSONStringValues && strings.Contains(content, "}}") {
				return "", fmt.Errorf("模板变量格式无效")
			}
			result.WriteString(content)
			break
		}
		result.WriteString(content[:start])
		content = content[start+2:]
		end := strings.Index(content, "}}")
		if end < 0 {
			return "", fmt.Errorf("模板变量格式无效")
		}
		path := content[:end]
		if !templatePathPattern.MatchString(path) {
			return "", fmt.Errorf("模板变量格式无效: %s", path)
		}
		value, err := resolveTemplatePath(path, context)
		if err != nil {
			return "", err
		}
		if escapeJSONStringValues && isInsideJSONString(result.String()) {
			escaped, err := json.Marshal(fmt.Sprint(value))
			if err != nil {
				return "", err
			}
			result.Write(escaped[1 : len(escaped)-1])
		} else if escapeJSONStringValues {
			encoded, err := json.Marshal(value)
			if err != nil {
				return "", err
			}
			result.Write(encoded)
		} else {
			result.WriteString(fmt.Sprint(value))
		}
		content = content[end+2:]
	}
	return result.String(), nil
}

func isInsideJSONString(content string) bool {
	inString := false
	escaped := false
	for _, char := range content {
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && inString {
			escaped = true
			continue
		}
		if char == '"' {
			inString = !inString
		}
	}
	return inString
}

func resolveTemplatePath(path string, context TemplateContext) (any, error) {
	if _, ok := commonTemplatePaths[path]; !ok {
		if _, ok := templatePathsByMessageType[context.MessageType][path]; !ok {
			return nil, fmt.Errorf("未知变量: %s", path)
		}
	}
	var current any = context.Values
	for _, segment := range strings.Split(path, ".") {
		values, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("缺少变量: %s", path)
		}
		value, ok := values[segment]
		if !ok || value == nil {
			return nil, fmt.Errorf("缺少变量: %s", path)
		}
		current = value
	}
	return current, nil
}
