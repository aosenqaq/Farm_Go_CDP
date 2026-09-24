package messagepush

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
		DailyTime:                "7:5",
		AbnormalTimeoutThreshold: -1,
		LogScanIntervalSec:       1,
		HTTPTimeoutMS:            250,
		PushRetryCount:           12,
		SelectedChannels:         []string{"webhook", "webhook", "unknown", "dingtalk"},
		ChannelFormats:           map[string]string{"dingtalk": "text", "wecom": "bogus"},
		Channels:                 Channels{WebhookMethod: "patch"},
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

func TestTodayKeyUsesBeijingDate(t *testing.T) {
	// 2026-07-11 22:30 UTC is already 2026-07-12 in Beijing.
	now := time.Date(2026, 7, 11, 22, 30, 0, 0, time.UTC)
	if got := TodayKey(now); got != "2026-07-12" {
		t.Fatalf("TodayKey=%q", got)
	}
	if got := formatBeijingTime(now); got != "2026-07-12 06:30:00" {
		t.Fatalf("formatBeijingTime=%q", got)
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
		Enabled:          true,
		SelectedChannels: []string{"webhook"},
		Channels:         Channels{WebhookURL: server.URL},
		PushRetryCount:   3,
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

func TestSendEndpointsRejectInvalidRawCardTemplateBeforeDelivery(t *testing.T) {
	for _, send := range []struct {
		name string
		call func(*Service, context.Context, Config) (SendResult, error)
	}{
		{"test", (*Service).SendTest},
		{"daily", (*Service).SendDailyNow},
	} {
		t.Run(send.name, func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			service := NewService(ServiceOptions{HTTPClient: server.Client()})
			_, err := send.call(service, context.Background(), Config{
				Enabled:          true,
				SelectedChannels: []string{"wecom"},
				Channels:         Channels{WecomWebhook: server.URL},
				Templates: map[MessageType]map[string]Template{
					MessageTypeDaily: {
						"wecom": {Enabled: true, Mode: TemplateModeCard, Content: `{"title":"missing envelope"}`},
					},
				},
			})
			if err == nil || !strings.Contains(err.Error(), "卡片") {
				t.Fatalf("err=%v", err)
			}
			if attempts != 0 {
				t.Fatalf("invalid config made %d requests", attempts)
			}
		})
	}
}

func TestSendEndpointsUseConfiguredAccountTemplateContext(t *testing.T) {
	for _, send := range []struct {
		name string
		call func(*Service, context.Context, Config) (SendResult, error)
	}{
		{"test", (*Service).SendTest},
		{"daily", (*Service).SendDailyNow},
	} {
		t.Run(send.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			service := NewService(ServiceOptions{HTTPClient: server.Client(), AccountKey: "gid:10001", AccountGID: "10001"})
			_, err := send.call(service, context.Background(), Config{
				Enabled:          true,
				SelectedChannels: []string{"webhook"},
				Channels:         Channels{WebhookURL: server.URL},
				Templates: map[MessageType]map[string]Template{
					MessageTypeDaily: {
						"webhook": {Enabled: true, Mode: TemplateModeJSON, Content: `{"key":"{{account.key}}","gid":"{{account.gid}}"}`},
					},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if body["key"] != "gid:10001" || body["gid"] != "10001" {
				t.Fatalf("body=%#v", body)
			}
		})
	}
}

func TestSendDailyNowUsesLiveReportProviderValues(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := NewService(ServiceOptions{
		HTTPClient: server.Client(),
		AccountKey: "gid:10001",
		AccountGID: "10001",
		Now:        func() time.Time { return time.Date(2026, 7, 12, 9, 0, 0, 0, time.Local) },
		DailyReportProvider: func(context.Context, time.Time) (DailyReport, error) {
			return DailyReport{
				Summary: "真实运行数据",
				Values: map[string]any{
					"name":              "Dpo.L",
					"gold":              3891552777,
					"bean":              455521,
					"warehouseEstimate": 115998840,
					"sellAmount":        10336,
					"sellCount":         24,
				},
			}, nil
		},
	})
	_, err := service.SendDailyNow(context.Background(), Config{
		Enabled:          true,
		DailyEnabled:     true,
		SelectedChannels: []string{"webhook"},
		Channels:         Channels{WebhookURL: server.URL},
		Templates: map[MessageType]map[string]Template{
			MessageTypeDaily: {
				"webhook": {Enabled: true, Mode: TemplateModeJSON, Content: `{"date":"{{daily.date}}","name":"{{daily.name}}","gold":{{daily.gold}},"bean":{{daily.bean}},"warehouseEstimate":{{daily.warehouseEstimate}},"sellAmount":{{daily.sellAmount}},"sellCount":{{daily.sellCount}},"summary":"{{daily.summary}}"}`},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if body["date"] != "2026-07-11" || body["name"] != "Dpo.L" || body["gold"] != float64(3891552777) || body["bean"] != float64(455521) || body["warehouseEstimate"] != float64(115998840) || body["sellAmount"] != float64(10336) || body["sellCount"] != float64(24) || body["summary"] != "真实运行数据" {
		t.Fatalf("daily body=%#v", body)
	}
}

func TestSendTestUsesEmptyAccountTemplateContextByDefault(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := NewService(ServiceOptions{HTTPClient: server.Client()})
	_, err := service.SendTest(context.Background(), Config{
		SelectedChannels: []string{"webhook"},
		Channels:         Channels{WebhookURL: server.URL},
		Templates: map[MessageType]map[string]Template{
			MessageTypeDaily: {
				"webhook": {Enabled: true, Mode: TemplateModeJSON, Content: `{"key":"{{account.key}}","gid":"{{account.gid}}"}`},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if body["key"] != "" || body["gid"] != "" {
		t.Fatalf("body=%#v", body)
	}
}

func TestSendEndpointsRetainLegacyTitleInBarkTextEnvelope(t *testing.T) {
	for _, send := range []struct {
		name    string
		title   string
		call    func(*Service, context.Context, Config) (SendResult, error)
		enabled bool
	}{
		{"test", "农场测试推送", (*Service).SendTest, false},
		{"daily", "农场资产日报", (*Service).SendDailyNow, true},
	} {
		t.Run(send.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			service := NewService(ServiceOptions{HTTPClient: server.Client()})
			_, err := send.call(service, context.Background(), Config{
				Enabled:          send.enabled,
				SelectedChannels: []string{"bark"},
				Channels:         Channels{BarkServerURL: server.URL, BarkDeviceKey: "device"},
				Templates: map[MessageType]map[string]Template{
					MessageTypeDaily: {
						"bark": {Enabled: true, Mode: TemplateModeText, Content: "{{payload.title}} body"},
					},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if body["title"] != send.title || body["body"] != send.title+" body" {
				t.Fatalf("body=%#v", body)
			}
		})
	}
}

func TestSendEndpointsRetainLegacyTitleInNtfyMarkdownEnvelope(t *testing.T) {
	for _, send := range []struct {
		name    string
		title   string
		call    func(*Service, context.Context, Config) (SendResult, error)
		enabled bool
	}{
		{"test", "农场测试推送", (*Service).SendTest, false},
		{"daily", "农场资产日报", (*Service).SendDailyNow, true},
	} {
		t.Run(send.name, func(t *testing.T) {
			var text string
			var title string
			var markdown string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				text = string(raw)
				title = r.Header.Get("Title")
				markdown = r.Header.Get("Markdown")
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			service := NewService(ServiceOptions{HTTPClient: server.Client()})
			_, err := send.call(service, context.Background(), Config{
				Enabled:          send.enabled,
				SelectedChannels: []string{"ntfy"},
				Channels:         Channels{NtfyServerURL: server.URL, NtfyTopic: "topic"},
				Templates: map[MessageType]map[string]Template{
					MessageTypeDaily: {
						"ntfy": {Enabled: true, Mode: TemplateModeMarkdown, Content: "**{{payload.title}}**"},
					},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if title != send.title || text != "**"+send.title+"**" || markdown != "yes" {
				t.Fatalf("title=%q text=%q markdown=%q", title, text, markdown)
			}
		})
	}
}

func TestSendTestDeliversSelectedRenderedTemplate(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	now := time.Date(2026, 7, 11, 8, 30, 0, 0, time.UTC)
	service := NewService(ServiceOptions{HTTPClient: server.Client(), Now: func() time.Time { return now }})
	_, err := service.SendTest(context.Background(), Config{
		SelectedChannels: []string{"webhook"},
		Channels:         Channels{WebhookURL: server.URL},
		Templates: map[MessageType]map[string]Template{
			MessageTypeDaily: {
				"webhook": {Enabled: true, Mode: TemplateModeJSON, Content: `{"kind":"{{event.type}}","date":"{{daily.date}}","summary":"{{daily.summary}}"}`},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if body["kind"] != "test" || body["date"] != "2026-07-11" || body["summary"] != "这是一条来自 Farm_Go 的测试消息。" {
		t.Fatalf("body=%#v", body)
	}
}

func TestSendDailyNowDeliversSelectedRenderedTemplate(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	now := time.Date(2026, 7, 11, 8, 30, 0, 0, time.UTC)
	service := NewService(ServiceOptions{HTTPClient: server.Client(), Now: func() time.Time { return now }})
	_, err := service.SendDailyNow(context.Background(), Config{
		Enabled:          true,
		SelectedChannels: []string{"webhook"},
		Channels:         Channels{WebhookURL: server.URL},
		Templates: map[MessageType]map[string]Template{
			MessageTypeDaily: {
				"webhook": {Enabled: true, Mode: TemplateModeJSON, Content: `{"kind":"{{event.type}}","date":"{{daily.date}}","summary":"{{daily.summary}}"}`},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if body["kind"] != "daily" || body["date"] != "2026-07-10" || body["summary"] != "今日推送模块已就绪。\n后续可接入实时资产统计。" {
		t.Fatalf("body=%#v", body)
	}
}

func TestSendDailyNowDeliversUnquotedNumericTemplatePlaceholder(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := NewService(ServiceOptions{HTTPClient: server.Client()})
	_, err := service.SendDailyNow(context.Background(), Config{
		Enabled:          true,
		SelectedChannels: []string{"webhook"},
		Channels:         Channels{WebhookURL: server.URL},
		Templates: map[MessageType]map[string]Template{
			MessageTypeDaily: {
				"webhook": {Enabled: true, Mode: TemplateModeJSON, Content: `{"count":{{event.count}}}`},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if body["count"] != float64(1) {
		t.Fatalf("body=%#v", body)
	}
}

func TestWebhookHeadersRejectInvalidJSON(t *testing.T) {
	service := NewService(ServiceOptions{Store: NewMemoryStore()})
	_, err := service.SendTest(context.Background(), Config{
		Enabled:          true,
		SelectedChannels: []string{"webhook"},
		Channels:         Channels{WebhookURL: "https://example.test/hook", WebhookHeaders: "{bad"},
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
		Store:      store,
		HTTPClient: server.Client(),
		Now:        func() time.Time { return time.Date(2026, 7, 9, 10, 0, 0, 0, time.Local) },
	})
	config := NormalizeConfig(Config{
		Enabled:          true,
		DailyEnabled:     true,
		DailyTime:        "09:00",
		SelectedChannels: []string{"webhook"},
		Channels:         Channels{WebhookURL: server.URL},
	})
	if _, err := service.SaveConfig(context.Background(), config); err != nil {
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

func TestSaveConfigPreservesDisabledMessageTypes(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(ServiceOptions{Store: store})
	state, err := service.SaveConfig(context.Background(), Config{
		Enabled:          true,
		DailyTime:        "09:00",
		SelectedChannels: []string{"webhook"},
		Channels:         Channels{WebhookURL: "https://example.test/hook"},
	})
	if err != nil {
		t.Fatalf("save config: %v", err)
	}
	if state.Config.AbnormalEnabled || state.Config.SuspectedEnabled || state.Config.RecoveryEnabled || state.Config.RestartEnabled || state.Config.LogMonitorEnabled {
		t.Fatalf("disabled type switches should stay disabled: %#v", state.Config)
	}
}

func TestSaveConfigDoesNotPersistWhenStateCannotBeLoaded(t *testing.T) {
	store := &stateLoadFailingStore{config: `{"enabled":false}`}
	service := NewService(ServiceOptions{Store: store})

	_, err := service.SaveConfig(context.Background(), Config{
		Enabled:          true,
		SelectedChannels: []string{"webhook"},
		Channels:         Channels{WebhookURL: "https://example.test/hook"},
	})
	if err == nil || !strings.Contains(err.Error(), "load state failed") {
		t.Fatalf("expected state load error, got %v", err)
	}
	if store.saveCalls != 0 {
		t.Fatalf("config was persisted before state load failed: calls=%d config=%s", store.saveCalls, store.config)
	}
	if store.config != `{"enabled":false}` {
		t.Fatalf("failed save changed config: %s", store.config)
	}
}

func TestLoadConfigDefaultsMissingMessageTypeSwitches(t *testing.T) {
	service := NewService(ServiceOptions{Store: rawStore{
		config: `{"enabled":true,"selectedChannels":["webhook"],"channels":{"webhookUrl":"https://example.test/hook"}}`,
		state:  `{}`,
	}})
	state, err := service.State(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !state.Config.AbnormalEnabled || !state.Config.SuspectedEnabled || !state.Config.RecoveryEnabled || !state.Config.RestartEnabled || !state.Config.LogMonitorEnabled {
		t.Fatalf("missing type switches should default on for migrated configs: %#v", state.Config)
	}
}

type MemoryStore struct {
	Config Config
	State  State
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) LoadMessagePushConfigJSON(context.Context) (string, error) {
	raw, err := marshalJSON(s.Config)
	return raw, err
}

func (s *MemoryStore) SaveMessagePushConfigJSON(_ context.Context, raw string) error {
	return unmarshalJSON(raw, &s.Config)
}

func (s *MemoryStore) LoadMessagePushStateJSON(context.Context) (string, error) {
	raw, err := marshalJSON(s.State)
	return raw, err
}

func (s *MemoryStore) SaveMessagePushStateJSON(_ context.Context, raw string) error {
	return unmarshalJSON(raw, &s.State)
}

type rawStore struct {
	config string
	state  string
}

type stateLoadFailingStore struct {
	config    string
	saveCalls int
}

func (s *stateLoadFailingStore) LoadMessagePushConfigJSON(context.Context) (string, error) {
	return s.config, nil
}

func (s *stateLoadFailingStore) SaveMessagePushConfigJSON(_ context.Context, raw string) error {
	s.saveCalls++
	s.config = raw
	return nil
}

func (s *stateLoadFailingStore) LoadMessagePushStateJSON(context.Context) (string, error) {
	return "", errors.New("load state failed")
}

func (s *stateLoadFailingStore) SaveMessagePushStateJSON(context.Context, string) error {
	return nil
}

func (s rawStore) LoadMessagePushConfigJSON(context.Context) (string, error) {
	return s.config, nil
}

func (s rawStore) SaveMessagePushConfigJSON(context.Context, string) error {
	return nil
}

func (s rawStore) LoadMessagePushStateJSON(context.Context) (string, error) {
	return s.state, nil
}

func (s rawStore) SaveMessagePushStateJSON(context.Context, string) error {
	return nil
}
