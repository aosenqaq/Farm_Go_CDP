# Auto Plant Execution Pacing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Make random land order and random inter-land planting delays effective without repeating strategy or backpack resolution.

**Architecture:** Go resolves empty lands, strategy, and seed allocation once into seed-bound plans. Each one remains a single gameCtl.autoPlant call. The JavaScript runtime converts the plan to dispatch units, preserves or shuffles those units, then sequentially dispatches them with optional waits. Normal plants have one land per unit; four-grid plants have one complete 2x2 group per unit.

**Tech Stack:** Go, JavaScript, Node VM, React, TypeScript, Vitest.

---

## File Structure

- resources/wmpf/button.js: execution-settings normalization, dispatch-unit ordering and pacing, deadline checks, result summary.
- scripts/test-auto-plant-execution.js: Node VM regression test for the real runtime source.
- auto_plant_button_test.go: structural assertion that verification awaits the paced runtime dispatcher.
- internal/farm/automation/runtime_own.go: deadline and timeout calculation for every seed-bound plan.
- internal/farm/automation/runtime_own_test.go: timing, single backpack read, and plan-call-count tests.
- frontend/src/views/AutomationView.tsx: plant delay validation and input maximum.
- frontend/src/views/AutomationView.test.tsx: persistence and invalid-range UI tests.

### Task 1: Validate Planting Delay Settings

**Files:**
- Modify: frontend/src/views/AutomationView.tsx:505-529,1138-1153,1605-1635
- Modify: frontend/src/views/AutomationView.test.tsx:913-948,1211-1257

- [ ] **Step 1: Write failing frontend tests**

Add a successful interactive save test that enables config-autoFarmPlantRandomizedEnabled, changes its range to 1200 and 3600, clicks .automation-save-button, and checks the saved config values.

Add a table-driven invalid-range test using these exact cases:

~~~
['5000', '1000', '最大种植随机延迟不能小于最小种植随机延迟']
['-1', '1000', '种植随机延迟必须是 0 到 10000 的整数']
['100', '10001', '种植随机延迟必须是 0 到 10000 的整数']
~~~

Each case must assert that onSaveState is never called. Extend the static rendering test to assert that both planting delay inputs contain max="10000".

- [ ] **Step 2: Verify the UI tests fail**

Run from frontend/:

~~~powershell
npm test -- AutomationView.test.tsx
~~~

Expected: FAIL because saveSettings validates only friend-steal delay and the planting fields have no maximum.

- [ ] **Step 3: Implement the validation**

Add the following helper next to friendStealRandomDelayValidationError:

~~~tsx
const AUTO_PLANT_RANDOM_DELAY_MAX_MS = 10_000;

function plantRandomDelayValidationError(config: Record<string, any>) {
  const min = Number(config.autoFarmPlantRandomizedDelayMinMs ?? 100);
  const max = Number(config.autoFarmPlantRandomizedDelayMaxMs ?? 500);
  if (
    !Number.isInteger(min) ||
    !Number.isInteger(max) ||
    min < 0 ||
    max < 0 ||
    min > AUTO_PLANT_RANDOM_DELAY_MAX_MS ||
    max > AUTO_PLANT_RANDOM_DELAY_MAX_MS
  ) {
    return '种植随机延迟必须是 0 到 10000 的整数';
  }
  if (max < min) {
    return '最大种植随机延迟不能小于最小种植随机延迟';
  }
  return '';
}
~~~

Replace the validation declaration at the start of saveSettings with:

~~~tsx
const validationError =
  friendStealRandomDelayValidationError(draftStateRef.current.config) ||
  plantRandomDelayValidationError(draftStateRef.current.config);
~~~

Add max={AUTO_PLANT_RANDOM_DELAY_MAX_MS} and disabled={!configBool('autoFarmPlantRandomizedEnabled', false)} to both planting-delay inputs. Leave all existing config keys and defaults unchanged.

- [ ] **Step 4: Verify and commit**

~~~powershell
cd frontend
npm test -- AutomationView.test.tsx
git add src/views/AutomationView.tsx src/views/AutomationView.test.tsx
git commit -m "fix: validate auto plant delay range"
~~~

Expected: focused Vitest suite PASS.

### Task 2: Pace Runtime Dispatch Units

**Files:**
- Modify: resources/wmpf/button.js:14179-14374
- Create: scripts/test-auto-plant-execution.js
- Modify: auto_plant_button_test.go:8-29

- [ ] **Step 1: Write the failing Node VM test**

Create scripts/test-auto-plant-execution.js. It must inject these three local runtime functions into the VM immediately before G.gameCtl is assigned:

