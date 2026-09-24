# Scheduled Update Check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a persisted, default-enabled recurring update check with a user-configurable 120-minute default interval.

**Architecture:** Store update preferences independently in the global SQLite settings table and expose them through authorized Wails methods. `AuthorizedApp` owns a completion-aware timer and de-duplicates all update requests, while `SettingsView` edits and saves only update preferences.

**Tech Stack:** Go, SQLite, Wails v2, React 18, TypeScript, Vitest, react-test-renderer, CSS.

---

## File Map

- Create `internal/storage/update_check_settings.go`: preference type, defaults, validation, load, and save.
- Create `internal/storage/update_check_settings_test.go`: persistence and validation behavior.
- Create `update_check_settings_api.go`: authorized Wails load/save methods.
- Modify `authorization_gate.go` and `authorization_gate_test.go`: register and verify the new methods.
- Modify `frontend/wailsjs/go/main/App.js`, `frontend/wailsjs/go/main/App.d.ts`, and `frontend/wailsjs/go/models.ts`: generated Wails surface, then verify by a Wails build.
- Create `frontend/src/lib/updateCheckScheduler.ts`: completion-aware recurring timeout lifecycle.
- Create `frontend/src/lib/updateCheckScheduler.test.ts`: fake-timer scheduler tests.
- Modify `frontend/src/AuthorizedApp.tsx` and `frontend/src/App.test.tsx`: preference loading, request de-duplication, scheduling, and propagation.
- Modify `frontend/src/views/SettingsView.tsx`, `frontend/src/views/SettingsView.test.tsx`, and `frontend/src/style.css`: controls, independent save state, validation, and layout.
- Generate `frontend/dist/*`: production bundle embedded by `main.go`.

### Task 1: Persist Update Check Preferences

**Files:**
- Create: `internal/storage/update_check_settings.go`
- Create: `internal/storage/update_check_settings_test.go`

- [ ] **Step 1: Write failing storage tests**

Cover these exact behaviors with real `Store` instances opened in `t.TempDir()`:

```go
func TestUpdateCheckPreferencesDefaultToEnabledEvery120Minutes(t *testing.T) {
    store := openTestStore(t)
    got, err := store.LoadUpdateCheckPreferences(context.Background())
    if err != nil { t.Fatal(err) }
    want := UpdateCheckPreferences{Enabled: true, IntervalMinutes: 120}
    if got != want { t.Fatalf("got %#v, want %#v", got, want) }
}

func TestUpdateCheckPreferencesRoundTrip(t *testing.T) {
    store := openTestStore(t)
    want := UpdateCheckPreferences{Enabled: false, IntervalMinutes: 45}
    if err := store.SaveUpdateCheckPreferences(context.Background(), want); err != nil { t.Fatal(err) }
    got, err := store.LoadUpdateCheckPreferences(context.Background())
    if err != nil { t.Fatal(err) }
    if got != want { t.Fatalf("got %#v, want %#v", got, want) }
}
```

Also seed `update.checkIntervalMinutes` directly with `0`, `10081`, and non-numeric text and assert each load falls back to 120. Assert saves at 1 and 10080 succeed, while 0 and 10081 return `ErrInvalidUpdateCheckInterval`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/storage -run UpdateCheckPreferences -count=1`

Expected: compile failure because `UpdateCheckPreferences` and its load/save methods do not exist.

- [ ] **Step 3: Implement minimal storage model**

Define:

```go
const (
    DefaultUpdateCheckIntervalMinutes = 120
    MinUpdateCheckIntervalMinutes = 1
    MaxUpdateCheckIntervalMinutes = 10080
)

var ErrInvalidUpdateCheckInterval = errors.New("update check interval must be between 1 and 10080 minutes")

type UpdateCheckPreferences struct {
    Enabled         bool `json:"enabled"`
    IntervalMinutes int  `json:"intervalMinutes"`
}
```

Use global keys `update.checkEnabled` and `update.checkIntervalMinutes`. Missing or corrupt values load through `DefaultUpdateCheckPreferences()`. `SaveUpdateCheckPreferences` validates first, then calls `saveSettingsForAccount` once so both fields persist together.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/storage -run UpdateCheckPreferences -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- internal/storage/update_check_settings.go internal/storage/update_check_settings_test.go
git commit -m "feat: persist update check preferences"
```

### Task 2: Expose Authorized Preference APIs

**Files:**
- Create: `update_check_settings_api.go`
- Modify: `authorization_gate.go`
- Modify: `authorization_gate_test.go`
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/main/App.d.ts`
- Modify: `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Write failing API tests**

Add tests that construct `newAuthorizedTestApp(t)`, open its store, and verify:

```go
got := app.UpdateCheckPreferences()
if got.Enabled != true || got.IntervalMinutes != 120 { t.Fatalf("unexpected defaults: %#v", got) }

saved, err := app.SaveUpdateCheckPreferences(storage.UpdateCheckPreferences{Enabled: true, IntervalMinutes: 30})
if err != nil { t.Fatal(err) }
if saved.IntervalMinutes != 30 { t.Fatalf("unexpected saved value: %#v", saved) }
```

