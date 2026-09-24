# Desktop Floating Status Window Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an always-on-top independent floating status window that shows runtime/guardian/authorization/automation health plus run statistics, with tray show/hide and light actions (open main window, collapse, refresh, start/stop automation).

**Architecture:** Keep one Farm_Go process. The main Wails window remains the full console. A Go-managed secondary Win32 + WebView2 tool window loads a lightweight React float entry (`#/float`). Because Wails bindings exist only on the main window, the float window talks to the shared `App` through a process-local loopback HTTP bridge on `127.0.0.1`. Preferences (visible/collapsed/x/y) live in SQLite settings under the global account. Tray gains a show/hide float item; closing the float only hides it.

**Tech Stack:** Go 1.25, Wails v2, `github.com/energye/systray`, Win32 + WebView2 (`golang.org/x/sys/windows`, `github.com/wailsapp/go-webview2`), React 18, TypeScript, Vite, Vitest, existing App bindings/status helpers.

**Spec:** `docs/superpowers/specs/2026-07-15-desktop-floating-status-window-design.md`

---

## File Structure

- Create: `floating_window_state.go` - preference DTO, defaults, clamp helpers, pure state transitions.
- Create: `floating_window_state_test.go` - preference normalization and clamp tests.
- Create: `internal/storage/floating_window.go` - load/save floating window preferences in settings table.
- Create: `internal/storage/floating_window_test.go` - storage round-trip tests.
- Create: `floating_window_api.go` - loopback HTTP bridge handlers over existing App methods.
- Create: `floating_window_api_test.go` - HTTP bridge contract tests with unauthorized/authorized cases.
- Create: `floating_window_events.go` - port workbench task event helpers used by the snapshot.
- Create: `floating_window_host.go` - host interface + no-op host.
- Create: `floating_window_host_windows.go` - Win32 always-on-top tool window + WebView2 host.
- Create: `floating_window_host_windows_test.go` - pure host helper tests.
- Create: `floating_window_controller.go` - App show/hide/toggle/state APIs and lifecycle glue.
- Create: `floating_window_controller_test.go` - controller tests with fake host/store.
- Create: `frontend/src/float/FloatingStatusApp.tsx` - float root UI and actions.
- Create: `frontend/src/float/FloatingStatusApp.test.tsx` - UI state/action tests.
- Create: `frontend/src/float/api.ts` - same-origin float API client.
- Create: `frontend/src/float/api.test.ts` - client path tests.
- Create: `frontend/src/float/labels.ts` - connection/guardian/authorization/automation labels.
- Modify: `frontend/src/main.tsx` - branch main app vs float app by `location.hash`.
- Modify: `frontend/src/style.css` - compact float card styles.
- Modify: `tray.go` - add floating window menu item.
- Modify: `desktop_lifecycle_test.go` - cover new tray routing.
- Modify: `authorization_gate.go` / `authorization_gate_test.go` - bootstrap policies for float management methods.
- Modify: `app.go` - wire controller/host/store fields and startup restore / shutdown destroy.
- Modify: `main.go` - share embedded frontend assets with the float local server if needed.

Important constraint:
- Do **not** call `window.go.main.App.*` from the float WebView. Use the loopback bridge.
- Main-window tray callbacks still call normal Go methods on `App`.
- Bind only `127.0.0.1`. Prefer same-origin relative `/api/float/*` paths.

---

### Task 1: Floating Window Preference Model And Storage

**Files:**
- Create: `floating_window_state.go`
- Create: `floating_window_state_test.go`
- Create: `internal/storage/floating_window.go`
- Create: `internal/storage/floating_window_test.go`

- [ ] **Step 1: Write failing preference tests**

```go
// floating_window_state_test.go
package main

import "testing"

func TestDefaultFloatingWindowState(t *testing.T) {
	state := DefaultFloatingWindowState()
	if state.Visible {
		t.Fatal("default visible should be false")
	}
	if state.Collapsed {
		t.Fatal("default collapsed should be false")
	}
	if state.Width != 320 || state.ExpandedHeight != 220 || state.CollapsedHeight != 56 {
		t.Fatalf("unexpected default size: %#v", state)
	}
}

func TestClampFloatingWindowPositionKeepsWindowOnWorkArea(t *testing.T) {
	state := FloatingWindowState{X: -4000, Y: 9000, Width: 320, ExpandedHeight: 220}
	work := Rect{X: 0, Y: 0, Width: 1920, Height: 1040}
	got := ClampFloatingWindowState(state, work)
	if got.X < work.X || got.Y < work.Y {
		t.Fatalf("clamped off work area: %#v", got)
	}
	if got.X+got.Width > work.X+work.Width {
		t.Fatalf("right edge outside work area: %#v", got)
	}
	if got.Y+got.ExpandedHeight > work.Y+work.Height {
		t.Fatalf("bottom edge outside work area: %#v", got)
	}
}

func TestApplyFloatingWindowVisibility(t *testing.T) {
	state := DefaultFloatingWindowState()
	next := ApplyFloatingWindowVisibility(state, true)
	if !next.Visible {
		t.Fatal("expected visible")
	}
	next = ApplyFloatingWindowVisibility(next, false)
	if next.Visible {
		t.Fatal("expected hidden")
	}
}
```

