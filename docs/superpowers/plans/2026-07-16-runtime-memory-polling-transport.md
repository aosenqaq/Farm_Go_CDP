# Runtime Memory Polling Transport Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace recurring Wails binding reads with bounded same-origin poll requests so the WebView2 renderer no longer grows linearly.

**Architecture:** A typed Go handler under the existing Wails AssetServer serves dashboard, land, and dog-guard snapshots. A typed frontend client and one abortable serial polling controller consume those resources; Wails remains in use for finite commands and infrequent reads. Automatic QQ patch inspection is keyed to a runtime instance instead of running forever.

**Tech Stack:** Go 1.24, `net/http`, Wails v2 AssetServer, React 18, TypeScript, Vitest, PowerShell process sampling.

---

## File Structure

- Create `poll_api.go`: poll response types, App-backed snapshot service, route validation, authorization, and JSON responses.
- Create `poll_api_test.go`: handler protocol, snapshot composition, authorization, land/dog routing, and bounded error tests.
- Modify `main.go`: route `/farm-api/poll/` through the poll handler before existing `/farm-assets/` handling.
- Create `frontend/src/lib/pollApi.ts`: typed same-origin fetch client and response validation.
- Create `frontend/src/lib/pollApi.test.ts`: URL, abort, HTTP error, and shape-validation tests.
- Create `frontend/src/lib/pollingController.ts`: one-in-flight polling lifecycle with abort and failure-transition suppression.
- Create `frontend/src/lib/pollingController.test.ts`: fake-timer lifecycle tests.
- Modify `frontend/src/AuthorizedApp.tsx`: one dashboard poll, fetch-based dog-guard polling, and once-per-runtime patch inspection.
- Modify `frontend/src/App.test.tsx`: source-level regression checks for removal of recurring Wails calls and the permanent patch interval.
- Modify `frontend/src/views/AssetsLandView.tsx`: use the poll client and abortable controller for initial/full/delta land reads.
- Modify `frontend/src/views/AssetsLandView.test.tsx`: verify land reads no longer import or call Wails bindings.
- Modify `frontend/src/views/lib/landDetailsPolling.ts`: retain only delta merging after the generic controller takes lifecycle ownership.
- Modify `frontend/src/views/lib/landDetailsPolling.test.ts`: remove the obsolete lifecycle test and retain merge behavior coverage.
- Modify `scripts/measure-land-webview2-memory.ps1`: record the root and all WebView2 process groups, working set, handles, and threads needed by the acceptance test.

### Task 1: Poll HTTP Protocol

**Files:**
- Create: `poll_api_test.go`
- Create: `poll_api.go`

- [ ] **Step 1: Write failing route, method, query, and authorization tests**

Create a fake service in `poll_api_test.go` whose methods return small deterministic values. Add table-driven tests using `httptest.NewRecorder` for:

```go
type fakeRuntimePollService struct {
	authorized     bool
	dashboardTabs  []string
	landRevisions  []string
	dogGuardReads  int
}

func (s *fakeRuntimePollService) Authorize() error {
	if !s.authorized {
		return ErrLicenseRequired
	}
	return nil
}

func (s *fakeRuntimePollService) Dashboard(activeTab string) dashboardPollResponse {
	s.dashboardTabs = append(s.dashboardTabs, activeTab)
	return dashboardPollResponse{Events: []eventbus.Event{}}
}

func (s *fakeRuntimePollService) Land(revision string) farm.LandDetailsDeltaPayload {
	s.landRevisions = append(s.landRevisions, revision)
	return farm.LandDetailsDeltaPayload{Full: revision == "", Revision: "r1", Lands: []farm.LandDetailsItem{}, RemovedLandIDs: []int{}}
}

func (s *fakeRuntimePollService) DogGuard() social.DogGuardState {
	s.dogGuardReads++
	return social.DogGuardState{Results: []social.DogGuardRow{}}
}

func TestRuntimePollHandlerRejectsUnsupportedRequests(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		want   int
	}{
		{name: "method", method: http.MethodPost, target: "/farm-api/poll/dashboard?activeTab=workspace", want: http.StatusMethodNotAllowed},
		{name: "route", method: http.MethodGet, target: "/farm-api/poll/unknown", want: http.StatusNotFound},
		{name: "tab", method: http.MethodGet, target: "/farm-api/poll/dashboard?activeTab=invalid", want: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.target, nil)
			newRuntimePollHandler(&fakeRuntimePollService{authorized: true}).ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, test.want, recorder.Body.String())
			}
			if recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q", recorder.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestRuntimePollHandlerRejectsUnauthorizedRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/farm-api/poll/dashboard?activeTab=workspace", nil)
	newRuntimePollHandler(&fakeRuntimePollService{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() > 256 {
		t.Fatalf("unauthorized response is unbounded: %d bytes", recorder.Body.Len())
	}
}

func TestWritePollJSONReturnsBoundedEncodingError(t *testing.T) {
	recorder := httptest.NewRecorder()
	writePollJSON(recorder, http.StatusOK, map[string]any{"unsupported": make(chan int)})
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() > 256 {
		t.Fatalf("encoding error response is unbounded: %d bytes", recorder.Body.Len())
	}
}
```