Add `UpdateCheckPreferences` and `SaveUpdateCheckPreferences` to the exported-method authorization assertions. Verify an unauthorized app cannot load or save preferences.

- [ ] **Step 2: Verify RED**

Run: `go test . -run 'UpdateCheckPreferences|AuthorizationPolicies' -count=1`

Expected: compile or policy failure because the methods are absent.

- [ ] **Step 3: Implement the API and policy entries**

Use methods with these signatures:

```go
func (a *App) UpdateCheckPreferences() storage.UpdateCheckPreferences
func (a *App) SaveUpdateCheckPreferences(input storage.UpdateCheckPreferences) (storage.UpdateCheckPreferences, error)
```

Both call `requireAuthorized` with their exact exported method names. Load failures return safe defaults and record `a.lastErr`; save failures return the unchanged input plus the error. Add both method names as `authorizationRequired`.

- [ ] **Step 4: Verify GREEN and regenerate bindings**

Run: `go test . -run 'UpdateCheckPreferences|AuthorizationPolicies' -count=1`

Expected: PASS.

Run: `$env:PATH = "$env:USERPROFILE\go\bin;$env:PATH"; wails build -s -m -nosyncgomod -o Farm_Go-bindings-check.exe`

Expected: exit 0 and Wails bindings expose `UpdateCheckPreferences()` and `SaveUpdateCheckPreferences(arg1: storage.UpdateCheckPreferences)`; preserve unrelated pre-existing whitespace changes in `models.ts`.

- [ ] **Step 5: Commit**

```powershell
git add -- update_check_settings_api.go authorization_gate.go authorization_gate_test.go frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/models.ts
git commit -m "feat: expose update check preferences"
```

### Task 3: Build A Completion-Aware Scheduler

**Files:**
- Create: `frontend/src/lib/updateCheckScheduler.ts`
- Create: `frontend/src/lib/updateCheckScheduler.test.ts`

- [ ] **Step 1: Write failing fake-timer tests**

Specify this API:

```ts
const stop = startRecurringUpdateChecks({
  intervalMinutes: 120,
  check: async () => undefined,
});
stop();
```

With `vi.useFakeTimers()`, assert no call before `120 * 60_000`, one call at the boundary, no second timeout while the returned promise is unresolved, a fresh full interval after settlement, and no calls after `stop()`.

- [ ] **Step 2: Verify RED**

Run: `npm test -- --run src/lib/updateCheckScheduler.test.ts` from `frontend`.

Expected: FAIL because the module does not exist.

- [ ] **Step 3: Implement the minimal scheduler**

```ts
export function startRecurringUpdateChecks(options: {
  intervalMinutes: number;
  check: () => Promise<unknown>;
}) {
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const delay = options.intervalMinutes * 60_000;
  const schedule = () => {
    timer = setTimeout(async () => {
      try { await options.check(); } finally { if (!stopped) schedule(); }
    }, delay);
  };
  schedule();
  return () => {
    stopped = true;
    if (timer !== undefined) clearTimeout(timer);
  };
}
```

- [ ] **Step 4: Verify GREEN**

Run: `npm test -- --run src/lib/updateCheckScheduler.test.ts` from `frontend`.

Expected: all scheduler tests PASS with fake timers restored after each test.

- [ ] **Step 5: Commit**

```powershell
git add -- frontend/src/lib/updateCheckScheduler.ts frontend/src/lib/updateCheckScheduler.test.ts
git commit -m "feat: add recurring update scheduler"
```

### Task 4: Integrate Scheduling In AuthorizedApp

**Files:**
- Modify: `frontend/src/AuthorizedApp.tsx`
- Modify: `frontend/src/App.test.tsx`

- [ ] **Step 1: Write failing integration-oriented tests**

Add source contract assertions and exported pure helper tests showing that:

```ts
expect(normalizeUpdateCheckPreferences(undefined)).toEqual({ enabled: true, intervalMinutes: 120 });
expect(normalizeUpdateCheckPreferences({ enabled: false, intervalMinutes: 45 })).toEqual({ enabled: false, intervalMinutes: 45 });
expect(normalizeUpdateCheckPreferences({ enabled: true, intervalMinutes: 0 })).toEqual({ enabled: true, intervalMinutes: 120 });
```

Assert `AuthorizedApp.tsx` imports the preference APIs and scheduler, owns an in-flight update promise ref, starts recurring checks only when enabled, and passes preferences plus a save callback to `SettingsView`.

- [ ] **Step 2: Verify RED**

Run: `npm test -- --run src/App.test.tsx` from `frontend`.

Expected: FAIL because the helper and scheduling integration are absent.

- [ ] **Step 3: Implement root ownership**

