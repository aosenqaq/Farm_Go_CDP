# Message Push Templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Add account-scoped, channel-specific templates for all six message types and send them from guardian, runtime-log, and daily-report events.

**Architecture:** Message-push owns template catalog, rendering, event dispatch, and five-minute deduplication. Guard emits state-transition callbacks; App persists runtime events, then enqueues them to a dedicated bounded message-delivery worker that is separate from the farm automation scheduler and its locks. The React view edits templates in the existing message-push configuration and previews/tests unsaved drafts through Wails.

**Tech Stack:** Go 1.25, Wails v2, React 18, TypeScript, Vitest, Go testing, existing net/http clients.

---

## File Map

| File | Change |
| --- | --- |
| internal/messagepush/types.go | Add template, catalog, log-rule, dedupe, preview, and event DTOs. |
| internal/messagepush/templates.go | Create catalog, built-in defaults, normalization, and save validation. |
| internal/messagepush/render.go | Create restricted {{path}} renderer and payload validation. |
| internal/messagepush/dispatch.go | Create runtime-event mapping, log filtering, dedupe, and fallback dispatch. |
| internal/messagepush/service.go | Add preview, draft test, live dispatch, and rendered daily send. |
| internal/messagepush/sender.go | Build text, markdown, card, and JSON provider request bodies. |
| internal/messagepush/template_test.go | Add migration, rendering, validation, and request tests. |
| internal/messagepush/dispatch_test.go | Add type mapping, dedupe, fallback, and log-rule tests. |
| internal/runtime/guard/types.go | Add lifecycle-event DTO and manager callback. |
| internal/runtime/guard/manager.go | Emit suspected, abnormal, recovery, and restart-completed transitions. |
| internal/runtime/guard/manager_test.go | Add lifecycle transition tests. |
| app.go and app_test.go | Bind guard callback, own the dedicated delivery queue/worker, route persisted events, and expose preview/test endpoints. |
| frontend/src/views/MessagePushView.tsx | Add template workspace and backend preview/test actions. |
| frontend/src/views/MessagePushView.test.tsx | Add editor, preview, restore-default, and Wails contract assertions. |
| frontend/src/style.css | Add responsive, scoped template-editor styles. |
| frontend/wailsjs/go/main/App.js, App.d.ts, models.ts | Regenerate from exported Go types and methods. |

### Task 1: Persisted Template Configuration

**Files:**
- Modify: internal/messagepush/types.go
- Modify: internal/messagepush/config.go
- Create: internal/messagepush/templates.go
- Create: internal/messagepush/template_test.go

- [ ] **Step 1: Write failing migration and validation tests**

~~~go
func TestLegacyConfigReceivesBuiltInTemplates(t *testing.T) {
    service := NewService(ServiceOptions{Store: rawStore{
        config: "{\"enabled\":true,\"selectedChannels\":[\"webhook\"],\"channels\":{\"webhookUrl\":\"https://example.test/hook\"}}",
        state:  "{}",
    }})
    state, err := service.State(context.Background())
    if err != nil { t.Fatal(err) }
    got := state.Config.Templates[MessageTypeAbnormal]["webhook"]
    if got.Mode != TemplateModeJSON || got.Content == "" { t.Fatalf("template=%#v", got) }
}

func TestTemplateRejectsUnsupportedMode(t *testing.T) {
    err := ValidateTemplate(MessageTypeLogMonitor, "qmsg", Template{Mode: TemplateModeCard, Content: "{}"})
    if err == nil || !strings.Contains(err.Error(), "不支持") { t.Fatalf("err=%v", err) }
}
~~~

- [ ] **Step 2: Verify the tests fail**

Run: go test ./internal/messagepush -run 'TestLegacyConfigReceivesBuiltInTemplates|TestTemplateRejectsUnsupportedMode' -count=1

Expected: FAIL because template DTOs and validation are absent.

- [ ] **Step 3: Add the DTOs and catalog**