- [ ] **Step 2: Run the focused Go test and verify RED**

Run: `go test . -run '^TestRuntimePollHandler' -count=1`

Expected: FAIL because `newRuntimePollHandler` and `fakeRuntimePollService` do not exist.

- [ ] **Step 3: Implement the minimal protocol shell**

In `poll_api.go`, import `bytes`, `encoding/json`, `net/http`, `strings`, `Farm_Go/internal/eventbus`, `Farm_Go/internal/farm`, and `Farm_Go/internal/farm/social`. Define the fixed prefix, valid tabs, JSON errors, and a small service interface:

```go
const runtimePollPrefix = "/farm-api/poll/"

var runtimePollTabs = map[string]struct{}{
	"workspace": {}, "automation": {}, "assets": {}, "social": {},
	"account": {}, "guard": {}, "logs": {}, "message_push": {}, "settings": {},
}

type runtimePollService interface {
	Authorize() error
	Dashboard(activeTab string) dashboardPollResponse
	Land(revision string) farm.LandDetailsDeltaPayload
	DogGuard() social.DogGuardState
}

type dashboardPollResponse struct {
	Events []eventbus.Event `json:"events"`
}

func writePollJSON(w http.ResponseWriter, status int, value any) {
	var payload bytes.Buffer
	if err := json.NewEncoder(&payload).Encode(value); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("{\"error\":\"poll response encoding failed\"}\n"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(payload.Bytes())
}

func newRuntimePollHandler(service runtimePollService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writePollJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		resource := strings.TrimPrefix(r.URL.Path, runtimePollPrefix)
		if resource != "dashboard" && resource != "land" && resource != "dog-guard" {
			writePollJSON(w, http.StatusNotFound, map[string]string{"error": "poll resource not found"})
			return
		}
		if err := service.Authorize(); err != nil {
			writePollJSON(w, http.StatusUnauthorized, map[string]string{"error": "authorization required"})
			return
		}
		switch resource {
		case "dashboard":
			activeTab := r.URL.Query().Get("activeTab")
			if _, ok := runtimePollTabs[activeTab]; !ok {
				writePollJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid activeTab"})
				return
			}
			writePollJSON(w, http.StatusOK, service.Dashboard(activeTab))
		case "land":
			writePollJSON(w, http.StatusOK, service.Land(r.URL.Query().Get("revision")))
		case "dog-guard":
			writePollJSON(w, http.StatusOK, service.DogGuard())
		}
	})
}
```

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run: `go test . -run '^TestRuntimePollHandler' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the protocol**

```powershell
git add poll_api.go poll_api_test.go
git commit -m "feat: add bounded runtime poll routes"
```

### Task 2: App-Backed Poll Snapshots

**Files:**
- Modify: `poll_api.go`
- Modify: `poll_api_test.go`

- [ ] **Step 1: Write failing dashboard, land, and dog-guard service tests**

Add tests that use `newAuthorizedTestApp(t)` and `newAppWithFakeRuntime`:

```go
func TestAppRuntimePollDashboardScopesRunStatisticsToWorkspace(t *testing.T) {
	app, _ := newAppWithFakeRuntime(t, map[string]any{})
	service := newAppRuntimePollService(app)
	workspace := service.Dashboard("workspace")
	if workspace.RunStatistics == nil {
		t.Fatal("workspace response omitted run statistics")
	}
	settings := service.Dashboard("settings")
	if settings.RunStatistics != nil {
		t.Fatalf("settings response included run statistics: %#v", settings.RunStatistics)
	}
	if workspace.Status.Target != "qq_ws" || workspace.Events == nil || workspace.AutomationState.FeatureGroups == nil {
		t.Fatalf("incomplete dashboard response: %#v", workspace)
	}
}

func TestAppRuntimePollLandDelegatesRevision(t *testing.T) {
	app, _ := newAppWithFakeRuntime(t, map[string]any{"gameCtl.getFarmStatus": map[string]any{"grids": []any{}}})
	service := newAppRuntimePollService(app)
	first := service.Land("")
	if !first.Full || first.Revision == "" {
		t.Fatalf("first land response = %#v", first)
	}
	second := service.Land(first.Revision)
	if second.Full || second.Revision != first.Revision {
		t.Fatalf("unchanged land response = %#v", second)
	}
}
```

Also assert that `service.DogGuard().Results` is non-nil and `service.Authorize()` fails for `NewApp()` without `authorizationForTests`.

- [ ] **Step 2: Run the service tests and verify RED**

Run: `go test . -run '^TestAppRuntimePoll' -count=1`

Expected: FAIL because `newAppRuntimePollService` is undefined and the initial `dashboardPollResponse` does not yet contain the required snapshot fields.

- [ ] **Step 3: Implement the App adapter and response type**

Expand `dashboardPollResponse` and add the App adapter to `poll_api.go` with imports for `automation`, `farmruntime`, and `guard`:

```go
type dashboardPollResponse struct {
	Status          farmruntime.Status       `json:"status"`
	GuardStatus     GuardianStatusDTO        `json:"guardStatus"`
	BindingStatus   guard.AutoBindOwnerResult `json:"bindingStatus"`
	Events          []eventbus.Event          `json:"events"`
	AutomationState automation.State          `json:"automationState"`
	RunStatistics   *farm.RunStatistics       `json:"runStatistics,omitempty"`
	PatchStatus     QQDebugPatchStatus         `json:"patchStatus"`
}