```go
// internal/storage/floating_window_test.go
package storage

import (
	"context"
	"testing"
)

func TestFloatingWindowSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	in := FloatingWindowSettings{Visible: true, Collapsed: true, X: 120, Y: 80}
	if err := store.SaveFloatingWindowSettings(ctx, in); err != nil {
		t.Fatal(err)
	}
	out, err := store.LoadFloatingWindowSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Visible || !out.Collapsed || out.X != 120 || out.Y != 80 {
		t.Fatalf("out = %#v", out)
	}
}
```

- [ ] **Step 2: Run tests to verify RED**

```powershell
go test . -run "Test(DefaultFloatingWindowState|ClampFloatingWindowPosition|ApplyFloatingWindowVisibility)" -count=1
go test ./internal/storage -run TestFloatingWindowSettingsRoundTrip -count=1
```

Expected: FAIL because types/functions do not exist.

- [ ] **Step 3: Implement preference model and storage**

```go
// floating_window_state.go
package main

type Rect struct {
	X, Y, Width, Height int
}

type FloatingWindowState struct {
	Visible         bool `json:"visible"`
	Collapsed       bool `json:"collapsed"`
	X               int  `json:"x"`
	Y               int  `json:"y"`
	Width           int  `json:"width"`
	ExpandedHeight  int  `json:"expandedHeight"`
	CollapsedHeight int  `json:"collapsedHeight"`
}

func DefaultFloatingWindowState() FloatingWindowState {
	return FloatingWindowState{
		Width:           320,
		ExpandedHeight:  220,
		CollapsedHeight: 56,
	}
}

func ApplyFloatingWindowVisibility(state FloatingWindowState, visible bool) FloatingWindowState {
	state.Visible = visible
	return state
}

func ClampFloatingWindowState(state FloatingWindowState, work Rect) FloatingWindowState {
	if state.Width <= 0 {
		state.Width = 320
	}
	if state.ExpandedHeight <= 0 {
		state.ExpandedHeight = 220
	}
	if state.CollapsedHeight <= 0 {
		state.CollapsedHeight = 56
	}
	height := state.ExpandedHeight
	if state.Collapsed {
		height = state.CollapsedHeight
	}
	if work.Width <= 0 || work.Height <= 0 {
		return state
	}
	if state.X < work.X {
		state.X = work.X
	}
	if state.Y < work.Y {
		state.Y = work.Y
	}
	if state.X+state.Width > work.X+work.Width {
		state.X = work.X + work.Width - state.Width
	}
	if state.Y+height > work.Y+work.Height {
		state.Y = work.Y + work.Height - height
	}
	if state.X < work.X {
		state.X = work.X
	}
	if state.Y < work.Y {
		state.Y = work.Y
	}
	return state
}

func DefaultFloatingWindowPosition(work Rect) (x, y int) {
	state := DefaultFloatingWindowState()
	margin := 24
	x = work.X + work.Width - state.Width - margin
	y = work.Y + work.Height - state.ExpandedHeight - margin
	if x < work.X {
		x = work.X
	}
	if y < work.Y {
		y = work.Y
	}
	return x, y
}
```

```go
// internal/storage/floating_window.go
package storage

import (
	"context"
	"strconv"
)

type FloatingWindowSettings struct {
	Visible   bool `json:"visible"`
	Collapsed bool `json:"collapsed"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
}

func DefaultFloatingWindowSettings() FloatingWindowSettings {
	return FloatingWindowSettings{}
}

func (s *Store) LoadFloatingWindowSettings(ctx context.Context) (FloatingWindowSettings, error) {
	values, err := s.loadSettingsForAccount(ctx, GlobalSettingsAccountKey, []string{
		"floatingWindow.visible",
		"floatingWindow.collapsed",
		"floatingWindow.x",
		"floatingWindow.y",
	})
	if err != nil {
		return FloatingWindowSettings{}, err
	}
	settings := DefaultFloatingWindowSettings()
	if value := values["floatingWindow.visible"]; value != "" {
		settings.Visible = value == "true"
	}
	if value := values["floatingWindow.collapsed"]; value != "" {
		settings.Collapsed = value == "true"
	}
	if value := values["floatingWindow.x"]; value != "" {
		settings.X, err = strconv.Atoi(value)
		if err != nil {
			return FloatingWindowSettings{}, err
		}
	}
	if value := values["floatingWindow.y"]; value != "" {
		settings.Y, err = strconv.Atoi(value)
		if err != nil {
			return FloatingWindowSettings{}, err
		}
	}
	return settings, nil
}

func (s *Store) SaveFloatingWindowSettings(ctx context.Context, settings FloatingWindowSettings) error {
	return s.saveSettingsForAccount(ctx, GlobalSettingsAccountKey, map[string]string{
		"floatingWindow.visible":   strconv.FormatBool(settings.Visible),
		"floatingWindow.collapsed": strconv.FormatBool(settings.Collapsed),
		"floatingWindow.x":         strconv.Itoa(settings.X),
		"floatingWindow.y":         strconv.Itoa(settings.Y),
	})
}
```

- [ ] **Step 4: Run tests to verify GREEN**

```powershell
go test . -run "Test(DefaultFloatingWindowState|ClampFloatingWindowPosition|ApplyFloatingWindowVisibility)" -count=1
go test ./internal/storage -run TestFloatingWindowSettingsRoundTrip -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add floating_window_state.go floating_window_state_test.go internal/storage/floating_window.go internal/storage/floating_window_test.go
git commit -m "feat: add floating window preference model and storage"
```

### Task 2: Float Snapshot DTO And Loopback API

**Files:**
- Create: `floating_window_api.go`
- Create: `floating_window_api_test.go`
- Create: `floating_window_events.go`
- Modify: `authorization_gate.go`
- Modify: `authorization_gate_test.go`
- Modify: `app.go` (fields for float API server)

- [ ] **Step 1: Write failing API tests**

```go
func TestFloatingWindowSnapshotUnauthorizedStillReturnsLicense(t *testing.T) {
	app := NewApp()
	app.authorizationForTests = false

	snapshot := app.FloatingWindowSnapshot()
	if snapshot.Authorization.Authorized {
		t.Fatal("expected unauthorized snapshot")
	}
	if snapshot.Automation.Enabled {
		t.Fatal("automation should be disabled when unauthorized")
	}
}