In types.go add MessageType constants: abnormal, suspected, recovery, restart, daily, and log_monitor. Add TemplateMode constants text, markdown, card, and json. Add Template with Enabled, Mode, and Content; LogMonitorRule with Enabled, Source, Type, and Keyword; and DedupeRecord with FirstAt, LastAt, and Count.

Extend Config with Templates map[MessageType]map[string]Template and LogMonitorRules []LogMonitorRule. Extend State with RecentDedupe map[string]DedupeRecord. Extend ViewState with TemplateCatalog []TemplateDefinition.

In templates.go implement TemplateDefinitions, DefaultTemplate, SupportedTemplateModes, NormalizeTemplates, NormalizeLogMonitorRules, ValidateTemplate, and ValidateConfigTemplates. Defaults must cover every type/channel pair. Capabilities are: text for every existing channel; markdown for serverchan, pushplus, wecom, dingtalk, feishu, telegram, and ntfy; card for wecom, dingtalk, and feishu; JSON for webhook, wecom, dingtalk, and feishu.

- [ ] **Step 4: Wire backward-compatible normalization**

Call NormalizeTemplates and NormalizeLogMonitorRules from NormalizeConfig. SaveConfig must validate before writing. loadConfig must retain existing type-switch migration and then add defaults in memory. Explicitly disabled templates must remain disabled.

- [ ] **Step 5: Verify and commit**

Run: go test ./internal/messagepush -run 'TestLegacyConfigReceivesBuiltInTemplates|TestTemplateRejectsUnsupportedMode|TestLoadConfigDefaultsMissingMessageTypeSwitches' -count=1

Expected: PASS.

~~~powershell
git add internal/messagepush/types.go internal/messagepush/config.go internal/messagepush/templates.go internal/messagepush/template_test.go
git commit -m "feat: add message push template configuration"
~~~

### Task 2: Render Templates and Build Provider Payloads

**Files:**
- Create: internal/messagepush/render.go
- Modify: internal/messagepush/sender.go
- Modify: internal/messagepush/service.go
- Modify: internal/messagepush/template_test.go

- [ ] **Step 1: Write failing renderer and request tests**

~~~go
func TestRenderTemplateUsesAllowedVariables(t *testing.T) {
    rendered, err := RenderTemplate(Template{Mode: TemplateModeMarkdown, Content: "{{runtime.target}} {{error.message}}"}, TemplateContext{
        MessageType: MessageTypeAbnormal,
        Values: map[string]any{"runtime": map[string]any{"target": "qq_ws"}, "error": map[string]any{"message": "timeout"}},
    })
    if err != nil || rendered.Text != "qq_ws timeout" { t.Fatalf("rendered=%#v err=%v", rendered, err) }
}

func TestRenderTemplateRejectsUnknownVariable(t *testing.T) {
    _, err := RenderTemplate(Template{Mode: TemplateModeText, Content: "{{process.env}}"}, TemplateContext{MessageType: MessageTypeDaily})
    if err == nil || !strings.Contains(err.Error(), "未知变量") { t.Fatalf("err=%v", err) }
}
~~~

Also add an httptest Webhook case that decodes a rendered JSON body and expects kind "abnormal" and message "timeout".

- [ ] **Step 2: Verify the tests fail**

Run: go test ./internal/messagepush -run 'TestRenderTemplateUsesAllowedVariables|TestRenderTemplateRejectsUnknownVariable|TestWebhookJSONTemplateSendsRenderedObject' -count=1

Expected: FAIL because TemplateContext and RenderTemplate are absent.

- [ ] **Step 3: Implement the restricted renderer**

Create TemplateContext with MessageType and Values. Create RenderedPayload with Kind, Mode, Text, JSON, and Context. Match only {{identifier(.identifier)*}}, resolve paths only in nested map values, and reject absent paths or paths outside the selected type allowlist. Do not add expression evaluation or general-purpose template execution.

For text and markdown, return Text. For card and JSON, render then decode to a map object; reject a non-object or malformed JSON.