~~~js
G.__testNormalizeAutoPlantExecutionSettings = normalizeAutoPlantExecutionSettings;
G.__testOrderAutoPlantExecutionUnits = orderAutoPlantExecutionUnits;
G.__testDispatchAutoPlantExecutionUnits = dispatchAutoPlantExecutionUnits;
~~~

Use the same fs/path/vm setup as scripts/test-protocol-failure-text.js. With injected no-op Cocos globals, assert all of the following:

~~~
normalize({ enabled: true, min: 20, max: 20 })
  -> { randomOrderEnabled: false, randomDelayEnabled: true, minDelayMs: 20, maxDelayMs: 20, deadlineAtMs: 0 }

order([{single:[1]}, {single:[2]}, {single:[3]}], randomOrder=false)
  -> [[1], [2], [3]]

dispatch three single units with a fixed 20ms delay
  -> dispatch order [1,2,3], waits [20,20], exactly two waits

order([{multi:[1,2,5,6]}, {single:[8]}], randomOrder=true, random()=0)
  -> [[8], [1,2,5,6]]

dispatch with now=95, deadline=100, fixed 10ms delay
  -> plant_execution_timeout and only first unit dispatched
~~~

Add a structural Go assertion in TestAutoPlantPreservesIncompleteFourGridReasonBeforeVerification that the verified function contains:

~~~go
plantResult = await plantSeedsOnLands(candidate, targetLandIds, opts);
~~~

- [ ] **Step 2: Verify the new tests fail**

~~~powershell
node scripts/test-auto-plant-execution.js
go test . -run '^TestAutoPlantPreservesIncompleteFourGridReasonBeforeVerification$' -count=1
~~~

Expected: both commands FAIL because the helpers do not exist and plantSeedsOnLands is currently synchronous.

- [ ] **Step 3: Add execution settings and ordering helpers**

Immediately before plantSeedsOnLands in resources/wmpf/button.js, add these complete helpers:

~~~js
const AUTO_PLANT_RANDOM_DELAY_MAX_MS = 10000;

function normalizeAutoPlantExecutionSettings(opts) {
  const source = opts && typeof opts === 'object' ? opts : {};
  const normalizeDelay = function (value) {
    const number = Number(value);
    if (!Number.isFinite(number)) return 0;
    return Math.min(AUTO_PLANT_RANDOM_DELAY_MAX_MS, Math.max(0, Math.trunc(number)));
  };
  const minDelayMs = normalizeDelay(source.autoPlantRandomizedDelayMinMs);
  const requestedMaxDelayMs = normalizeDelay(source.autoPlantRandomizedDelayMaxMs);
  const deadlineValue = Number(source.plantExecutionDeadlineAtMs);
  return {
    randomOrderEnabled: source.autoPlantRandomOrderEnabled === true,
    randomDelayEnabled: source.autoPlantRandomizedEnabled === true,
    minDelayMs: minDelayMs,
    maxDelayMs: Math.max(minDelayMs, requestedMaxDelayMs),
    deadlineAtMs: Number.isFinite(deadlineValue) && deadlineValue > 0 ? Math.trunc(deadlineValue) : 0,
  };
}

function orderAutoPlantExecutionUnits(units, settings, randomFn) {
  const ordered = Array.isArray(units) ? units.slice() : [];
  if (!settings.randomOrderEnabled) return ordered;
  const random = typeof randomFn === 'function' ? randomFn : Math.random;
  for (let index = ordered.length - 1; index > 0; index -= 1) {
    const raw = Number(random());
    const bounded = Number.isFinite(raw) ? Math.max(0, Math.min(0.9999999999999999, raw)) : 0;
    const target = Math.floor(bounded * (index + 1));
    const value = ordered[index];
    ordered[index] = ordered[target];
    ordered[target] = value;
  }
  return ordered;
}

function autoPlantExecutionDelayMs(settings, randomFn) {
  if (!settings.randomDelayEnabled) return 0;
  if (settings.maxDelayMs <= settings.minDelayMs) return settings.minDelayMs;
  const random = typeof randomFn === 'function' ? randomFn : Math.random;
  const raw = Number(random());
  const bounded = Number.isFinite(raw) ? Math.max(0, Math.min(0.9999999999999999, raw)) : 0;
  return settings.minDelayMs + Math.floor(bounded * (settings.maxDelayMs - settings.minDelayMs + 1));
}
~~~

Add this dispatcher directly after those helpers:

~~~js
async function dispatchAutoPlantExecutionUnits(units, settings, dispatchUnit, nowFn, waitFn, randomFn) {
  const ordered = orderAutoPlantExecutionUnits(units, settings, randomFn);
  const now = typeof nowFn === 'function' ? nowFn : Date.now;
  const waitFor = typeof waitFn === 'function' ? waitFn : wait;
  const payloads = [];
  const delaysMs = [];
  for (let index = 0; index < ordered.length; index += 1) {
    if (settings.deadlineAtMs > 0 && now() >= settings.deadlineAtMs) {
      return { ok: false, reason: 'plant_execution_timeout', units: ordered, payloads: payloads, delaysMs: delaysMs };
    }
    if (index > 0) {
      const requestedDelayMs = autoPlantExecutionDelayMs(settings, randomFn);
      if (requestedDelayMs > 0) {
        const remainingMs = settings.deadlineAtMs > 0 ? settings.deadlineAtMs - now() : requestedDelayMs;
        const actualDelayMs = settings.deadlineAtMs > 0 ? Math.max(0, Math.min(requestedDelayMs, remainingMs)) : requestedDelayMs;
        if (actualDelayMs > 0) {
          delaysMs.push(actualDelayMs);
          await waitFor(actualDelayMs);
        }
        if (settings.deadlineAtMs > 0 && now() >= settings.deadlineAtMs) {
          return { ok: false, reason: 'plant_execution_timeout', units: ordered, payloads: payloads, delaysMs: delaysMs };
        }
      }
    }
    payloads.push(dispatchUnit(ordered[index]));
  }
  return { ok: true, units: ordered, payloads: payloads, delaysMs: delaysMs };
}
~~~

- [ ] **Step 4: Replace synchronous dispatch with units**

Change plantSeedsOnLands to async. Keep its current initial validation and its existing no_complete_multi_land_group result unchanged.

For the multi-land success branch, construct one unit per grouped.groups entry and call:

~~~js
const execution = await dispatchAutoPlantExecutionUnits(
  grouped.groups.map(function (group) {
    return { kind: 'multi', landIds: orderMultiLandPlantGroupForDispatch(group) };
  }),
  normalizeAutoPlantExecutionSettings(opts),
  function (unit) { return dispatchMultiLandPlant(plantOrSeedId, unit.landIds); }
);
~~~

For the ordinary-land branch, construct one unit per normalized land ID and call:

~~~js
const execution = await dispatchAutoPlantExecutionUnits(
  normalizedLandIds.map(function (landId) {
    return { kind: 'single', landIds: [landId] };
  }),
  normalizeAutoPlantExecutionSettings(opts),
  function (unit) { return dispatchSingleLandPlant(plantOrSeedId, unit.landIds[0]); }
);
~~~

In either branch, return planted:false and reason: execution.reason when execution.ok is false. Do not run verification after plant_execution_timeout. On success, use execution.payloads in the existing result and add only this bounded summary:

~~~js
execution: {
  unitCount: execution.units.length,
  orderedLandIds: execution.units.map(function (unit) { return unit.landIds.slice(); }),
  delaysMs: execution.delaysMs.slice(),
  randomOrderEnabled: normalizeAutoPlantExecutionSettings(opts).randomOrderEnabled,
  randomDelayEnabled: normalizeAutoPlantExecutionSettings(opts).randomDelayEnabled,
}
~~~

Replace the current un-awaited invocation in plantSeedsOnLandsVerified with:

~~~js
plantResult = await plantSeedsOnLands(candidate, targetLandIds, opts);
~~~

After the existing four-grid early return, add an equivalent plant_execution_timeout early return. It must preserve reason and execution and skip await wait(waitAfterPlantMs).

- [ ] **Step 5: Verify and commit the runtime change**

~~~powershell
node scripts/test-auto-plant-execution.js
go test . -run '^TestAutoPlant' -count=1
git add resources/wmpf/button.js scripts/test-auto-plant-execution.js auto_plant_button_test.go
git commit -m "feat: pace auto plant execution"
~~~

Expected: PASS. Random delay yields N - 1 waits; delay-off preserves the original Go plan order; random ordering never splits a four-grid unit.

### Task 3: Set a Bounded Facade Deadline Per Plan

**Files:**
- Modify: internal/farm/automation/runtime_own.go:299-408,764-783
- Modify: internal/farm/automation/runtime_own_test.go:792-837,996-1073

- [ ] **Step 1: Write failing timing and plan-count tests**

Add this pure timing test:

~~~go
func TestOwnPlantRuntimeTimingUsesMaximumInterLandDelay(t *testing.T) {
  timeout, deadline := ownPlantRuntimeTiming(map[string]any{
    "autoPlantRandomizedEnabled": true,
    "autoPlantRandomizedDelayMaxMs": 500,
    "emptyLandIds": []int{1, 2, 3},
  }, time.UnixMilli(1000))
  if timeout != 61*time.Second {
    t.Fatalf("timeout = %v, want %v", timeout, 61*time.Second)
  }
  if deadline != 62000 {
    t.Fatalf("deadline = %d, want 62000", deadline)
  }
}
~~~

