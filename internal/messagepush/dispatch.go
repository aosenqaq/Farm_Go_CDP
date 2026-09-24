package messagepush

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const dispatchDedupeWindow = 5 * time.Minute

type RuntimeLog struct {
	Level   string
	Source  string
	Type    string
	Message string
	Data    map[string]any
}

type MessageEvent struct {
	Type          MessageType
	AccountKey    string
	RuntimeTarget string
	Error         string
	Trigger       string
	RecoveryVia   string
	OccurredAt    time.Time
	Values        map[string]any
	RuntimeLog    *RuntimeLog
}

type DispatchResult struct {
	Sent            bool
	Suppressed      bool
	SuppressedCount int
	Send            SendResult
	FallbackError   string
}

func MatchLogMonitorRule(rules []LogMonitorRule, log RuntimeLog) bool {
	level := strings.ToLower(strings.TrimSpace(log.Level))
	if level != "warn" && level != "warning" && level != "error" {
		return false
	}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if !matchesLogConstraint(rule.Source, log.Source, false) || !matchesLogConstraint(rule.Type, log.Type, false) || !matchesLogConstraint(rule.Keyword, log.Message, true) {
			continue
		}
		return true
	}
	return false
}

func matchesLogConstraint(rule, value string, contains bool) bool {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return true
	}
	if contains {
		return strings.Contains(strings.ToLower(value), strings.ToLower(rule))
	}
	return strings.EqualFold(rule, strings.TrimSpace(value))
}

func (s *Service) Dispatch(ctx context.Context, event MessageEvent) (DispatchResult, error) {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()
	return s.dispatch(ctx, event)
}

