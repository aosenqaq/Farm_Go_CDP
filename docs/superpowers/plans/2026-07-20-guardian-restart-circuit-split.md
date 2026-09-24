# Guardian Restart Circuit Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Separate automatic failure-recovery circuit accounting from manual and scheduled restarts while enforcing a fixed eight-attempt global safety limit.

**Architecture:** Keep both rolling windows inside `guard.Manager`, which already serializes and reserves every restart. Preserve the existing settings and JSON contracts by making `restartCountInWindow` the automatic-attempt count; track total attempts privately and combine automatic, global-safety, and missing-callback causes into the existing circuit status.

**Tech Stack:** Go 1.25, existing guard test clock and manager tests, React 18, TypeScript, Vitest.

---

### Task 1: Isolate The Automatic Restart Quota

**Files:**
- Modify: `internal/runtime/guard/manager_test.go`
- Modify: `internal/runtime/guard/manager.go`

- [x] **Step 1: Replace the shared-quota regression test with the desired behavior**

Replace `TestManagerManualAndScheduledRestartsShareQuota` with focused tests that use `MaxRestartsPer10Min = 1` and assert:

```go
func TestManagerManualAndScheduledRestartsDoNotConsumeAutomaticQuota(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.MaxRestartsPer10Min = 1
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 1
	var triggers []string
	m := NewManager(ManagerOptions{
		Settings: settings,
		Now:      clock.Now,
		Restart: func(reason RestartReason) (RestartResult, error) {
			triggers = append(triggers, restartTrigger(reason))
			return RestartResult{}, errors.New("restart failed")
		},
	})

	_, _ = m.ManualRestart(RuntimeSnapshot{RuntimeTarget: "qq_ws"}, "operator request")
	clock.Advance(time.Minute)
	m.tick(clock.Now(), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	waitForRestartIdle(t, m)

	status := m.Status()
	if status.CircuitOpen || status.RestartCountInWindow != 0 {
		t.Fatalf("routine restarts consumed automatic quota: %#v", status)
	}
	if !reflect.DeepEqual(triggers, []string{"manual", "scheduled"}) {
		t.Fatalf("restart triggers = %#v", triggers)
	}
}
```

Add a second test that performs one failed automatic restart, opens the automatic circuit on the next automatic error, then proves one manual restart and one due scheduled restart still reach the callback while another automatic request does not.

- [x] **Step 2: Run the focused tests and verify RED**

```powershell
go test ./internal/runtime/guard -run 'TestManager(ManualAndScheduledRestartsDoNotConsumeAutomaticQuota|AutomaticCircuitAllowsRoutineRestarts)' -count=1
```

Expected: FAIL because `restartTimes` currently includes manual and scheduled attempts and the common circuit blocks them.

- [x] **Step 3: Split automatic accounting from circuit causes**

In `manager.go`:

- rename `restartTimes` to `automaticRestartTimes`;
- replace `quotaCircuitOpen` with `automaticCircuitOpen`, `safetyCircuitOpen`, and `callbackCircuitOpen` booleans;
- change `trimRestartWindowLocked` so it trims `automaticRestartTimes` and publishes only that length through `RestartCountInWindow`;
- append to `automaticRestartTimes` only when `restartTrigger(reason) == "auto"`;
- make automatic quota checks apply only to automatic reasons;
- allow manual and scheduled reservations while only `automaticCircuitOpen` is true;
- preserve the missing callback circuit, but make a manual missing-callback attempt leave `RestartCountInWindow == 0`;
- on a routine restart callback failure while the automatic circuit is open, restore `PhaseCircuitOpen` instead of leaving the manager in `PhaseRestarting`.

Use explicit circuit causes:

```go
type circuitCause uint8

const (
	circuitCauseAutomatic circuitCause = iota + 1
	circuitCauseSafety
	circuitCauseCallback
)

const (
	automaticCircuitError = "10 minutes automatic restart limit reached"
	globalCircuitError    = "10 minutes total restart safety limit reached"
)

func (m *Manager) circuitErrorLocked() string {
	switch {
	case m.callbackCircuitOpen:
		return "restart callback is not configured"
	case m.safetyCircuitOpen:
		return globalCircuitError
	case m.automaticCircuitOpen:
		return automaticCircuitError
	default:
		return ""
	}
}
```