func TestFloatingWindowAPIStartStopRequiresAuthorization(t *testing.T) {
	app := NewApp()
	app.authorizationForTests = false
	server := httptest.NewServer(app.floatingWindowHTTPHandler())
	defer server.Close()

	resp, err := http.Post(server.URL+"/api/float/automation/start", "application/json", bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestFloatingWindowAPISnapshotJSONShape(t *testing.T) {
	app := NewApp()
	app.authorizationForTests = true
	server := httptest.NewServer(app.floatingWindowHTTPHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/float/snapshot")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"runtime", "guardian", "authorization", "automation", "runStatistics", "latestTask", "window"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("missing key %s in %#v", key, body)
		}
	}
}
```

- [ ] **Step 2: Run RED**

```powershell
go test . -run "TestFloatingWindow(SnapshotUnauthorized|APIStartStop|APISnapshotJSONShape)" -count=1
```

Expected: FAIL.

- [ ] **Step 3: Implement snapshot builder, event helpers, and HTTP bridge**

```go
// floating_window_api.go
package main

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"

	"Farm_Go/internal/farm"
	"Farm_Go/internal/license"
	farmruntime "Farm_Go/internal/runtime"
)

type FloatingLatestTaskDTO struct {
	Time    string `json:"time"`
	Name    string `json:"name"`
	Result  string `json:"result"`
	Level   string `json:"level"`
	Present bool   `json:"present"`
}

type FloatingAutomationDTO struct {
	Enabled       bool   `json:"enabled"`
	RunningTaskID string `json:"runningTaskId,omitempty"`
}

type FloatingWindowSnapshot struct {
	Runtime       farmruntime.Status    `json:"runtime"`
	Guardian      GuardianStatusDTO     `json:"guardian"`
	Authorization license.Status        `json:"authorization"`
	Automation    FloatingAutomationDTO `json:"automation"`
	RunStatistics farm.RunStatistics    `json:"runStatistics"`
	LatestTask    FloatingLatestTaskDTO `json:"latestTask"`
	Window        FloatingWindowState   `json:"window"`
	Error         string                `json:"error,omitempty"`
}

func (a *App) FloatingWindowSnapshot() FloatingWindowSnapshot {
	snapshot := FloatingWindowSnapshot{
		Authorization: a.LicenseStatus(),
		Window:        a.currentFloatingWindowState(),
	}
	if a.requireAuthorized("FloatingWindowSnapshot") != nil {
		return snapshot
	}
	snapshot.Runtime = a.RuntimeStatus()
	snapshot.Guardian = a.GuardianStatus()
	scheduler := a.FarmAutomationSchedulerState()
	snapshot.Automation = FloatingAutomationDTO{Enabled: scheduler.Enabled, RunningTaskID: scheduler.RunningTaskID}
	snapshot.RunStatistics = a.FarmWorkspaceRunStatistics()
	snapshot.LatestTask = a.latestFloatingTask()
	return snapshot
}

func (a *App) latestFloatingTask() FloatingLatestTaskDTO {
	for _, event := range a.RuntimeEvents(40) {
		if !isFloatingWorkbenchTaskEvent(event) {
			continue
		}
		return FloatingLatestTaskDTO{
			Time:    event.Timestamp.Format("15:04:05"),
			Name:    floatingTaskName(event),
			Result:  floatingTaskResult(event),
			Level:   string(event.Level),
			Present: true,
		}
	}
	return FloatingLatestTaskDTO{Present: false, Name: "任务日志", Result: "暂无任务结果"}
}

func (a *App) floatingWindowHTTPHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/float/snapshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, a.FloatingWindowSnapshot())
	})
	mux.HandleFunc("/api/float/automation/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if a.requireAuthorized("StartFarmAutomationScheduler") != nil {
			http.Error(w, ErrLicenseRequired.Error(), http.StatusForbidden)
			return
		}
		writeJSON(w, http.StatusOK, a.StartFarmAutomationScheduler())
	})
	mux.HandleFunc("/api/float/automation/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if a.requireAuthorized("StopFarmAutomationScheduler") != nil {
			http.Error(w, ErrLicenseRequired.Error(), http.StatusForbidden)
			return
		}
		writeJSON(w, http.StatusOK, a.StopFarmAutomationScheduler())
	})
	mux.HandleFunc("/api/float/window", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, a.GetFloatingWindowState())
		case http.MethodPost:
			var body FloatingWindowState
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, a.SaveFloatingWindowState(body))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/float/open-main", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		a.ShowMainWindow()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/float/hide", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		a.HideFloatingWindow()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	return mux
}