- [ ] **Step 4: Adapt existing request builders**

Make sender.go consume RenderedPayload. For text/markdown, retain the current channel envelope and set its provider-specific content field. For card/JSON, use the rendered object as the provider body. Preserve existing credentials, DingTalk signing, Webhook headers, retry behavior, and non-2xx handling.

Refactor SendTest and SendDailyNow to create a mock or daily TemplateContext, resolve the selected template, and use a single sendRendered path while retaining SendResult and push history compatibility.

- [ ] **Step 5: Verify and commit**

Run: go test ./internal/messagepush -run 'TestRenderTemplate|TestWebhookJSONTemplateSendsRenderedObject|TestSendTestRetriesUntilWebhookSucceeds' -count=1

Expected: PASS.

~~~powershell
git add internal/messagepush/render.go internal/messagepush/sender.go internal/messagepush/service.go internal/messagepush/template_test.go
git commit -m "feat: render channel-specific push templates"
~~~

### Task 3: Dispatch, Log Rules, Dedupe, and Fallback

**Files:**
- Create: internal/messagepush/dispatch.go
- Modify: internal/messagepush/service.go
- Create: internal/messagepush/dispatch_test.go

- [ ] **Step 1: Write failing dispatch tests**

~~~go
func TestDispatchSuppressesSameAbnormalForFiveMinutes(t *testing.T) {
    now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
    service := newWebhookService(t, func() time.Time { return now })
    first, _ := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"})
    now = now.Add(time.Minute)
    second, _ := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"})
    if !first.Sent || second.Sent || second.SuppressedCount != 1 { t.Fatalf("first=%#v second=%#v", first, second) }
}

func TestRecoveryBypassesAbnormalSuppression(t *testing.T) {
    now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
    service := newWebhookService(t, func() time.Time { return now })
    if _, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"}); err != nil { t.Fatal(err) }
    now = now.Add(time.Minute)
    result, err := service.Dispatch(context.Background(), MessageEvent{Type: MessageTypeRecovery, RuntimeTarget: "qq_ws", RecoveryVia: "guardian"})
    if err != nil || !result.Sent || result.Suppressed { t.Fatalf("result=%#v err=%v", result, err) }
}

func TestLogRuleRequiresWarningOrErrorAndMatch(t *testing.T) {
    rules := []LogMonitorRule{{Enabled: true, Source: "guardian", Type: "guardian.network.failed", Keyword: "timeout"}}
    if MatchLogMonitorRule(rules, RuntimeLog{Level: "info", Source: "guardian", Type: "guardian.network.failed", Message: "timeout"}) { t.Fatal("info event matched") }
    if MatchLogMonitorRule(rules, RuntimeLog{Level: "warn", Source: "guardian", Type: "guardian.network.failed", Message: "closed"}) { t.Fatal("unmatched warning matched") }
    if !MatchLogMonitorRule(rules, RuntimeLog{Level: "error", Source: "guardian", Type: "guardian.network.failed", Message: "timeout"}) { t.Fatal("matching error did not match") }
}

func newWebhookService(t *testing.T, now func() time.Time) *Service {
    t.Helper()
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
    t.Cleanup(server.Close)
    service := NewService(ServiceOptions{Store: NewMemoryStore(), HTTPClient: server.Client(), Now: now})
    _, err := service.SaveConfig(context.Background(), Config{Enabled: true, AbnormalEnabled: true, SuspectedEnabled: true, RecoveryEnabled: true, RestartEnabled: true, DailyEnabled: true, LogMonitorEnabled: true, SelectedChannels: []string{"webhook"}, Channels: Channels{WebhookURL: server.URL}})
    if err != nil { t.Fatal(err) }
    return service
}
~~~

- [ ] **Step 2: Verify the tests fail**

Run: go test ./internal/messagepush -run 'TestDispatchSuppressesSameAbnormalForFiveMinutes|TestRecoveryBypassesAbnormalSuppression|TestLogRuleRequiresWarningOrErrorAndMatch' -count=1

