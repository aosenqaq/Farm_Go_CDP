# Friend Steal Random Delay Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in, configurable random delay that runs once after automatic friend-steal candidates are found and before their existing concurrent protocol work begins.

**Architecture:** Persist the toggle and millisecond range in the existing automation config map. `RuntimeFacade` resolves those values, waits once with the task context by reusing `sleepWithContext`, then invokes the unchanged friend-steal worker pool. The tests use equal, short delay bounds to verify timing without introducing new runtime dependencies.

**Tech Stack:** Go, React 18, TypeScript, Vitest, Go standard-library `math/rand` and `time`.

---

## File Structure

- `internal/farm/automation/config.go`: default values for the new config keys.
- `internal/farm/automation/runtime_friend.go`: normalized delay settings and one shared wait.
- `internal/farm/automation/catalog_test.go`: default-state regression coverage.
- `internal/farm/automation/runtime_friend_test.go`: deterministic range, timing, and cancellation coverage.
- `frontend/src/views/AutomationView.tsx`: controls and pre-save range validation.
- `frontend/src/views/AutomationView.test.tsx`: UI rendering, disabled state, persistence, and validation coverage.

### Task 1: Add Default Configuration

**Files:**
- Modify: `internal/farm/automation/config.go:44-46,93-95`
- Modify: `internal/farm/automation/catalog_test.go:219-254`

- [ ] **Step 1: Write the failing default-state test**

Extend `TestDefaultStateIncludesReferenceDetailedConfig` with:

```go
"autoFarmFriendStealRandomDelayEnabled": false,
"autoFarmFriendStealRandomDelayMinMs":   1000,
"autoFarmFriendStealRandomDelayMaxMs":   5000,
```

- [ ] **Step 2: Verify it fails**

Run:

```powershell
go test ./internal/farm/automation -run '^TestDefaultStateIncludesReferenceDetailedConfig$' -count=1
```

Expected: FAIL because the three keys are absent.

- [ ] **Step 3: Add the defaults**

Add these fields next to `autoFarmFriendStealConcurrency` in `DefaultConfig()`:

```go
"autoFarmFriendStealRandomDelayEnabled": false,
"autoFarmFriendStealRandomDelayMinMs":   1000,
"autoFarmFriendStealRandomDelayMaxMs":   5000,
```

Do not add a migration. `MergeConfigWithDefaults` already applies defaults to saved configurations that omit new keys.

- [ ] **Step 4: Verify the test passes**

Run:

```powershell
go test ./internal/farm/automation -run '^TestDefaultStateIncludesReferenceDetailedConfig$' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the config contract**

```powershell
git add internal/farm/automation/config.go internal/farm/automation/catalog_test.go; git commit -m "feat: add friend steal delay settings"
```

### Task 2: Delay One Batch Before Worker Concurrency

**Files:**
- Modify: `internal/farm/automation/runtime_friend.go:3-12,60-62,277-290`
- Modify: `internal/farm/automation/runtime_friend_test.go:74-125,after TestRuntimeFacadeFriendStealHarvestsFirstCollectableFriend`

- [ ] **Step 1: Write failing runtime tests**

Add a test caller modeled on `blockingFriendHelpCaller`:

```go
type blockingFriendStealCaller struct {
    friends []any
    started chan int
    release chan struct{}
}

func (c *blockingFriendStealCaller) Call(_ context.Context, method string, args []any, _ time.Duration) (any, error) {
    switch method {
    case "gameCtl.getFriendList":
        return c.friends, nil
    case "gameCtl.inspectFriendFarmByProtocol":
        gid := intFromAny(mapFromAny(args[0])["hostGid"])
        c.started <- gid
        <-c.release
        return map[string]any{"ok": true, "workLandIds": map[string]any{"collect": []any{gid}}}, nil
    case "gameCtl.friendHarvestLandsByProtocol":
        return map[string]any{"ok": true}, nil
    default:
        return nil, errors.New("unexpected method: " + method)
    }
}
```

Add these tests:

```go
func TestRuntimeFacadeFriendStealWaitsOnceBeforeStartingSharedBatch(t *testing.T)
func TestRuntimeFacadeFriendStealSkipsRandomDelayWhenDisabled(t *testing.T)
func TestRuntimeFacadeFriendStealDoesNotStartProtocolsWhenDelayIsCanceled(t *testing.T)
func TestFriendStealRandomDelaySettingsNormalizesMissingAndInvalidRanges(t *testing.T)
```

The shared-batch test uses three collectable friends, concurrency `3`, and equal short bounds. Assert no inspection starts before half the delay has elapsed, then assert all three inspections start after the delay and before the test caller is released. The disabled test asserts inspection starts promptly despite a configured range. The cancellation test cancels during a one-second delay and asserts no inspect or harvest protocol was called. The normalization test covers missing defaults, negative values clamped to `0`, and reversed bounds clamped to equal bounds.

- [ ] **Step 2: Verify the runtime tests fail**

Run:

```powershell
go test ./internal/farm/automation -run '^(TestRuntimeFacadeFriendStealWaitsOnceBeforeStartingSharedBatch|TestRuntimeFacadeFriendStealSkipsRandomDelayWhenDisabled|TestRuntimeFacadeFriendStealDoesNotStartProtocolsWhenDelayIsCanceled|TestFriendStealRandomDelaySettingsNormalizesMissingAndInvalidRanges)$' -count=1
```

Expected: FAIL because the runtime has no delay settings or delay method.

- [ ] **Step 3: Implement the shared delay**

In `runtime_friend.go`, import `math/rand` and add the keys, defaults, and resolver below. Keep it near `friendStealConcurrency`.

```go
const (
    friendStealRandomDelayEnabledKey = "autoFarmFriendStealRandomDelayEnabled"
    friendStealRandomDelayMinMsKey   = "autoFarmFriendStealRandomDelayMinMs"
    friendStealRandomDelayMaxMsKey   = "autoFarmFriendStealRandomDelayMaxMs"
    friendStealRandomDelayMinDefault = 1000
    friendStealRandomDelayMaxDefault = 5000
)

