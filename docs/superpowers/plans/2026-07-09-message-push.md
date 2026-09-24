# Message Push Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Farm_Go message push module with persisted config/state, single-channel HTTP delivery, test/daily send actions, scheduled daily checks, and the approved A-style control-room UI with independent message type switches.

**Architecture:** Add a focused Go package `internal/messagepush` for config normalization, channel selection, payload sending, retry handling, recent push state, and due daily checks. Store config/state JSON through `internal/storage`, expose Wails methods from `app.go`, then replace the current stub React view with a dense control-room interface that talks only to Wails APIs.

**Tech Stack:** Go 1.25, Wails v2 bindings, SQLite-backed `settings`, React 18, Vitest server-rendered component tests, lucide-react icons.

---

## File Structure

- Create `internal/messagepush/types.go`: public config/state/payload/result types, channel metadata, and defaults.
- Create `internal/messagepush/config.go`: normalization, channel configuration detection, clock parsing, and `nextRunAt` calculation.
- Create `internal/messagepush/service.go`: `Service`, state assembly, save config, test send, daily test, due daily check, recent push persistence.
- Create `internal/messagepush/sender.go`: per-channel HTTP request construction and retry loop.
- Create `internal/messagepush/messagepush_test.go`: package tests for normalization, channel selection, retry, state retention, header validation, and daily once-only behavior.
- Modify `internal/storage/settings.go`: add JSON helpers for `messagePush.config` and `messagePush.state`.
- Modify `internal/storage/storage_test.go`: add message push config/state round-trip tests.
- Modify `app.go`: add `messagePush` service field, startup/shutdown lifecycle, Wails methods, and event logging.
- Modify `app_test.go`: add App-level tests for save/test/daily actions using a local `httptest.Server`.
- Modify `frontend/src/views/MessagePushView.tsx`: replace the current stub with the new UI.
- Create `frontend/src/views/MessagePushView.test.tsx`: render and behavior-facing tests for the six switches and action buttons.
- Modify `frontend/src/App.tsx`: pass no extra props; it will import new Wails methods from generated bindings.
- Modify `frontend/wailsjs/go/main/App.js`, `frontend/wailsjs/go/main/App.d.ts`: add generated-style stubs if `wails generate module` is not available in the environment.
- Modify `frontend/wailsjs/go/models.ts`: add `messagepush` namespace if generation does not update it.
- Modify `frontend/src/style.css`: add scoped message-push layout and control styles.

## Task 1: Storage Round Trip

**Files:**
- Modify: `internal/storage/settings.go`
- Modify: `internal/storage/storage_test.go`

- [ ] **Step 1: Write failing storage tests**

Add this test to `internal/storage/storage_test.go`:

```go
func TestStoreMessagePushJSONRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	config := `{"enabled":true,"selectedChannels":["webhook"],"channels":{"webhookUrl":"https://example.test/hook"}}`
	if err := store.SaveMessagePushConfigJSON(ctx, config); err != nil {
		t.Fatalf("save message push config: %v", err)
	}
	gotConfig, err := store.LoadMessagePushConfigJSON(ctx)
	if err != nil {
		t.Fatalf("load message push config: %v", err)
	}
	if gotConfig != config {
		t.Fatalf("config mismatch: %s", gotConfig)
	}

	state := `{"recentPushes":[{"kind":"test","title":"农场测试推送","ok":true}]}`
	if err := store.SaveMessagePushStateJSON(ctx, state); err != nil {
		t.Fatalf("save message push state: %v", err)
	}
	gotState, err := store.LoadMessagePushStateJSON(ctx)
	if err != nil {
		t.Fatalf("load message push state: %v", err)
	}
	if gotState != state {
		t.Fatalf("state mismatch: %s", gotState)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run: `go test ./internal/storage -run TestStoreMessagePushJSONRoundTrip -count=1`

Expected: FAIL with `store.SaveMessagePushConfigJSON undefined`.

- [ ] **Step 3: Implement storage helpers**

Add these methods to `internal/storage/settings.go` below `SaveRuntimeSettings`:

```go
func (s *Store) LoadMessagePushConfigJSON(ctx context.Context) (string, error) {
	values, err := s.loadSettings(ctx, []string{"messagePush.config"})
	if err != nil {
		return "", err
	}
	return values["messagePush.config"], nil
}