type appRuntimePollService struct{ app *App }

func newAppRuntimePollService(app *App) runtimePollService {
	return &appRuntimePollService{app: app}
}

func (s *appRuntimePollService) Authorize() error {
	return s.app.requireAuthorized("RuntimePoll")
}

func (s *appRuntimePollService) Dashboard(activeTab string) dashboardPollResponse {
	response := dashboardPollResponse{
		Status:          s.app.RuntimeStatus(),
		GuardStatus:     s.app.GuardianStatus(),
		BindingStatus:   s.app.AutoBindHostProcess(),
		Events:          s.app.RuntimeEvents(120),
		AutomationState: s.app.FarmAutomationState(),
		PatchStatus:     s.app.QQDebugPatchStatus(),
	}
	if activeTab == "workspace" {
		statistics := s.app.FarmWorkspaceRunStatistics()
		response.RunStatistics = &statistics
	}
	return response
}

func (s *appRuntimePollService) Land(revision string) farm.LandDetailsDeltaPayload {
	return s.app.FarmLandDetailsSince(revision)
}

func (s *appRuntimePollService) DogGuard() social.DogGuardState {
	return s.app.FarmSocialDogGuardState()
}
```

- [ ] **Step 4: Run service and handler tests**

Run: `go test . -run '^(TestAppRuntimePoll|TestRuntimePollHandler)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit snapshot composition**

```powershell
git add poll_api.go poll_api_test.go
git commit -m "feat: compose runtime poll snapshots"
```

### Task 3: AssetServer Integration

**Files:**
- Modify: `main.go`
- Modify: `poll_api_test.go`

- [ ] **Step 1: Write a failing middleware routing test**

Add a test proving poll requests do not fall through and ordinary assets do:

```go
func TestAssetMiddlewareRoutesOnlyFarmPollPrefix(t *testing.T) {
	app := newAuthorizedTestApp(t)
	fallthrough := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallthrough++
		w.WriteHeader(http.StatusNoContent)
	})
	handler := newAssetMiddleware(app)(next)

	pollRecorder := httptest.NewRecorder()
	handler.ServeHTTP(pollRecorder, httptest.NewRequest(http.MethodGet, "/farm-api/poll/dog-guard", nil))
	if pollRecorder.Code != http.StatusOK || fallthrough != 0 {
		t.Fatalf("poll status=%d fallthrough=%d", pollRecorder.Code, fallthrough)
	}

	assetRecorder := httptest.NewRecorder()
	handler.ServeHTTP(assetRecorder, httptest.NewRequest(http.MethodGet, "/index.html", nil))
	if assetRecorder.Code != http.StatusNoContent || fallthrough != 1 {
		t.Fatalf("asset status=%d fallthrough=%d", assetRecorder.Code, fallthrough)
	}
}
```

- [ ] **Step 2: Run the test and verify RED**

Run: `go test . -run '^TestAssetMiddlewareRoutesOnlyFarmPollPrefix$' -count=1`

Expected: FAIL because `newAssetMiddleware` is undefined.

- [ ] **Step 3: Extract and wire the middleware**

Add to `main.go`:

```go
func newAssetMiddleware(app *App) func(http.Handler) http.Handler {
	pollHandler := newRuntimePollHandler(newAppRuntimePollService(app))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, runtimePollPrefix) {
				pollHandler.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/farm-assets/") && farm.ServeLocalGameConfigImage(w, r) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

Replace the inline `AssetServer.Middleware` function with `Middleware: newAssetMiddleware(app)`.

- [ ] **Step 4: Run all root package tests**

Run: `go test . -count=1`

Expected: PASS.

- [ ] **Step 5: Commit AssetServer integration**

```powershell
git add main.go poll_api_test.go
git commit -m "feat: serve runtime polls through asset middleware"
```

### Task 4: Typed Frontend Poll Client

**Files:**
- Create: `frontend/src/lib/pollApi.test.ts`
- Create: `frontend/src/lib/pollApi.ts`

- [ ] **Step 1: Write failing fetch and validation tests**

Test the exact URLs, abort propagation, non-2xx errors, and malformed payloads:

```ts
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PollApiError, pollDashboard, pollDogGuard, pollLand } from './pollApi';

afterEach(() => vi.unstubAllGlobals());

