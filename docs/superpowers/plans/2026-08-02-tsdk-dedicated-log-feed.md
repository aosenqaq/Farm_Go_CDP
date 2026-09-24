# TSDK Dedicated Log Feed Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the newest 100 global TSDK records available independently from the active account's newest 120 ordinary runtime events.

**Architecture:** Replace the merged runtime-event read with two account-scoped reads: `RuntimeEvents` reads only the current game account and `TSDKRuntimeEvents` reads only `global:tsdk`. Return both arrays in the existing dashboard poll, retain them in separate React state, and route the dedicated array only to the TSDK status signal and dialog.

**Tech Stack:** Go 1.25/Wails, SQLite-backed runtime event storage with in-memory fallback, React 18, TypeScript, Vitest, `react-test-renderer`.

---

## File Map

- Modify `app.go`: expose separate ordinary and TSDK event readers and remove merged limiting.
- Modify `app_test.go`: prove persistent and memory-fallback feeds retain independent capacities and remain cross-account.
- Modify `poll_api.go`: add `tsdkEvents` to the existing dashboard response and populate it with a 100-record read.
- Modify `poll_api_test.go`: verify the dashboard returns non-nil independent arrays with limits 120 and 100.
- Modify `frontend/src/lib/pollApi.ts`: make `tsdkEvents` a required validated response field.
- Modify `frontend/src/lib/pollApi.test.ts`: cover valid and missing-`tsdkEvents` responses.
- Modify `frontend/src/AuthorizedApp.tsx`: own and update global TSDK state without clearing it during account changes.
- Modify `frontend/src/App.test.tsx`: lock down the dedicated polling state and account-reset behavior.
- Modify `frontend/src/views/FarmWorkspaceView.tsx`: forward the dedicated array to the overview.
- Modify `frontend/src/views/FarmWorkspaceView.test.tsx`: verify workspace forwarding and satisfy the required prop in existing fixtures.
- Modify `frontend/src/views/OverviewView.tsx`: use ordinary events for task rows and TSDK events for the signal/dialog.
- Modify `frontend/src/views/OverviewView.test.tsx`: prove the TSDK signal and dialog ignore the ordinary event array.
- Preserve `resources/gameConfig.bundle.zip`: it is a pre-existing user-owned worktree change and must not be staged or reverted.

### Task 1: Separate Backend Event Reads

**Files:**
- Modify: `app_test.go:2860-2930`
- Modify: `app.go:899-951`

- [ ] **Step 1: Replace merged-feed tests with a failing persistent-storage separation test**

Replace `TestAppRuntimeEventsShareTSDKLogsAcrossAccounts` with this test. It writes 105 TSDK events before 130 newer ordinary events, so the assertion proves the latest 100 TSDK entries survive even after the ordinary 120-record window is full:

```go
func TestAppRuntimeEventsKeepPersistentTSDKFeedIndependent(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	app.store = store

	app.currentAccountKey = "gid:10001"
	for index := 0; index < 105; index++ {
		app.recordRuntimeLogEvent(eventbus.Event{
			Source:  "qq_ws",
			Type:    "qqhost.log",
			Message: fmt.Sprintf("[TSDK-BLOCK] persistent-%03d", index),
		})
	}
	app.currentAccountKey = "gid:10002"
	for index := 0; index < 130; index++ {
		app.recordEvent(eventbus.Event{
			Source:  "account",
			Type:    "ordinary",
			Message: fmt.Sprintf("ordinary-%03d", index),
		})
	}

	ordinary := app.RuntimeEvents(120)
	if len(ordinary) != 120 {
		t.Fatalf("ordinary event count = %d, want 120", len(ordinary))
	}
	for _, event := range ordinary {
		if isTSDKBlockRuntimeEvent(event) {
			t.Fatalf("ordinary feed contains TSDK event: %#v", event)
		}
	}

	tsdk := app.TSDKRuntimeEvents(100)
	if len(tsdk) != 100 {
		t.Fatalf("TSDK event count = %d, want 100", len(tsdk))
	}
	if !strings.Contains(tsdk[0].Message, "persistent-104") || !strings.Contains(tsdk[99].Message, "persistent-005") {
		t.Fatalf("TSDK feed is not the newest 100 records: first=%q last=%q", tsdk[0].Message, tsdk[99].Message)
	}
}
```

