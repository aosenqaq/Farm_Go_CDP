package messagepush

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDispatchSuppressesSameAbnormalForFiveMinutes(t *testing.T) {
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	service := newDispatchWebhookService(t, func() time.Time { return now })

	first, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	second, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Sent || !second.Suppressed || second.Sent || second.SuppressedCount != 1 {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
}

func TestRecoveryBypassesAbnormalSuppression(t *testing.T) {
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	service := newDispatchWebhookService(t, func() time.Time { return now })
	if _, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	result, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeRecovery, RuntimeTarget: "qq_ws", RecoveryVia: "guardian"})
	if err != nil || !result.Sent || result.Suppressed {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestLogRuleRequiresWarningOrErrorAndMatch(t *testing.T) {
	rules := []LogMonitorRule{{Enabled: true, Source: "guardian", Type: "guardian.network.failed", Keyword: "timeout"}}
	if MatchLogMonitorRule(rules, RuntimeLog{Level: "info", Source: "guardian", Type: "guardian.network.failed", Message: "timeout"}) {
		t.Fatal("info event matched")
	}
	if MatchLogMonitorRule(rules, RuntimeLog{Level: "warn", Source: "guardian", Type: "guardian.network.failed", Message: "closed"}) {
		t.Fatal("unmatched warning matched")
	}
	if !MatchLogMonitorRule(rules, RuntimeLog{Level: "error", Source: "guardian", Type: "guardian.network.failed", Message: "timeout"}) {
		t.Fatal("matching error did not match")
	}
}

func TestDispatchFallsBackWhenSavedTemplateCannotRender(t *testing.T) {
	service := newDispatchWebhookService(t, func() time.Time { return time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC) })
	config, err := service.loadConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	config.Templates[MessageTypeAbnormal]["webhook"] = Template{Enabled: true, Mode: TemplateModeJSON, Content: `{"title":"{{error.missing}}"}`}
	if err := service.saveConfig(context.Background(), config); err != nil {
		t.Fatal(err)
	}

	result, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"})
	if err != nil || !result.Sent || result.FallbackError == "" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestRunDueDailyDispatchesOnlyOnceAfterSuccessfulSend(t *testing.T) {
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	store := NewMemoryStore()
	service := NewService(ServiceOptions{Store: store, HTTPClient: server.Client(), Now: func() time.Time { return now }})
	if _, err := service.SaveConfig(context.Background(), dispatchConfig(server.URL)); err != nil {
		t.Fatal(err)
	}
	first, err := service.RunDueDaily(context.Background())
	if err != nil || !first.OK {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	second, err := service.RunDueDaily(context.Background())
	if err != nil || second.OK || second.Summary != "already_sent" || attempts != 1 {
		t.Fatalf("second=%#v err=%v attempts=%d", second, err, attempts)
	}
}

func TestDispatchSkipsMessagePushRuntimeLogs(t *testing.T) {
	service := newDispatchWebhookService(t, func() time.Time { return time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC) })
	result, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeLogMonitor, RuntimeLog: &RuntimeLog{Level: "error", Source: "message_push", Type: "message_push.sent", Message: "sent"}})
	if err != nil || result.Sent || result.Suppressed {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func newDispatchWebhookService(t *testing.T, now func() time.Time) *Service {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(server.Close)
	service := NewService(ServiceOptions{Store: NewMemoryStore(), HTTPClient: server.Client(), Now: now})
	if _, err := service.SaveConfig(context.Background(), dispatchConfig(server.URL)); err != nil {
		t.Fatal(err)
	}
	return service
}

func dispatchConfig(webhookURL string) Config {
	return Config{
		Enabled: true, AbnormalEnabled: true, SuspectedEnabled: true, RecoveryEnabled: true, RestartEnabled: true, DailyEnabled: true, LogMonitorEnabled: true,
		SelectedChannels: []string{"webhook"}, Channels: Channels{WebhookURL: webhookURL},
		LogMonitorRules: []LogMonitorRule{{Enabled: true, Source: "guardian", Type: "guardian.network.failed", Keyword: "timeout"}},
	}
}

func TestDispatchHonorsDisabledTemplate(t *testing.T) {
	service := newDispatchWebhookService(t, time.Now)
	config, err := service.loadConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	config.Templates[MessageTypeAbnormal]["webhook"] = Template{Enabled: false, Mode: TemplateModeJSON, Content: `{"title":"disabled"}`}
	if err := service.saveConfig(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	result, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"})
	if err != nil || result.Sent || result.Send.Summary != "template_disabled" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestDispatchKeepsPushHistoryWhenPersistingDedupe(t *testing.T) {
	store := NewMemoryStore()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(server.Close)
	service := NewService(ServiceOptions{Store: store, HTTPClient: server.Client()})
	if _, err := service.SaveConfig(context.Background(), dispatchConfig(server.URL)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"}); err != nil {
		t.Fatal(err)
	}
	state, err := service.State(context.Background())
	if err != nil || len(state.RecentPushes) != 1 || state.RecentPushes[0].Kind != string(MessageTypeAbnormal) {
		t.Fatalf("state=%#v err=%v", state, err)
	}
}

func TestDispatchRendersEventTimeInBeijingTime(t *testing.T) {
	now := time.Date(2026, 7, 13, 3, 41, 16, 0, time.UTC)
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	service := NewService(ServiceOptions{Store: NewMemoryStore(), HTTPClient: server.Client(), Now: func() time.Time { return now }})
	config := dispatchConfig(server.URL)
	config.Templates = map[MessageType]map[string]Template{
		MessageTypeAbnormal: {
			"webhook": {Enabled: true, Mode: TemplateModeJSON, Content: `{"time":"{{event.time}}"}`},
		},
	}
	if _, err := service.SaveConfig(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"}); err != nil {
		t.Fatal(err)
	}
	if got := payload["time"]; got != "2026-07-13 11:41:16" {
		t.Fatalf("event time=%#v", got)
	}
}

func TestDispatchRecordsPushTimeInBeijingTime(t *testing.T) {
	now := time.Date(2026, 7, 13, 3, 41, 16, 0, time.UTC)
	service := newDispatchWebhookService(t, func() time.Time { return now })
	if _, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"}); err != nil {
		t.Fatal(err)
	}
	state, err := service.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(state.RecentPushes) != 1 || state.RecentPushes[0].Time != "2026-07-13 11:41:16" {
		t.Fatalf("recent pushes=%#v", state.RecentPushes)
	}
}

func TestSampleTemplateContextUsesFriendlyBeijingTime(t *testing.T) {
	now := time.Date(2026, 7, 13, 3, 41, 16, 0, time.UTC)
	context := sampleTemplateContext(MessageTypeAbnormal, now)
	event, ok := context.Values["event"].(map[string]any)
	if !ok {
		t.Fatalf("event missing: %#v", context.Values)
	}
	if got := event["time"]; got != "2026-07-13 11:41:16" {
		t.Fatalf("sample event.time=%#v", got)
	}
	context = validationTemplateContext(MessageTypeAbnormal)
	event, ok = context.Values["event"].(map[string]any)
	if !ok {
		t.Fatalf("validation event missing: %#v", context.Values)
	}
	if got := event["time"]; got != "2026-01-02 11:04:05" {
		t.Fatalf("validation event.time=%#v", got)
	}
}

func TestDispatchReportsSuppressedOccurrencesAfterDedupeWindow(t *testing.T) {
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	var counts []float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		counts = append(counts, payload["count"].(float64))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	service := NewService(ServiceOptions{Store: NewMemoryStore(), HTTPClient: server.Client(), Now: func() time.Time { return now }})
	config := dispatchConfig(server.URL)
	config.Templates = map[MessageType]map[string]Template{MessageTypeAbnormal: {"webhook": {Enabled: true, Mode: TemplateModeJSON, Content: `{"count":{{event.count}}}`}}}
	if _, err := service.SaveConfig(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	for _, advance := range []time.Duration{0, time.Minute, 6 * time.Minute} {
		now = now.Add(advance)
		if _, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(counts) != 2 || counts[0] != 1 || counts[1] != 2 {
		t.Fatalf("counts=%v", counts)
	}
}

func TestDispatchFallsBackWhenSavedCardFailsProviderEnvelopeValidation(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	var diagnostic struct {
		account string
		kind    MessageType
		channel string
		err     error
	}
	service := NewService(ServiceOptions{
		Store:      NewMemoryStore(),
		HTTPClient: server.Client(),
		AccountKey: "gid:10001",
		FallbackDiagnostic: func(account string, kind MessageType, channel string, err error) {
			diagnostic.account, diagnostic.kind, diagnostic.channel, diagnostic.err = account, kind, channel, err
		},
	})
	config := dispatchConfig("")
	config.SelectedChannels = []string{"wecom"}
	config.Channels.WecomWebhook = server.URL
	config.Templates = map[MessageType]map[string]Template{MessageTypeAbnormal: {"wecom": {Enabled: true, Mode: TemplateModeCard, Content: `{"msgtype":"text"}`}}}
	if err := service.saveConfig(context.Background(), NormalizeConfig(config)); err != nil {
		t.Fatal(err)
	}

	result, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"})
	if err != nil || !result.Sent || result.FallbackError == "" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if payload["msgtype"] != "template_card" || diagnostic.account != "gid:10001" || diagnostic.kind != MessageTypeAbnormal || diagnostic.channel != "wecom" || diagnostic.err == nil {
		t.Fatalf("payload=%#v diagnostic=%#v", payload, diagnostic)
	}
}

func TestDispatchHonorsEveryMessageTypeGate(t *testing.T) {
	for _, test := range []struct {
		name      string
		typeValue MessageType
		event     MessageEvent
		disable   func(*Config)
	}{
		{"abnormal", MessageTypeAbnormal, MessageEvent{Error: "timeout"}, func(c *Config) { c.AbnormalEnabled = false }},
		{"suspected", MessageTypeSuspected, MessageEvent{Error: "timeout"}, func(c *Config) { c.SuspectedEnabled = false }},
		{"recovery", MessageTypeRecovery, MessageEvent{RecoveryVia: "guardian"}, func(c *Config) { c.RecoveryEnabled = false }},
		{"restart", MessageTypeRestart, MessageEvent{Trigger: "manual"}, func(c *Config) { c.RestartEnabled = false }},
		{"daily", MessageTypeDaily, MessageEvent{}, func(c *Config) { c.DailyEnabled = false }},
		{"log", MessageTypeLogMonitor, MessageEvent{RuntimeLog: &RuntimeLog{Level: "error", Source: "guardian", Type: "guardian.network.failed", Message: "timeout"}}, func(c *Config) { c.LogMonitorEnabled = false }},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := newDispatchWebhookService(t, time.Now)
			config, err := service.loadConfig(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			test.disable(&config)
			if err := service.saveConfig(context.Background(), config); err != nil {
				t.Fatal(err)
			}
			test.event.Type = test.typeValue
			result, err := service.Dispatch(context.Background(), test.event)
			if err != nil || result.Sent || result.Suppressed {
				t.Fatalf("result=%#v err=%v", result, err)
			}
		})
	}
}

func TestRestartBypassesAbnormalSuppression(t *testing.T) {
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	service := newDispatchWebhookService(t, func() time.Time { return now })
	if _, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	result, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeRestart, RuntimeTarget: "qq_ws", Error: "timeout"})
	if err != nil || !result.Sent || result.Suppressed {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestDispatchDedupePersistsAndScopesByAccount(t *testing.T) {
	store := NewMemoryStore()
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { attempts++; w.WriteHeader(http.StatusOK) }))
	t.Cleanup(server.Close)
	first := NewService(ServiceOptions{Store: store, HTTPClient: server.Client(), AccountKey: "gid:10001"})
	if _, err := first.SaveConfig(context.Background(), dispatchConfig(server.URL)); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"}); err != nil {
		t.Fatal(err)
	}
	recreated := NewService(ServiceOptions{Store: store, HTTPClient: server.Client(), AccountKey: "gid:10001"})
	duplicate, err := recreated.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"})
	if err != nil || !duplicate.Suppressed {
		t.Fatalf("duplicate=%#v err=%v", duplicate, err)
	}
	otherAccount := NewService(ServiceOptions{Store: store, HTTPClient: server.Client(), AccountKey: "gid:10002"})
	other, err := otherAccount.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"})
	if err != nil || !other.Sent || attempts != 2 {
		t.Fatalf("other=%#v err=%v attempts=%d", other, err, attempts)
	}
}