describe('poll API', () => {
  it('encodes dashboard and land queries on the embedded origin', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({
        status: {}, guardStatus: {}, bindingStatus: {}, events: [], automationState: {}, patchStatus: {},
      }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        full: false, revision: 'r 2', lands: [], removedLandIds: [],
      }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    const signal = new AbortController().signal;
    await pollDashboard('workspace', signal);
    await pollLand('r 1', signal);
    expect(fetchMock.mock.calls[0][0]).toBe('/farm-api/poll/dashboard?activeTab=workspace');
    expect(fetchMock.mock.calls[1][0]).toBe('/farm-api/poll/land?revision=r+1');
    expect(fetchMock.mock.calls[0][1]).toEqual({ cache: 'no-store', signal });
  });

  it('returns a status-bearing error and rejects malformed responses', async () => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(new Response('{"error":"authorization required"}', { status: 401 }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 })));
    await expect(pollDogGuard(new AbortController().signal)).rejects.toMatchObject<Partial<PollApiError>>({ status: 401 });
    await expect(pollLand('', new AbortController().signal)).rejects.toThrow('Invalid land poll response');
  });
});
```

- [ ] **Step 2: Run the client test and verify RED**

Run: `npm --prefix frontend test -- src/lib/pollApi.test.ts`

Expected: FAIL because `pollApi.ts` does not exist.

- [ ] **Step 3: Implement the typed client**

Define `PollApiError`, `DashboardPollResponse`, `LandPollResponse`, and these functions in `pollApi.ts`:

```ts
import type { Tab } from '../components/AppShell';
import type { RuntimeEventDto } from './events';
import type { QQPatchStatusLike } from './startupInjection';
import type { FarmAutomationState } from '../views/AutomationView';
import type { HostBindingStatusDto } from '../views/GuardView';
import type { RuntimeStatusDto, WorkspaceRunStatistics } from '../views/OverviewView';
import type { DogGuardState } from '../views/SocialView';

export type DashboardPollResponse = {
  status: RuntimeStatusDto;
  guardStatus: Record<string, unknown>;
  bindingStatus: HostBindingStatusDto;
  events: RuntimeEventDto[];
  automationState: FarmAutomationState;
  runStatistics?: WorkspaceRunStatistics;
  patchStatus: QQPatchStatusLike;
};

export type PollLandRecord = Record<string, unknown> & { id: string; landId: number };

export type LandPollResponse = {
  full: boolean;
  revision: string;
  status?: string;
  message?: string;
  farmType?: string;
  totalGrids?: number;
  lands: PollLandRecord[];
  removedLandIds: number[];
  actions?: Array<Record<string, unknown>>;
  runtimeError?: string;
};

export class PollApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
    this.name = 'PollApiError';
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

async function readPoll(path: string, signal: AbortSignal): Promise<unknown> {
  const response = await fetch(path, { cache: 'no-store', signal });
  if (!response.ok) {
    throw new PollApiError(`Poll request failed with HTTP ${response.status}`, response.status);
  }
  return response.json();
}

export async function pollDashboard(activeTab: Tab, signal: AbortSignal): Promise<DashboardPollResponse> {
  const value = await readPoll(`/farm-api/poll/dashboard?${new URLSearchParams({ activeTab })}`, signal);
  if (!isRecord(value) || !isRecord(value.status) || !isRecord(value.guardStatus) ||
      !isRecord(value.bindingStatus) || !Array.isArray(value.events) ||
      !isRecord(value.automationState) || !isRecord(value.patchStatus)) {
    throw new Error('Invalid dashboard poll response');
  }
  return value as DashboardPollResponse;
}

export async function pollLand(revision: string, signal: AbortSignal): Promise<LandPollResponse> {
  const value = await readPoll(`/farm-api/poll/land?${new URLSearchParams({ revision })}`, signal);
  if (!isRecord(value) || typeof value.full !== 'boolean' || typeof value.revision !== 'string' ||
      !Array.isArray(value.lands) || !Array.isArray(value.removedLandIds)) {
    throw new Error('Invalid land poll response');
  }
  return value as LandPollResponse;
}