Keep the existing `fmt` and `strings` imports; both are already used elsewhere in `app_test.go`.

- [ ] **Step 2: Replace the merged memory-fallback test with a failing separation test**

Replace `TestAppRuntimeEventsMemoryFallbackIncludesGlobalTSDK` with:

```go
func TestAppRuntimeEventsKeepMemoryTSDKFeedIndependent(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	app.store = store
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	app.currentAccountKey = "gid:10001"
	for index := 0; index < 105; index++ {
		app.recordRuntimeLogEvent(eventbus.Event{
			Type:    "qqhost.log",
			Message: fmt.Sprintf("[TSDK-BLOCK] memory-%03d", index),
		})
	}
	app.currentAccountKey = "gid:10002"
	for index := 0; index < 130; index++ {
		app.recordEvent(eventbus.Event{
			Type:    "ordinary",
			Message: fmt.Sprintf("ordinary-%03d", index),
		})
	}

	ordinary := app.RuntimeEvents(120)
	if len(ordinary) != 120 {
		t.Fatalf("ordinary fallback count = %d, want 120", len(ordinary))
	}
	for _, event := range ordinary {
		if isTSDKBlockRuntimeEvent(event) {
			t.Fatalf("ordinary fallback contains TSDK event: %#v", event)
		}
	}

	tsdk := app.TSDKRuntimeEvents(100)
	if len(tsdk) != 100 {
		t.Fatalf("TSDK fallback count = %d, want 100", len(tsdk))
	}
	if !strings.Contains(tsdk[0].Message, "memory-104") || !strings.Contains(tsdk[99].Message, "memory-005") {
		t.Fatalf("TSDK fallback is not the newest 100 records: first=%q last=%q", tsdk[0].Message, tsdk[99].Message)
	}
}
```

Add an authorization regression for the new exported reader:

```go
func TestAppTSDKRuntimeEventsRequiresAuthorization(t *testing.T) {
	app := NewApp()
	if events := app.TSDKRuntimeEvents(100); len(events) != 0 {
		t.Fatalf("unauthorized TSDK events = %#v, want empty", events)
	}
}
```

- [ ] **Step 3: Run the focused tests and verify RED**

Run:

```powershell
go test . -run 'TestApp(RuntimeEventsKeep(Persistent|Memory)TSDKFeedIndependent|TSDKRuntimeEventsRequiresAuthorization)' -count=1
```

Expected: build FAIL with `app.TSDKRuntimeEvents undefined`, proving the independent read API does not exist yet.

- [ ] **Step 4: Implement account-specific readers and remove merged limiting**

Replace the current `RuntimeEvents` and `mergeRuntimeEvents` block in `app.go` with:

```go
func (a *App) RuntimeEvents(limit int) []eventbus.Event {
	if a.requireAuthorized("RuntimeEvents") != nil {
		return []eventbus.Event{}
	}
	return a.runtimeEventsForAccount(a.accountKey(), limit)
}

func (a *App) TSDKRuntimeEvents(limit int) []eventbus.Event {
	if a.requireAuthorized("TSDKRuntimeEvents") != nil {
		return []eventbus.Event{}
	}
	return a.runtimeEventsForAccount(tsdkRuntimeAccountKey, limit)
}

func (a *App) runtimeEventsForAccount(accountKey string, limit int) []eventbus.Event {
	accountKey = storage.NormalizeAccountKey(accountKey)
	if limit <= 0 {
		limit = 100
	}
	if a.store != nil {
		events, err := a.store.ListRuntimeEventsForAccount(a.contextOrBackground(), accountKey, limit)
		if err == nil {
			return events
		}
		a.lastErr = err
	}

	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()
	capacity := limit
	if capacity > len(a.memoryEvents) {
		capacity = len(a.memoryEvents)
	}
	events := make([]eventbus.Event, 0, capacity)
	for index := len(a.memoryEvents) - 1; index >= 0 && len(events) < limit; index-- {
		entry := a.memoryEvents[index]
		if entry.accountKey == accountKey {
			events = append(events, entry.event)
		}
	}
	return events
}
```

