# TSDK Global Live Log Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Reliably deliver TSDK startup/success logs, share them across game accounts, and render newly polled records at the top of an already-open dialog.

**Architecture:** Add a bounded pre-connection log queue inside the QQ host asset. Classify `[TSDK-BLOCK]` backend events into a dedicated global account scope and merge that scope with the active account at read time. Keep the current 2.5-second dashboard poll and make frontend TSDK ordering explicit and immutable.

**Tech Stack:** JavaScript runtime asset, Go 1.25/Wails backend, SQLite storage, React 18, TypeScript, Vitest, `react-test-renderer`.

---

## File Map

- Modify `resources/qq/qq-host.js`: queue log packets produced before the QQ socket opens and flush them after connection.
- Modify `internal/runtime/qqws/host_asset_test.go`: execute the real host asset in a Node VM harness and verify queued delivery.
- Modify `app.go`: classify TSDK events into a global scope and merge global/current-account reads.
- Modify `app_test.go`: cover persistent and memory-fallback cross-account behavior, ordering, and limits.
- Modify `frontend/src/lib/events.ts`: provide immutable newest-first TSDK filtering.
- Modify `frontend/src/lib/events.test.ts`: test filtering and deterministic order.
- Create `frontend/src/components/TsdkBlockDialog.test.tsx`: test initial ordering and prop-driven live updates.
- Modify `frontend/src/components/TsdkBlockDialog.tsx`: consume the tested ordering helper.
- Preserve `resources/wmpf/button.js`: retain its existing uncommitted immediate startup event; do not alter interception hosts or feature IDs.

### Task 1: Queue QQ Logs Until The Socket Opens

**Files:**
- Modify: `internal/runtime/qqws/host_asset_test.go`
- Modify: `resources/qq/qq-host.js:24-39,213-220,370-379`

- [x] **Step 1: Write the failing host-asset behavior test**

Add a test that runs `resources/qq/qq-host.js` in Node's real `vm` module with a mocked QQ socket task. The inline harness must assert that the immediate TSDK init packet is absent before `onOpen`, then present exactly once after `onOpen`:

```go
func TestQQHostQueuesTSDKInitLogUntilSocketOpen(t *testing.T) {
	assetPath, err := filepath.Abs("../../../resources/qq/qq-host.js")
	if err != nil {
		t.Fatalf("resolve QQ host asset: %v", err)
	}
	script := `
const fs = require("node:fs");
const vm = require("node:vm");
const source = fs.readFileSync(process.argv[1], "utf8");
const sent = [];
let open;
const socket = {
  onOpen(fn) { open = fn; }, onMessage() {}, onError() {}, onClose() {},
  send(value) { sent.push(JSON.parse(value.data)); }, close() {}
};
const sandbox = {
  qq: {
    connectSocket() { return socket; }, request() {},
    getSystemInfoSync() { return { AppPlatform: "qq" }; }
  },
  console: { log() {} }, Date, Math, JSON,
  setTimeout() { return 1; }, clearTimeout() {},
  setInterval() { return 1; }, clearInterval() {}
};
vm.runInNewContext(source, sandbox);
const initLogs = () => sent.filter((packet) =>
  packet.type === "log" && packet.payload.message.includes("TSDK-BLOCK v2 init")
);
if (initLogs().length !== 0) throw new Error("init log sent before socket open");
open();
if (initLogs().length !== 1) throw new Error("init log count after open: " + initLogs().length);
`
	command := exec.Command("node", "-e", script, assetPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("QQ host queue behavior failed: %v\n%s", err, output)
	}
}
```

Add `os/exec` and `path/filepath` imports.

- [x] **Step 2: Run the test and verify RED**

Run: `go test ./internal/runtime/qqws -run TestQQHostQueuesTSDKInitLogUntilSocketOpen -count=1`

Expected: FAIL with `init log count after open: 0`, proving the existing pre-open TSDK packet is discarded.

- [x] **Step 3: Implement a bounded pending-log queue**

Add queue state and a fixed capacity:

```javascript
var pendingLogLimit = 50;

var state = {
  url: defaults.url,
  phase: "idle",
  seq: 0,
  socket: null,
  socketKind: null,
  reconnectTimer: null,
  heartbeatTimer: null,
  readyPollTimer: null,
  manualStop: false,
  lastHelloAck: null,
  lastGameCtlReady: null,
  lastError: null,
  clientId: "qq-farm-" + Math.random().toString(36).slice(2, 10),
  pendingLogs: []
};
```