func (s *Store) SaveMessagePushConfigJSON(ctx context.Context, raw string) error {
	return s.saveSetting(ctx, "messagePush.config", raw)
}

func (s *Store) LoadMessagePushStateJSON(ctx context.Context) (string, error) {
	values, err := s.loadSettings(ctx, []string{"messagePush.state"})
	if err != nil {
		return "", err
	}
	return values["messagePush.state"], nil
}

func (s *Store) SaveMessagePushStateJSON(ctx context.Context, raw string) error {
	return s.saveSetting(ctx, "messagePush.state", raw)
}

func (s *Store) saveSetting(ctx context.Context, key string, value string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, now)
	return err
}
```

Refactor the loop in `SaveRuntimeSettings` to call `s.saveSetting(ctx, key, value)`.

- [ ] **Step 4: Run storage tests**

Run: `go test ./internal/storage -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/storage/settings.go internal/storage/storage_test.go
git commit -m "feat: persist message push settings"
```

## Task 2: Message Push Core Types and Config

**Files:**
- Create: `internal/messagepush/types.go`
- Create: `internal/messagepush/config.go`
- Create: `internal/messagepush/messagepush_test.go`

- [ ] **Step 1: Write failing config tests**

Create `internal/messagepush/messagepush_test.go` with:

```go
package messagepush

import (
	"reflect"
	"testing"
	"time"
)

func TestDefaultConfigEnablesExistingAbnormalSubtypes(t *testing.T) {
	got := NormalizeConfig(Config{})
	if got.AbnormalEnabled != true || got.SuspectedEnabled != true || got.RecoveryEnabled != true {
		t.Fatalf("expected abnormal subtype defaults enabled: %#v", got)
	}
	if got.DailyTime != "09:00" || got.HTTPTimeoutMS != 10000 || got.PushRetryCount != 3 {
		t.Fatalf("unexpected defaults: %#v", got)
	}
}

func TestNormalizeConfigClampsNumbersAndChannels(t *testing.T) {
	got := NormalizeConfig(Config{
		DailyTime:                 "7:5",
		AbnormalTimeoutThreshold:  -1,
		LogScanIntervalSec:        1,
		HTTPTimeoutMS:             250,
		PushRetryCount:            12,
		SelectedChannels:          []string{"webhook", "webhook", "unknown", "dingtalk"},
		ChannelFormats:            map[string]string{"dingtalk": "text", "wecom": "bogus"},
		Channels:                  Channels{WebhookMethod: "patch"},
	})
	if got.DailyTime != "07:05" {
		t.Fatalf("daily time mismatch: %q", got.DailyTime)
	}
	if got.AbnormalTimeoutThreshold != 1 || got.LogScanIntervalSec != 5 || got.HTTPTimeoutMS != 1000 || got.PushRetryCount != 10 {
		t.Fatalf("clamp mismatch: %#v", got)
	}
	if !reflect.DeepEqual(got.SelectedChannels, []string{"webhook"}) {
		t.Fatalf("selected channel mismatch: %#v", got.SelectedChannels)
	}
	if got.ChannelFormats["dingtalk"] != "text" || got.ChannelFormats["wecom"] != "text" {
		t.Fatalf("format mismatch: %#v", got.ChannelFormats)
	}
	if got.Channels.WebhookMethod != "PATCH" {
		t.Fatalf("method mismatch: %q", got.Channels.WebhookMethod)
	}
}