Expected: FAIL because MessageEvent and Dispatch are absent.

- [ ] **Step 3: Implement live event dispatch**

Create MessageEvent with Type, AccountKey, RuntimeTarget, Error, Trigger, RecoveryVia, OccurredAt, Values, and optional RuntimeLog. Create DispatchResult with Sent, Suppressed, SuppressedCount, Send, and FallbackError.

Dispatch must honor the global switch, type switch, and template Enabled flag. Build a SHA-256 key from account, type, target, and normalized error/rule signature. Persist the key in State.RecentDedupe, suppress matches younger than five minutes, increment Count, and trim stale records. Recovery, restart completion, and daily do not use this suppression window.

- [ ] **Step 4: Add safe log matching and template fallback**

RuntimeLog contains Level, Source, Type, Message, and Data. MatchLogMonitorRule must reject info events and only match enabled rules whose nonblank source/type/keyword constraints all match.

On saved-template render failure, send DefaultTemplate for the same type/channel, return FallbackError, and leave an audit hook for App. Do not dispatch any RuntimeLog with Source message_push. Refactor RunDueDaily to dispatch a daily event and set LastDailySummaryDateKey only after a successful non-suppressed send.

- [ ] **Step 5: Verify and commit**

Run: go test ./internal/messagepush -run 'TestDispatch|TestRecoveryBypasses|TestLogRule|TestRunDueDailySendsOnlyOncePerTargetDate' -count=1

Expected: PASS.

~~~powershell
git add internal/messagepush/dispatch.go internal/messagepush/dispatch_test.go internal/messagepush/service.go
git commit -m "feat: dispatch deduplicated message push events"
~~~

### Task 4: Emit Guard Lifecycle Transitions

**Files:**
- Modify: internal/runtime/guard/types.go
- Modify: internal/runtime/guard/manager.go
- Modify: internal/runtime/guard/manager_test.go

- [ ] **Step 1: Write failing guard-event tests**

~~~go
func TestManagerEmitsSuspectedThenRecovery(t *testing.T) {
    events := []LifecycleEvent{}
    manager := NewManager(ManagerOptions{
        Settings: Settings{Enabled: true, FailureRecoveryEnabled: true, TimeoutThreshold: 3},
        OnLifecycleEvent: func(event LifecycleEvent) { events = append(events, event) },
    })
    manager.Arm("test")
    manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
    manager.NoteHealthy(RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})
    if got := eventKinds(events); !reflect.DeepEqual(got, []LifecycleKind{LifecycleSuspected, LifecycleRecovery}) { t.Fatalf("events=%#v", events) }
}

func eventKinds(events []LifecycleEvent) []LifecycleKind {
    kinds := make([]LifecycleKind, 0, len(events))
    for _, event := range events { kinds = append(kinds, event.Kind) }
    return kinds
}

func TestManagerEmitsRestartCompletionAndAbnormalFailure(t *testing.T) {
    events := make(chan LifecycleEvent, 3)
    manager := NewManager(ManagerOptions{
        Settings: Settings{Enabled: true, FailureRecoveryEnabled: true, TimeoutThreshold: 1},
        Restart: func(RestartReason) (RestartResult, error) { return RestartResult{}, errors.New("restart failed") },
        OnLifecycleEvent: func(event LifecycleEvent) { events <- event },
    })
    manager.Arm("test")
    manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
    got := []LifecycleKind{}
    for len(got) < 3 { got = append(got, (<-events).Kind) }
    if !reflect.DeepEqual(got, []LifecycleKind{LifecycleSuspected, LifecycleRestartCompleted, LifecycleAbnormal}) { t.Fatalf("kinds=%#v", got) }
}
~~~

Add a restart callback failure case expecting restart_completed then abnormal.

- [ ] **Step 2: Verify the tests fail**

Run: go test ./internal/runtime/guard -run 'TestManagerEmitsSuspectedThenRecovery|TestManagerEmitsRestartCompletionAndAbnormalFailure' -count=1