Replace `sendLog` and add `flushPendingLogs`:

```javascript
function sendLog(level, message, extra) {
  logLocal(level, message, extra);
  var payload = {
    level: level,
    message: message,
    extra: extra === undefined ? null : extra
  };
  if (sendTyped("log", payload)) return true;
  if (state.pendingLogs.length >= pendingLogLimit) state.pendingLogs.shift();
  state.pendingLogs.push(payload);
  return false;
}

function flushPendingLogs() {
  while (state.pendingLogs.length > 0) {
    if (!sendTyped("log", state.pendingLogs[0])) return;
    state.pendingLogs.shift();
  }
}
```

Flush after the connection's own status log and hello packet so the backend session exists before queued runtime logs arrive:

```javascript
sendLog("info", "socket connected", { kind: kind, url: state.url });
sendHello();
flushPendingLogs();
startHeartbeat();
```

- [x] **Step 4: Run the focused test and verify GREEN**

Run: `go test ./internal/runtime/qqws -run 'TestQQHost(AssetIsAdaptedForFarmGo|QueuesTSDKInitLogUntilSocketOpen)' -count=1`

Expected: PASS for both host asset tests.

- [x] **Step 5: Commit the transport change**

```bash
git add resources/qq/qq-host.js internal/runtime/qqws/host_asset_test.go
git commit -m "fix: retain TSDK logs until QQ socket connects"
```

### Task 2: Store TSDK Events Globally And Merge Reads

**Files:**
- Modify: `app_test.go`
- Modify: `app.go:328-337,897-927,3356-3384,4496-4529`

- [x] **Step 1: Write failing persistent-scope and memory-fallback tests**

Add tests using the wished-for `recordRuntimeLogEvent` entry point:

```go
func TestAppRuntimeEventsShareTSDKLogsAcrossAccounts(t *testing.T) {
	ctx := context.Background()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(ctx, t.TempDir())
	if err != nil { t.Fatalf("open store: %v", err) }
	defer store.Close()
	app.store = store

	app.currentAccountKey = "gid:10001"
	app.recordRuntimeLogEvent(eventbus.Event{Source: "qq_ws", Type: "qqhost.log", Message: "[TSDK-BLOCK] TSDK-BLOCK v2 init"})
	app.recordEvent(eventbus.Event{Source: "account", Type: "private.one", Message: "account one"})
	app.currentAccountKey = "gid:10002"
	app.recordRuntimeLogEvent(eventbus.Event{Source: "qq_ws", Type: "qqhost.log", Message: "[TSDK-BLOCK] Layer1 interceptors ready"})
	app.recordEvent(eventbus.Event{Source: "account", Type: "private.two", Message: "account two"})

	events := app.RuntimeEvents(3)
	if len(events) != 3 { t.Fatalf("events = %#v", events) }
	if events[0].Type != "private.two" || !strings.Contains(events[1].Message, "Layer1") || !strings.Contains(events[2].Message, "v2 init") {
		t.Fatalf("events are not merged newest first: %#v", events)
	}
	for _, event := range events {
		if event.Type == "private.one" { t.Fatalf("leaked account-one event: %#v", events) }
	}
}

func TestAppRuntimeEventsMemoryFallbackIncludesGlobalTSDK(t *testing.T) {
	app := newAuthorizedTestApp(t)
	app.currentAccountKey = "gid:10001"
	app.recordRuntimeLogEvent(eventbus.Event{Type: "qqhost.log", Message: "[TSDK-BLOCK] TSDK-BLOCK v2 init"})
	app.currentAccountKey = "gid:10002"

	events := app.RuntimeEvents(10)
	if len(events) != 1 || !strings.Contains(events[0].Message, "v2 init") {
		t.Fatalf("global memory event missing: %#v", events)
	}
}
```

- [x] **Step 2: Run the tests and verify RED**

Run: `go test . -run 'TestAppRuntimeEvents(ShareTSDKLogsAcrossAccounts|MemoryFallbackIncludesGlobalTSDK)' -count=1`

Expected: build FAIL because `recordRuntimeLogEvent` is undefined.

- [x] **Step 3: Implement classification and global recording**

Add a private scope constant and helper near the embedded asset declarations:

```go
const tsdkRuntimeAccountKey = "global:tsdk"

func isTSDKBlockRuntimeEvent(event eventbus.Event) bool {
	return event.Type == "qqhost.log" && strings.Contains(event.Message, "[TSDK-BLOCK]")
}

func (a *App) recordRuntimeLogEvent(event eventbus.Event) {
	if isTSDKBlockRuntimeEvent(event) {
		a.recordEventForAccount(tsdkRuntimeAccountKey, event)
		return
	}
	a.recordEvent(event)
}
```