func TestPickAvailableChannelsUsesConfiguredFallback(t *testing.T) {
	config := NormalizeConfig(Config{
		SelectedChannels: []string{"serverchan"},
		Channels: Channels{
			WebhookURL: "https://example.test/hook",
		},
	})
	got := PickAvailableChannels(config, nil)
	if !reflect.DeepEqual(got, []string{"webhook"}) {
		t.Fatalf("fallback mismatch: %#v", got)
	}

	requested := PickAvailableChannels(config, []string{"serverchan"})
	if len(requested) != 0 {
		t.Fatalf("requested unavailable channel should not fallback: %#v", requested)
	}
}

func TestNextDailyRunAtIsDueNowWhenEnabledAndUnsent(t *testing.T) {
	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.Local)
	config := NormalizeConfig(Config{Enabled: true, DailyEnabled: true, DailyTime: "09:00"})
	state := State{}
	next := NextDailyRunAt(config, state, now)
	if next == nil || !next.Equal(now) {
		t.Fatalf("expected due now, got %v", next)
	}
}
```

- [ ] **Step 2: Run failing config tests**

Run: `go test ./internal/messagepush -run 'TestDefaultConfig|TestNormalizeConfig|TestPickAvailableChannels|TestNextDailyRunAt' -count=1`

Expected: FAIL because the package/files do not exist.

- [ ] **Step 3: Implement `types.go`**

Create `internal/messagepush/types.go`:

```go
package messagepush

