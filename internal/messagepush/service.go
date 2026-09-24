package messagepush

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Store interface {
	LoadMessagePushConfigJSON(context.Context) (string, error)
	SaveMessagePushConfigJSON(context.Context, string) error
	LoadMessagePushStateJSON(context.Context) (string, error)
	SaveMessagePushStateJSON(context.Context, string) error
}

type DailyReport struct {
	AccountGID string
	DateKey    string
	Summary    string
	Values     map[string]any
}

type DailyReportProvider func(context.Context, time.Time) (DailyReport, error)

type ServiceOptions struct {
	Store               Store
	HTTPClient          *http.Client
	Now                 func() time.Time
	AccountKey          string
	AccountGID          string
	DailyReportProvider DailyReportProvider
	FallbackDiagnostic  func(string, MessageType, string, error)
}

type Service struct {
	store               Store
	httpClient          *http.Client
	now                 func() time.Time
	accountKey          string
	accountGID          string
	dailyReportProvider DailyReportProvider
	fallbackDiagnostic  func(string, MessageType, string, error)
	dispatchMu          sync.Mutex
}

var beijingLocation = time.FixedZone("CST", 8*60*60)

const beijingTimeLayout = "2006-01-02 15:04:05"

func formatBeijingTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.In(beijingLocation).Format(beijingTimeLayout)
}

func formatBeijingDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.In(beijingLocation).Format("2006-01-02")
}

// displayBeijingTime normalizes stored timestamps (RFC3339 or already-friendly) for UI/templates.
func displayBeijingTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return formatBeijingTime(parsed)
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return formatBeijingTime(parsed)
	}
	if parsed, err := time.ParseInLocation(beijingTimeLayout, value, beijingLocation); err == nil {
		return formatBeijingTime(parsed)
	}
	return value
}

func displayPushRecords(records []PushRecord) []PushRecord {
	if len(records) == 0 {
		return records
	}
	out := make([]PushRecord, len(records))
	copy(out, records)
	for i := range out {
		out[i].Time = displayBeijingTime(out[i].Time)
	}
	return out
}

func NewService(options ServiceOptions) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Service{store: options.Store, httpClient: client, now: now, accountKey: options.AccountKey, accountGID: options.AccountGID, dailyReportProvider: options.DailyReportProvider, fallbackDiagnostic: options.FallbackDiagnostic}
}

func (s *Service) State(ctx context.Context) (ViewState, error) {
	config, err := s.loadConfig(ctx)
	if err != nil {
		return ViewState{}, err
	}
	state, err := s.loadState(ctx)
	if err != nil {
		return ViewState{}, err
	}
	return s.viewState(config, state), nil
}

func (s *Service) SaveConfig(ctx context.Context, config Config) (ViewState, error) {
	if err := validateTemplateChannelKeyCollisions(config.Templates); err != nil {
		return ViewState{}, err
	}
	config = NormalizeConfig(config)
	if err := ValidateConfigTemplates(config); err != nil {
		return ViewState{}, err
	}
	state, err := s.loadState(ctx)
	if err != nil {
		return ViewState{}, err
	}
	if err := s.saveConfig(ctx, config); err != nil {
		return ViewState{}, err
	}
	return s.viewState(config, state), nil
}

func (s *Service) SendTest(ctx context.Context, config Config) (SendResult, error) {
	var err error
	config, err = validateDeliveryConfig(config)
	if err != nil {
		return SendResult{}, err
	}
	payload := Payload{
		Kind:  "test",
		Title: "农场测试推送",
		Lines: []string{"这是一条来自 Farm_Go 的测试消息。", "如果你看到它，推送通道已经连通。"},
	}
	templateContext, err := s.dailyTemplateTestContext(ctx, "test", "这是一条来自 Farm_Go 的测试消息。", payload.Title)
	if err != nil {
		return SendResult{}, err
	}
	return s.sendTemplate(ctx, config, MessageTypeDaily, templateContext, payload)
}