func TestDispatchTrimsStaleDedupeRecords(t *testing.T) {
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	rawState, err := marshalJSON(State{RecentDedupe: map[string]DedupeRecord{"stale": {FirstAt: now.Add(-10 * time.Minute).Format(time.RFC3339), LastAt: now.Add(-10 * time.Minute).Format(time.RFC3339)}}})
	if err != nil {
		t.Fatal(err)
	}
	store := &dispatchRawStore{state: rawState}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(server.Close)
	service := NewService(ServiceOptions{Store: store, HTTPClient: server.Client(), Now: func() time.Time { return now }})
	if _, err := service.SaveConfig(context.Background(), dispatchConfig(server.URL)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"}); err != nil {
		t.Fatal(err)
	}
	state, err := service.loadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.RecentDedupe["stale"]; ok {
		t.Fatalf("stale dedupe remained: %#v", state.RecentDedupe)
	}
}

type dispatchRawStore struct {
	config string
	state  string
}

func (s *dispatchRawStore) LoadMessagePushConfigJSON(context.Context) (string, error) {
	return s.config, nil
}
func (s *dispatchRawStore) SaveMessagePushConfigJSON(_ context.Context, raw string) error {
	s.config = raw
	return nil
}
func (s *dispatchRawStore) LoadMessagePushStateJSON(context.Context) (string, error) {
	return s.state, nil
}
func (s *dispatchRawStore) SaveMessagePushStateJSON(_ context.Context, raw string) error {
	s.state = raw
	return nil
}