export async function pollDogGuard(signal: AbortSignal): Promise<DogGuardState> {
  const value = await readPoll('/farm-api/poll/dog-guard', signal);
  if (!isRecord(value) || !Array.isArray(value.results)) {
    throw new Error('Invalid dog guard poll response');
  }
  return value as DogGuardState;
}
```

- [ ] **Step 4: Run the client test and TypeScript build**

Run: `npm --prefix frontend test -- src/lib/pollApi.test.ts`

Expected: PASS.

Run: `npm --prefix frontend run build`

Expected: PASS.

- [ ] **Step 5: Commit the client**

```powershell
git add frontend/src/lib/pollApi.ts frontend/src/lib/pollApi.test.ts
git commit -m "feat: add typed frontend poll client"
```

### Task 5: Abortable Serial Polling Controller

**Files:**
- Create: `frontend/src/lib/pollingController.test.ts`
- Create: `frontend/src/lib/pollingController.ts`

- [ ] **Step 1: Write failing lifecycle tests**

Use fake timers to prove immediate execution, no overlap, abort, late-result suppression, duplicate-start protection, and one report per failure transition:

```ts
it('allows one request at a time and aborts it on stop', async () => {
  vi.useFakeTimers();
  let resolveRead: ((value: string) => void) | undefined;
  let capturedSignal: AbortSignal | undefined;
  const read = vi.fn((signal: AbortSignal) => {
    capturedSignal = signal;
    return new Promise<string>((resolve) => { resolveRead = resolve; });
  });
  const apply = vi.fn();
  const controller = createPollingController({ read, apply, reportError: vi.fn(), intervalMs: 1000 });
  controller.start();
  controller.start();
  await vi.advanceTimersByTimeAsync(3000);
  expect(read).toHaveBeenCalledTimes(1);
  controller.stop();
  expect(capturedSignal?.aborted).toBe(true);
  resolveRead?.('late');
  await Promise.resolve();
  expect(apply).not.toHaveBeenCalled();
});
```

Add a second test where two consecutive failures call `reportError` once, a later success resets the transition, and a following failure reports again.

- [ ] **Step 2: Run the test and verify RED**

Run: `npm --prefix frontend test -- src/lib/pollingController.test.ts`

Expected: FAIL because `createPollingController` is undefined.

- [ ] **Step 3: Implement the controller**

Implement `start`, `refresh`, and `stop` with one `AbortController`, a generation counter, a `failed` flag, and an optional `shouldStop(error)` predicate. `start` calls `refresh` immediately and schedules the normal interval. `refresh` is a no-op while a request is active. `stop` clears the interval, increments the generation, aborts the active request, and prevents late apply/error calls.

```ts
export type PollingController = { start(): void; refresh(): void; stop(): void };

export function createPollingController<T>(options: {
  read: (signal: AbortSignal) => Promise<T>;
  apply: (value: T) => void;
  reportError: (error: unknown) => void;
  intervalMs: number;
  shouldStop?: (error: unknown) => boolean;
}): PollingController {
  let timer: ReturnType<typeof setInterval> | undefined;
  let request: AbortController | undefined;
  let generation = 0;
  let running = false;
  let failed = false;
  const stop = () => {
    running = false;
    generation += 1;
    if (timer) clearInterval(timer);
    timer = undefined;
    request?.abort();
    request = undefined;
  };
  const refresh = () => {
    if (!running || request) return;
    const requestGeneration = generation;
    request = new AbortController();
    const signal = request.signal;
    void options.read(signal).then((value) => {
      if (running && generation === requestGeneration && !signal.aborted) {
        failed = false;
        options.apply(value);
      }
    }).catch((error) => {
      if (!running || generation !== requestGeneration || signal.aborted) return;
      if (!failed) options.reportError(error);
      failed = true;
      if (options.shouldStop?.(error)) stop();
    }).finally(() => {
      if (generation === requestGeneration) request = undefined;
    });
  };
  return {
    start() {
      if (running) return;
      running = true;
      refresh();
      timer = setInterval(refresh, options.intervalMs);
    },
    refresh,
    stop,
  };
}
```

- [ ] **Step 4: Run lifecycle tests and the frontend build**

Run: `npm --prefix frontend test -- src/lib/pollingController.test.ts`

Expected: PASS.

Run: `npm --prefix frontend run build`

Expected: PASS.

- [ ] **Step 5: Commit the controller**

```powershell
git add frontend/src/lib/pollingController.ts frontend/src/lib/pollingController.test.ts
git commit -m "feat: add abortable serial polling controller"
```

### Task 6: Authorized Dashboard, Dog Guard, and Patch Lifecycle

**Files:**
- Modify: `frontend/src/App.test.tsx`
- Modify: `frontend/src/AuthorizedApp.tsx`

- [ ] **Step 1: Write failing source and patch-key regression tests**

In `App.test.tsx`, add a source test that requires `pollDashboard`, `pollDogGuard`, and `createPollingController`, and rejects recurring `RuntimeStatus`, `RuntimeEvents`, `GuardianStatus`, `AutoBindHostProcess`, and `QQDebugPatchStatus` calls inside interval callbacks. Export and test these pure patch-check helpers from `AuthorizedApp.tsx`:

```ts
export function runtimePatchCheckKey(status: RuntimeStatusDto) {
  if (status.target !== 'qq_ws' || status.ready !== true || status.connected !== true) return '';
  return `${status.target}:${status.instanceId || 'ready'}`;
}

export function runtimePatchCheckDecision(checkedKey: string, status: RuntimeStatusDto) {
  const nextCheckedKey = runtimePatchCheckKey(status);
  return {
    nextCheckedKey,
    shouldRun: nextCheckedKey !== '' && nextCheckedKey !== checkedKey,
  };
}

