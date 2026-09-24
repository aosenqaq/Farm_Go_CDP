package messagepush

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

var SupportedChannelTypes = []string{"serverchan", "pushplus", "qmsg", "wecom", "dingtalk", "feishu", "telegram", "bark", "ntfy", "webhook"}

var ChannelLabels = map[string]string{
	"serverchan": "Server酱",
	"pushplus":   "PushPlus",
	"qmsg":       "Qmsg酱",
	"wecom":      "企业微信",
	"dingtalk":   "钉钉",
	"feishu":     "飞书",
	"telegram":   "Telegram",
	"bark":       "Bark",
	"ntfy":       "ntfy",
	"webhook":    "Webhook",
}

func NormalizeConfig(config Config) Config {
	if isBlankConfig(config) {
		config.AbnormalEnabled = true
		config.SuspectedEnabled = true
		config.RecoveryEnabled = true
		config.RestartEnabled = true
		config.LogMonitorEnabled = true
	}
	config.DailyTime = normalizeDailyTime(config.DailyTime)
	config.AbnormalTimeoutThreshold = clampInt(config.AbnormalTimeoutThreshold, 1, 99, 5)
	config.LogScanIntervalSec = clampInt(config.LogScanIntervalSec, 5, 3600, 30)
	config.HTTPTimeoutMS = clampInt(config.HTTPTimeoutMS, 1000, 120000, 10000)
	config.PushRetryCount = clampInt(config.PushRetryCount, 1, 10, 3)
	config.SelectedChannels = normalizeSelectedChannels(config.SelectedChannels)
	config.ChannelFormats = normalizeChannelFormats(config.ChannelFormats)
	config.Channels = normalizeChannels(config.Channels)
	config.Templates = NormalizeTemplates(config.Templates)
	config.LogMonitorRules = NormalizeLogMonitorRules(config.LogMonitorRules)
	return config
}

func isBlankConfig(config Config) bool {
	return !config.Enabled &&
		!config.AbnormalEnabled &&
		!config.SuspectedEnabled &&
		!config.RecoveryEnabled &&
		!config.DailyEnabled &&
		!config.DailyMarkdownCardEnabled &&
		!config.RestartEnabled &&
		!config.LogMonitorEnabled &&
		config.DailyTime == "" &&
		config.AbnormalTimeoutThreshold == 0 &&
		config.LogScanIntervalSec == 0 &&
		config.HTTPTimeoutMS == 0 &&
		config.PushRetryCount == 0 &&
		len(config.SelectedChannels) == 0 &&
		len(config.ChannelFormats) == 0 &&
		config.Channels == (Channels{}) &&
		len(config.Templates) == 0 &&
		len(config.LogMonitorRules) == 0
}

func ConfiguredChannelTypes(config Config) []string {
	channels := config.Channels
	configured := []string{}
	for _, channelType := range SupportedChannelTypes {
		switch channelType {
		case "serverchan":
			if strings.TrimSpace(channels.ServerChanSendKey) != "" {
				configured = append(configured, channelType)
			}
		case "pushplus":
			if strings.TrimSpace(channels.PushPlusToken) != "" {
				configured = append(configured, channelType)
			}
		case "qmsg":
			if strings.TrimSpace(channels.QmsgKey) != "" {
				configured = append(configured, channelType)
			}
		case "wecom":
			if strings.TrimSpace(channels.WecomWebhook) != "" {
				configured = append(configured, channelType)
			}
		case "dingtalk":
			if strings.TrimSpace(channels.DingtalkWebhook) != "" {
				configured = append(configured, channelType)
			}
		case "feishu":
			if strings.TrimSpace(channels.FeishuWebhook) != "" {
				configured = append(configured, channelType)
			}
		case "telegram":
			if strings.TrimSpace(channels.TelegramBotToken) != "" && strings.TrimSpace(channels.TelegramChatID) != "" {
				configured = append(configured, channelType)
			}
		case "bark":
			if strings.TrimSpace(channels.BarkDeviceKey) != "" {
				configured = append(configured, channelType)
			}
		case "ntfy":
			if strings.TrimSpace(channels.NtfyTopic) != "" {
				configured = append(configured, channelType)
			}
		case "webhook":
			if strings.TrimSpace(channels.WebhookURL) != "" {
				configured = append(configured, channelType)
			}
		}
	}
	return configured
}

func PickAvailableChannels(config Config, requested []string) []string {
	config = NormalizeConfig(config)
	configured := setFromSlice(ConfiguredChannelTypes(config))
	candidates := requested
	if len(candidates) == 0 {
		candidates = config.SelectedChannels
	}
	for _, channelType := range candidates {
		if configured[channelType] {
			return []string{channelType}
		}
	}
	if len(requested) > 0 {
		return nil
	}
	for _, channelType := range SupportedChannelTypes {
		if configured[channelType] {
			return []string{channelType}
		}
	}
	return nil
}

