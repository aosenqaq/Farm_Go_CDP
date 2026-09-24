# 延迟施肥提交 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a scoped delayed-fertilizer setting that submits selected fertilization sources one land at a time with a configurable 500ms default interval, while preserving the normal rapid batch path.

**Architecture:** Persist the toggle, scopes, and interval in the existing automation config map. A small Go policy helper converts the saved config and an explicit source scope into runtime options. The Go entry points attach that policy to `gameCtl.fertilizeLandsBatch`; the WMPF runtime branches between its current prepare-then-flush batch path and immediate per-land serial dispatch.

**Tech Stack:** Go, React/TypeScript, Vitest, Node VM regression script, WMPF JavaScript runtime.

---

## File Structure

- Create: `internal/farm/automation/fertilizer_submission.go` — resolves saved delay settings into runtime dispatch options.
- Create: `internal/farm/automation/fertilizer_submission_test.go` — pure policy-contract coverage.
- Modify: `internal/farm/automation/runtime_own.go` — attaches `planting` or `rush` policy options to automatic fertilizer calls.
- Modify: `internal/farm/automation/runtime_own_test.go` — verifies automatic payloads carry the expected policy.
- Modify: `app.go` — applies `manual` policy only for the bulk land-details request.
- Modify: `app_test.go` — verifies the manual bulk endpoint forwards policy options.
- Modify: `resources/wmpf/button.js` — dispatches selected calls serially without losing each activity crop's native payload.
- Modify: `scripts/test-fertilizer-repeat-skip.js` — verifies the batch and serial event timelines.
- Modify: `frontend/src/views/AutomationView.tsx` — renders the delayed-submission setting under fertilizer settings.
- Modify: `frontend/src/views/AutomationView.test.tsx` — covers the new visible controls and enabled settings state.

### Task 1: Resolve Delayed Submission Policy

**Files:**
- Create: `internal/farm/automation/fertilizer_submission.go`
- Test: `internal/farm/automation/fertilizer_submission_test.go`

- [ ] **Step 1: Write the failing policy-contract tests**

Create tests for `FertilizerSubmissionRuntimeArgs(config, scope)` covering disabled mode, a selected scope, an unselected scope, an enabled legacy config without a saved scope list, an explicit empty scope list, and a custom interval:

```go
func TestFertilizerSubmissionRuntimeArgs(t *testing.T) {
	cases := []struct {
		name       string
		config     map[string]any
		scope      string
		wantMode   string
		wantWaitMS int
	}{
		{"disabled", map[string]any{}, "planting", "batch", 0},
		{"selected planting", map[string]any{
			"autoFarmFertilizerDelayedSubmitEnabled": true,
			"autoFarmFertilizerDelayedSubmitScopes":  []any{"planting"},
		}, "planting", "serial", 500},
		{"unselected rush", map[string]any{
			"autoFarmFertilizerDelayedSubmitEnabled": true,
			"autoFarmFertilizerDelayedSubmitScopes":  []any{"planting"},
		}, "rush", "batch", 0},
		{"legacy enabled defaults all scopes", map[string]any{
			"autoFarmFertilizerDelayedSubmitEnabled": true,
		}, "manual", "serial", 500},
		{"explicit empty scope disables all", map[string]any{
			"autoFarmFertilizerDelayedSubmitEnabled": true,
			"autoFarmFertilizerDelayedSubmitScopes":  []any{},
		}, "manual", "batch", 0},
		{"custom interval", map[string]any{
			"autoFarmFertilizerDelayedSubmitEnabled":    true,
			"autoFarmFertilizerDelayedSubmitScopes":     []any{"rush"},
			"autoFarmFertilizerDelayedSubmitIntervalMs": 850,
		}, "rush", "serial", 850},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FertilizerSubmissionRuntimeArgs(tc.config, tc.scope)
			if got["fertilizerSubmissionMode"] != tc.wantMode || intFromAny(got["betweenLandWait"]) != tc.wantWaitMS {
				t.Fatalf("runtime args = %#v, want mode=%q wait=%d", got, tc.wantMode, tc.wantWaitMS)
			}
		})
	}
}
```

- [ ] **Step 2: Run the policy tests and verify they fail**