it('keys one automatic patch check to each ready runtime instance', () => {
  const ready = { target: 'qq_ws', phase: 'ready', ready: true, connected: true, instanceId: 'one' } as const;
  const stopped = { target: 'qq_ws', phase: 'idle', ready: false, connected: false } as const;
  expect(runtimePatchCheckDecision('', ready)).toEqual({ nextCheckedKey: 'qq_ws:one', shouldRun: true });
  expect(runtimePatchCheckDecision('qq_ws:one', ready)).toEqual({ nextCheckedKey: 'qq_ws:one', shouldRun: false });
  expect(runtimePatchCheckDecision('qq_ws:one', stopped)).toEqual({ nextCheckedKey: '', shouldRun: false });
  expect(runtimePatchCheckDecision('', ready).shouldRun).toBe(true);
});
```

- [ ] **Step 2: Run the App test and verify RED**

Run: `npm --prefix frontend test -- src/App.test.tsx`

Expected: FAIL because the poll client is unused and `runtimePatchCheckKey` is missing.

- [ ] **Step 3: Replace dashboard and dog-guard recurring bindings**

In `AuthorizedApp.tsx`:

- remove recurring imports of `RuntimeStatus`, `RuntimeEvents`, `GuardianStatus`, and `AutoBindHostProcess`;
- import `pollDashboard`, `pollDogGuard`, `PollApiError`, and `createPollingController`;
- keep `FarmAutomationState` and `FarmSocialDogGuardState` only for finite post-command refreshes;
- keep one dashboard controller in a ref so existing `refreshStatus()` command completions trigger `controller.refresh()`;
- capture the account generation in the dashboard `read` callback and apply the whole validated response only if it is still current;
- set status, normalized guard status, binding status, events, automation state, optional run statistics, and patch status from one response;
- stop dashboard or dog-guard polling on HTTP `401` using `error instanceof PollApiError && error.status === 401`;
- replace the social-tab dog-guard interval with a tab-scoped controller using the existing 1800 ms cadence.

The dashboard effect must have this shape:

```ts
useEffect(() => {
  const polling = createPollingController({
    intervalMs: 2500,
    read: async (signal) => ({
      token: accountScopeGenerationRef.current!.capture(),
      value: await pollDashboard(activeTab, signal),
    }),
    apply: ({ token, value }) => {
      if (!accountScopeGenerationRef.current!.isCurrent(token)) return;
      setStatus(value.status);
      setGuardStatus(normalizeGuardianStatus(value.guardStatus));
      setBindingStatus(value.bindingStatus);
      setEvents(value.events);
      setAutomationState(value.automationState);
      if (value.runStatistics) setRunStatistics(value.runStatistics);
      setPatchStatus(value.patchStatus);
    },
    reportError: console.error,
    shouldStop: (error) => error instanceof PollApiError && error.status === 401,
  });
  dashboardPollingRef.current = polling;
  polling.start();
  return () => {
    if (dashboardPollingRef.current === polling) dashboardPollingRef.current = null;
    polling.stop();
  };
}, [activeTab]);
```

- [ ] **Step 4: Make automatic patch inspection finite**

Replace the 10-second interval effect with one execution per `runtimePatchCheckDecision`. Store `nextCheckedKey` in a ref before starting the async operation; the empty decision key rearms the same instance after readiness is lost. Preserve the existing diagnostic, install, script-hash comparison, and injection-dialog behavior. Do not schedule an interval and do not call `QQDebugPatchStatus` after the automatic install; the next dashboard snapshot supplies passive status.

- [ ] **Step 5: Run focused tests and build**

Run: `npm --prefix frontend test -- src/App.test.tsx src/lib/pollApi.test.ts src/lib/pollingController.test.ts`

Expected: PASS.

Run: `npm --prefix frontend run build`

Expected: PASS.

- [ ] **Step 6: Commit AuthorizedApp migration**

```powershell
git add frontend/src/AuthorizedApp.tsx frontend/src/App.test.tsx
git commit -m "fix: move global polling off Wails bindings"
```

### Task 7: Land Polling Migration

**Files:**
- Modify: `frontend/src/views/AssetsLandView.test.tsx`
- Modify: `frontend/src/views/AssetsLandView.tsx`
- Modify: `frontend/src/views/lib/landDetailsPolling.test.ts`
- Modify: `frontend/src/views/lib/landDetailsPolling.ts`

- [ ] **Step 1: Write failing land transport regressions**

Update the Assets view source test to assert that the file imports `pollLand` and `createPollingController`, and no longer contains `FarmLandDetails` or `FarmLandDetailsSince`. Remove those two obsolete functions from the Wails module mock. Keep the existing merge test in `landDetailsPolling.test.ts` and delete only the lifecycle test that belongs to the new generic controller.

- [ ] **Step 2: Run the land tests and verify RED**

Run: `npm --prefix frontend test -- src/views/AssetsLandView.test.tsx src/views/lib/landDetailsPolling.test.ts`

Expected: FAIL because `AssetsLandView.tsx` still imports and calls both Wails land methods.

- [ ] **Step 3: Use one controller for initial, delta, and manual full refreshes**

In `AssetsLandView.tsx`:

- import `pollLand` and `createPollingController`;
- remove `FarmLandDetails`, `FarmLandDetailsSince`, `createLandDetailsPolling`, and `landRefreshInFlight`;
- keep `mergeLandDetailsDelta`;
- keep a `landPollingRef` and set `landRevision.current = ''` before a manual full refresh;
- start the controller only while `activeTab === 'lands'`;
- pass the current revision to `pollLand` so the first empty revision returns a full delta;
- merge every result through `mergeLandDetailsDelta` and clear loading/error state;
- abort and suppress late results during tab changes and unmount.

Reduce `landDetailsPolling.ts` to the `LandRecord`, snapshot/delta types, and `mergeLandDetailsDelta` function. The generic controller now owns timers and cancellation.

- [ ] **Step 4: Run land tests and build**

Run: `npm --prefix frontend test -- src/views/AssetsLandView.test.tsx src/views/lib/landDetailsPolling.test.ts src/lib/pollingController.test.ts`

Expected: PASS.

Run: `npm --prefix frontend run build`

Expected: PASS.

- [ ] **Step 5: Commit land migration**

```powershell
git add frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx frontend/src/views/lib/landDetailsPolling.ts frontend/src/views/lib/landDetailsPolling.test.ts
git commit -m "fix: move land polling off Wails bindings"
```

### Task 8: Memory Sampling Output and Full Regression

**Files:**
- Modify: `scripts/measure-land-webview2-memory.ps1`

- [ ] **Step 1: Extend the sampler fields before using it for acceptance**

For every sample, preserve the existing renderer private-byte total and add:

```powershell
rootPrivateBytes, rootWorkingSetBytes, rootHandles, rootThreads,
rendererWorkingSetBytes, rendererHandles, rendererThreads,
webviewProcessCount, totalPrivateBytes, totalWorkingSetBytes
```

Resolve descendants on every sample so renderer restarts cannot silently invalidate the observation. Keep `rendererCount` and throw when no renderer exists.

- [ ] **Step 2: Verify the script parses and its parameter guard works**

Run:

```powershell
$tokens = $null
$errors = $null
[System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path 'scripts/measure-land-webview2-memory.ps1'), [ref]$tokens, [ref]$errors) | Out-Null
if ($errors.Count -ne 0) { $errors | Format-List; exit 1 }
try {
  & scripts/measure-land-webview2-memory.ps1 -FarmGoProcessId $PID -Samples 1 -IntervalSeconds 1 -OutputPath "$env:TEMP/farm-go-invalid-sample.csv"
  throw 'Sampler unexpectedly accepted a process without WebView2 descendants.'
} catch {
  if ($_.Exception.Message -notmatch 'No WebView2 renderer was found') { throw }
}
```

Expected: parser succeeds; the second command fails with `No WebView2 renderer was found` because the PowerShell process has no renderer descendants.

- [ ] **Step 3: Run all automated verification**

Run:

```powershell
go test ./... -count=1
npm --prefix frontend test
npm --prefix frontend run build
npm --prefix frontend run build:protected
git diff --check
```

Expected: every command exits 0, all Go and Vitest tests pass, and both frontend builds complete without TypeScript or obfuscation errors.

- [ ] **Step 4: Commit the sampler**

```powershell
git add scripts/measure-land-webview2-memory.ps1
git commit -m "test: capture complete Farm Go memory metrics"
```

### Task 9: Protected EXE and Ten-Minute Runtime Acceptance

**Files:**
- Generated: the newest directory matching `release/farm-go-v1.0.5-*`
- Generated: `workspace-memory.csv` and `tab-polls-memory.csv` below that release directory

- [ ] **Step 1: Load the protected-release workflow**

Before packaging, read and follow the applicable `farm-exe-release-packaging` skill. Use the existing secure KAuth release environment; verify the five required variables are present without printing their values:

```powershell
'KAUTH_PROGRAM_ID','KAUTH_PROGRAM_SECRET','KAUTH_MERCHANT_PUBLIC_KEY','FARM_GO_VERSION_NO','FARM_GO_VERSION_NAME' |
  ForEach-Object { if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($_, 'Process'))) { throw "Missing release environment variable: $_" } }