func TestDispatchConcurrentIdenticalEventsSendOnce(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	started := make(chan struct{})
	release := make(chan struct{})
	var blockFirst sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		mu.Unlock()
		blockFirst.Do(func() {
			close(started)
			<-release
		})
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	service := NewService(ServiceOptions{Store: NewMemoryStore(), HTTPClient: server.Client()})
	if _, err := service.SaveConfig(context.Background(), dispatchConfig(server.URL)); err != nil {
		t.Fatal(err)
	}

	results := make(chan DispatchResult, 2)
	for range 2 {
		go func() {
			result, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"})
			if err != nil {
				t.Errorf("dispatch: %v", err)
			}
			results <- result
		}()
	}
	<-started
	close(release)
	first, second := <-results, <-results
	mu.Lock()
	gotAttempts := attempts
	mu.Unlock()
	if gotAttempts != 1 || (first.Sent == second.Sent) || (first.Suppressed == second.Suppressed) {
		t.Fatalf("attempts=%d first=%#v second=%#v", gotAttempts, first, second)
	}
}

func TestDispatchConcurrentSuppressionsKeepExactCount(t *testing.T) {
	const workers = 12
	service := newDispatchWebhookService(t, time.Now)
	if _, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"}); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan DispatchResult, workers)
	for range workers {
		go func() {
			<-start
			result, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"})
			if err != nil {
				t.Errorf("dispatch: %v", err)
			}
			results <- result
		}()
	}
	close(start)
	for range workers {
		result := <-results
		if !result.Suppressed {
			t.Fatalf("result=%#v", result)
		}
	}
	state, err := service.loadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(state.RecentDedupe) != 1 {
		t.Fatalf("dedupe=%#v", state.RecentDedupe)
	}
	for _, record := range state.RecentDedupe {
		if record.Count != workers {
			t.Fatalf("record=%#v, want count=%d", record, workers)
		}
	}
}