Delete `mergeRuntimeEvents`. Keep the `sort` import because `app.go` still uses `sort.Ints` in another function.

- [ ] **Step 5: Run backend event tests and verify GREEN**

Run:

```powershell
go test . -run 'TestAppRuntimeEvents' -count=1
```

Expected: PASS, including current-account isolation, persistent separation, memory separation, newest-first ordering, and the 100/120 limits.

- [ ] **Step 6: Commit the backend separation**

```powershell
git add app.go app_test.go
git commit -m "fix: separate TSDK runtime event history"
```

### Task 2: Add The Dedicated Feed To Dashboard Polling

**Files:**
- Modify: `poll_api_test.go:23-30,91-105`
- Modify: `poll_api.go:31-59`

- [ ] **Step 1: Write a failing dashboard limit-and-separation test**

Add this test to `poll_api_test.go`:

```go
func TestAppRuntimePollDashboardReturnsIndependentEventFeeds(t *testing.T) {
	app := newAuthorizedTestApp(t)
	for index := 0; index < 105; index++ {
		app.recordRuntimeLogEvent(eventbus.Event{Type: "qqhost.log", Message: "[TSDK-BLOCK] ready"})
	}
	for index := 0; index < 125; index++ {
		app.recordEvent(eventbus.Event{Type: "ordinary", Message: "ordinary"})
	}

	response := newAppRuntimePollService(app).Dashboard("settings")
	if len(response.Events) != 120 {
		t.Fatalf("ordinary event count = %d, want 120", len(response.Events))
	}
	if len(response.TSDKEvents) != 100 {
		t.Fatalf("TSDK event count = %d, want 100", len(response.TSDKEvents))
	}
	for _, event := range response.Events {
		if isTSDKBlockRuntimeEvent(event) {
			t.Fatalf("ordinary dashboard feed contains TSDK event: %#v", event)
		}
	}
	for _, event := range response.TSDKEvents {
		if !isTSDKBlockRuntimeEvent(event) {
			t.Fatalf("TSDK dashboard feed contains ordinary event: %#v", event)
		}
	}
}
```

Also change the existing fake response so JSON handler tests always return both arrays:

```go
return dashboardPollResponse{
	Events:     []eventbus.Event{},
	TSDKEvents: []eventbus.Event{},
}
```

In `TestAppRuntimePollDashboardScopesRunStatisticsToWorkspace`, extend the completeness condition:

```go
if workspace.Status.Target != "qq_ws" || workspace.Events == nil || workspace.TSDKEvents == nil || workspace.AutomationState.FeatureGroups == nil {
	t.Fatalf("incomplete dashboard response: %#v", workspace)
}
```

- [ ] **Step 2: Run the focused poll test and verify RED**

Run:

```powershell
go test . -run 'TestAppRuntimePollDashboard(ReturnsIndependentEventFeeds|ScopesRunStatisticsToWorkspace)' -count=1
```

Expected: build FAIL because `dashboardPollResponse.TSDKEvents` is undefined.

- [ ] **Step 3: Add `tsdkEvents` to the dashboard response**

Add the field beside `Events` in `dashboardPollResponse`:

```go
Events          []eventbus.Event          `json:"events"`
TSDKEvents      []eventbus.Event          `json:"tsdkEvents"`
AutomationState automation.State          `json:"automationState"`
```

Populate both limits independently in `appRuntimePollService.Dashboard`:

```go
Events:          s.app.RuntimeEvents(120),
TSDKEvents:      s.app.TSDKRuntimeEvents(100),
AutomationState: s.app.FarmAutomationState(),
```

- [ ] **Step 4: Run poll tests and verify GREEN**

Run:

```powershell
go test . -run 'Test(AppRuntimePoll|RuntimePollHandler|WritePollJSON)' -count=1
```

Expected: PASS with dashboard responses containing non-nil `events` and `tsdkEvents` arrays.

- [ ] **Step 5: Commit the poll contract**

```powershell
git add poll_api.go poll_api_test.go
git commit -m "feat: poll dedicated TSDK event history"
```