Add a facade test with three empty lands and two backpack seed types of counts one and two. Configure backpack_first with random delay enabled. Assert exactly one gameCtl.getSeedList call, exactly two gameCtl.autoPlant calls, and that every payload contains a positive plantExecutionDeadlineAtMs.

- [ ] **Step 2: Verify the facade tests fail**

~~~powershell
go test ./internal/farm/automation -run '^(TestOwnPlantRuntimeTimingUsesMaximumInterLandDelay|TestRuntimeFacadeOwnPlantUsesOneBackpackReadAndOneCallPerSeedPlan)$' -count=1
~~~

Expected: FAIL because the timing helper and deadline payload do not exist.

- [ ] **Step 3: Add bounded timing calculation**

Add this code near baseOwnPlantOptions in runtime_own.go:

~~~go
const (
  ownPlantRuntimeBaseTimeout = 60 * time.Second
  ownPlantRuntimeMaxTimeout = 15 * time.Minute
  ownPlantRandomDelayMaxMs = 10_000
)

func ownPlantRuntimeTiming(plantOpts map[string]any, now time.Time) (time.Duration, int64) {
  timeout := ownPlantRuntimeBaseTimeout
  landIDs := normalizeUniquePositiveInts(sliceFromAny(plantOpts["emptyLandIds"]))
  if !boolConfigAny(plantOpts["autoPlantRandomizedEnabled"]) || len(landIDs) < 2 {
    return timeout, now.Add(timeout).UnixMilli()
  }
  maxDelayMs := intFromAny(plantOpts["autoPlantRandomizedDelayMaxMs"])
  if maxDelayMs < 0 {
    maxDelayMs = 0
  }
  if maxDelayMs > ownPlantRandomDelayMaxMs {
    maxDelayMs = ownPlantRandomDelayMaxMs
  }
  if maxDelayMs == 0 {
    return timeout, now.Add(timeout).UnixMilli()
  }
  maxExtra := ownPlantRuntimeMaxTimeout - ownPlantRuntimeBaseTimeout
  maxIntervals := int64(maxExtra / (time.Duration(maxDelayMs) * time.Millisecond))
  intervals := int64(len(landIDs) - 1)
  if intervals >= maxIntervals {
    timeout = ownPlantRuntimeMaxTimeout
  } else {
    timeout += time.Duration(intervals*int64(maxDelayMs)) * time.Millisecond
  }
  return timeout, now.Add(timeout).UnixMilli()
}
~~~

Immediately before each gameCtl.autoPlant call, set timing and replace its fixed timeout:

~~~go
plantTimeout, deadlineAtMs := ownPlantRuntimeTiming(plantOpts, time.Now())
plantOpts["plantExecutionDeadlineAtMs"] = deadlineAtMs
plantResult, err := r.caller.Call(ctx, "gameCtl.autoPlant", []any{plantOpts}, plantTimeout)
~~~

Do not split emptyLandIds in Go. One seed-bound plan stays one runtime call, preserving one strategy/backpack resolution pass and one fertilizer follow-up per plan.

- [ ] **Step 4: Verify and commit facade behavior**

~~~powershell
go test ./internal/farm/automation -run '^TestRuntimeFacadeOwnPlant' -count=1
git add internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go
git commit -m "fix: bound auto plant runtime deadlines"
~~~

Expected: PASS, including existing four-grid fallback and plant-fertilizer coverage.

### Task 4: Full Verification

**Files:**
- Verify: all files from Tasks 1-3.

- [ ] **Step 1: Run local automated suites**

~~~powershell
node scripts/test-auto-plant-execution.js
go test . -run '^TestAutoPlant' -count=1
go test ./internal/farm/automation -count=1
cd frontend
npm test
npm run build
cd ..
go test ./... -count=1
git diff --check
git status --short
~~~

Expected: every test and build PASS; no whitespace errors; no unexpected source changes.

- [ ] **Step 2: Run the zero-UI runtime acceptance check**

Install the current QQ patch, restart the host, reset runtime spies, and use the runtime diagnostic API to run one three-land normal-crop plan with fixed 100ms delay and random order disabled. Confirm three plant dispatches occur in plan order, exactly two intervals are near 100ms, one seed-list read occurs before dispatch, and no click, target popup, or UI-open event appears. Store only disposable captures in data/debug-captures/ and do not commit them.

- [ ] **Step 3: Commit only a required post-suite correction**

~~~powershell
git add resources/wmpf/button.js scripts/test-auto-plant-execution.js auto_plant_button_test.go internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx
git commit -m "fix: verify auto plant execution pacing"
~~~

Run this command only when a source correction was needed after the full verification. Otherwise do not create an empty commit.