type Config struct {
	Enabled                  bool              `json:"enabled"`
	AbnormalEnabled          bool              `json:"abnormalEnabled"`
	SuspectedEnabled         bool              `json:"suspectedEnabled"`
	RecoveryEnabled          bool              `json:"recoveryEnabled"`
	DailyEnabled             bool              `json:"dailyEnabled"`
	DailyMarkdownCardEnabled bool              `json:"dailyMarkdownCardEnabled"`
	RestartEnabled           bool              `json:"restartEnabled"`
	DailyTime                string            `json:"dailyTime"`
	LogMonitorEnabled        bool              `json:"logMonitorEnabled"`
	AbnormalTimeoutThreshold int               `json:"abnormalTimeoutThreshold"`
	LogScanIntervalSec       int               `json:"logScanIntervalSec"`
	HTTPTimeoutMS            int               `json:"httpTimeoutMs"`
	PushRetryCount           int               `json:"pushRetryCount"`
	SelectedChannels         []string          `json:"selectedChannels"`
	ChannelFormats           map[string]string `json:"channelFormats"`
	Channels                 Channels          `json:"channels"`
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
	LastDailySummaryDateKey   string       `json:"lastDailySummaryDateKey"`
	LastDailySummaryAt        string       `json:"lastDailySummaryAt"`
	LastDailySummaryCheckAt   string       `json:"lastDailySummaryCheckAt"`
	LastDailySummarySkipReason string      `json:"lastDailySummarySkipReason"`
	RecentPushes             []PushRecord `json:"recentPushes"`
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

type DailyState struct {
	Enabled          bool     `json:"enabled"`
	Channels         []string `json:"channels"`
	Time             string   `json:"time"`
	NextRunAt        string   `json:"nextRunAt,omitempty"`
	LastSummaryDateKey string `json:"lastSummaryDateKey,omitempty"`
	LastSummaryAt    string   `json:"lastSummaryAt,omitempty"`
	LastCheckedAt    string   `json:"lastCheckedAt,omitempty"`
	LastSkipReason   string   `json:"lastSkipReason,omitempty"`
}

type ViewState struct {
	Enabled            bool          `json:"enabled"`
	Config             Config        `json:"config"`
	ConfiguredChannels []string      `json:"configuredChannels"`
	Channels           []ChannelMeta `json:"channels"`
	Daily              DailyState    `json:"daily"`
	RecentPushes       []PushRecord  `json:"recentPushes"`
	LogMonitorEnabled  bool          `json:"logMonitorEnabled"`
}
```

- [ ] **Step 4: Implement `config.go`**

Create `internal/messagepush/config.go` with supported channel constants, `NormalizeConfig`, `ConfiguredChannelTypes`, `PickAvailableChannels`, `BuildChannelMetaList`, `NextDailyRunAt`, `TodayKey`, and `ShiftDateKey`.

Key implementation requirements:

```go
var SupportedChannelTypes = []string{"serverchan", "pushplus", "qmsg", "wecom", "dingtalk", "feishu", "telegram", "bark", "ntfy", "webhook"}

var ChannelLabels = map[string]string{
	"serverchan": "Server酱",
	"pushplus": "PushPlus",
	"qmsg": "Qmsg酱",
	"wecom": "企业微信",
	"dingtalk": "钉钉",
	"feishu": "飞书",
	"telegram": "Telegram",
	"bark": "Bark",
	"ntfy": "ntfy",
	"webhook": "Webhook",
}
```

Normalization must:

- default `AbnormalEnabled`, `SuspectedEnabled`, `RecoveryEnabled`, `RestartEnabled`, and `LogMonitorEnabled` to `true` when false-value config is missing.
- normalize `DailyTime` to `HH:mm`, fallback `09:00`.
- clamp threshold `1..99`, scan interval `5..3600`, HTTP timeout `1000..120000`, retry count `1..10`.
- force `SelectedChannels` to at most one valid unique channel.
- default `BarkServerURL=https://api.day.app`, `NtfyServerURL=https://ntfy.sh`, `WebhookMethod=POST`.

- [ ] **Step 5: Run config tests**

Run: `go test ./internal/messagepush -run 'TestDefaultConfig|TestNormalizeConfig|TestPickAvailableChannels|TestNextDailyRunAt' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/messagepush/types.go internal/messagepush/config.go internal/messagepush/messagepush_test.go
git commit -m "feat: add message push config model"
```

## Task 3: Sender and Service

**Files:**
- Modify: `internal/messagepush/messagepush_test.go`
- Create: `internal/messagepush/sender.go`
- Create: `internal/messagepush/service.go`

- [ ] **Step 1: Add failing sender/service tests**

Append tests covering:

```go
func TestSendTestRetriesUntilWebhookSucceeds(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := NewMemoryStore()
	service := NewService(ServiceOptions{Store: store, HTTPClient: server.Client()})
	result, err := service.SendTest(context.Background(), Config{
		Enabled: true,
		SelectedChannels: []string{"webhook"},
		Channels: Channels{WebhookURL: server.URL},
		PushRetryCount: 3,
	})
	if err != nil {
		t.Fatalf("send test: %v", err)
	}
	if !result.OK || attempts != 2 {
		t.Fatalf("expected retry success, attempts=%d result=%#v", attempts, result)
	}
	if len(store.State.RecentPushes) != 1 || !store.State.RecentPushes[0].OK {
		t.Fatalf("recent push not recorded: %#v", store.State)
	}
}

func TestWebhookHeadersRejectInvalidJSON(t *testing.T) {
	service := NewService(ServiceOptions{Store: NewMemoryStore()})
	_, err := service.SendTest(context.Background(), Config{
		Enabled: true,
		SelectedChannels: []string{"webhook"},
		Channels: Channels{WebhookURL: "https://example.test/hook", WebhookHeaders: "{bad"},
	})
	if err == nil || !strings.Contains(err.Error(), "请求头 JSON 无效") {
		t.Fatalf("expected invalid headers error, got %v", err)
	}
}

func TestRunDueDailySendsOnlyOncePerTargetDate(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := NewMemoryStore()
	service := NewService(ServiceOptions{
		Store: store,
		HTTPClient: server.Client(),
		Now: func() time.Time { return time.Date(2026, 7, 9, 10, 0, 0, 0, time.Local) },
	})
	config := NormalizeConfig(Config{
		Enabled: true,
		DailyEnabled: true,
		DailyTime: "09:00",
		SelectedChannels: []string{"webhook"},
		Channels: Channels{WebhookURL: server.URL},
	})
	if err := service.SaveConfig(context.Background(), config); err != nil {
		t.Fatalf("save config: %v", err)
	}
	first, err := service.RunDueDaily(context.Background())
	if err != nil {
		t.Fatalf("run due daily first: %v", err)
	}
	second, err := service.RunDueDaily(context.Background())
	if err != nil {
		t.Fatalf("run due daily second: %v", err)
	}
	if !first.OK || second.OK || second.Summary != "already_sent" || attempts != 1 {
		t.Fatalf("daily once mismatch attempts=%d first=%#v second=%#v", attempts, first, second)
	}
}
```

Also define a test-only `MemoryStore` in the test file that implements the service store interface.

- [ ] **Step 2: Run failing tests**

Run: `go test ./internal/messagepush -run 'TestSendTest|TestWebhookHeaders|TestRunDueDaily' -count=1`

Expected: FAIL because `Service`, `NewService`, and `NewMemoryStore` are not defined.

- [ ] **Step 3: Implement sender**

Create `internal/messagepush/sender.go`:

- `sendByChannel(ctx, client, config, channelType, payload) error`
- Implement `webhook`, `serverchan`, `pushplus`, `wecom`, `dingtalk`, `feishu`, `telegram`, `bark`, `ntfy`, `qmsg`.
- Use JSON bodies for webhook-style services.
- Parse `WebhookHeaders` as JSON object and apply string values.
- Treat HTTP status outside `200..299` as error.

- [ ] **Step 4: Implement service**

Create `internal/messagepush/service.go`:

- Define:

```go
type Store interface {
	LoadMessagePushConfigJSON(context.Context) (string, error)
	SaveMessagePushConfigJSON(context.Context, string) error
	LoadMessagePushStateJSON(context.Context) (string, error)
	SaveMessagePushStateJSON(context.Context, string) error
}

type ServiceOptions struct {
	Store Store
	HTTPClient *http.Client
	Now func() time.Time
}
```

- Implement:
  - `NewService(options ServiceOptions) *Service`
  - `State(ctx context.Context) (ViewState, error)`
  - `SaveConfig(ctx context.Context, config Config) (ViewState, error)`
  - `SendTest(ctx context.Context, config Config) (SendResult, error)`
  - `SendDailyNow(ctx context.Context, config Config) (SendResult, error)`
  - `RunDueDaily(ctx context.Context) (SendResult, error)`
  - recent push retention at 20 records.

- [ ] **Step 5: Run messagepush tests**

Run: `go test ./internal/messagepush -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/messagepush
git commit -m "feat: add message push service"
```

## Task 4: App Wails API and Scheduler

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`

- [ ] **Step 1: Write failing App API tests**

Add tests to `app_test.go`:

```go
func TestAppMessagePushSaveAndState(t *testing.T) {
	app := NewApp()
	ctx := context.Background()
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store
	app.ensureMessagePushService()

	state := app.SaveMessagePushConfig(messagepush.Config{
		Enabled: true,
		SelectedChannels: []string{"webhook"},
		Channels: messagepush.Channels{WebhookURL: "https://example.test/hook"},
	})
	if !state.Config.Enabled || state.Config.SelectedChannels[0] != "webhook" {
		t.Fatalf("unexpected saved state: %#v", state)
	}
	loaded := app.MessagePushState()
	if !loaded.Config.Enabled || loaded.ConfiguredChannels[0] != "webhook" {
		t.Fatalf("unexpected loaded state: %#v", loaded)
	}
}
```

Add a second App test using `httptest.Server` for `SendMessagePushTest`.

- [ ] **Step 2: Run failing App tests**

Run: `go test . -run TestAppMessagePush -count=1`

Expected: FAIL because App methods and fields are missing.

- [ ] **Step 3: Wire service into App**

Modify `app.go`:

- Add import `Farm_Go/internal/messagepush`.
- Add fields:

```go
messagePushMu sync.Mutex
messagePush *messagepush.Service
messagePushCancel context.CancelFunc
messagePushHTTPClient *http.Client
```

- Add helper:

```go
func (a *App) ensureMessagePushService() *messagepush.Service {
	a.messagePushMu.Lock()
	defer a.messagePushMu.Unlock()
	if a.messagePush != nil {
		return a.messagePush
	}
	a.messagePush = messagepush.NewService(messagepush.ServiceOptions{
		Store: a.store,
		HTTPClient: a.messagePushHTTPClient,
	})
	return a.messagePush
}
```

- Start daily ticker in `startup` after `a.store = store`.
- Stop ticker in `shutdown`.
- Add Wails methods:

```go
func (a *App) MessagePushState() messagepush.ViewState
func (a *App) SaveMessagePushConfig(config messagepush.Config) messagepush.ViewState
func (a *App) SendMessagePushTest(config messagepush.Config) messagepush.SendResult
func (a *App) SendMessagePushDailyTest(config messagepush.Config) messagepush.SendResult
func (a *App) RunDueMessagePushDaily() messagepush.SendResult
```

Each method records `eventbus.Event` with source `message_push`.

- [ ] **Step 4: Run App tests**

Run: `go test . -run TestAppMessagePush -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add app.go app_test.go
git commit -m "feat: expose message push APIs"
```

## Task 5: Frontend Bindings and View Tests

**Files:**
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/main/App.d.ts`
- Modify: `frontend/wailsjs/go/models.ts`
- Create: `frontend/src/views/MessagePushView.test.tsx`
- Modify: `frontend/src/views/MessagePushView.tsx`

- [ ] **Step 1: Add failing frontend tests**

Create `frontend/src/views/MessagePushView.test.tsx`:

```tsx
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { MessagePushView } from './MessagePushView';

vi.mock('../../wailsjs/go/main/App', () => ({
  MessagePushState: vi.fn(() => Promise.resolve({
    config: {
      enabled: true,
      abnormalEnabled: true,
      suspectedEnabled: true,
      recoveryEnabled: true,
      restartEnabled: true,
      dailyEnabled: false,
      logMonitorEnabled: true,
      dailyTime: '09:00',
      selectedChannels: ['webhook'],
      channelFormats: {},
      channels: { webhookUrl: 'https://example.test/hook', webhookMethod: 'POST' },
    },
    configuredChannels: ['webhook'],
    channels: [{ type: 'webhook', label: 'Webhook', configured: true }],
    daily: { enabled: false, time: '09:00' },
    recentPushes: [],
    logMonitorEnabled: true,
  })),
  SaveMessagePushConfig: vi.fn(),
  SendMessagePushTest: vi.fn(),
  SendMessagePushDailyTest: vi.fn(),
}));

describe('MessagePushView', () => {
  it('renders the six independent message type switches', () => {
    const html = renderToStaticMarkup(<MessagePushView />);
    expect(html).toContain('异常告警');
    expect(html).toContain('疑似异常');
    expect(html).toContain('恢复通知');
    expect(html).toContain('重启通知');
    expect(html).toContain('资产日报');
    expect(html).toContain('日志监控');
  });

  it('renders channel configuration and test actions', () => {
    const html = renderToStaticMarkup(<MessagePushView />);
    expect(html).toContain('推送渠道');
    expect(html).toContain('测试推送');
    expect(html).toContain('日报测试');
    expect(html).not.toContain('待迁移');
  });
});
```

- [ ] **Step 2: Run failing frontend test**

Run: `cd frontend; npm test -- MessagePushView`

Expected: FAIL because the current stub lacks the new labels and mocked Wails methods are not imported.

- [ ] **Step 3: Add Wails binding stubs**

If `wails generate module` is available, run it. If not, add generated-style methods:

`frontend/wailsjs/go/main/App.js`:

```js
export function MessagePushState() {
  return window['go']['main']['App']['MessagePushState']();
}
export function SaveMessagePushConfig(arg1) {
  return window['go']['main']['App']['SaveMessagePushConfig'](arg1);
}
export function SendMessagePushTest(arg1) {
  return window['go']['main']['App']['SendMessagePushTest'](arg1);
}
export function SendMessagePushDailyTest(arg1) {
  return window['go']['main']['App']['SendMessagePushDailyTest'](arg1);
}
export function RunDueMessagePushDaily() {
  return window['go']['main']['App']['RunDueMessagePushDaily']();
}
```

`frontend/wailsjs/go/main/App.d.ts` should export matching `Promise<any>` signatures or generated `messagepush` types.

- [ ] **Step 4: Replace MessagePushView**

Implement `MessagePushView.tsx` with:

- local `draft` state seeded from `MessagePushState`.
- six type cards mapped to keys `abnormalEnabled`, `suspectedEnabled`, `recoveryEnabled`, `restartEnabled`, `dailyEnabled`, `logMonitorEnabled`.
- single-channel selector buttons.
- channel-specific field groups for all supported channels.
- advanced numeric/time fields.
- `SaveMessagePushConfig`, `SendMessagePushTest`, and `SendMessagePushDailyTest` handlers.
- recent push list.

- [ ] **Step 5: Run frontend test**

Run: `cd frontend; npm test -- MessagePushView`

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add frontend/src/views/MessagePushView.tsx frontend/src/views/MessagePushView.test.tsx frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/models.ts
git commit -m "feat: build message push view"
```

## Task 6: Styling, Full Verification, and Build

**Files:**
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Add scoped CSS**

Add styles for:

- `.message-push-view`
- `.message-push-type-grid`
- `.message-push-type-card`
- `.message-push-type-card.on`
- `.message-push-layout`
- `.message-push-channel-grid`
- `.message-push-field-grid`
- `.message-push-recent-list`

Keep cards at 8px radius or less and avoid nested cards.

- [ ] **Step 2: Run Go tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Run frontend tests**

Run: `cd frontend; npm test`

Expected: PASS.

- [ ] **Step 4: Build frontend**

Run: `cd frontend; npm run build`

Expected: PASS and Vite emits a new `frontend/dist/assets/index-*.js` and `index-*.css`.

- [ ] **Step 5: Confirm generated asset reference changed**

Run: `cd frontend; Get-Item dist/index.html; Select-String -Path dist/index.html -Pattern '/assets/index-'`

Expected: `dist/index.html` references the newly generated asset names.

- [ ] **Step 6: Final status**

Run: `git status --short`

Expected: only intentional source changes plus the pre-existing `frontend-vite.out.log` if it remains dirty.

- [ ] **Step 7: Commit**

```powershell
git add frontend/src/style.css
git commit -m "style: polish message push control room"
```

## Self-Review

- Spec coverage: tasks cover storage, config/state, single-channel sends, retry, tests, Wails APIs, scheduled daily checks, six type switches, test/daily actions, recent push records, and frontend build.
- Scope control: automatic task execution result notifications, full log scanning, multi-channel delivery, and complete historical daily statistics remain out of scope.
- Type consistency: config keys match the approved spec: `abnormalEnabled`, `suspectedEnabled`, `recoveryEnabled`, `restartEnabled`, `dailyEnabled`, `logMonitorEnabled`, `dailyMarkdownCardEnabled`, `selectedChannels`, `channelFormats`, and `channels`.
- Red-flag scan: the plan avoids unfinished markers and vague implementation gaps. The only fallback is Wails binding generation when the local generator is unavailable, with exact manual stubs provided.