func BuildChannelMetaList(config Config) []ChannelMeta {
	configured := setFromSlice(ConfiguredChannelTypes(config))
	metas := make([]ChannelMeta, 0, len(SupportedChannelTypes))
	for _, channelType := range SupportedChannelTypes {
		metas = append(metas, ChannelMeta{
			Type:       channelType,
			Label:      ChannelLabels[channelType],
			Configured: configured[channelType],
		})
	}
	return metas
}

func NextDailyRunAt(config Config, state State, now time.Time) *time.Time {
	config = NormalizeConfig(config)
	if !config.Enabled || !config.DailyEnabled {
		return nil
	}
	now = now.In(beijingLocation)
	today := TodayKey(now)
	if state.LastDailySummaryDateKey == today {
		return nil
	}
	hour, minute := dailyHourMinute(config.DailyTime)
	due := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, beijingLocation)
	if !now.Before(due) {
		return &now
	}
	return &due
}

func TodayKey(now time.Time) string {
	return formatBeijingDate(now)
}

func ShiftDateKey(now time.Time, days int) string {
	return formatBeijingDate(now.In(beijingLocation).AddDate(0, 0, days))
}

func normalizeDailyTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "09:00"
	}
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "09:00"
	}
	hour, errHour := strconv.Atoi(parts[0])
	minute, errMinute := strconv.Atoi(parts[1])
	if errHour != nil || errMinute != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return "09:00"
	}
	return twoDigit(hour) + ":" + twoDigit(minute)
}

func dailyHourMinute(value string) (int, int) {
	parts := strings.Split(normalizeDailyTime(value), ":")
	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])
	return hour, minute
}

func normalizeSelectedChannels(values []string) []string {
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if isSupportedChannel(key) {
			return []string{key}
		}
	}
	return nil
}

func normalizeChannelFormats(values map[string]string) map[string]string {
	formats := map[string]string{}
	for _, channelType := range SupportedChannelTypes {
		format := strings.ToLower(strings.TrimSpace(values[channelType]))
		if format != "markdown" {
			format = "text"
		}
		formats[channelType] = format
	}
	return formats
}

func normalizeChannels(channels Channels) Channels {
	channels.ServerChanSendKey = strings.TrimSpace(channels.ServerChanSendKey)
	channels.PushPlusToken = strings.TrimSpace(channels.PushPlusToken)
	channels.QmsgKey = strings.TrimSpace(channels.QmsgKey)
	channels.QmsgType = strings.TrimSpace(channels.QmsgType)
	channels.QmsgTarget = strings.TrimSpace(channels.QmsgTarget)
	channels.WecomWebhook = strings.TrimSpace(channels.WecomWebhook)
	channels.DingtalkWebhook = strings.TrimSpace(channels.DingtalkWebhook)
	channels.DingtalkSecret = strings.TrimSpace(channels.DingtalkSecret)
	channels.FeishuWebhook = strings.TrimSpace(channels.FeishuWebhook)
	channels.TelegramBotToken = strings.TrimSpace(channels.TelegramBotToken)
	channels.TelegramChatID = strings.TrimSpace(channels.TelegramChatID)
	channels.BarkDeviceKey = strings.TrimSpace(channels.BarkDeviceKey)
	channels.NtfyTopic = strings.TrimSpace(channels.NtfyTopic)
	channels.WebhookURL = strings.TrimSpace(channels.WebhookURL)
	channels.WebhookHeaders = strings.TrimSpace(channels.WebhookHeaders)
	if strings.TrimSpace(channels.BarkServerURL) == "" {
		channels.BarkServerURL = "https://api.day.app"
	}
	if strings.TrimSpace(channels.NtfyServerURL) == "" {
		channels.NtfyServerURL = "https://ntfy.sh"
	}
	channels.BarkServerURL = strings.TrimRight(strings.TrimSpace(channels.BarkServerURL), "/")
	channels.NtfyServerURL = strings.TrimRight(strings.TrimSpace(channels.NtfyServerURL), "/")
	method := strings.ToUpper(strings.TrimSpace(channels.WebhookMethod))
	if method == "" {
		method = "POST"
	}
	channels.WebhookMethod = method
	return channels
}

func isSupportedChannel(channelType string) bool {
	for _, supported := range SupportedChannelTypes {
		if supported == channelType {
			return true
		}
	}
	return false
}

func setFromSlice(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}

func clampInt(value int, min int, max int, fallback int) int {
	if value == 0 {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func twoDigit(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

func appendPath(rawURL string, parts ...string) string {
	base := strings.TrimRight(rawURL, "/")
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, url.PathEscape(part))
	}
	return base + "/" + strings.Join(escaped, "/")
}