func (s *Service) SendDailyNow(ctx context.Context, config Config) (SendResult, error) {
	var err error
	config, err = validateDeliveryConfig(config)
	if err != nil {
		return SendResult{}, err
	}
	payload := Payload{
		Kind:  "daily",
		Title: "农场资产日报",
		Lines: []string{"日报数据已从当前运行账户读取。"},
		Meta:  map[string]any{"dateKey": ShiftDateKey(s.now(), -1)},
	}
	templateContext, err := s.dailyTemplateContext(ctx, "daily", "今日推送模块已就绪。\n后续可接入实时资产统计。", payload.Title)
	if err != nil {
		return SendResult{}, err
	}
	return s.sendTemplate(ctx, config, MessageTypeDaily, templateContext, payload)
}

func (s *Service) PreviewTemplate(config Config, messageType MessageType, channelType string) (TemplatePreview, error) {
	_, template, channelType, err := selectedDraftTemplate(config, messageType, channelType)
	if err != nil {
		return TemplatePreview{MessageType: messageType, Channel: channelType}, err
	}
	rendered, err := RenderTemplate(template, sampleTemplateContext(messageType, s.now()))
	if err != nil {
		return TemplatePreview{MessageType: messageType, Channel: channelType}, err
	}
	return TemplatePreview{OK: true, MessageType: messageType, Channel: channelType, Rendered: rendered}, nil
}

func (s *Service) SendTemplateTest(ctx context.Context, config Config, messageType MessageType, channelType string) (SendResult, error) {
	config, template, channelType, err := selectedDraftTemplate(config, messageType, channelType)
	if err != nil {
		return SendResult{}, err
	}
	templateContext := sampleTemplateContext(messageType, s.now())
	if messageType == MessageTypeDaily {
		templateContext, err = s.dailyTemplateContext(ctx, "daily", "日报数据已从当前运行账户读取。", "农场模板测试")
		if err != nil {
			return SendResult{}, err
		}
	}
	rendered, err := RenderTemplate(template, templateContext)
	if err != nil {
		return SendResult{}, err
	}
	result := ChannelResult{Type: channelType}
	for attempt := 1; attempt <= config.PushRetryCount; attempt++ {
		result.Attempts = attempt
		err = sendRenderedByChannel(ctx, s.httpClient, config, channelType, rendered)
		if err == nil {
			result.OK = true
			break
		}
		result.Error = err.Error()
	}
	send := SendResult{
		OK:      result.OK,
		Summary: resultSummary(result.OK, []ChannelResult{result}),
		Results: []ChannelResult{result},
		Payload: Payload{Kind: "test", Title: "农场模板测试", Lines: []string{messageTypeLabel(messageType)}},
	}
	if !send.OK {
		return send, errors.New(send.Summary)
	}
	return send, nil
}

func selectedDraftTemplate(config Config, messageType MessageType, channelType string) (Config, Template, string, error) {
	config, err := validateDeliveryConfig(config)
	if err != nil {
		return Config{}, Template{}, "", err
	}
	if !isSupportedMessageType(messageType) {
		return Config{}, Template{}, "", errors.New("未知消息类型")
	}
	channelType = strings.ToLower(strings.TrimSpace(channelType))
	if !isSupportedChannel(channelType) {
		return Config{}, Template{}, "", errors.New("未知推送渠道")
	}
	selected := false
	for _, channel := range PickAvailableChannels(config, nil) {
		if channel == channelType {
			selected = true
			break
		}
	}
	if !selected {
		return Config{}, Template{}, "", errors.New("请先配置并选择该推送渠道")
	}
	template := config.Templates[messageType][channelType]
	if !template.Enabled {
		return Config{}, Template{}, "", errors.New("当前模板未启用")
	}
	return config, template, channelType, nil
}