`openCircuitLocked(cause circuitCause)` must set the matching boolean, recompute `status.CircuitOpen`, `status.Phase`, and `status.LastActionError`, and emit the existing abnormal lifecycle notification only on the transition from no active cause to at least one active cause. Existing callers that passed `true` use `circuitCauseAutomatic`; the missing-callback path uses `circuitCauseCallback`.

- [x] **Step 4: Run the focused tests and verify GREEN**

Run the command from Step 2 and expect PASS.

- [x] **Step 5: Run existing automatic and callback circuit tests**

```powershell
go test ./internal/runtime/guard -run 'TestManager(OpensCircuitAfterTooManyRestarts|ManualRestartWithoutCallbackPreservesCircuitBehavior|AutomaticallyClosesCircuitAfterWindowExpires)' -count=1
```

Expected: PASS after updating the missing-callback count assertion from `1` to `0` while retaining its circuit-open and error assertions.

### Task 2: Add The Fixed Global Safety Window

**Files:**
- Modify: `internal/runtime/guard/manager_test.go`
- Modify: `internal/runtime/guard/manager.go`

- [x] **Step 1: Write the failing global safety test**

Add a synchronous manual-restart test with a callback that returns an error so reconnect grace does not suppress attempts:

```go
func TestManagerGlobalSafetyLimitRejectsNinthMixedRestart(t *testing.T) {
	settings := enabledSettings()
	settings.MaxRestartsPer10Min = 4
	calls := 0
	m := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(RestartReason) (RestartResult, error) {
			calls++
			return RestartResult{}, errors.New("restart failed")
		},
	})

	for index := 0; index < 8; index++ {
		_, _ = m.ManualRestart(RuntimeSnapshot{RuntimeTarget: "qq_ws"}, "operator request")
	}
	result, err := m.ManualRestart(RuntimeSnapshot{RuntimeTarget: "qq_ws"}, "operator request")
	if err != nil || result.Status != "circuit_open" || result.Reason != globalCircuitError {
		t.Fatalf("ninth restart result = %#v, err = %v", result, err)
	}
	status := m.Status()
	if calls != 8 || !status.CircuitOpen || status.RestartCountInWindow != 0 || status.LastActionError != globalCircuitError {
		t.Fatalf("global safety status = %#v, calls = %d", status, calls)
	}
}
```

Then make the first four attempts automatic and the next four a mix of manual and scheduled reasons to prove every classification shares the same total window.

- [x] **Step 2: Run the global safety test and verify RED**

```powershell
go test ./internal/runtime/guard -run TestManagerGlobalSafetyLimitRejectsNinthMixedRestart -count=1
```

Expected: FAIL because no fixed global window exists.

- [x] **Step 3: Implement total attempt accounting**

Add:

```go
const globalRestartLimit = 8
```

Add `totalRestartTimes []time.Time` to `Manager`. At the start of `reserveRestartLocked`, reject every trigger when `safetyCircuitOpen` is true or the trimmed total count is already eight. For an accepted reservation, append its timestamp to `totalRestartTimes`; append the same timestamp to `automaticRestartTimes` only for `auto`.

Extend `trimRestartWindowLocked` to trim both slices and extend `maybeCloseCircuitLocked` to clear `safetyCircuitOpen` when `len(totalRestartTimes) < globalRestartLimit`. The global error has priority over the automatic error while both causes are active.

- [x] **Step 4: Run the global and classification tests and verify GREEN**

```powershell
go test ./internal/runtime/guard -run 'TestManager(GlobalSafetyLimitRejectsNinthMixedRestart|ManualAndScheduledRestartsDoNotConsumeAutomaticQuota|AutomaticCircuitAllowsRoutineRestarts)' -count=1
```

Expected: PASS.

### Task 3: Verify Independent Circuit Expiration And Worker Wake-Up

**Files:**
- Modify: `internal/runtime/guard/manager_test.go`
- Modify: `internal/runtime/guard/manager.go`

- [x] **Step 1: Write the failing independent-expiration test**

Use `newGuardTestClock` to reserve four manual attempts at time zero, advance five minutes, reserve four automatic attempts, and trigger both circuits. After advancing another six minutes, assert the global circuit has expired but automatic requests remain blocked and a manual request is accepted. After advancing five more minutes, assert an automatic request is accepted and the circuit closes.

