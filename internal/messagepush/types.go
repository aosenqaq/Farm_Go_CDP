package messagepush

type MessageType string

const (
	MessageTypeAbnormal   MessageType = "abnormal"
	MessageTypeSuspected  MessageType = "suspected"
	MessageTypeRecovery   MessageType = "recovery"
	MessageTypeRestart    MessageType = "restart"
	MessageTypeDaily      MessageType = "daily"
	MessageTypeLogMonitor MessageType = "log_monitor"
)

type TemplateMode string

const (
	TemplateModeText     TemplateMode = "text"
	TemplateModeMarkdown TemplateMode = "markdown"
	TemplateModeCard     TemplateMode = "card"
	TemplateModeJSON     TemplateMode = "json"
)

type Template struct {
	Enabled bool         `json:"enabled"`
	Mode    TemplateMode `json:"mode"`
	Content string       `json:"content"`
}

type LogMonitorRule struct {
	Enabled bool   `json:"enabled"`
	Source  string `json:"source"`
	Type    string `json:"type"`
	Keyword string `json:"keyword"`
}

type DedupeRecord struct {
	FirstAt string `json:"firstAt"`
	LastAt  string `json:"lastAt"`
	Count   int    `json:"count"`
}

type ChannelMode struct {
	Channel string         `json:"channel"`
	Modes   []TemplateMode `json:"modes"`
}

type TemplateVariable struct {
	Path        string `json:"path"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

type TemplateDefinition struct {
	Type         MessageType        `json:"type"`
	Label        string             `json:"label"`
	ChannelModes []ChannelMode      `json:"channelModes"`
	Variables    []TemplateVariable `json:"variables"`
}

type Config struct {
	Enabled                  bool                                `json:"enabled"`
	AbnormalEnabled          bool                                `json:"abnormalEnabled"`
	SuspectedEnabled         bool                                `json:"suspectedEnabled"`
	RecoveryEnabled          bool                                `json:"recoveryEnabled"`
	DailyEnabled             bool                                `json:"dailyEnabled"`
	DailyMarkdownCardEnabled bool                                `json:"dailyMarkdownCardEnabled"`
	RestartEnabled           bool                                `json:"restartEnabled"`
	DailyTime                string                              `json:"dailyTime"`
	LogMonitorEnabled        bool                                `json:"logMonitorEnabled"`
	AbnormalTimeoutThreshold int                                 `json:"abnormalTimeoutThreshold"`
	LogScanIntervalSec       int                                 `json:"logScanIntervalSec"`
	HTTPTimeoutMS            int                                 `json:"httpTimeoutMs"`
	PushRetryCount           int                                 `json:"pushRetryCount"`
	SelectedChannels         []string                            `json:"selectedChannels"`
	ChannelFormats           map[string]string                   `json:"channelFormats"`
	Channels                 Channels                            `json:"channels"`
	Templates                map[MessageType]map[string]Template `json:"templates"`
	LogMonitorRules          []LogMonitorRule                    `json:"logMonitorRules"`
}

type Channels struct {
	ServerChanSendKey string `json:"serverChanSendKey"`
	PushPlusToken     string `json:"pushPlusToken"`
	QmsgKey           string `json:"qmsgKey"`
	QmsgType          string `json:"qmsgType"`
	QmsgTarget        string `json:"qmsgTarget"`
	WecomWebhook      string `json:"wecomWebhook"`
	DingtalkWebhook   string `json:"dingtalkWebhook"`
	DingtalkSecret    string `json:"dingtalkSecret"`
	FeishuWebhook     string `json:"feishuWebhook"`
	TelegramBotToken  string `json:"telegramBotToken"`
	TelegramChatID    string `json:"telegramChatId"`
	BarkServerURL     string `json:"barkServerUrl"`
	BarkDeviceKey     string `json:"barkDeviceKey"`
	NtfyServerURL     string `json:"ntfyServerUrl"`
	NtfyTopic         string `json:"ntfyTopic"`
	WebhookURL        string `json:"webhookUrl"`
	WebhookMethod     string `json:"webhookMethod"`
	WebhookHeaders    string `json:"webhookHeaders"`
}

type ChannelMeta struct {
	Type       string `json:"type"`
	Label      string `json:"label"`
	Configured bool   `json:"configured"`
}

type State struct {
	LastDailySummaryDateKey    string                  `json:"lastDailySummaryDateKey"`
	LastDailySummaryAt         string                  `json:"lastDailySummaryAt"`
	LastDailySummaryCheckAt    string                  `json:"lastDailySummaryCheckAt"`
	LastDailySummarySkipReason string                  `json:"lastDailySummarySkipReason"`
	RecentPushes               []PushRecord            `json:"recentPushes"`
	RecentDedupe               map[string]DedupeRecord `json:"recentDedupe"`
}

type PushRecord struct {
	Time     string   `json:"time"`
	Kind     string   `json:"kind"`
	Title    string   `json:"title"`
	OK       bool     `json:"ok"`
	Channels []string `json:"channels"`
	Error    string   `json:"error,omitempty"`
}

type Payload struct {
	Kind  string         `json:"kind"`
	Title string         `json:"title"`
	Lines []string       `json:"lines"`
	Meta  map[string]any `json:"meta,omitempty"`
}

type ChannelResult struct {
	Type     string `json:"type"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Attempts int    `json:"attempts"`
}

type SendResult struct {
	OK      bool            `json:"ok"`
	Summary string          `json:"summary"`
	Results []ChannelResult `json:"results"`
	Payload Payload         `json:"payload"`
}

type TemplatePreview struct {
	OK          bool            `json:"ok"`
	Error       string          `json:"error,omitempty"`
	MessageType MessageType     `json:"messageType"`
	Channel     string          `json:"channel"`
	Rendered    RenderedPayload `json:"rendered"`
}

type DailyState struct {
	Enabled            bool     `json:"enabled"`
	Channels           []string `json:"channels"`
	Time               string   `json:"time"`
	NextRunAt          string   `json:"nextRunAt,omitempty"`
	LastSummaryDateKey string   `json:"lastSummaryDateKey,omitempty"`
	LastSummaryAt      string   `json:"lastSummaryAt,omitempty"`
	LastCheckedAt      string   `json:"lastCheckedAt,omitempty"`
	LastSkipReason     string   `json:"lastSkipReason,omitempty"`
}

type ViewState struct {
	Enabled            bool                 `json:"enabled"`
	Config             Config               `json:"config"`
	ConfiguredChannels []string             `json:"configuredChannels"`
	Channels           []ChannelMeta        `json:"channels"`
	Daily              DailyState           `json:"daily"`
	RecentPushes       []PushRecord         `json:"recentPushes"`
	LogMonitorEnabled  bool                 `json:"logMonitorEnabled"`
	TemplateCatalog    []TemplateDefinition `json:"templateCatalog"`
}