### Task 3: Retain Dedicated TSDK State In The Frontend

**Files:**
- Modify: `frontend/src/lib/pollApi.test.ts`
- Modify: `frontend/src/lib/pollApi.ts:7-15,46-54`
- Modify: `frontend/src/App.test.tsx:65-76,166-177`
- Modify: `frontend/src/AuthorizedApp.tsx:216-231,445-469,710-760,977-987`

- [ ] **Step 1: Write failing poll-validator tests**

Add `tsdkEvents: []` to the valid dashboard fixture in `pollApi.test.ts`:

```typescript
status: {}, guardStatus: {}, bindingStatus: {}, events: [], tsdkEvents: [], automationState: {}, patchStatus: {},
```

Then add a malformed-response test that proves the field is required:

```typescript
it('rejects a dashboard response without the dedicated TSDK feed', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
    status: {},
    guardStatus: {},
    bindingStatus: {},
    events: [],
    automationState: {},
    patchStatus: {},
  }), { status: 200 })));

  await expect(pollDashboard('workspace', new AbortController().signal))
    .rejects.toThrow('Invalid dashboard poll response');
});
```

- [ ] **Step 2: Write a failing root-state wiring test**

Add this source-boundary test to `frontend/src/App.test.tsx`:

```typescript
it('keeps global TSDK events in dedicated polling state across account resets', () => {
  const source = readFileSync(new URL('./AuthorizedApp.tsx', import.meta.url), 'utf8');

  expect(source).toContain('const [tsdkEvents, setTsdkEvents] = useState<RuntimeEventDto[]>([]);');
  expect(source).toContain('setTsdkEvents(value.tsdkEvents);');
  expect(source).toContain('tsdkEvents={tsdkEvents}');
  expect(source).not.toContain('setTsdkEvents([]);');
});
```

The negative assertion is intentional: TSDK history is runtime-global and must not be cleared in either `clearConfirmedRuntimeAccount` or the successful account-confirmation reset.

- [ ] **Step 3: Run both frontend tests and verify RED**

Run from `frontend`:

```powershell
npm test -- src/lib/pollApi.test.ts src/App.test.tsx
```

Expected: FAIL because `pollDashboard` still accepts a missing `tsdkEvents` field and `AuthorizedApp` has no dedicated state.

- [ ] **Step 4: Require and validate the dashboard field**

Add the required array to `DashboardPollResponse` in `pollApi.ts`:

```typescript
events: RuntimeEventDto[];
tsdkEvents: RuntimeEventDto[];
automationState: FarmAutomationState;
```

Extend the validation condition:

```typescript
if (!isRecord(value) || !isRecord(value.status) || !isRecord(value.guardStatus) ||
    !isRecord(value.bindingStatus) || !Array.isArray(value.events) || !Array.isArray(value.tsdkEvents) ||
    !isRecord(value.automationState) || !isRecord(value.patchStatus)) {
  throw new Error('Invalid dashboard poll response');
}
```

- [ ] **Step 5: Add global TSDK state without account-scope clearing**

In `AuthorizedApp.tsx`, add state beside ordinary events:

```typescript
const [events, setEvents] = useState<RuntimeEventDto[]>([]);
const [tsdkEvents, setTsdkEvents] = useState<RuntimeEventDto[]>([]);
```

Apply both fields during the existing dashboard poll:

```typescript
setBindingStatus(value.bindingStatus);
setEvents(value.events);
setTsdkEvents(value.tsdkEvents);
setAutomationState(value.automationState);
```

Pass the dedicated state to the workspace:

```tsx
events={events}
tsdkEvents={tsdkEvents}
runStatistics={runStatistics}
```

Do not add `setTsdkEvents([])` to `clearConfirmedRuntimeAccount` or `confirmRuntimeAccount`; their existing `setEvents([])` calls continue clearing only account-owned ordinary events.

- [ ] **Step 6: Run the focused state and validator tests and verify GREEN**

Run from `frontend`:

```powershell
npm test -- src/lib/pollApi.test.ts src/App.test.tsx
```

Expected: PASS, including rejection of a poll response that omits `tsdkEvents` and preservation of global TSDK state across account resets.