Run: `go test ./internal/farm/automation -run '^TestFertilizerSubmissionRuntimeArgs$' -count=1`

Expected: FAIL because `FertilizerSubmissionRuntimeArgs` does not exist.

- [ ] **Step 3: Implement the minimal policy helper**

Add constants for the three config keys, the `planting`, `rush`, and `manual` scope values, and the default `500` interval. Implement `FertilizerSubmissionRuntimeArgs` with these rules:

```go
func FertilizerSubmissionRuntimeArgs(config map[string]any, scope string) map[string]any {
	if !boolFromAny(config["autoFarmFertilizerDelayedSubmitEnabled"]) || !fertilizerDelayedScopeSelected(config, scope) {
		return map[string]any{"fertilizerSubmissionMode": "batch"}
	}

	interval := defaultFertilizerDelayedSubmitIntervalMs
	if raw, ok := config["autoFarmFertilizerDelayedSubmitIntervalMs"]; ok {
		interval = max(0, intFromAny(raw))
	}
	return map[string]any{
		"fertilizerSubmissionMode": "serial",
		"betweenLandWait":          interval,
	}
}
```

`fertilizerDelayedScopeSelected` must use all three scopes only when the scope config key is absent; an explicitly saved empty list means no scope is selected.

- [ ] **Step 4: Run the policy tests and verify they pass**

Run: `go test ./internal/farm/automation -run '^TestFertilizerSubmissionRuntimeArgs$' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the policy helper**

```powershell
git add internal/farm/automation/fertilizer_submission.go internal/farm/automation/fertilizer_submission_test.go
git commit -m "feat: resolve delayed fertilizer submission policy"
```

### Task 2: Attach Explicit Policy to Every In-Scope Caller

**Files:**
- Modify: `internal/farm/automation/runtime_own.go`
- Modify: `internal/farm/automation/runtime_own_test.go`
- Modify: `app.go`
- Modify: `app_test.go`
- Modify: `frontend/src/views/AssetsLandView.tsx`

- [ ] **Step 1: Write failing automatic and manual forwarding tests**

Add `TestRuntimeFacadeDelayedPlantingSubmissionUsesPlantingScope` in `runtime_own_test.go` with delayed submission enabled for only `planting`; execute `own_plant` and assert its `gameCtl.fertilizeLandsBatch` payload includes:

```go
"fertilizerSubmissionMode": "serial",
"betweenLandWait":          500,
```

Add `TestRuntimeFacadeDelayedAutomaticFertilizerRequiresRushScope`; enable only `planting`, execute `own_fertilizer`, and assert that its payload stays `"fertilizerSubmissionMode": "batch"`. Add `TestRuntimeFacadeMultiSeasonDelayedSubmissionUsesPlantingScope`; invoke the existing multi-season helper from an automatic-collection fixture and assert that it includes serial mode because multi-season fertilizer is classified as `planting`.

Add `TestFarmLandRushDelayedManualSubmission` in `app_test.go`; save the enabled `manual` setting, call:

```go
app.FarmLandRush(map[string]any{
	"landIds":                    []any{float64(1), float64(2)},
	"fertilizerMode":             "organic",
	"harvestLinkEnabled":         false,
	"fertilizerSubmissionScope": "manual",
})
```

and assert the captured `gameCtl.fertilizeLandsBatch` argument contains serial mode and `500` wait milliseconds. Add `TestFarmLandRushLeavesUnmarkedRequestInBatchMode` without `fertilizerSubmissionScope`; it must not add serial mode, proving the manual rush dialog is unaffected.

- [ ] **Step 2: Run the new Go tests and verify they fail**

Run: `go test ./internal/farm/automation -run '^TestRuntimeFacade(DelayedPlantingSubmissionUsesPlantingScope|DelayedAutomaticFertilizerRequiresRushScope|MultiSeasonDelayedSubmissionUsesPlantingScope)$' -count=1`

Run: `go test . -run '^TestFarmLandRush(DelayedManualSubmission|LeavesUnmarkedRequestInBatchMode)$' -count=1`

Expected: FAIL because no caller forwards `fertilizerSubmissionMode` or `betweenLandWait`.

- [ ] **Step 3: Apply policy to automatic and manual payloads**

In `runtime_own.go`, merge `FertilizerSubmissionRuntimeArgs(r.config, "planting")` into:

- the successful automatic planting fertilizer payload;
- every `fertilizeMultiSeasonAfterHarvest` payload, including the automatic-collection and linked-harvest paths.

Merge `FertilizerSubmissionRuntimeArgs(r.config, "rush")` into each normal automatic fertilizer and linked-harvest fertilizer payload. Keep the existing `source`, mode, land IDs, and linked-harvest fields unchanged.

In `AssetsLandView.tsx`, add `fertilizerSubmissionScope: 'manual'` only to `submitBulkFertilizer`. Do not add it to `submitLandRush` or the single-card `FarmFertilizeLand` action.

In `FarmLandRush`, after `buildLandRushRuntimeArgs`, detect that exact `manual` marker and merge:

```go
for key, value := range automation.FertilizerSubmissionRuntimeArgs(
	a.farmAutomationStateForAccount(a.accountKey()).Config,
	"manual",
) {
	args[key] = value
}
```

Leave the generic land-rush dialog unmarked so it retains its existing behavior.

- [ ] **Step 4: Run the forwarding tests and verify they pass**

Run: `go test ./internal/farm/automation -run '^TestRuntimeFacade(DelayedPlantingSubmissionUsesPlantingScope|DelayedAutomaticFertilizerRequiresRushScope|MultiSeasonDelayedSubmissionUsesPlantingScope)$' -count=1`

Run: `go test . -run '^TestFarmLandRush(DelayedManualSubmission|LeavesUnmarkedRequestInBatchMode)$' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit source-policy forwarding**