Add `UpdateCheckPreferencesDto`, `defaultUpdateCheckPreferences`, and `normalizeUpdateCheckPreferences` in `AuthorizedApp.tsx`. Load preferences on mount with a default fallback. Replace direct update calls with one `runUpdateCheck` function backed by `useRef<Promise<UpdateStateDto> | null>`.

Keep two result policies:

```ts
const checkForUpdates = async () => {
  const next = await runUpdateCheck();
  setUpdateState(next);
  return next;
};

const checkForUpdatesAutomatically = async () => {
  try {
    const next = await runUpdateCheck();
    if (!next.errorCode) setUpdateState(next);
  } catch {
    // Automatic checks remain non-blocking.
  }
};
```

Run the startup check once. In a separate effect, call `startRecurringUpdateChecks` only when preferences are enabled, and return its stop function. The save callback calls the backend first and only then updates root preference state, causing the effect to reset the countdown.

- [ ] **Step 4: Verify GREEN**

Run: `npm test -- --run src/App.test.tsx src/lib/updateCheckScheduler.test.ts` from `frontend`.

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- frontend/src/AuthorizedApp.tsx frontend/src/App.test.tsx
git commit -m "feat: schedule authorized update checks"
```

### Task 5: Add Update Preference Controls

**Files:**
- Modify: `frontend/src/views/SettingsView.tsx`
- Modify: `frontend/src/views/SettingsView.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing UI tests**

Render the view with default preferences and assert `定时检查更新`, a checked checkbox named `scheduled-update-enabled`, interval input named `scheduled-update-interval` with value 120, `分钟/次`, and an independent button named `save-update-preferences`.

With `react-test-renderer`, cover:

```ts
await renderer.root.findByProps({ name: 'scheduled-update-enabled' }).props.onChange({ target: { checked: false } });
expect(renderer.root.findByProps({ name: 'scheduled-update-interval' }).props.disabled).toBe(true);
```

Assert values 0, 10081, and decimals show `请输入 1 到 10080 之间的整数分钟数` without invoking the save prop. Assert a valid value calls the save prop and a rejected save shows its error without changing the active preference received from the root.

- [ ] **Step 2: Verify RED**

Run: `npm test -- --run src/views/SettingsView.test.tsx` from `frontend`.

Expected: FAIL because the controls and props do not exist.

- [ ] **Step 3: Implement controls and styling**

Extend props with:

```ts
updateCheckPreferences?: UpdateCheckPreferencesDto;
onSaveUpdateCheckPreferences?: (value: UpdateCheckPreferencesDto) => Promise<UpdateCheckPreferencesDto>;
```

Keep local draft state synchronized when root preferences change. The checkbox uses the existing check-row pattern; the number input is disabled when unchecked and uses `min={1}`, `max={10080}`, and `step={1}`. Add a compact inline `分钟/次` suffix and a save button using the existing `Save` / `Loader2` icon treatment. Saving validates `Number.isInteger(interval)` and the documented bounds before calling the prop. Keep save feedback local to the update panel.

Use CSS grid/flex constraints so the input, suffix, buttons, and status text wrap cleanly at narrow desktop widths without changing the established restrained settings-panel styling.

- [ ] **Step 4: Verify GREEN**

Run: `npm test -- --run src/views/SettingsView.test.tsx src/App.test.tsx` from `frontend`.

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- frontend/src/views/SettingsView.tsx frontend/src/views/SettingsView.test.tsx frontend/src/style.css
git commit -m "feat: configure scheduled update checks"
```

### Task 6: Full Verification And Embedded Frontend Build

**Files:**
- Generated: `frontend/dist/index.html`
- Generated: `frontend/dist/assets/*`

- [ ] **Step 1: Run backend verification**

Run: `go test ./... -count=1`

Expected: all Go packages PASS.

- [ ] **Step 2: Run frontend verification**

Run: `npm test -- --run` from `frontend`.

Expected: all Vitest files PASS with zero failures.

- [ ] **Step 3: Build the actual embedded frontend**

Run: `npm run build` from `frontend`.

Expected: `tsc` and Vite exit 0. This repository embeds `frontend/dist` through `main.go`; it does not have the root `pnpm run frontend:build` / `public/app` deployment surface mentioned by older project documents.

- [ ] **Step 4: Verify generated artifact and diff quality**

Run:

```powershell
Get-Content frontend/dist/index.html
Get-ChildItem frontend/dist/assets | Sort-Object LastWriteTime -Descending | Select-Object -First 5 Name, LastWriteTime
git diff --check
git status --short
```

Expected: `index.html` references a freshly generated hashed asset, its timestamp is current, `git diff --check` reports no errors, and only intended files plus pre-existing user changes remain.

- [ ] **Step 5: Commit generated bundle if tracked**

Check `git ls-files frontend/dist`. If tracked, stage the changed generated files and commit them as `build: refresh embedded frontend`; if ignored, leave them untracked and report the verified timestamp and asset name.
