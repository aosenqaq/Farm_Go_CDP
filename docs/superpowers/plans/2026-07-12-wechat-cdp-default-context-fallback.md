# WeChat CDP Default Context Fallback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow the WeChat CDP link to become ready when WeChat reports weak explicit execution contexts but the actual game runtime is reachable through `Runtime.evaluate` without a `contextId`.

**Architecture:** Keep the existing strict context scoring and strong explicit-context preference. After the existing 250 ms settling period, probe the default execution environment even when explicit contexts exist; feed that probe through the same score threshold and later `gameCtl` injection verification, so the fallback cannot mark an unrelated browser context ready.

**Tech Stack:** Go 1.25.5, Gorilla WebSocket-backed CDP transport, existing `internal/runtime/wmpf` and `internal/runtime/cdp` test helpers.

---

## File Structure

- Modify `internal/runtime/wmpf/link.go`: broaden the existing default-context probe condition inside `waitForGameCtlExecutionContext`; no new exported API.
- Modify `internal/runtime/wmpf/link_test.go`: make the existing runtime-client fake return context-specific probe values and add focused fallback/regression tests.

Do not change the score threshold in `internal/runtime/cdp/context.go`, the 8-second production timeout, Frida injection, WebSocket lifecycle, frontend state rendering, or unrelated connection behavior.

### Task 1: Reproduce weak explicit context with usable default context

**Files:**
- Modify: `internal/runtime/wmpf/link_test.go:2184`
- Test: `internal/runtime/wmpf/link_test.go`

- [ ] **Step 1: Extend the existing test client with context-specific evaluation results**

Add two fields to `blockingRuntimeClient`:

```go
type blockingRuntimeClient struct {
	connectEntered chan struct{}
	connectRelease chan struct{}
	connectOnce    sync.Once
	sendEntered    chan struct{}
	sendOnce       sync.Once
	blockSend      bool
	closed         chan struct{}
	closeOnce      sync.Once
	mu             sync.Mutex
	closeWaitCalls int
	contexts       []cdp.ExecutionContext
	evaluateValue  any
	evaluateValues map[int]any
	evaluatedIDs   []int
}
```

Replace its `Evaluate` implementation with:

```go
func (c *blockingRuntimeClient) Evaluate(_ context.Context, _ string, contextID int, _ time.Duration) (any, error) {
	c.mu.Lock()
	c.evaluatedIDs = append(c.evaluatedIDs, contextID)
	value, configured := c.evaluateValues[contextID]
	fallback := c.evaluateValue
	c.mu.Unlock()
	if configured {
		return value, nil
	}
	return fallback, nil
}
```

Add a snapshot helper next to `closeWaitCallCount`:

```go
func (c *blockingRuntimeClient) evaluatedContextIDs() []int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]int(nil), c.evaluatedIDs...)
}
```

- [ ] **Step 2: Add the failing fallback test**

Place this test near the existing execution-context selection tests:

```go
func TestCDPLinkFallsBackToDefaultRuntimeWhenReportedContextsAreWeak(t *testing.T) {
	client := newBlockingRuntimeClient(true, false)
	client.contexts = []cdp.ExecutionContext{{
		ID:     7,
		Name:   "worker",
		Origin: "https://servicewechat.com",
	}}
	client.evaluateValues = map[int]any{
		7: map[string]any{
			"hasDocument": true,
			"hasWx":       true,
		},
		0: map[string]any{
			"hasCc":         true,
			"hasGameGlobal": true,
		},
	}
	link := NewCDPLink(
		ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		CDPLinkConfig{},
		farmruntime.NewManager(),
	)

	selected, err := link.waitForGameCtlExecutionContext(context.Background(), client, "gameContext", time.Second)
	if err != nil {
		t.Fatalf("select execution context: %v", err)
	}
	if selected.ID != defaultExecutionContextID || selected.Name != "default" {
		t.Fatalf("expected default execution context, got %#v", selected)
	}
	if got := client.evaluatedContextIDs(); !slices.Contains(got, 7) || !slices.Contains(got, 0) {
		t.Fatalf("expected explicit and default probes, got %v", got)
	}
}
```

Add `"slices"` to the test file imports.

- [ ] **Step 3: Run the focused test and verify RED**

Run:

```powershell
go test ./internal/runtime/wmpf -run '^TestCDPLinkFallsBackToDefaultRuntimeWhenReportedContextsAreWeak$' -count=1 -v
```

Expected: FAIL after about one second with `runtime context is not ready`. This proves that reported weak contexts currently suppress the default-context fallback.

### Task 2: Enable the safe default-context fallback

**Files:**
- Modify: `internal/runtime/wmpf/link.go:1198`
- Test: `internal/runtime/wmpf/link_test.go`

- [ ] **Step 1: Broaden the existing fallback condition**

In `waitForGameCtlExecutionContext`, replace:

```go
if len(contexts) == 0 && time.Since(startedAt) >= 250*time.Millisecond {
```

with:

```go
if time.Since(startedAt) >= 250*time.Millisecond {
```