Expected: FAIL because lifecycle types and the callback are absent.

- [ ] **Step 3: Add a lock-safe callback contract**

Add LifecycleKind values suspected, abnormal, recovery, and restart_completed; add LifecycleEvent containing target, error, trigger, result, streak, threshold, and timestamp. Extend ManagerOptions with OnLifecycleEvent.

Copy callback/event data while locked, then invoke it after unlock through a panic-safe helper. Never call application code while Manager.mu is held.

- [ ] **Step 4: Emit only genuine transitions**

NoteRuntimeError emits suspected once for the initial restartable monitored failure. NoteHealthy emits recovery only when the prior state was suspected, degraded, waiting_reconnect, or circuit_open. executeRestart emits restart_completed for manual, scheduled, and automatic attempts and emits abnormal after an automatic restart failure. openCircuitLocked emits abnormal only on closed-to-open transition.

- [ ] **Step 5: Verify and commit**

Run: go test ./internal/runtime/guard -run 'TestManagerEmits|TestManager' -count=1

Expected: PASS.

~~~powershell
git add internal/runtime/guard/types.go internal/runtime/guard/manager.go internal/runtime/guard/manager_test.go
git commit -m "feat: emit process guard lifecycle events"
~~~

### Task 5: Integrate the Independent App Delivery Worker and Wails Endpoints

**Files:**
- Modify: app.go
- Modify: app_test.go

- [ ] **Step 1: Write failing captured-account and recursion tests**

~~~go
func TestAppDispatchesGuardEventForCapturedAccount(t *testing.T) {
    app, store, server := newMessagePushApp(t)
    account := app.ConfirmRuntimeAccount(RuntimeAccount{GID: 10001})
    if account.Error != "" { t.Fatal(account.Error) }
    _, err := app.SaveMessagePushConfig(messagepush.Config{Enabled: true, AbnormalEnabled: true, SelectedChannels: []string{"webhook"}, Channels: messagepush.Channels{WebhookURL: server.URL}})
    if err != nil { t.Fatal(err) }
    app.dispatchMessagePushEventForAccount("gid:10001", messagepush.MessageEvent{Type: messagepush.MessageTypeAbnormal, RuntimeTarget: "qq_ws", Error: "timeout"})
    if pushes := app.MessagePushState().RecentPushes; len(pushes) != 1 || pushes[0].Kind != "abnormal" { t.Fatalf("pushes=%#v", pushes) }
    events, err := store.ListRuntimeEventsForAccount(context.Background(), "gid:10001", 20)
    if err != nil || !containsRuntimeEventType(events, "message_push.sent") { t.Fatalf("events=%#v err=%v", events, err) }
}

func newMessagePushApp(t *testing.T) (*App, *storage.Store, *httptest.Server) {
    t.Helper()
    store, err := storage.Open(context.Background(), t.TempDir())
    if err != nil { t.Fatal(err) }
    t.Cleanup(func() { _ = store.Close() })
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
    t.Cleanup(server.Close)
    app := NewApp()
    app.store = store
    app.messagePushHTTPClient = server.Client()
    return app, store, server
}
~~~

Add a case that records a message_push.sent event and asserts no HTTP request occurs.

~~~go
func TestAppDoesNotRecursivelyDispatchMessagePushAuditEvents(t *testing.T) {
    app, _, _ := newMessagePushApp(t)
    app.recordEventForAccount("gid:10001", eventbus.Event{Source: "message_push", Type: "message_push.sent", Message: "sent"})
    if state := app.MessagePushState(); len(state.RecentPushes) != 0 { t.Fatalf("pushes=%#v", state.RecentPushes) }
}
~~~

- [ ] **Step 2: Verify the tests fail**

Run: go test . -run 'TestAppDispatchesGuardEventForCapturedAccount|TestAppDoesNotRecursivelyDispatchMessagePushAuditEvents' -count=1

Expected: FAIL because the dispatcher helper is absent.