Also add a `nextWakeDelay` assertion proving an open safety circuit wakes at the oldest `totalRestartTimes` expiry rather than waiting only on automatic timestamps.

- [x] **Step 2: Run the expiration tests and verify RED**

```powershell
go test ./internal/runtime/guard -run 'TestManager(CircuitCausesExpireIndependently|SafetyCircuitWakeUsesOldestTotalAttempt)' -count=1
```

Expected: FAIL until `maybeCloseCircuitLocked` and `nextWakeDelay` understand both windows.

- [x] **Step 3: Complete circuit refresh and wake scheduling**

Update `nextWakeDelay` as follows:

```go
if m.automaticCircuitOpen && len(m.automaticRestartTimes) > 0 {
	shorten(m.automaticRestartTimes[0].Add(restartWindow))
}
if m.safetyCircuitOpen && len(m.totalRestartTimes) > 0 {
	shorten(m.totalRestartTimes[0].Add(restartWindow))
}
```

Make `maybeCloseCircuitLocked` clear each quota cause independently, recompute the surviving error, and call `setWatchingPhaseLocked` only when no cause remains, no restart is running, and reconnect grace is clear. Never auto-clear `callbackCircuitOpen`.

- [x] **Step 4: Format and run the guard package**

```powershell
gofmt -w internal/runtime/guard/manager.go internal/runtime/guard/manager_test.go
go test ./internal/runtime/guard -count=1
```

Expected: PASS.

- [x] **Step 5: Commit backend behavior**

```powershell
git add internal/runtime/guard/manager.go internal/runtime/guard/manager_test.go
git commit -m "fix: separate guardian restart circuit quotas"
```

### Task 4: Clarify The Guard Metric

**Files:**
- Modify: `frontend/src/views/GuardView.test.tsx`
- Modify: `frontend/src/views/GuardView.tsx`

- [x] **Step 1: Write the failing label test**

In the basic guard render test, assert:

```ts
expect(html).toContain('异常重启');
expect(html).not.toContain('重启窗口');
```

- [x] **Step 2: Run the focused frontend test and verify RED**

```powershell
Set-Location frontend
npm test -- src/views/GuardView.test.tsx
```

Expected: FAIL because the metric is still labelled `重启窗口`.

- [x] **Step 3: Change the metric label**

In `GuardView.tsx`, change only:

```tsx
<Metric label="异常重启" value={restartQuota} />
```

- [x] **Step 4: Run the focused frontend test and build**

```powershell
Set-Location frontend
npm test -- src/views/GuardView.test.tsx
npm run build
```

Expected: both commands exit 0.

- [x] **Step 5: Commit the UI clarification**

```powershell
git add frontend/src/views/GuardView.tsx frontend/src/views/GuardView.test.tsx
git commit -m "fix: clarify guardian automatic restart quota"
```

### Task 5: Update Documentation And Run Full Verification

**Files:**
- Modify: `README.md`
- Review: `internal/runtime/guard/manager.go`
- Review: `internal/runtime/guard/manager_test.go`
- Review: `frontend/src/views/GuardView.tsx`
- Review: `frontend/src/views/GuardView.test.tsx`

- [x] **Step 1: Update the guardian behavior summary**

Replace the README statement that all process restarts are limited to four with text stating that automatic failure-recovery restarts allow four attempts per rolling ten minutes, manual and scheduled restarts do not consume that quota, and all restart types share a fixed eight-attempt safety limit.

- [x] **Step 2: Run whitespace and focused verification**

```powershell
git diff --check
go test ./internal/runtime/guard -count=1
Set-Location frontend
npm test -- src/views/GuardView.test.tsx
npm run build
```

Expected: all commands exit 0.

- [x] **Step 3: Run the full project test suites**

```powershell
Set-Location ..
go test ./... -count=1
Set-Location frontend
npm test
```

Expected: all Go packages and all Vitest files pass.

- [x] **Step 4: Commit documentation**

```powershell
Set-Location ..
git add README.md
git commit -m "docs: explain guardian restart safety limits"
```

- [x] **Step 5: Inspect the final diff and history**

```powershell
git status --short
git log -5 --oneline --decorate
```

Expected: clean worktree with the design, plan, backend, UI, and README commits at the branch tip.