- [ ] **Step 7: Commit the frontend poll state**

```powershell
git add frontend/src/lib/pollApi.ts frontend/src/lib/pollApi.test.ts frontend/src/AuthorizedApp.tsx frontend/src/App.test.tsx
git commit -m "feat: retain dedicated TSDK poll state"
```

### Task 4: Route The Dedicated Feed To The TSDK UI

**Files:**
- Modify: `frontend/src/views/FarmWorkspaceView.test.tsx`
- Modify: `frontend/src/views/FarmWorkspaceView.tsx:23-35,116-155`
- Modify: `frontend/src/views/OverviewView.test.tsx`
- Modify: `frontend/src/views/OverviewView.tsx:43-65,104-105,253-256`

- [ ] **Step 1: Write a failing overview separation test**

Add this test to `OverviewView.test.tsx`:

```tsx
it('uses only the dedicated TSDK feed for its signal and dialog', () => {
  const renderer = create(
    <OverviewView
      status={{ ...readyStatus, phase: 'listening', connected: false, ready: false }}
      events={[{
        id: 1,
        timestamp: '2026-08-02T10:00:00+08:00',
        level: 'error',
        source: 'qq_ws',
        type: 'qqhost.log',
        message: '[TSDK-BLOCK] ordinary-feed-failure',
        data: { status: 'init_err' },
      }]}
      tsdkEvents={[{
        id: 2,
        timestamp: '2026-08-02T10:00:01+08:00',
        level: 'info',
        source: 'qq_ws',
        type: 'qqhost.log',
        message: '[TSDK-BLOCK] dedicated-feed-ready',
      }]}
      onRefresh={() => undefined}
    />,
  );

  expect(JSON.stringify(renderer.toJSON())).toContain('TSDK 已拦截');
  act(() => renderer.root.findByProps({ title: '点击查看 TSDK 拦截详情' }).props.onClick());
  const rendered = JSON.stringify(renderer.toJSON());
  expect(rendered).toContain('[TSDK-BLOCK] dedicated-feed-ready');
  expect(rendered).not.toContain('[TSDK-BLOCK] ordinary-feed-failure');
});
```

For every other `<OverviewView>` fixture in this test file, add the required empty prop directly after `events`:

```tsx
events={[]}
tsdkEvents={[]}
```

For the fixture with a populated ordinary array, add `tsdkEvents={[]}` after the closing `events={...}` expression.

- [ ] **Step 2: Write a failing workspace-forwarding test**

Add this test to `FarmWorkspaceView.test.tsx`:

```tsx
it('forwards dedicated TSDK events to the overview', () => {
  const html = renderToStaticMarkup(
    <FarmWorkspaceView
      area="workspace"
      status={status}
      guardStatus={guardStatus}
      bindingStatus={null}
      events={[]}
      tsdkEvents={[{
        id: 1,
        timestamp: '2026-08-02T10:00:00+08:00',
        level: 'info',
        source: 'qq_ws',
        type: 'qqhost.log',
        message: '[TSDK-BLOCK] workspace-ready',
      }]}
      onRefresh={noop}
      onLaunch={noop}
      onRestart={noop}
      onToggleGuard={noop}
    />,
  );

  expect(html).toContain('TSDK 已拦截');
});
```

For every other `<FarmWorkspaceView>` fixture in this test file, add `tsdkEvents={[]}` immediately after its existing `events` prop:

```tsx
events={[]}
tsdkEvents={[]}
```

- [ ] **Step 3: Run the focused view tests and verify RED**

Run from `frontend`:

```powershell
npm test -- src/views/OverviewView.test.tsx src/views/FarmWorkspaceView.test.tsx
```

Expected: TypeScript transform/test FAIL because neither component accepts `tsdkEvents` yet.

- [ ] **Step 4: Add and forward the workspace prop**

In `FarmWorkspaceView.tsx`, add the required prop:

```typescript
events: RuntimeEventDto[];
tsdkEvents: RuntimeEventDto[];
runStatistics?: WorkspaceRunStatistics | null;
```

Destructure it beside `events`:

```typescript
bindingStatus,
events,
tsdkEvents,
runStatistics,
```

Forward both arrays to the overview:

```tsx
return <OverviewView status={status} licenseStatus={licenseStatus} guardStatus={guardStatus} events={events} tsdkEvents={tsdkEvents} runStatistics={runStatistics} onRefresh={onRefresh} />;
```

- [ ] **Step 5: Use the dedicated prop in `OverviewView`**

Add the required prop and destructure it:

```typescript
type OverviewViewProps = {
  status: RuntimeStatusDto;
  licenseStatus?: AuthorizationServiceStatus;
  guardStatus?: GuardStatusDto;
  events: RuntimeEventDto[];
  tsdkEvents: RuntimeEventDto[];
  onRefresh: () => void;
  runStatistics?: WorkspaceRunStatistics | null;
};

export function OverviewView({ status, licenseStatus = unavailableAuthorizationServiceStatus, guardStatus, events, tsdkEvents, onRefresh, runStatistics }: OverviewViewProps) {
```

Keep `events.filter(isWorkbenchTaskEvent)` unchanged for task activity. Change only the TSDK consumers:

```typescript
const tsdkSignal = tsdkBlockSignal(tsdkEvents, status);
```

```tsx
<TsdkBlockDialog
  open={tsdkDialogOpen}
  events={tsdkEvents}
  onClose={() => setTsdkDialogOpen(false)}
/>
```

The dialog already uses `tsdkBlockEventsNewestFirst`, so new poll records remain ordered from newest to oldest without mutating the response array.

- [ ] **Step 6: Run focused frontend tests and verify GREEN**

Run from `frontend`:

```powershell
npm test -- src/views/OverviewView.test.tsx src/views/FarmWorkspaceView.test.tsx src/components/TsdkBlockDialog.test.tsx src/lib/events.test.ts
```

Expected: PASS, proving the dedicated feed drives the indicator/dialog and the newest record remains at the top.

- [ ] **Step 7: Commit the view routing**

```powershell
git add frontend/src/views/OverviewView.tsx frontend/src/views/OverviewView.test.tsx frontend/src/views/FarmWorkspaceView.tsx frontend/src/views/FarmWorkspaceView.test.tsx
git commit -m "fix: isolate TSDK logs from ordinary event UI"
```

### Task 5: Full Regression Verification

**Files:**
- Verify only; no production files should change.

- [ ] **Step 1: Format modified Go files**

```powershell
gofmt -w app.go app_test.go poll_api.go poll_api_test.go
```

Expected: command exits successfully. Review `git diff --check` afterward to catch whitespace errors.

- [ ] **Step 2: Run all frontend tests**

Run from `frontend`:

```powershell
npm test
```

Expected: all Vitest suites PASS, including poll validation, dedicated state wiring, view separation, and newest-first dialog updates.

- [ ] **Step 3: Build the frontend**

Run from `frontend`:

```powershell
npm run build
```

Expected: TypeScript and Vite build complete successfully, proving every required `tsdkEvents` prop is wired.

- [ ] **Step 4: Run the full Go test suite**

Run from the repository root:

```powershell
go test ./... -count=1
```

Expected: all Go packages PASS.

- [ ] **Step 5: Review the final diff and worktree boundaries**

```powershell
git diff --check
git status --short
git diff -- app.go app_test.go poll_api.go poll_api_test.go frontend/src/lib/pollApi.ts frontend/src/lib/pollApi.test.ts frontend/src/AuthorizedApp.tsx frontend/src/App.test.tsx frontend/src/views/FarmWorkspaceView.tsx frontend/src/views/FarmWorkspaceView.test.tsx frontend/src/views/OverviewView.tsx frontend/src/views/OverviewView.test.tsx
```

Expected: no whitespace errors; only the planned source/test files plus the pre-existing `resources/gameConfig.bundle.zip` change appear. Do not stage `resources/gameConfig.bundle.zip`.

- [ ] **Step 6: Commit any verification-only formatting adjustments**

If `gofmt` changed tracked planned files after their task commits, stage only those named Go files and commit them:

```powershell
git add app.go app_test.go poll_api.go poll_api_test.go
git commit -m "style: format dedicated TSDK feed changes"
```

If `gofmt` produced no diff, skip this commit.