func TestDispatchRetainsExpiredAggregateAfterFailedOutbound(t *testing.T) {
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	fail := false
	var counts []float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		counts = append(counts, payload["count"].(float64))
		shouldFail := fail
		mu.Unlock()
		if shouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	service := NewService(ServiceOptions{Store: NewMemoryStore(), HTTPClient: server.Client(), Now: func() time.Time { return now }})
	config := dispatchConfig(server.URL)
	config.PushRetryCount = 1
	config.Templates = map[MessageType]map[string]Template{MessageTypeAbnormal: {"webhook": {Enabled: true, Mode: TemplateModeJSON, Content: `{"count":{{event.count}}}`}}}
	if _, err := service.SaveConfig(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if result, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"}); err != nil || !result.Suppressed {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	now = now.Add(6 * time.Minute)
	mu.Lock()
	fail = true
	mu.Unlock()
	if _, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"}); err == nil {
		t.Fatal("expected failed outbound")
	}
	mu.Lock()
	fail = false
	mu.Unlock()
	result, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"})
	if err != nil || !result.Sent {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(counts) != 3 || counts[2] != 2 {
		t.Fatalf("counts=%v", counts)
	}
}

func TestDispatchRecoversFromFallbackDiagnosticPanic(t *testing.T) {
	service := newDispatchWebhookService(t, time.Now)
	config, err := service.loadConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	config.Templates[MessageTypeAbnormal]["webhook"] = Template{Enabled: true, Mode: TemplateModeJSON, Content: `{"title":"{{error.missing}}"}`}
	if err := service.saveConfig(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	service.fallbackDiagnostic = func(string, MessageType, string, error) { panic("diagnostic failure") }
	result, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, Error: "timeout"})
	if err != nil || !result.Sent || result.FallbackError == "" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