type friendStealRandomDelaySettings struct {
    enabled bool
    minMs   int
    maxMs   int
}

func friendStealRandomDelaySettingsFromConfig(config map[string]any) friendStealRandomDelaySettings {
    settings := friendStealRandomDelaySettings{minMs: friendStealRandomDelayMinDefault, maxMs: friendStealRandomDelayMaxDefault}
    if config == nil {
        return settings
    }
    settings.enabled = boolConfigAny(config[friendStealRandomDelayEnabledKey])
    if value, ok := config[friendStealRandomDelayMinMsKey]; ok {
        settings.minMs = intFromAny(value)
    }
    if value, ok := config[friendStealRandomDelayMaxMsKey]; ok {
        settings.maxMs = intFromAny(value)
    }
    if settings.minMs < 0 {
        settings.minMs = 0
    }
    if settings.maxMs < 0 {
        settings.maxMs = 0
    }
    if settings.maxMs < settings.minMs {
        settings.maxMs = settings.minMs
    }
    return settings
}
```

Implement `waitFriendStealRandomDelay(ctx)` with the following body. It chooses an inclusive value only once, then uses existing `sleepWithContext`.

```go
settings := friendStealRandomDelaySettingsFromConfig(r.config)
if !settings.enabled {
    return nil
}
waitMs := settings.minMs
if settings.maxMs > settings.minMs {
    randomIntn := r.friendStealRandomIntn
    if randomIntn == nil {
        randomIntn = rand.Intn
    }
    waitMs += randomIntn(settings.maxMs-settings.minMs+1)
}
return sleepWithContext(ctx, time.Duration(waitMs)*time.Millisecond)
```

After `friends = limitFriendStealCandidates(friends, r.config)` and before `runFriendStealCandidates`, add:

```go
if err := r.waitFriendStealRandomDelay(ctx); err != nil {
    return ActionResult{OK: false, Status: StatusFailed, TaskID: taskID, Message: "随机偷菜延迟已取消：" + err.Error()}
}
```

- [ ] **Step 4: Verify the focused runtime tests pass**

Run:

```powershell
go test ./internal/farm/automation -run '^(TestRuntimeFacadeFriendStealWaitsOnceBeforeStartingSharedBatch|TestRuntimeFacadeFriendStealSkipsRandomDelayWhenDisabled|TestRuntimeFacadeFriendStealDoesNotStartProtocolsWhenDelayIsCanceled|TestFriendStealRandomDelaySettingsNormalizesMissingAndInvalidRanges)$' -count=1
```

Expected: PASS without real one-to-five-second sleeps.

- [ ] **Step 5: Run the automation package suite**

Run:

```powershell
go test ./internal/farm/automation -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the runtime behavior**

```powershell
git add internal/farm/automation/runtime_friend.go internal/farm/automation/runtime_friend_test.go; git commit -m "feat: delay friend steal batches randomly"
```

### Task 3: Add Friend-Settings Controls and Validation

**Files:**
- Modify: `frontend/src/views/AutomationView.tsx:474-491,1942-1961,1068-1089`
- Modify: `frontend/src/views/AutomationView.test.tsx:1011-1107`

- [ ] **Step 1: Write failing frontend tests**

Extend `renders editable friend automation detailed settings`:

```ts
expect(html).toContain('随机偷取延迟');
expect(html).toContain('name="config-autoFarmFriendStealRandomDelayEnabled"');
expect(html).toContain('name="config-autoFarmFriendStealRandomDelayMinMs"');
expect(html).toContain('name="config-autoFarmFriendStealRandomDelayMaxMs"');
```

Add an interactive test that enables the checkbox, changes min to `1200` and max to `3600`, clicks the settings save button, and asserts the captured `onSaveState` config contains `true`, `1200`, and `3600`. Assert both range inputs are disabled in default settings. Add an invalid-range test that uses min `5000` and max `1000`, clicks save, asserts `onSaveState` was not called, and checks for `最大随机偷取延迟不能小于最小随机偷取延迟`.

- [ ] **Step 2: Verify the frontend tests fail**

Run from `frontend/`:

```powershell
npm test -- AutomationView.test.tsx
```

Expected: FAIL because the controls and validation message do not exist.

- [ ] **Step 3: Implement the controls and validation**

In the friends settings renderer, derive:

```tsx
const friendStealRandomDelayEnabled = configBool('autoFarmFriendStealRandomDelayEnabled', false);
```

Immediately after the existing friend-steal row, add a second `automation-friend-action-row steal`:

```tsx
<label className="automation-settings-check">
  <input checked={friendStealRandomDelayEnabled} name="config-autoFarmFriendStealRandomDelayEnabled" type="checkbox" onChange={(event) => updateConfig('autoFarmFriendStealRandomDelayEnabled', event.currentTarget.checked)} />
  <span>随机偷取延迟</span>
</label>
<label className="automation-settings-field">
  最小延迟(ms)
  <input disabled={!friendStealRandomDelayEnabled} min={0} name="config-autoFarmFriendStealRandomDelayMinMs" type="number" value={configNumber('autoFarmFriendStealRandomDelayMinMs', 1000)} onChange={(event) => updateConfig('autoFarmFriendStealRandomDelayMinMs', numberFromInput(event.currentTarget.value, 1000))} />
</label>
<label className="automation-settings-field">
  最大延迟(ms)
  <input disabled={!friendStealRandomDelayEnabled} min={0} name="config-autoFarmFriendStealRandomDelayMaxMs" type="number" value={configNumber('autoFarmFriendStealRandomDelayMaxMs', 5000)} onChange={(event) => updateConfig('autoFarmFriendStealRandomDelayMaxMs', numberFromInput(event.currentTarget.value, 5000))} />
</label>
```

Add `friendStealRandomDelayValidationError(config)` beside `numberFromInput`. It returns an empty string for nonnegative integer values when `max >= min`; otherwise it returns exactly one of:

```ts
'随机偷取延迟必须是非负整数'
'最大随机偷取延迟不能小于最小随机偷取延迟'
```

At the start of `saveSettings`, validate `draftStateRef.current.config`. On error, set `saveMessage`, set `saveToastVisible` false, and return before `onSaveState`. Reuse the existing `.automation-friend-action-row.steal` layout; it already provides the approved three-column desktop and one-column mobile layout, so no CSS change is needed.

- [ ] **Step 4: Verify the focused frontend tests pass**

Run from `frontend/`:

```powershell
npm test -- AutomationView.test.tsx
```

Expected: PASS.

- [ ] **Step 5: Build the embedded frontend**

Run from `frontend/`:

```powershell
npm run build
Get-ChildItem dist\index.html, dist\assets\index-*.js | Select-Object Name, LastWriteTime, Length
```

Expected: the TypeScript and Vite build succeed, and `frontend/dist` contains the new embedded application assets.

- [ ] **Step 6: Commit the UI**

```powershell
git add frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx; git commit -m "feat: configure random friend steal delay"
```

### Task 4: Run Final Verification

**Files:**
- Verify: all files changed in Tasks 1-3.

- [ ] **Step 1: Run all frontend tests**

Run from `frontend/`:

```powershell
npm test
```

Expected: PASS.

- [ ] **Step 2: Rebuild the frontend bundle**

Run from `frontend/`:

```powershell
npm run build
```

Expected: PASS and `frontend/dist` is regenerated.

- [ ] **Step 3: Run all Go tests**

Run from the repository root:

```powershell
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Inspect the final change set**

Run:

```powershell
git diff master...HEAD --check
git status --short
```

Expected: no whitespace errors and no unexpected files. `frontend/dist` remains ignored.

- [ ] **Step 5: Commit a verification-only correction when required**

Only if a correction was required after the full suites:

```powershell
git add internal/farm/automation/config.go internal/farm/automation/catalog_test.go internal/farm/automation/runtime.go internal/farm/automation/runtime_friend.go internal/farm/automation/runtime_friend_test.go frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx; git commit -m "fix: verify friend steal random delay"
```

Otherwise, do not create an empty commit.