Keep the existing default probe, `SelectBestProbe`, score threshold, context conversion, and `ensureGameCtlReady` flow unchanged. Explicit strong contexts still return before the 250 ms fallback window, while weak explicit contexts no longer prevent a default evaluation.

- [ ] **Step 2: Run the focused test and verify GREEN**

Run:

```powershell
go test ./internal/runtime/wmpf -run '^TestCDPLinkFallsBackToDefaultRuntimeWhenReportedContextsAreWeak$' -count=1 -v
```

Expected: PASS; the recorded evaluations include explicit context `7` and default evaluation context `0`.

- [ ] **Step 3: Add a regression test proving explicit game context remains preferred**

```go
func TestCDPLinkPrefersStrongReportedContextBeforeDefaultFallback(t *testing.T) {
	client := newBlockingRuntimeClient(true, false)
	client.contexts = []cdp.ExecutionContext{{
		ID:     7,
		Name:   "gameContext",
		Origin: "https://servicewechat.com",
	}}
	client.evaluateValues = map[int]any{
		7: map[string]any{"hasCc": true},
		0: map[string]any{"hasCc": true, "hasGameCtl": true},
	}
	link := NewCDPLink(
		ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		CDPLinkConfig{},
		farmruntime.NewManager(),
	)

	selected, err := link.waitForGameCtlExecutionContext(context.Background(), client, "gameContext", time.Second)
	if err != nil {
		t.Fatalf("select execution context: %v", err)
	}
	if selected.ID != 7 {
		t.Fatalf("expected reported game context, got %#v", selected)
	}
	if got := client.evaluatedContextIDs(); !slices.Equal(got, []int{7}) {
		t.Fatalf("default fallback should not run after a strong explicit match, got %v", got)
	}
}
```

- [ ] **Step 4: Add a regression test proving weak default context is rejected**

```go
func TestCDPLinkRejectsWeakReportedAndDefaultContexts(t *testing.T) {
	client := newBlockingRuntimeClient(true, false)
	client.contexts = []cdp.ExecutionContext{{ID: 7, Name: "worker"}}
	client.evaluateValues = map[int]any{
		7: map[string]any{"hasDocument": true},
		0: map[string]any{"hasGameGlobal": true},
	}
	link := NewCDPLink(
		ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		CDPLinkConfig{},
		farmruntime.NewManager(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	selected, err := link.waitForGameCtlExecutionContext(ctx, client, "gameContext", time.Second)
	if err == nil {
		t.Fatalf("expected weak contexts to remain unready, selected %#v", selected)
	}
	if !strings.Contains(err.Error(), "runtime context is not ready") && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected weak-context error: %v", err)
	}
}
```

- [ ] **Step 5: Run all focused context-selection tests**

Run:

```powershell
go test ./internal/runtime/wmpf -run 'TestCDPLink(FallsBackToDefaultRuntimeWhenReportedContextsAreWeak|PrefersStrongReportedContextBeforeDefaultFallback|RejectsWeakReportedAndDefaultContexts|FallsBackToDefaultRuntimeEvaluateWhenNoContextEventsArrive|ProbesExecutionContextsUntilGameCtlReady)' -count=1 -v
```

Expected: all selected tests PASS.

- [ ] **Step 6: Format and commit the isolated fix**

Run:

```powershell
gofmt -w internal/runtime/wmpf/link.go internal/runtime/wmpf/link_test.go
git add internal/runtime/wmpf/link.go internal/runtime/wmpf/link_test.go
git commit -m "fix: probe default wechat cdp context"
```

Do not stage the user's unrelated modified or untracked files.

### Task 3: Verify connection-path regressions and concurrency safety

**Files:**
- Verify only: `internal/runtime/wmpf`
- Verify only: `internal/runtime/cdp`
- Verify only: `internal/runtime`

- [ ] **Step 1: Run the complete relevant package suite**

Run:

```powershell
go test ./internal/runtime/cdp ./internal/runtime/wmpf ./internal/runtime -count=1
```

Expected: all three packages report `ok`.

- [ ] **Step 2: Run the WMPF package under the race detector**

Run:

```powershell
go test -race ./internal/runtime/wmpf -count=1
```

Expected: `ok Farm_Go/internal/runtime/wmpf` with no race report.

- [ ] **Step 3: Confirm the final diff is scoped**

Run:

```powershell
git diff HEAD^ -- internal/runtime/wmpf/link.go internal/runtime/wmpf/link_test.go
git status --short
```

Expected: the commit changes only the default-probe condition and its tests. Pre-existing changes such as `internal/runtime/wmpf/frida_test.go`, `resources/gameConfig.bundle.zip`, `cmd/`, and `resources/wmpf/frida/config/addresses.20079.json` remain untouched and unstaged.

## Acceptance Criteria

- A strong reported `gameContext` remains the first choice.
- A weak reported context no longer blocks a strong default execution environment.
- Default evaluation omits `contextId` through the existing `evaluateContextID(defaultExecutionContextID) == 0` behavior.
- Weak reported and weak default contexts never reach `ready`.
- Existing `gameCtl` injection and required-method verification remain the final readiness gate.
- Relevant normal and race-detector test suites pass.