- [ ] **Step 3: Write failing isolated-scheduler tests**

~~~go
func TestMessagePushDeliveryQueueDoesNotUseFarmAutomationScheduler(t *testing.T) {
    app := NewApp()
    if app.messagePushQueue == nil || app.messagePushWorkerCancel == nil { t.Fatal("message delivery worker was not initialized") }
    if app.automationScheduler == nil { t.Fatal("test requires the existing primary scheduler") }
    if reflect.ValueOf(app.messagePushQueue).Pointer() == reflect.ValueOf(app.automationScheduler).Pointer() { t.Fatal("message queue shares the automation scheduler") }
}

func TestFullMessagePushQueueRecordsDropWithoutBlockingPublisher(t *testing.T) {
    // Fill a capacity-one queue, enqueue another event, and assert queue_dropped is persisted immediately.
}
~~~

- [ ] **Step 4: Verify the scheduler tests fail**

Run: go test . -run 'TestMessagePushDeliveryQueueDoesNotUseFarmAutomationScheduler|TestFullMessagePushQueueRecordsDropWithoutBlockingPublisher' -count=1

Expected: FAIL because the dedicated queue and worker do not exist.

- [ ] **Step 5: Add the dedicated bounded worker**

Add an App-owned buffered channel of account-key/event pairs, a delivery-worker cancellation function, and a worker wait group. Start one worker during application startup beside the current daily timer. The worker reads only its own queue, calls dispatchMessagePushEventForAccount synchronously, and never calls farm automation scheduler APIs or shares its locks.

Use a nonblocking enqueue helper from guardian and runtime-event publishers. On a full queue, persist a message_push.queue_dropped warning through recordMessagePushEventForAccount; never create an unbounded goroutine to compensate. Stop the worker and wait for it during shutdown before closing storage. The daily timer must enqueue its due event to this worker rather than issue HTTP directly.

- [ ] **Step 6: Route guard and runtime events**

Pass the lifecycle callback to guard.NewManager in NewApp. Convert it to MessageEvent and enqueue it with the captured account key.

At the end of recordEventForAccount, after memory/database persistence, skip Source message_push. For remaining records, convert eventbus.Event to RuntimeLog and enqueue it with the captured account key. The worker calls a synchronous private dispatchMessagePushEventForAccount helper, obtains messagePushServiceForAccount(accountKey), records sent/suppressed/failure/fallback audit events through recordMessagePushEventForAccount, and never reads a.accountKey later.

- [ ] **Step 7: Add Wails preview and draft-test methods**

Add exported methods:

~~~go
func (a *App) PreviewMessagePushTemplate(config messagepush.Config, messageType string, channel string) messagepush.TemplatePreview
func (a *App) SendMessagePushTemplateTest(config messagepush.Config, messageType string, channel string) messagepush.SendResult
~~~

They validate the supplied unsaved draft. Preview performs no HTTP request. Draft test uses mock context and only the requested channel; it neither saves configuration nor changes dedupe/daily state.

- [ ] **Step 8: Verify and commit**

Run: go test . -run 'TestApp.*MessagePush|TestAppDispatchesGuardEventForCapturedAccount|TestAppDoesNotRecursivelyDispatchMessagePushAuditEvents' -count=1

Expected: PASS.

~~~powershell
git add app.go app_test.go
git commit -m "feat: wire runtime events to message templates"
~~~

### Task 6: Build the Template Workspace and Regenerate Bindings

**Files:**
- Modify: frontend/src/views/MessagePushView.tsx
- Modify: frontend/src/views/MessagePushView.test.tsx
- Modify: frontend/src/style.css
- Modify generated: frontend/wailsjs/go/main/App.js
- Modify generated: frontend/wailsjs/go/main/App.d.ts
- Modify generated: frontend/wailsjs/go/models.ts

- [ ] **Step 1: Add failing workflow and contract assertions**