```powershell
git add internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go app.go app_test.go frontend/src/views/AssetsLandView.tsx
git commit -m "feat: scope delayed fertilizer submission sources"
```

### Task 3: Preserve Activity Payloads in Both Dispatch Modes

**Files:**
- Modify: `resources/wmpf/button.js`
- Modify: `scripts/test-fertilizer-repeat-skip.js`

- [ ] **Step 1: Extend the VM regression script with a serial-order assertion**

Add `verifySerialBatchDispatchesOneLandAtATime` using a manager that emits a distinct nested `activityCrop.ticket` per land. Call the batch helper with:

```js
{
  betweenLandWait: 0,
  fertilizerSubmissionMode: 'serial',
  syncTargets: false,
}
```

Assert the observable timeline is exactly:

```js
[
  'perform:201',
  'dispatch:201',
  'perform:202',
  'dispatch:202',
]
```

and the delivered tickets are `activity-201` and `activity-202`. Keep the existing batch assertion as `perform:101`, `perform:102`, `dispatch:101`, `dispatch:102`.

- [ ] **Step 2: Run the script and verify the serial assertion fails**

Run: `node scripts/test-fertilizer-repeat-skip.js`

Expected: FAIL because the current helper always suppresses native dispatch and flushes after all lands are prepared.

- [ ] **Step 3: Branch dispatch behavior on the explicit runtime mode**

In `dispatchFertilizerProtocolBatch`, normalize:

```js
const serialSubmit = String(opts.fertilizerSubmissionMode || 'batch').toLowerCase() === 'serial';
```

For both modes, continue setting `manager.multiLand = false`, synchronizing the target land, and capturing the complete native payload from the intercepted event.

For batch mode, retain the current behavior: suppress each native event, append it to `preparedDispatches`, then restore `message.dispatchEvent` and flush all payloads without an inter-dispatch wait.

For serial mode, do not suppress the captured native event. Let it call the original message dispatcher immediately, record that dispatch in `dispatches`, and wait `betweenLandWait` only before preparing the next land. Do not put serial payloads into the delayed flush list. On either-mode failure, restore the dispatcher and manager state and stop processing further lands.

- [ ] **Step 4: Run the VM regression script and verify both timelines pass**

Run: `node scripts/test-fertilizer-repeat-skip.js`

Expected: `season cache and native per-land batch dispatch pass` with no assertion failures.

- [ ] **Step 5: Commit dual dispatch support**

```powershell
git add resources/wmpf/button.js scripts/test-fertilizer-repeat-skip.js
git commit -m "feat: add serial delayed fertilizer dispatch"
```