func validateDeliveryConfig(config Config) (Config, error) {
	if err := validateTemplateChannelKeyCollisions(config.Templates); err != nil {
		return Config{}, err
	}
	config = NormalizeConfig(config)
	if err := ValidateConfigTemplates(config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (s *Service) dailyTemplateContext(ctx context.Context, eventType string, summary string, title string) (TemplateContext, error) {
	return s.dailyTemplateContextWithReport(ctx, eventType, summary, title, true, true)
}

func (s *Service) dailyTemplateTestContext(ctx context.Context, eventType string, summary string, title string) (TemplateContext, error) {
	return s.dailyTemplateContextWithReport(ctx, eventType, summary, title, false, false)
}

func (s *Service) dailyTemplateContextWithReport(ctx context.Context, eventType string, summary string, title string, useLiveReport bool, previousDay bool) (TemplateContext, error) {
	now := s.now()
	accountGID := s.accountGID
	reportDateKey := TodayKey(now)
	if previousDay {
		reportDateKey = ShiftDateKey(now, -1)
	}
	dailyValues := map[string]any{
		"date": reportDateKey, "summary": summary, "name": "Farm_Go", "level": 0,
		"gold": 0, "bean": 0, "warehouseEstimate": 0, "warehouseSellableCount": 0,
		"sellCount": 0, "sellAmount": 0, "runs": 0, "collect": 0, "water": 0,
		"steal": 0, "help": 0, "mischiefGrass": 0, "mischiefBug": 0,
	}
	if useLiveReport && s.dailyReportProvider != nil {
		report, err := s.dailyReportProvider(ctx, now)
		if err != nil {
			return TemplateContext{}, err
		}
		for key, value := range report.Values {
			dailyValues[key] = value
		}
		if strings.TrimSpace(report.Summary) != "" {
			dailyValues["summary"] = report.Summary
		}
		if strings.TrimSpace(report.DateKey) != "" {
			dailyValues["date"] = report.DateKey
		}
		if strings.TrimSpace(report.AccountGID) != "" {
			accountGID = report.AccountGID
		}
	}
	return TemplateContext{
		MessageType: MessageTypeDaily,
		Values: map[string]any{
			"event":   map[string]any{"type": eventType, "time": formatBeijingTime(now), "count": 1},
			"account": map[string]any{"key": s.accountKey, "gid": accountGID},
			"payload": map[string]any{"title": title},
			"runtime": map[string]any{"target": "Farm_Go"},
			"daily":   dailyValues,
		},
	}, nil
}

func (s *Service) RunDueDaily(ctx context.Context) (SendResult, error) {
	config, err := s.loadConfig(ctx)
	if err != nil {
		return SendResult{}, err
	}
	state, err := s.loadState(ctx)
	if err != nil {
		return SendResult{}, err
	}
	now := s.now()
	state.LastDailySummaryCheckAt = formatBeijingTime(now)
	if next := NextDailyRunAt(config, state, now); next == nil || !next.Equal(now) {
		state.LastDailySummarySkipReason = "not_due"
		if state.LastDailySummaryDateKey == TodayKey(now) {
			state.LastDailySummarySkipReason = "already_sent"
		}
		if err := s.saveState(ctx, state); err != nil {
			return SendResult{}, err
		}
		return SendResult{OK: false, Summary: state.LastDailySummarySkipReason}, nil
	}
	templateContext, err := s.dailyTemplateContext(ctx, "daily", "今日推送模块已就绪。\n后续可接入实时资产统计。", "农场资产日报")
	if err != nil {
		state.LastDailySummarySkipReason = err.Error()
		_ = s.saveState(ctx, state)
		return SendResult{}, err
	}
	dispatch, err := s.Dispatch(ctx, MessageEvent{
		Type:       MessageTypeDaily,
		OccurredAt: now,
		Values:     templateContext.Values,
	})
	result := dispatch.Send
	if err != nil {
		state.LastDailySummarySkipReason = err.Error()
		_ = s.saveState(ctx, state)
		return result, err
	}
	if !dispatch.Sent || dispatch.Suppressed {
		state.LastDailySummarySkipReason = result.Summary
		if err := s.saveState(ctx, state); err != nil {
			return result, err
		}
		return result, nil
	}
	state, err = s.loadState(ctx)
	if err != nil {
		return result, err
	}
	state.LastDailySummaryDateKey = TodayKey(now)
	state.LastDailySummaryAt = formatBeijingTime(now)
	state.LastDailySummaryCheckAt = formatBeijingTime(now)
	state.LastDailySummarySkipReason = ""
	if err := s.saveState(ctx, state); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Service) send(ctx context.Context, config Config, payload Payload) (SendResult, error) {
	channels := PickAvailableChannels(config, nil)
	if len(channels) == 0 {
		return SendResult{}, errors.New("请先配置并选择一个推送渠道")
	}
	if !config.Enabled && payload.Kind != "test" {
		return SendResult{}, errors.New("消息推送总开关未开启")
	}
	results := make([]ChannelResult, 0, len(channels))
	overallOK := false
	for _, channelType := range channels {
		result := ChannelResult{Type: channelType}
		var lastErr error
		for attempt := 1; attempt <= config.PushRetryCount; attempt++ {
			result.Attempts = attempt
			lastErr = sendByChannel(ctx, s.httpClient, config, channelType, payload)
			if lastErr == nil {
				result.OK = true
				break
			}
		}
		if lastErr != nil {
			result.Error = lastErr.Error()
		}
		if result.OK {
			overallOK = true
		}
		results = append(results, result)
	}
	sendResult := SendResult{OK: overallOK, Summary: resultSummary(overallOK, results), Results: results, Payload: payload}
	if err := s.recordPush(ctx, payload, channels, sendResult); err != nil {
		return sendResult, err
	}
	if !overallOK {
		return sendResult, errors.New(sendResult.Summary)
	}
	return sendResult, nil
}

func (s *Service) sendTemplate(ctx context.Context, config Config, messageType MessageType, templateContext TemplateContext, legacyPayload Payload) (SendResult, error) {
	channels := PickAvailableChannels(config, nil)
	if len(channels) == 0 {
		return SendResult{}, errors.New("请先配置并选择一个推送渠道")
	}
	if !config.Enabled && legacyPayload.Kind != "test" {
		return SendResult{}, errors.New("消息推送总开关未开启")
	}
	results := make([]ChannelResult, 0, len(channels))
	sentChannels := make([]string, 0, len(channels))
	overallOK := false
	for _, channelType := range channels {
		template := config.Templates[messageType][channelType]
		if !template.Enabled {
			continue
		}
		result := ChannelResult{Type: channelType}
		rendered, err := RenderTemplate(template, templateContext)
		if err == nil {
			for attempt := 1; attempt <= config.PushRetryCount; attempt++ {
				result.Attempts = attempt
				err = sendRenderedByChannel(ctx, s.httpClient, config, channelType, rendered)
				if err == nil {
					result.OK = true
					break
				}
			}
		}
		if err != nil {
			result.Error = err.Error()
		}
		if result.OK {
			overallOK = true
		}
		results = append(results, result)
		sentChannels = append(sentChannels, channelType)
	}
	sendResult := SendResult{OK: overallOK, Summary: resultSummary(overallOK, results), Results: results, Payload: legacyPayload}
	if err := s.recordPush(ctx, legacyPayload, sentChannels, sendResult); err != nil {
		return sendResult, err
	}
	if !overallOK {
		return sendResult, errors.New(sendResult.Summary)
	}
	return sendResult, nil
}

func (s *Service) loadConfig(ctx context.Context) (Config, error) {
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
	config = applyMissingTypeDefaults(raw, config)
	if err := validateTemplateChannelKeyCollisions(config.Templates); err != nil {
		return Config{}, err
	}
	config = NormalizeConfig(config)
	if err := ValidateConfigTemplates(config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (s *Service) saveConfig(ctx context.Context, config Config) error {
	if s.store == nil {
		return nil
	}
	raw, err := marshalJSON(config)
	if err != nil {
		return err
	}
	return s.store.SaveMessagePushConfigJSON(ctx, raw)
}

func (s *Service) loadState(ctx context.Context) (State, error) {
	if s.store == nil {
		return State{RecentPushes: []PushRecord{}, RecentDedupe: map[string]DedupeRecord{}}, nil
	}
	raw, err := s.store.LoadMessagePushStateJSON(ctx)
	if err != nil {
		return State{}, err
	}
	if strings.TrimSpace(raw) == "" {
		return State{RecentPushes: []PushRecord{}, RecentDedupe: map[string]DedupeRecord{}}, nil
	}
	var state State
	if err := unmarshalJSON(raw, &state); err != nil {
		return State{}, err
	}
	if state.RecentPushes == nil {
		state.RecentPushes = []PushRecord{}
	}
	if state.RecentDedupe == nil {
		state.RecentDedupe = map[string]DedupeRecord{}
	}
	return state, nil
}

func (s *Service) saveState(ctx context.Context, state State) error {
	if len(state.RecentPushes) > 20 {
		state.RecentPushes = state.RecentPushes[:20]
	}
	if s.store == nil {
		return nil
	}
	raw, err := marshalJSON(state)
	if err != nil {
		return err
	}
	return s.store.SaveMessagePushStateJSON(ctx, raw)
}

func (s *Service) recordPush(ctx context.Context, payload Payload, channels []string, result SendResult) error {
	state, err := s.loadState(ctx)
	if err != nil {
		return err
	}
	record := PushRecord{
		Time:     formatBeijingTime(s.now()),
		Kind:     payload.Kind,
		Title:    payload.Title,
		OK:       result.OK,
		Channels: channels,
	}
	if !result.OK {
		record.Error = result.Summary
	}
	state.RecentPushes = append([]PushRecord{record}, state.RecentPushes...)
	return s.saveState(ctx, state)
}

func (s *Service) viewState(config Config, state State) ViewState {
	configured := ConfiguredChannelTypes(config)
	next := NextDailyRunAt(config, state, s.now())
	daily := DailyState{
		Enabled:            config.DailyEnabled,
		Channels:           PickAvailableChannels(config, nil),
		Time:               config.DailyTime,
		LastSummaryDateKey: state.LastDailySummaryDateKey,
		LastSummaryAt:      displayBeijingTime(state.LastDailySummaryAt),
		LastCheckedAt:      displayBeijingTime(state.LastDailySummaryCheckAt),
		LastSkipReason:     state.LastDailySummarySkipReason,
	}
	if next != nil {
		daily.NextRunAt = formatBeijingTime(*next)
	}
	return ViewState{
		Enabled:            config.Enabled,
		Config:             config,
		ConfiguredChannels: configured,
		Channels:           BuildChannelMetaList(config),
		Daily:              daily,
		RecentPushes:       displayPushRecords(state.RecentPushes),
		LogMonitorEnabled:  config.LogMonitorEnabled,
		TemplateCatalog:    TemplateDefinitions(),
	}
}

func resultSummary(ok bool, results []ChannelResult) string {
	if ok {
		return "sent"
	}
	for _, result := range results {
		if result.Error != "" {
			return result.Error
		}
	}
	return "send_failed"
}

func marshalJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func unmarshalJSON(raw string, target any) error {
	return json.Unmarshal([]byte(raw), target)
}

func applyMissingTypeDefaults(raw string, config Config) Config {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return config
	}
	if _, ok := fields["abnormalEnabled"]; !ok {
		config.AbnormalEnabled = true
	}
	if _, ok := fields["suspectedEnabled"]; !ok {
		config.SuspectedEnabled = true
	}
	if _, ok := fields["recoveryEnabled"]; !ok {
		config.RecoveryEnabled = true
	}
	if _, ok := fields["restartEnabled"]; !ok {
		config.RestartEnabled = true
	}
	if _, ok := fields["logMonitorEnabled"]; !ok {
		config.LogMonitorEnabled = true
	}
	return config
}