~~~ts
it("renders the template workflow", () => {
  const html = renderToStaticMarkup(<MessagePushView />);
  expect(html).toContain("模板");
  expect(html).toContain("日志监控规则");
  expect(html).toContain("可用变量");
  expect(html).toContain("预览");
  expect(html).toContain("恢复默认");
  expect(html).toContain("测试当前模板");
});

it("uses the preview and template-test bindings", () => {
  expect(messagePushViewSource).toContain("PreviewMessagePushTemplate");
  expect(messagePushViewSource).toContain("SendMessagePushTemplateTest");
});
~~~

- [ ] **Step 2: Verify the tests fail**

Run: npm --prefix frontend test -- --run src/views/MessagePushView.test.tsx

Expected: FAIL until the workspace imports and uses the new bindings.

- [ ] **Step 3: Implement a draft-safe editor**

Use generated messagepush Config, Template, ViewState, and TemplatePreview types. Add selectedTemplateType, selectedTemplateChannel, templateDraft, preview, and templateNotice state.

Render a template panel that selects type, configured channel, and a mode limited by state.templateCatalog; edit full content in a stable textarea; list allowed variables; preview the backend-rendered result; restore built-in defaults; and send the current unsaved draft for test. Updating a template must immutably update only draft.templates[type][channel]. The existing Save button remains the only persistence action.

When selectedTemplateType is log_monitor, render a compact LogMonitorRule list below the template editor. Each row has enabled, source, type, and keyword inputs plus a delete icon; add-row creates an enabled empty rule. Write all changes immutably to draft.logMonitorRules, require at least one nonblank constraint before save, and explain in the empty state that no rule means no log notifications.

- [ ] **Step 4: Add scoped responsive styles**

Use existing message-push panel/button classes. Add only message-push-template selectors for editor grid, stable textarea height, JSON preview, errors, and narrow viewport stacking. Do not create nested cards or a new page-level surface.

- [ ] **Step 5: Regenerate Wails bindings and verify frontend**

Run: wails build -clean

Expected: generated files include PreviewMessagePushTemplate, SendMessagePushTemplateTest, Template, TemplateDefinition, TemplatePreview, LogMonitorRule, and expanded Config/State models. Do not hand-edit them.

Run: npm --prefix frontend test -- --run src/views/MessagePushView.test.tsx

Expected: PASS.

Run: npm --prefix frontend run build

Expected: PASS.

- [ ] **Step 6: Commit UI and generated code**

~~~powershell
git add frontend/src/views/MessagePushView.tsx frontend/src/views/MessagePushView.test.tsx frontend/src/style.css frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/models.ts
git commit -m "feat: add message template workspace"
~~~

### Task 7: Full Regression and Delivery Verification

**Files:**
- Modify only if verification identifies a defect in the previous tasks.

- [ ] **Step 1: Format and check whitespace**

Run: gofmt -w internal/messagepush/*.go internal/runtime/guard/manager.go internal/runtime/guard/types.go app.go app_test.go

Run: git diff --check

Expected: no formatting or whitespace errors.

- [ ] **Step 2: Run all backend tests**

Run: go test ./... -count=1

Expected: PASS, including message-push, guard, storage, and account-scope tests.

- [ ] **Step 3: Run all frontend tests and build**

Run: npm --prefix frontend test

Expected: PASS.

Run: npm --prefix frontend run build

Expected: PASS.

- [ ] **Step 4: Run a desktop build and inspect scope**

Run: wails build -clean

Expected: PASS and generated Wails bindings are synchronized.

Run: git status --short

Expected: only intentional template work plus pre-existing user modifications. Preserve unrelated changes.

- [ ] **Step 5: Commit final corrective changes only if verification required them**

~~~powershell
git add internal/messagepush internal/runtime/guard/manager.go internal/runtime/guard/types.go app.go app_test.go frontend/src/views/MessagePushView.tsx frontend/src/views/MessagePushView.test.tsx frontend/src/style.css frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/models.ts
git commit -m "test: verify message push template workflow"
~~~