Route both ingress paths through it:

```go
qqLink.OnLog(func(level, message string, data map[string]any) {
	app.recordRuntimeLogEvent(eventbus.Event{
		Level: eventbus.Level(level), Source: "qq_ws", Type: "qqhost.log",
		Message: message, Data: data,
	})
})
```

In `handleCDPHostLogEvent`, replace the final `recordEvent` call with `recordRuntimeLogEvent` while retaining its payload validation.

- [x] **Step 4: Merge current and global stored events before limiting**

Add a stable merge helper:

```go
func mergeRuntimeEvents(limit int, groups ...[]eventbus.Event) []eventbus.Event {
	merged := make([]eventbus.Event, 0)
	for _, group := range groups { merged = append(merged, group...) }
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].ID != merged[j].ID { return merged[i].ID > merged[j].ID }
		return merged[i].Timestamp.After(merged[j].Timestamp)
	})
	if len(merged) > limit { merged = merged[:limit] }
	return merged
}
```

For persistent reads, fetch at most `limit` from the active account and the global TSDK scope, then merge and limit:

```go
ctx := a.contextOrBackground()
accountEvents, accountErr := a.store.ListRuntimeEventsForAccount(ctx, accountKey, limit)
tsdkEvents, tsdkErr := a.store.ListRuntimeEventsForAccount(ctx, tsdkRuntimeAccountKey, limit)
if accountErr == nil && tsdkErr == nil {
	return mergeRuntimeEvents(limit, accountEvents, tsdkEvents)
}
if accountErr != nil { a.lastErr = accountErr } else { a.lastErr = tsdkErr }
```

In the memory fallback, include an entry when its scope equals either `accountKey` or `tsdkRuntimeAccountKey`. Preserve the reverse iteration so it remains newest first and stop at `limit`.

- [x] **Step 5: Run focused backend tests and verify GREEN**

Run: `go test . -run 'TestAppRuntimeEvents' -count=1`

Expected: PASS, including existing account-isolation tests and the two new global TSDK tests.

- [x] **Step 6: Commit the backend change**

```bash
git add app.go app_test.go
git commit -m "fix: expose TSDK runtime logs across accounts"
```

### Task 3: Render Newest TSDK Events First And Update In Place

**Files:**
- Modify: `frontend/src/lib/events.test.ts`
- Modify: `frontend/src/lib/events.ts:127-160`
- Create: `frontend/src/components/TsdkBlockDialog.test.tsx`
- Modify: `frontend/src/components/TsdkBlockDialog.tsx:3-23`

- [x] **Step 1: Write the failing ordering-helper test**

Import `tsdkBlockEventsNewestFirst` and add:

```typescript
it('filters TSDK logs and returns newest first without mutating input', () => {
  const events = [
    { id: 1, timestamp: '2026-08-02T10:00:00+08:00', level: 'info', source: 'qq_ws', type: 'qqhost.log', message: '[TSDK-BLOCK] start' },
    { id: 3, timestamp: '2026-08-02T10:00:02+08:00', level: 'info', source: 'qq_ws', type: 'qqhost.log', message: '[TSDK-BLOCK] ready' },
    { id: 2, timestamp: '2026-08-02T10:00:01+08:00', level: 'info', source: 'account', type: 'account.confirmed', message: 'confirmed' },
  ];
  const originalIds = events.map((event) => event.id);

  expect(tsdkBlockEventsNewestFirst(events).map((event) => event.id)).toEqual([3, 1]);
  expect(events.map((event) => event.id)).toEqual(originalIds);
});
```

- [x] **Step 2: Write the failing open-dialog update test**

Create `TsdkBlockDialog.test.tsx` using `react-test-renderer`:

```tsx
import { act, create } from 'react-test-renderer';
import { describe, expect, it } from 'vitest';
import { TsdkBlockDialog } from './TsdkBlockDialog';

const oldEvent = { id: 1, timestamp: '2026-08-02T10:00:00+08:00', level: 'info', source: 'qq_ws', type: 'qqhost.log', message: '[TSDK-BLOCK] start' };
const newEvent = { id: 2, timestamp: '2026-08-02T10:00:01+08:00', level: 'info', source: 'qq_ws', type: 'qqhost.log', message: '[TSDK-BLOCK] ready' };

it('keeps an open dialog live and places a new event first', () => {
  const renderer = create(<TsdkBlockDialog open events={[oldEvent]} onClose={() => undefined} />);
  expect(JSON.stringify(renderer.toJSON())).toContain('[TSDK-BLOCK] start');

  act(() => renderer.update(<TsdkBlockDialog open events={[newEvent, oldEvent]} onClose={() => undefined} />));
  const rendered = JSON.stringify(renderer.toJSON());
  expect(rendered.indexOf('[TSDK-BLOCK] ready')).toBeLessThan(rendered.indexOf('[TSDK-BLOCK] start'));
});
```