```

- [ ] **Step 2: Build the pre-VMP protected executable**

Run:

```powershell
& scripts/build-protected-release.ps1 -Version '1.0.5'
$releaseDir = Get-ChildItem -LiteralPath 'release' -Directory -Filter 'farm-go-v1.0.5-*' |
  Sort-Object LastWriteTime -Descending |
  Select-Object -First 1
if (-not $releaseDir) { throw 'The v1.0.5 release directory was not created.' }
$releaseExe = Join-Path $releaseDir.FullName 'pre-vmp\Farm_Go.exe'
```

Expected: `$releaseExe`, `PRE-VMP-MANIFEST.json`, and `VMP-GUI-CHECKLIST.md` exist below `$releaseDir`; the manifest SHA-256 matches the EXE.

- [ ] **Step 3: Run the workspace observation**

Start the new pre-VMP EXE with the normal authorized application data. Wait until the authorized workspace is visible, then run the sampler in the same PowerShell session:

```powershell
$farmProcess = Start-Process -FilePath $releaseExe -PassThru
$workspaceCsv = Join-Path $releaseDir.FullName 'workspace-memory.csv'
Read-Host 'Press Enter after the authorized workspace is visible' | Out-Null
& scripts/measure-land-webview2-memory.ps1 -FarmGoProcessId $farmProcess.Id -Samples 120 -IntervalSeconds 5 -OutputPath $workspaceCsv
```

Keep the default workspace open and do not perform manual actions during this ten-minute sample.

- [ ] **Step 4: Calculate the final-eight-minute acceptance values**

Import the CSV, drop the first 24 samples, and calculate the acceptance values with:

```powershell
function Get-SlopeMBPerSecond([double[]]$values, [double[]]$seconds) {
  $xMean = ($seconds | Measure-Object -Average).Average
  $yMean = ($values | Measure-Object -Average).Average
  $numerator = 0.0
  $denominator = 0.0
  for ($index = 0; $index -lt $values.Count; $index++) {
    $dx = $seconds[$index] - $xMean
    $numerator += $dx * ($values[$index] - $yMean)
    $denominator += $dx * $dx
  }
  return $numerator / $denominator
}
function Assert-MemoryAcceptance([string]$CsvPath) {
  $steady = @(Import-Csv -LiteralPath $CsvPath | Select-Object -Skip 24)
  if ($steady.Count -lt 96) { throw "Expected 96 steady samples, got $($steady.Count)." }
  $x = @(0..($steady.Count - 1) | ForEach-Object { $_ * 5.0 })
  $rendererMB = @($steady | ForEach-Object { [double]$_.privateBytes / 1MB })
  $rootMB = @($steady | ForEach-Object { [double]$_.rootPrivateBytes / 1MB })
  $rendererDeltaMB = $rendererMB[-1] - $rendererMB[0]
  $rendererSlope = Get-SlopeMBPerSecond $rendererMB $x
  $rootSlope = Get-SlopeMBPerSecond $rootMB $x
  if (@($steady.rendererCount | Select-Object -Unique).Count -ne 1 -or [int]$steady[0].rendererCount -ne 1) {
    throw 'Renderer count changed during the steady sample.'
  }
  if ($rendererDeltaMB -gt 64) { throw "Renderer delta exceeded 64 MB: $rendererDeltaMB" }
  if ($rendererSlope -gt 0.05) { throw "Renderer slope exceeded 0.05 MB/s: $rendererSlope" }
  if ($rootSlope -gt 0.05) { throw "Root slope exceeded 0.05 MB/s: $rootSlope" }
  [pscustomobject]@{ RendererDeltaMB = $rendererDeltaMB; RendererSlopeMBps = $rendererSlope; RootSlopeMBps = $rootSlope }
}
$workspaceMetrics = Assert-MemoryAcceptance $workspaceCsv
$workspaceMetrics | Format-List
```

Expected:

- renderer count remains 1;
- renderer final-minus-start is at most 64 MB;
- renderer fitted slope is at most 0.05 MB/second;
- root process private bytes have no positive linear trend comparable to the original renderer trend.

- [ ] **Step 5: Run the tab-scoped polling observation**

Close the first EXE normally, start `$releaseExe` again, open `好友社交` for at least two minutes, and then open `资产与土地 > 土地` for the remaining time. Run:

```powershell
$tabProcess = Start-Process -FilePath $releaseExe -PassThru
$tabPollsCsv = Join-Path $releaseDir.FullName 'tab-polls-memory.csv'
Read-Host 'Open Friends Social, then press Enter; switch to Assets and Land after two minutes' | Out-Null
& scripts/measure-land-webview2-memory.ps1 -FarmGoProcessId $tabProcess.Id -Samples 120 -IntervalSeconds 5 -OutputPath $tabPollsCsv
$tabMetrics = Assert-MemoryAcceptance $tabPollsCsv
$tabMetrics | Format-List
```

Expected: the same thresholds pass and dog-guard and land data visibly update.

- [ ] **Step 6: Verify affected workflows**

Using the new EXE, verify manual refresh, tab switching, runtime reconnect, account scope switching, one automatic patch inspection per ready runtime instance, land refresh after an action, dog-guard start/stop, and manual patch retry. Confirm Task Manager shows a stable seven-process group rather than growing process counts.

- [ ] **Step 7: Record evidence and final repository state**

Run:

```powershell
git status --short
git log -10 --oneline
Get-FileHash -Algorithm SHA256 -LiteralPath $releaseExe
```

Report the release path, hash, workspace and tab-poll slopes, start/end memory, automated test totals, and any untracked generated release artifacts. Do not claim completion unless all acceptance thresholds pass.