### Task 4: Expose the Settings in the Fertilizer Panel

**Files:**
- Modify: `frontend/src/views/AutomationView.tsx`
- Modify: `frontend/src/views/AutomationView.test.tsx`

- [ ] **Step 1: Write failing rendering tests for the enabled setting**

Create a fertilizer-panel render state with:

```ts
config: {
  ...state.config,
  autoFarmFertilizerDelayedSubmitEnabled: true,
  autoFarmFertilizerDelayedSubmitScopes: ['planting', 'rush', 'manual'],
  autoFarmFertilizerDelayedSubmitIntervalMs: 500,
}
```

Assert the static markup contains `延迟施肥`, `作用范围`, `种植策略`, `催熟策略`, `手动执行`, `地块间隔(毫秒)`, and these form names:

```ts
'config-autoFarmFertilizerDelayedSubmitEnabled'
'config-autoFarmFertilizerDelayedSubmitIntervalMs'
'config-autoFarmFertilizerDelayedSubmitScopes-planting'
'config-autoFarmFertilizerDelayedSubmitScopes-rush'
'config-autoFarmFertilizerDelayedSubmitScopes-manual'
```

Also render the default state and assert that the switch exists while the interval field is absent when the switch is disabled.

- [ ] **Step 2: Run the frontend test and verify it fails**

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: FAIL because the delayed-submission controls do not exist.

- [ ] **Step 3: Render the toggle, conditional fields, and multi-select scopes**

In the fertilizer block of `AutomationView.tsx`, use the existing `configBool`, `configNumber`, `stringListFromConfig`, and `toggleStringList` helpers. Render:

- a `延迟施肥` checkbox using `autoFarmFertilizerDelayedSubmitEnabled`;
- when enabled, three compact checkboxes with the `planting`, `rush`, and `manual` values using `autoFarmFertilizerDelayedSubmitScopes`;
- when enabled, a numeric `地块间隔(毫秒)` input bound to `autoFarmFertilizerDelayedSubmitIntervalMs`, defaulting to 500 and clamping at zero.

When the toggle is first enabled and the scope key is missing, use all three values as the rendered default. Do not change `autoFarmFertilizerEnabled`, task scheduling, fertilizer modes, or single-card actions.

- [ ] **Step 4: Run the frontend rendering test and verify it passes**

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: PASS.

- [ ] **Step 5: Commit the settings UI**

```powershell
git add frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx
git commit -m "feat: configure delayed fertilizer submission"
```

### Task 5: Run Focused Integration Verification

**Files:**
- Verify: `internal/farm/automation/fertilizer_submission_test.go`
- Verify: `internal/farm/automation/runtime_own_test.go`
- Verify: `app_test.go`
- Verify: `frontend/src/views/AutomationView.test.tsx`
- Verify: `frontend/src/views/AssetsLandView.test.tsx`
- Verify: `scripts/test-fertilizer-repeat-skip.js`

- [ ] **Step 1: Run the complete focused test set**

```powershell
go test ./internal/farm/automation -run '^Test(FertilizerSubmissionRuntimeArgs|RuntimeFacadeDelayedPlantingSubmissionUsesPlantingScope|RuntimeFacadeDelayedAutomaticFertilizerRequiresRushScope|RuntimeFacadeMultiSeasonDelayedSubmissionUsesPlantingScope)$' -count=1
go test . -run '^TestFarmLandRush(DelayedManualSubmission|LeavesUnmarkedRequestInBatchMode)$' -count=1
npm test -- --run src/views/AutomationView.test.tsx src/views/AssetsLandView.test.tsx
node scripts/test-fertilizer-repeat-skip.js
git diff --check
```

Expected: all commands exit 0. The Node script proves rapid batch order and serial order separately; the Go and frontend tests prove the selected scope is propagated from saved settings to the runtime.

- [ ] **Step 2: Compile all Go packages without running the full suite**

Run: `go test ./... -run '^$' -count=0`

Expected: all packages compile successfully.

- [ ] **Step 3: Review the final diff before handoff**

Run: `git status --short` and `git diff --check`.

Expected: only the files named in this plan, plus the already-approved fertilizer batch regression changes, are modified.