- [x] **Step 3: Run both tests and verify RED**

Run: `npm test -- src/lib/events.test.ts src/components/TsdkBlockDialog.test.tsx` from `frontend`.

Expected: FAIL because `tsdkBlockEventsNewestFirst` does not exist and the dialog currently reverses backend order.

- [x] **Step 4: Implement immutable newest-first ordering**

Add to `events.ts`:

```typescript
export function tsdkBlockEventsNewestFirst(events: RuntimeEventDto[]) {
  return events
    .filter(isTsdkBlockEvent)
    .map((event, index) => ({ event, index }))
    .sort((left, right) => {
      const idOrder = Number(right.event.id) - Number(left.event.id);
      if (Number.isFinite(idOrder) && idOrder !== 0) return idOrder;
      const timeOrder = new Date(String(right.event.timestamp)).getTime() - new Date(String(left.event.timestamp)).getTime();
      return Number.isFinite(timeOrder) && timeOrder !== 0 ? timeOrder : left.index - right.index;
    })
    .map(({ event }) => event);
}
```

Update `TsdkBlockDialog.tsx` to import the helper and replace the existing filter/reverse expression:

```typescript
const tsdkEvents = tsdkBlockEventsNewestFirst(events);
```

- [x] **Step 5: Run frontend tests and verify GREEN**

Run: `npm test -- src/lib/events.test.ts src/components/TsdkBlockDialog.test.tsx` from `frontend`.

Expected: PASS for both files.

- [x] **Step 6: Commit the frontend change**

```bash
git add frontend/src/lib/events.ts frontend/src/lib/events.test.ts frontend/src/components/TsdkBlockDialog.tsx frontend/src/components/TsdkBlockDialog.test.tsx
git commit -m "fix: show live TSDK logs newest first"
```

### Task 4: Verify The Full Change

**Files:**
- Verify: `resources/wmpf/button.js`
- Verify: all modified test and production files

- [x] **Step 1: Confirm both runtime assets emit startup and success evidence**

Run:

```powershell
rg -n "TSDK-BLOCK v2 init|Layer1 interceptors ready|Fetch interception enabled" resources/qq/qq-host.js resources/wmpf/button.js internal/runtime/wmpf/link.go
```

Expected: QQ and WMPF assets contain the immediate init and Layer1-ready messages; CDP link contains Fetch interception success.

- [x] **Step 2: Run focused Go tests**

Run:

```powershell
go test ./internal/runtime/qqws -count=1
go test . -run 'TestAppRuntimeEvents|TestAppCDPLinkConfigIncludesEmbeddedButtonScript' -count=1
```

Expected: both commands exit 0 with `ok` and no failures.

- [x] **Step 3: Run all frontend tests and build**

Run from `frontend`:

```powershell
npm test
npm run build
```

Expected: Vitest reports zero failing tests; TypeScript and Vite build exit 0.

- [x] **Step 4: Run full Go regression tests**

Run: `go test ./... -count=1`

Expected: all packages exit 0.

- [x] **Step 5: Inspect the final diff and account boundaries**

Run:

```powershell
git diff --check
git status --short
git diff -- app.go app_test.go resources/qq/qq-host.js internal/runtime/qqws/host_asset_test.go frontend/src/lib/events.ts frontend/src/lib/events.test.ts frontend/src/components/TsdkBlockDialog.tsx frontend/src/components/TsdkBlockDialog.test.tsx
```

Expected: no whitespace errors; ordinary account events still use `recordEvent`; only classified `[TSDK-BLOCK]` events use `global:tsdk`; no interception host or feature-ID changes are introduced.

## Execution Results

- QQ host pre-connection delivery test passed against the real injected asset.
- TSDK global-scope persistent and memory-fallback tests passed.
- Frontend focused tests passed, followed by all 419 frontend tests.
- `npm run build` completed successfully.
- `go test ./... -count=1` completed successfully for every Go package.