func (s *Service) dispatch(ctx context.Context, event MessageEvent) (DispatchResult, error) {
	if !isSupportedMessageType(event.Type) {
		return DispatchResult{}, fmt.Errorf("未知消息类型: %s", event.Type)
	}
	if event.Type == MessageTypeLogMonitor {
		if event.RuntimeLog == nil || strings.EqualFold(strings.TrimSpace(event.RuntimeLog.Source), "message_push") {
			return DispatchResult{}, nil
		}
	}

	config, err := s.loadDispatchConfig(ctx)
	if err != nil {
		return DispatchResult{}, err
	}
	if !config.Enabled || !messageTypeEnabled(config, event.Type) {
		return DispatchResult{}, nil
	}
	if event.Type == MessageTypeLogMonitor && !MatchLogMonitorRule(config.LogMonitorRules, *event.RuntimeLog) {
		return DispatchResult{}, nil
	}

	channels := PickAvailableChannels(config, nil)
	if len(channels) == 0 {
		return DispatchResult{}, errors.New("请先配置并选择一个推送渠道")
	}
	channelType := channels[0]
	template := config.Templates[event.Type][channelType]
	if !template.Enabled {
		return DispatchResult{Send: SendResult{Summary: "template_disabled"}}, nil
	}

	now := event.OccurredAt
	if now.IsZero() {
		now = s.now()
	}
	state, err := s.loadState(ctx)
	if err != nil {
		return DispatchResult{}, err
	}
	dedupeSignature := dispatchSignature(config, event)
	eventCount := 1
	if dispatchUsesDedupe(event.Type) {
		key := dispatchDedupeKey(s.dispatchAccountKey(event), event, dedupeSignature)
		if record, ok := state.RecentDedupe[key]; ok {
			if lastAt, parseErr := time.Parse(time.RFC3339, record.LastAt); parseErr == nil && now.Sub(lastAt) < dispatchDedupeWindow {
				record.LastAt = now.Format(time.RFC3339)
				record.Count++
				state.RecentDedupe[key] = record
				if err := s.saveState(ctx, state); err != nil {
					return DispatchResult{}, err
				}
				return DispatchResult{Suppressed: true, SuppressedCount: record.Count}, nil
			}
			eventCount += record.Count
		}
		if trimDedupeExcept(state.RecentDedupe, now, key) {
			if err := s.saveState(ctx, state); err != nil {
				return DispatchResult{}, err
			}
		}
	}

	send, fallbackError, err := s.dispatchTemplate(ctx, config, event, channelType, template, now, eventCount)
	result := DispatchResult{Sent: send.OK, Send: send, FallbackError: fallbackError}
	if err != nil {
		return result, err
	}
	if dispatchUsesDedupe(event.Type) && result.Sent {
		state, err = s.loadState(ctx)
		if err != nil {
			return result, err
		}
		key := dispatchDedupeKey(s.dispatchAccountKey(event), event, dedupeSignature)
		trimDedupe(state.RecentDedupe, now)
		state.RecentDedupe[key] = DedupeRecord{FirstAt: now.Format(time.RFC3339), LastAt: now.Format(time.RFC3339)}
		if err := s.saveState(ctx, state); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *Service) loadDispatchConfig(ctx context.Context) (Config, error) {
	if s.store == nil {
		return NormalizeConfig(Config{}), nil
	}
	raw, err := s.store.LoadMessagePushConfigJSON(ctx)
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(raw) == "" {
		return NormalizeConfig(Config{}), nil
	}
	var config Config
	if err := unmarshalJSON(raw, &config); err != nil {
		return Config{}, err
	}
	if err := validateTemplateChannelKeyCollisions(config.Templates); err != nil {
		return Config{}, err
	}
	return NormalizeConfig(applyMissingTypeDefaults(raw, config)), nil
}

func (s *Service) dispatchTemplate(ctx context.Context, config Config, event MessageEvent, channelType string, template Template, now time.Time, eventCount int) (SendResult, string, error) {
	context := s.dispatchTemplateContext(event, now, eventCount)
	rendered, renderErr := renderDispatchTemplate(template, context, channelType)
	fallbackError := ""
	if renderErr != nil {
		fallbackError = renderErr.Error()
		s.reportFallback(s.dispatchAccountKey(event), event.Type, channelType, renderErr)
		rendered, renderErr = renderDispatchTemplate(DefaultTemplate(event.Type, channelType), context, channelType)
	}
	if renderErr != nil {
		return SendResult{}, fallbackError, renderErr
	}
	result := ChannelResult{Type: channelType}
	for attempt := 1; attempt <= config.PushRetryCount; attempt++ {
		result.Attempts = attempt
		err := sendRenderedByChannel(ctx, s.httpClient, config, channelType, rendered)
		if err == nil {
			result.OK = true
			break
		} else {
			result.Error = err.Error()
		}
	}
	send := SendResult{
		OK:      result.OK,
		Summary: resultSummary(result.OK, []ChannelResult{result}),
		Results: []ChannelResult{result},
		Payload: Payload{Kind: string(event.Type), Title: messageTypeLabel(event.Type), Lines: []string{eventSummary(event)}},
	}
	if err := s.recordPush(ctx, send.Payload, []string{channelType}, send); err != nil {
		return send, fallbackError, err
	}
	if !send.OK {
		return send, fallbackError, errors.New(send.Summary)
	}
	return send, fallbackError, nil
}

func (s *Service) reportFallback(accountKey string, messageType MessageType, channelType string, err error) {
	if s.fallbackDiagnostic == nil {
		return
	}
	defer func() { _ = recover() }()
	s.fallbackDiagnostic(accountKey, messageType, channelType, err)
}

func renderDispatchTemplate(template Template, context TemplateContext, channelType string) (RenderedPayload, error) {
	rendered, err := RenderTemplate(template, context)
	if err != nil {
		return RenderedPayload{}, err
	}
	if template.Mode == TemplateModeCard {
		if err := validateCardEnvelope(channelType, rendered.JSON); err != nil {
			return RenderedPayload{}, err
		}
	}
	return rendered, nil
}

func (s *Service) dispatchTemplateContext(event MessageEvent, now time.Time, eventCount int) TemplateContext {
	target := strings.TrimSpace(event.RuntimeTarget)
	if target == "" {
		target = "Farm_Go"
	}
	values := map[string]any{
		"event":    map[string]any{"type": string(event.Type), "time": formatBeijingTime(now), "count": eventCount},
		"account":  map[string]any{"key": s.dispatchAccountKey(event), "gid": s.accountGID},
		"payload":  map[string]any{"title": messageTypeLabel(event.Type)},
		"runtime":  map[string]any{"target": target},
		"error":    map[string]any{"message": event.Error, "stage": event.Trigger},
		"restart":  map[string]any{"trigger": event.Trigger, "reason": event.Error, "ok": event.Error == "", "error": event.Error},
		"recovery": map[string]any{"duration": "", "via": event.RecoveryVia},
		"previous": map[string]any{"error": event.Error},
		"monitor":  map[string]any{"streak": 0, "threshold": 0},
		"daily":    map[string]any{"date": TodayKey(now), "summary": eventSummary(event)},
	}
	if event.RuntimeLog != nil {
		values["log"] = map[string]any{"level": event.RuntimeLog.Level, "source": event.RuntimeLog.Source, "type": event.RuntimeLog.Type, "message": event.RuntimeLog.Message, "data": event.RuntimeLog.Data}
	} else {
		values["log"] = map[string]any{"level": "", "source": "", "type": "", "message": "", "data": map[string]any{}}
	}
	for key, value := range event.Values {
		values[key] = value
	}
	return TemplateContext{MessageType: event.Type, Values: values}
}

func messageTypeEnabled(config Config, messageType MessageType) bool {
	switch messageType {
	case MessageTypeAbnormal:
		return config.AbnormalEnabled
	case MessageTypeSuspected:
		return config.SuspectedEnabled
	case MessageTypeRecovery:
		return config.RecoveryEnabled
	case MessageTypeRestart:
		return config.RestartEnabled
	case MessageTypeDaily:
		return config.DailyEnabled
	case MessageTypeLogMonitor:
		return config.LogMonitorEnabled
	default:
		return false
	}
}

func dispatchUsesDedupe(messageType MessageType) bool {
	return messageType != MessageTypeRecovery && messageType != MessageTypeRestart && messageType != MessageTypeDaily
}

func (s *Service) dispatchAccountKey(event MessageEvent) string {
	if key := strings.TrimSpace(event.AccountKey); key != "" {
		return key
	}
	return s.accountKey
}

func dispatchDedupeKey(accountKey string, event MessageEvent, signature string) string {
	value := strings.Join([]string{accountKey, string(event.Type), event.RuntimeTarget, signature}, "\x00")
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func dispatchSignature(config Config, event MessageEvent) string {
	if event.RuntimeLog != nil {
		for _, rule := range config.LogMonitorRules {
			if rule.Enabled && matchesLogConstraint(rule.Source, event.RuntimeLog.Source, false) && matchesLogConstraint(rule.Type, event.RuntimeLog.Type, false) && matchesLogConstraint(rule.Keyword, event.RuntimeLog.Message, true) {
				return normalizeDedupeText(rule.Source + "|" + rule.Type + "|" + rule.Keyword)
			}
		}
		return normalizeDedupeText(event.RuntimeLog.Source + "|" + event.RuntimeLog.Type + "|" + event.RuntimeLog.Message)
	}
	return normalizeDedupeText(event.Error)
}

func normalizeDedupeText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func trimDedupe(records map[string]DedupeRecord, now time.Time) bool {
	return trimDedupeExcept(records, now, "")
}

func trimDedupeExcept(records map[string]DedupeRecord, now time.Time, keep string) bool {
	trimmed := false
	for key, record := range records {
		if key == keep {
			continue
		}
		lastAt, err := time.Parse(time.RFC3339, record.LastAt)
		if err != nil || now.Sub(lastAt) >= dispatchDedupeWindow {
			delete(records, key)
			trimmed = true
		}
	}
	return trimmed
}

func eventSummary(event MessageEvent) string {
	if event.RuntimeLog != nil {
		return event.RuntimeLog.Message
	}
	if event.Error != "" {
		return event.Error
	}
	if event.RecoveryVia != "" {
		return event.RecoveryVia
	}
	return string(event.Type)
}