type floatAPIServer struct {
	server *http.Server
	addr   string
}

func (a *App) startFloatingWindowServer(assets fs.FS) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(assets)))
	mux.Handle("/api/float/", a.floatingWindowHTTPHandler())
	// If ServeMux nesting is awkward, register the float routes directly on mux.
	server := &http.Server{Handler: mux}
	a.floatAPI = &floatAPIServer{server: server, addr: ln.Addr().String()}
	go server.Serve(ln)
	return "http://" + a.floatAPI.addr, nil
}

func (a *App) stopFloatingWindowServer() {
	if a.floatAPI == nil || a.floatAPI.server == nil {
		return
	}
	_ = a.floatAPI.server.Close()
	a.floatAPI = nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
```

In `floating_window_events.go`, port frontend rules from `frontend/src/lib/events.ts`:
- `isFloatingWorkbenchTaskEvent`
- `floatingTaskName`
- `floatingTaskResult`

Register bootstrap policies in `authorization_gate.go`:

```go
"ShowFloatingWindow":      authorizationBootstrap,
"HideFloatingWindow":      authorizationBootstrap,
"ToggleFloatingWindow":    authorizationBootstrap,
"GetFloatingWindowState":  authorizationBootstrap,
"SaveFloatingWindowState": authorizationBootstrap,
"OpenMainWindowFromFloat": authorizationBootstrap,
"FloatingWindowSnapshot":  authorizationBootstrap,
```

Keep start/stop automation on existing required policies. Update `authorization_gate_test.go` bootstrap map.

Note on server wiring: prefer one same-origin loopback server that serves both embedded `frontend/dist` and `/api/float/*`. Float page navigates to `http://127.0.0.1:<port>/#/float` and calls relative `/api/float/...` paths (no CORS, no Wails bindings).

- [ ] **Step 4: Run GREEN**

```powershell
go test . -run "TestFloatingWindow(SnapshotUnauthorized|APIStartStop|APISnapshotJSONShape)|TestAuthorization" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add floating_window_api.go floating_window_api_test.go floating_window_events.go authorization_gate.go authorization_gate_test.go app.go
git commit -m "feat: add floating window snapshot API and loopback bridge"
```

### Task 3: Floating Window Controller And Fake Host

**Files:**
- Create: `floating_window_host.go`
- Create: `floating_window_controller.go`
- Create: `floating_window_controller_test.go`
- Modify: `app.go`

- [ ] **Step 1: Write failing controller tests**

```go
type fakeFloatHost struct {
	shown      bool
	url        string
	lastState  FloatingWindowState
	showErr    error
	showCalls  int
	hideCalls  int
	destroyCnt int
}

func (h *fakeFloatHost) Show(url string, state FloatingWindowState) error {
	h.showCalls++
	if h.showErr != nil {
		return h.showErr
	}
	h.shown = true
	h.url = url
	h.lastState = state
	return nil
}
func (h *fakeFloatHost) Hide() error                  { h.hideCalls++; h.shown = false; return nil }
func (h *fakeFloatHost) Destroy() error               { h.destroyCnt++; h.shown = false; return nil }
func (h *fakeFloatHost) IsVisible() bool              { return h.shown }
func (h *fakeFloatHost) ApplyState(state FloatingWindowState) error {
	h.lastState = state
	return nil
}
func (h *fakeFloatHost) WorkArea() Rect { return Rect{Width: 1920, Height: 1040} }

func TestShowFloatingWindowPersistsVisibleAndShowsHost(t *testing.T) {
	host := &fakeFloatHost{}
	app := NewApp()
	app.authorizationForTests = true
	app.floatHost = host
	app.floatUIBaseURL = "http://127.0.0.1:9"

	state := app.ShowFloatingWindow()
	if !state.Visible || !host.shown {
		t.Fatalf("state=%#v host=%#v", state, host)
	}
	if host.url == "" || !strings.Contains(host.url, "#/float") {
		t.Fatalf("url = %q", host.url)
	}
}

func TestHideFloatingWindowPersistsHidden(t *testing.T) {
	host := &fakeFloatHost{shown: true}
	app := NewApp()
	app.floatHost = host
	app.floatState = FloatingWindowState{Visible: true, Width: 320, ExpandedHeight: 220, CollapsedHeight: 56}

	state := app.HideFloatingWindow()
	if state.Visible || host.shown || host.hideCalls != 1 {
		t.Fatalf("state=%#v host=%#v", state, host)
	}
}

func TestToggleFloatingWindow(t *testing.T) {
	host := &fakeFloatHost{}
	app := NewApp()
	app.floatHost = host
	app.floatUIBaseURL = "http://127.0.0.1:9"
	app.floatState = DefaultFloatingWindowState()

	if !app.ToggleFloatingWindow().Visible {
		t.Fatal("first toggle should show")
	}
	if app.ToggleFloatingWindow().Visible {
		t.Fatal("second toggle should hide")
	}
}

func TestHideFloatingWindowDoesNotExitApplication(t *testing.T) {
	runtime := &fakeDesktopRuntime{}
	app := NewApp()
	app.desktopRuntime = runtime
	app.floatHost = &fakeFloatHost{shown: true}
	app.floatState = FloatingWindowState{Visible: true}

	app.HideFloatingWindow()
	if runtime.quitCalls != 0 {
		t.Fatalf("quitCalls = %d", runtime.quitCalls)
	}
}
```

- [ ] **Step 2: Run RED**

```powershell
go test . -run "Test(ShowFloatingWindow|HideFloatingWindow|ToggleFloatingWindow)" -count=1
```

Expected: FAIL.

- [ ] **Step 3: Implement controller + host interface**

```go
// floating_window_host.go
package main

type floatingWindowHost interface {
	Show(url string, state FloatingWindowState) error
	Hide() error
	Destroy() error
	IsVisible() bool
	ApplyState(state FloatingWindowState) error
	WorkArea() Rect
}

type noopFloatingWindowHost struct{}

func (noopFloatingWindowHost) Show(string, FloatingWindowState) error { return nil }
func (noopFloatingWindowHost) Hide() error                            { return nil }
func (noopFloatingWindowHost) Destroy() error                         { return nil }
func (noopFloatingWindowHost) IsVisible() bool                        { return false }
func (noopFloatingWindowHost) ApplyState(FloatingWindowState) error   { return nil }
func (noopFloatingWindowHost) WorkArea() Rect                         { return Rect{} }
```

```go
// floating_window_controller.go
package main

import (
	"fmt"
	"sync"

	"Farm_Go/internal/storage"
)

func (a *App) GetFloatingWindowState() FloatingWindowState {
	a.floatMu.Lock()
	defer a.floatMu.Unlock()
	return a.floatState
}

func (a *App) SaveFloatingWindowState(input FloatingWindowState) FloatingWindowState {
	a.floatMu.Lock()
	defer a.floatMu.Unlock()
	state := a.normalizeFloatingStateLocked(input)
	a.floatState = state
	a.persistFloatingStateLocked(state)
	if a.floatHost != nil && a.floatHost.IsVisible() {
		_ = a.floatHost.ApplyState(state)
	}
	return state
}

func (a *App) ShowFloatingWindow() FloatingWindowState {
	a.floatMu.Lock()
	defer a.floatMu.Unlock()
	state := a.normalizeFloatingStateLocked(a.floatState)
	if state.X == 0 && state.Y == 0 {
		state.X, state.Y = DefaultFloatingWindowPosition(a.floatHost.WorkArea())
	}
	state.Visible = true
	state = ClampFloatingWindowState(state, a.floatHost.WorkArea())
	url := buildFloatNavigateURL(a.floatUIBaseURL)
	if err := a.floatHost.Show(url, state); err != nil {
		state.Visible = false
		a.floatState = state
		a.persistFloatingStateLocked(state)
		return state
	}
	a.floatState = state
	a.persistFloatingStateLocked(state)
	return state
}

func (a *App) HideFloatingWindow() FloatingWindowState {
	a.floatMu.Lock()
	defer a.floatMu.Unlock()
	if a.floatHost != nil {
		_ = a.floatHost.Hide()
	}
	state := a.floatState
	state.Visible = false
	a.floatState = state
	a.persistFloatingStateLocked(state)
	return state
}

func (a *App) ToggleFloatingWindow() FloatingWindowState {
	if a.GetFloatingWindowState().Visible || (a.floatHost != nil && a.floatHost.IsVisible()) {
		return a.HideFloatingWindow()
	}
	return a.ShowFloatingWindow()
}

func (a *App) OpenMainWindowFromFloat() {
	a.ShowMainWindow()
}

func (a *App) currentFloatingWindowState() FloatingWindowState {
	a.floatMu.Lock()
	defer a.floatMu.Unlock()
	return a.floatState
}

func (a *App) normalizeFloatingStateLocked(input FloatingWindowState) FloatingWindowState {
	base := DefaultFloatingWindowState()
	base.Visible = input.Visible
	base.Collapsed = input.Collapsed
	base.X = input.X
	base.Y = input.Y
	if a.floatHost != nil {
		return ClampFloatingWindowState(base, a.floatHost.WorkArea())
	}
	return base
}

func (a *App) persistFloatingStateLocked(state FloatingWindowState) {
	if a.store == nil {
		return
	}
	_ = a.store.SaveFloatingWindowSettings(a.contextOrBackground(), storage.FloatingWindowSettings{
		Visible:   state.Visible,
		Collapsed: state.Collapsed,
		X:         state.X,
		Y:         state.Y,
	})
}

func (a *App) loadFloatingStateFromStore() {
	state := DefaultFloatingWindowState()
	if a.store != nil {
		if saved, err := a.store.LoadFloatingWindowSettings(a.contextOrBackground()); err == nil {
			state.Visible = saved.Visible
			state.Collapsed = saved.Collapsed
			state.X = saved.X
			state.Y = saved.Y
		}
	}
	a.floatMu.Lock()
	a.floatState = state
	a.floatMu.Unlock()
}

func (a *App) restoreFloatingWindowIfNeeded() {
	if a.GetFloatingWindowState().Visible {
		_ = a.ShowFloatingWindow()
	}
}

func (a *App) shutdownFloatingWindow() {
	if a.floatHost != nil {
		_ = a.floatHost.Destroy()
	}
	a.stopFloatingWindowServer()
}

func buildFloatNavigateURL(uiBase string) string {
	if uiBase == "" {
		return "/#/float"
	}
	return fmt.Sprintf("%s/#/float", strings.TrimRight(uiBase, "/"))
}
```

Add to `App`:

```go
floatMu         sync.Mutex
floatState      FloatingWindowState
floatHost       floatingWindowHost
floatAPI        *floatAPIServer
floatUIBaseURL  string
```

Initialize in `NewApp`:

```go
floatHost:  noopFloatingWindowHost{},
floatState: DefaultFloatingWindowState(),
```

- [ ] **Step 4: Run GREEN**

```powershell
go test . -run "Test(ShowFloatingWindow|HideFloatingWindow|ToggleFloatingWindow)" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add floating_window_host.go floating_window_controller.go floating_window_controller_test.go app.go
git commit -m "feat: add floating window controller with injectable host"
```

### Task 4: Tray Menu Integration

**Files:**
- Modify: `tray.go`
- Modify: `desktop_lifecycle_test.go`
- Modify: `app.go` tray callback wiring if needed

- [ ] **Step 1: Extend tray tests**

```go
func TestConfigureTrayIncludesFloatingWindowToggle(t *testing.T) {
	tray := &fakeTrayBackend{}
	var showMain, toggleFloat, quit int
	configureTray(tray, []byte("icon"), trayCallbacks{
		show:           func() { showMain++ },
		toggleFloating: func() { toggleFloat++ },
		quit:           func() { quit++ },
	})
	item := tray.menuItems["显示/隐藏悬浮窗"]
	if item == nil {
		t.Fatal("missing floating window menu item")
	}
	item.onClick()
	if toggleFloat != 1 {
		t.Fatalf("float toggle not routed, got %d", toggleFloat)
	}
	if tray.menuItems["显示主窗口"] == nil || tray.menuItems["退出程序"] == nil {
		t.Fatal("existing tray items missing")
	}
}
```

Use one stable menu label `显示/隐藏悬浮窗` calling `ToggleFloatingWindow`. This avoids brittle live label rewrites with the current tray package while preserving the show/hide capability from the spec.

- [ ] **Step 2: Run RED**

```powershell
go test . -run "TestConfigureTray|TestTray" -count=1
```

Expected: FAIL on missing callback field/menu item.

- [ ] **Step 3: Implement tray extension**

```go
type trayCallbacks struct {
	show           func()
	quit           func()
	toggleFloating func()
}

func (a *App) trayCallbacks() trayCallbacks {
	return trayCallbacks{
		show:           a.ShowMainWindow,
		quit:           a.ExitApplication,
		toggleFloating: func() { _ = a.ToggleFloatingWindow() },
	}
}

func configureTray(tray trayBackend, icon []byte, callbacks trayCallbacks) {
	tray.SetIcon(icon)
	tray.SetTooltip("Farm_Go")
	tray.SetOnDoubleClick(callbacks.show)
	showItem := tray.AddMenuItem("显示主窗口", "恢复 Farm_Go")
	floatItem := tray.AddMenuItem("显示/隐藏悬浮窗", "切换桌面悬浮状态窗")
	exitItem := tray.AddMenuItem("退出程序", "退出 Farm_Go")
	showItem.Click(callbacks.show)
	floatItem.Click(callbacks.toggleFloating)
	exitItem.Click(callbacks.quit)
}
```

Update existing tray tests that assert exact menu item keys.

- [ ] **Step 4: Run GREEN**

```powershell
go test . -run "TestConfigureTray|TestTray" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add tray.go desktop_lifecycle_test.go app.go
git commit -m "feat: add tray control for floating status window"
```

### Task 5: Windows Host Implementation (Always-On-Top WebView2)

**Files:**
- Create: `floating_window_host_windows.go`
- Create: `floating_window_host_stub.go` (`//go:build !windows`) if needed
- Create: `floating_window_host_windows_test.go`
- Modify: `app.go` / `main.go` startup to install real host and serve assets

- [ ] **Step 1: Write pure helper tests**

```go
func TestFloatingWindowStyleBitsIncludeToolWindowAndTopmostIntent(t *testing.T) {
	style, exStyle := floatingWindowStyles()
	if style == 0 || exStyle == 0 {
		t.Fatal("styles should be non-zero")
	}
}

func TestBuildFloatNavigateURL(t *testing.T) {
	got := buildFloatNavigateURL("http://127.0.0.1:1234")
	if got != "http://127.0.0.1:1234/#/float" {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 2: Run RED**

```powershell
go test . -run "Test(FloatingWindowStyleBits|BuildFloatNavigateURL)" -count=1
```

- [ ] **Step 3: Implement Windows host**

Requirements:

1. Create an overlapped popup/tool window with `WS_EX_TOOLWINDOW` and `WS_EX_TOPMOST`.
2. Embed WebView2 using `github.com/wailsapp/go-webview2/pkg/edge`, parented to the HWND.
3. `Show(url, state)` creates on first show if needed, positions with `SetWindowPos`, then shows.
4. `Hide()` uses `ShowWindow(SW_HIDE)` and must not quit the process.
5. `Destroy()` on app shutdown.
6. `WorkArea()` via work-area API; clamp through existing helpers.
7. Map native `WM_CLOSE` to hide, never app exit.
8. Frontend drag updates x/y through `/api/float/window`; host `ApplyState` repositions HWND.
9. Navigate to `buildFloatNavigateURL(floatUIBaseURL)`.

Serve assets and API on one loopback origin in startup:

```go
baseURL, err := a.startFloatingWindowServer(assets)
if err != nil {
	// log and continue; main app must still work
}
a.floatUIBaseURL = baseURL
```

On Windows:

```go
if runtime.GOOS == "windows" {
	a.floatHost = newWindowsFloatingWindowHost()
}
```

Keep unit tests on fake host.

- [ ] **Step 4: Run unit tests**

```powershell
go test . -run "Test(FloatingWindowStyleBits|BuildFloatNavigateURL|ShowFloatingWindow|HideFloatingWindow)" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add floating_window_host_windows.go floating_window_host_stub.go floating_window_host_windows_test.go floating_window_api.go app.go main.go
git commit -m "feat: add Windows always-on-top floating window host"
```

### Task 6: Frontend Float App Shell And Labels

**Files:**
- Create: `frontend/src/float/api.ts`
- Create: `frontend/src/float/api.test.ts`
- Create: `frontend/src/float/labels.ts`
- Create: `frontend/src/float/FloatingStatusApp.tsx`
- Create: `frontend/src/float/FloatingStatusApp.test.tsx`
- Modify: `frontend/src/main.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing frontend tests**

```tsx
// FloatingStatusApp.test.tsx
import renderer, { act } from 'react-test-renderer';
import { FloatingStatusApp } from './FloatingStatusApp';

const snapshot = {
  runtime: { target: 'qq_ws', phase: 'ready', connected: true, ready: true },
  guardian: { phase: 'watching', enabled: true, running: true },
  authorization: { authorized: true, heartbeatFailures: 0, expireTime: '2026-08-01 12:00:00', phase: 'authorized' },
  automation: { enabled: true, runningTaskId: '' },
  runStatistics: {
    startedAt: '2026-07-15T00:00:00Z',
    durationSeconds: 120,
    collect: 1,
    farm: 2,
    steal: 3,
    help: 4,
    mischief: 5,
    saleEstimate: 100,
    estimateReady: true,
  },
  latestTask: { present: true, time: '12:00:01', name: '自动收获', result: '完成', level: 'info' },
  window: { visible: true, collapsed: false, x: 1, y: 2, width: 320, expandedHeight: 220, collapsedHeight: 56 },
};

vi.mock('./api', () => ({
  fetchFloatSnapshot: vi.fn(async () => snapshot),
  startAutomation: vi.fn(async () => ({ enabled: true })),
  stopAutomation: vi.fn(async () => ({ enabled: false })),
  hideFloatingWindow: vi.fn(async () => undefined),
  openMainWindow: vi.fn(async () => undefined),
  saveFloatingWindowState: vi.fn(async (state) => state),
}));

test('renders expanded health and stats', async () => {
  let tree: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(<FloatingStatusApp />);
  });
  const text = JSON.stringify(tree!.toJSON());
  expect(text).toContain('Farm_Go 状态');
  expect(text).toContain('已就绪');
  expect(text).toContain('停止自动化');
});

test('disables automation actions when unauthorized', async () => {
  const api = await import('./api');
  (api.fetchFloatSnapshot as any).mockResolvedValueOnce({
    ...snapshot,
    authorization: { authorized: false, heartbeatFailures: 0, expireTime: '', phase: 'locked' },
    automation: { enabled: false },
  });
  let tree: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(<FloatingStatusApp />);
  });
  const button = tree!.root.findAll((node) => node.type === 'button' && String(node.children).includes('启动自动化'))[0];
  expect(button.props.disabled).toBe(true);
});
```

```ts
// api.test.ts
import { floatApiPath } from './api';

test('floatApiPath builds absolute same-origin paths', () => {
  expect(floatApiPath('/api/float/snapshot')).toBe('/api/float/snapshot');
});
```

- [ ] **Step 2: Run RED**

```powershell
cd frontend
npm test -- src/float
```

Expected: FAIL.

- [ ] **Step 3: Implement float frontend**

`frontend/src/float/api.ts`:

```ts
export function floatApiPath(path: string) {
  return path.startsWith('/') ? path : `/${path}`;
}

async function jsonFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(floatApiPath(path), {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<T>;
}

export const fetchFloatSnapshot = () => jsonFetch<FloatSnapshot>('/api/float/snapshot');
export const startAutomation = () => jsonFetch('/api/float/automation/start', { method: 'POST' });
export const stopAutomation = () => jsonFetch('/api/float/automation/stop', { method: 'POST' });
export const openMainWindow = () => jsonFetch('/api/float/open-main', { method: 'POST' });
export const hideFloatingWindow = () => jsonFetch('/api/float/hide', { method: 'POST' });
export const saveFloatingWindowState = (state: FloatWindowState) =>
  jsonFetch<FloatWindowState>('/api/float/window', {
    method: 'POST',
    body: JSON.stringify(state),
  });
```

`frontend/src/float/labels.ts`:
- connection summary from phase/connected/ready (same as OverviewView)
- guardian labels from existing `guardStatusLabel` semantics
- authorization labels (`已授权` / `心跳异常` / `未授权`)
- automation running/stopped

`frontend/src/float/FloatingStatusApp.tsx` responsibilities:
- on mount fetch snapshot, then poll every 2500ms
- expanded/collapsed rendering
- top bar buttons: collapse, open main, hide
- refresh button force fetch
- start/stop automation with loading + inline message
- drag: pointer down on top bar tracks movement and POSTs window x/y on pointer up
- unauthorized disables automation buttons

`frontend/src/main.tsx`:

```tsx
import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import { FloatingStatusApp } from './float/FloatingStatusApp'

const container = document.getElementById('root')!
const root = createRoot(container)
const isFloat = window.location.hash.startsWith('#/float')

root.render(
  <React.StrictMode>
    {isFloat ? <FloatingStatusApp /> : <App />}
  </React.StrictMode>
)
```

Add compact CSS under `.float-root`, `.float-card`, `.float-health`, `.float-metrics`, `.float-actions`, reusing existing signal colors in `style.css`.

- [ ] **Step 4: Run GREEN**

```powershell
cd frontend
npm test -- src/float
npm run build
```

Expected: PASS + production build success.

- [ ] **Step 5: Commit**

```powershell
git add frontend/src/float frontend/src/main.tsx frontend/src/style.css
git commit -m "feat: add floating status window React app"
```

### Task 7: End-To-End Wiring Hardening

**Files:**
- Modify: `app.go` startup/shutdown
- Modify: `floating_window_controller.go` / host as needed
- Modify/add tests for restore-on-startup preference

- [ ] **Step 1: Write startup restore test**

```go
func TestRestoreFloatingWindowIfNeededShowsWhenVisible(t *testing.T) {
	host := &fakeFloatHost{}
	app := NewApp()
	app.floatHost = host
	app.floatUIBaseURL = "http://127.0.0.1:1"
	app.floatState = FloatingWindowState{Visible: true, Width: 320, ExpandedHeight: 220, CollapsedHeight: 56}

	app.restoreFloatingWindowIfNeeded()
	if !host.shown {
		t.Fatal("expected host shown for restored preference")
	}
}

func TestRestoreFloatingWindowIfNeededSkipsWhenHidden(t *testing.T) {
	host := &fakeFloatHost{}
	app := NewApp()
	app.floatHost = host
	app.floatState = DefaultFloatingWindowState()

	app.restoreFloatingWindowIfNeeded()
	if host.shown || host.showCalls != 0 {
		t.Fatal("hidden preference must not auto-show")
	}
}
```

- [ ] **Step 2: Run RED/GREEN around restore helper**

```powershell
go test . -run TestRestoreFloatingWindowIfNeeded -count=1
```

- [ ] **Step 3: Wire startup/shutdown**

In `startup` after store is ready:

1. `a.loadFloatingStateFromStore()`
2. start float server with embedded assets; set `a.floatUIBaseURL`
3. install Windows host when applicable
4. `a.restoreFloatingWindowIfNeeded()`

In `shutdown` / `OnShutdown`:

1. `a.shutdownFloatingWindow()`
2. keep existing tray stop behavior

- [ ] **Step 4: Run broader backend tests**

```powershell
go test . -run "Floating|Tray|Authorization|Desktop" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add app.go floating_window_controller.go floating_window_controller_test.go main.go
git commit -m "feat: restore floating window preference on startup"
```

### Task 8: Manual Verification Checklist And Final Test Sweep

**Files:**
- None required beyond fixes found during verification.

- [ ] **Step 1: Automated sweep**

```powershell
go test ./...
cd frontend
npm test
npm run build
```

Expected: all PASS; frontend build emits `dist`.

- [ ] **Step 2: Manual Windows verification**

1. Launch app authorized.
2. Tray → `显示/隐藏悬浮窗` shows always-on-top card.
3. Collapse/expand works; size roughly 320x56 / 320x220.
4. Health row matches Overview semantics.
5. Stats refresh while main window is hidden to tray.
6. Start/stop automation from float matches Automation page.
7. Refresh forces snapshot update.
8. Open main window from float works.
9. Close float hides only float; app keeps running.
10. Restart app with float previously visible restores it; position roughly preserved.
11. Unauthorized/locked session: automation buttons disabled, no crash.
12. Exit from tray closes everything.

- [ ] **Step 3: Fix any defects found with focused tests first**

- [ ] **Step 4: Final commit if fixes landed**

```powershell
git add -A
git commit -m "fix: harden floating status window behavior"
```

---

## Spec Coverage Check

| Spec requirement | Task |
|---|---|
| Independent second window | Task 5 |
| Always-on-top | Task 5 |
| Tray show/hide | Task 4 |
| Connection/guardian/auth/automation + stats | Tasks 2, 6 |
| Light actions including start/stop automation | Tasks 2, 6 |
| Close hides, does not exit | Tasks 3, 4, 5 |
| Persist visible/collapsed/position | Tasks 1, 3, 7 |
| Default not auto-open first run | Task 1 defaults + Task 7 |
| Unauthorized limited mode | Tasks 2, 6 |
| Main minimize-to-tray leaves float alone | Tasks 3/4 (no coupling) |
| Reuse existing automation/status APIs | Task 2 |
| Tests for tray/state/UI | Tasks 1-7 |

## Risk Notes For Implementers

1. **Wails bindings will not work in the float WebView.** Use the loopback same-origin HTTP bridge only.
2. Keep the float React tree tiny. Never mount `AuthorizedApp`, land details, or social views there.
3. Prefer hide-without-destroy for the host HWND; recreate only if WebView2 recovery requires it.
4. Do not open remote ports. Bind only `127.0.0.1`.
5. If WebView2 host integration proves unstable in one pass, keep controller/API/UI complete and isolate host work behind the interface so the rest remains testable.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-15-desktop-floating-status-window.md`.

Two execution options:

1. **Subagent-Driven (recommended)** - fresh subagent per task, review between tasks, fast iteration
2. **Inline Execution** - execute tasks in this session with checkpoints

Which approach?
