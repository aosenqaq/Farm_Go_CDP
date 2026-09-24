# Fertilizer Repeat Skip Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Treat `same_fertilizer_type_already_used` as a successful season-scoped fertilizer skip and record missing-event diagnostics.

**Architecture:** The injected QQ runtime owns a per-season cache because it has live land, plant, and season data. It returns explicit skipped entries plus bounded diagnostics. Go accepts the known legacy `ok: false` skip result while preserving every other fertilizer error.

**Tech Stack:** JavaScript, Node `vm`, Go.

---

### Task 1: Runtime Cache Regression Test

**Files:**

- Create: `scripts/test-fertilizer-repeat-skip.js`
- Modify: `resources/wmpf/button.js:18260-18935`

- [ ] **Step 1: Write the failing Node VM test**

Create `scripts/test-fertilizer-repeat-skip.js` by adapting the VM bootstrap in `scripts/test-protocol-failure-text.js`. Read the real `button.js`, then inject these names before the `G.gameCtl = {` marker:

```js
G.__testFertilizerSeasonSkipKey = fertilizerSeasonSkipKey;
G.__testPartitionFertilizerSeasonSkips = partitionFertilizerSeasonSkips;
G.__testRememberFertilizerSeasonSkip = rememberFertilizerSeasonSkip;
G.__testBuildFertilizerProtocolFailureDiagnostics = buildFertilizerProtocolFailureDiagnostics;
```

Run the source in its existing minimal Cocos VM. Use two entries with `before` values `{ landId: 7, plantId: 1020003, currentSeason: 1 }` and `{ landId: 8, plantId: 1020004, currentSeason: 1 }`, then assert the cache behavior:

```js
assert.deepStrictEqual(partition([land7, land8]).pending.map((entry) => entry.landId), [7, 8]);
remember(land7);
assert.deepStrictEqual(partition([land7, land8]).skipped.map((entry) => entry.landId), [7]);
assert.deepStrictEqual(
  partition([{ ...land7, before: { ...land7.before, currentSeason: 2 } }]).pending.map((entry) => entry.landId),
  [7],
);
```

Pass a fixture for `fertilizer protocol event was not dispatched` to the diagnostics helper and assert it retains request IDs, mode, before-state, bucket counts, manager state, `captured: false`, and `dispatchCount: 0`.

- [ ] **Step 2: Run the Node test and verify it fails**

Run `node scripts/test-fertilizer-repeat-skip.js`.

Expected: FAIL because the injected runtime helpers are undefined.

- [ ] **Step 3: Commit the failing test**

Run `git add scripts/test-fertilizer-repeat-skip.js` followed by `git commit -m "test: cover fertilizer season skip runtime behavior"`.

### Task 2: Runtime Cache And Diagnostics

**Files:**

- Modify: `resources/wmpf/button.js:18260-18935`
- Test: `scripts/test-fertilizer-repeat-skip.js`

- [ ] **Step 1: Add the cache helpers**

Add this code immediately before `normalizeRepeatedFertilizerPromptReason`. The local `Set` is intentionally cleared when the QQ runtime reloads.

```js
const fertilizerSeasonSkipCache = new Set();

function fertilizerSeasonSkipKey(state) {
  const landId = normalizeLandId(state && state.landId);
  const plantId = Number(state && state.plantId);
  const season = Number(state && state.currentSeason);
  if (!landId || !Number.isFinite(plantId) || plantId <= 0 || !Number.isFinite(season) || season <= 0) return null;
  return [landId, plantId, Math.floor(season)].join(':');
}

function rememberFertilizerSeasonSkip(entry) {
  const key = fertilizerSeasonSkipKey(entry && entry.before);
  if (key) fertilizerSeasonSkipCache.add(key);
  return key;
}

function partitionFertilizerSeasonSkips(entries) {
  const skipped = [];
  const pending = [];
  (Array.isArray(entries) ? entries : []).forEach(function (entry) {
    const key = fertilizerSeasonSkipKey(entry && entry.before);
    if (key && fertilizerSeasonSkipCache.has(key)) skipped.push(entry);
    else pending.push(entry);
  });
  return { skipped: skipped, pending: pending };
}
```

- [ ] **Step 2: Use the cache in `fertilizeLandsBatch`**

Partition `targetEntries` before any interaction UI opens. Dispatch and verify only pending entries. Append a successful skipped result for every cached entry and return immediately when all targets are cached:

```js
{
  index: entry.index,
  landId: entry.landId,
  ok: true,
  action: 'skipped',
  reason: 'same_fertilizer_type_already_used',
  executionSource: 'fertilizer_season_skip_cache',
  before: summarizeFertilizerLandState(entry.before),
  after: summarizeFertilizerLandState(entry.before),
  deltaMatureInSec: 0,
  error: null,
}
```

Implement `summarizeFertilizerLandState` with exactly `plantId`, `stageKind`, `matureInSec`, `currentSeason`, and `totalSeason`; use it for `payload.before` and each result state.

After `resolveRepeatedFertilizerPrompt` identifies the known prompt, preserve entries with observed fertilizer effects. Convert only no-effect pending entries to the result above, call `rememberFertilizerSeasonSkip` for each, set `skippedCount`, set `failureCount` to zero, and return `ok: true`. This keeps mixed batches working without changing observed actions into skips.

- [ ] **Step 3: Add bounded missing-event diagnostics**

Return `captured: !!captured` from `dispatchFertilizerProtocolBatch`. Add this helper before `fertilizeLandsBatch`:

```js
function buildFertilizerProtocolFailureDiagnostics(payload, batchDispatch) {
  return {
    reason: 'fertilizer protocol event was not dispatched',
    requestedLandIds: Array.isArray(payload.landIds) ? payload.landIds.slice() : [],
    mode: payload.resolvedMode || null,
    before: Array.isArray(payload.before) ? payload.before.slice() : [],
    bucket: {
      selectedBefore: payload.selectedBucketCountBefore == null ? null : payload.selectedBucketCountBefore,
      selectedAfter: payload.selectedBucketCountAfter == null ? null : payload.selectedBucketCountAfter,
      delta: payload.selectedBucketDeltaCount == null ? null : payload.selectedBucketDeltaCount,
    },
    manager: payload.managerStateAfterPerform || null,
    protocol: {
      captured: !!(batchDispatch && batchDispatch.captured),
      eventName: batchDispatch && batchDispatch.eventName ? batchDispatch.eventName : null,
      dispatchCount: batchDispatch && Array.isArray(batchDispatch.dispatches) ? batchDispatch.dispatches.length : 0,
      performResult: summarizeSpyValue(batchDispatch && batchDispatch.result, 1),
      performError: batchDispatch && batchDispatch.error ? String(batchDispatch.error) : null,
    },
  };
}
```

Set `payload.failureDiagnostics` only if `performError === 'fertilizer protocol event was not dispatched'`. Leave error text, inventory, and auto-fill logic unchanged.

- [ ] **Step 4: Run the Node test and verify it passes**

Run `node scripts/test-fertilizer-repeat-skip.js`.

Expected: land 7 is skipped only for its original plant and season; the diagnostics fixture reports the uncaptured event without changing its reason.

- [ ] **Step 5: Commit the runtime implementation**

Run `git add resources/wmpf/button.js scripts/test-fertilizer-repeat-skip.js` followed by `git commit -m "fix: skip repeated fertilizer within a season"`.

### Task 3: Automation Response Parsing

**Files:**

- Modify: `internal/farm/automation/runtime_helpers.go:626-684`
- Modify: `internal/farm/automation/runtime_own.go:1180-1230`
- Modify: `internal/farm/automation/runtime_own_test.go:1988-2034`

- [ ] **Step 1: Write the failing Go regression test**

Add `TestRuntimeFacadeOwnFertilizerTreatsAlreadyUsedBatchAsSkip` next to `TestRuntimeFacadeOwnFertilizerReportsRuntimeNotOKResult`. Return a growing own-farm land, then return this fertilizer batch response:

```go
map[string]any{
    "ok":     false,
    "action": "skipped",
    "reason": "same_fertilizer_type_already_used",
    "results": []any{map[string]any{
        "ok": false, "action": "skipped",
        "reason": "same_fertilizer_type_already_used", "landId": float64(8),
    }},
}
```

Assert `result.OK`, `result.Status == StatusOK`, exactly two runtime calls, and a message containing `本季已施肥跳过 1 块`. Retain the existing `out_of_stock` regression as the negative case.

- [ ] **Step 2: Run the focused test and verify it fails**

Run `go test ./internal/farm/automation -run TestRuntimeFacadeOwnFertilizerTreatsAlreadyUsedBatchAsSkip -count=1`.

Expected: FAIL because every `ok: false` fertilizer result currently returns `StatusFailed`.

- [ ] **Step 3: Add exact-reason parsing and the scheduler branch**

Add these helpers in `runtime_helpers.go`:

```go
func isSameFertilizerTypeAlreadyUsed(value any) bool {
    result := mapFromAny(value)
    if strings.EqualFold(strings.TrimSpace(fmt.Sprint(result["reason"])), "same_fertilizer_type_already_used") {
        return true
    }
    entries := sliceFromAny(result["results"])
    if len(entries) == 0 {
        return false
    }
    for _, item := range entries {
        entry := mapFromAny(item)
        if !strings.EqualFold(strings.TrimSpace(fmt.Sprint(entry["reason"])), "same_fertilizer_type_already_used") {
            return false
        }
    }
    return true
}

func sameFertilizerTypeAlreadyUsedCount(value any, fallback int) int {
    count := 0
    for _, item := range sliceFromAny(mapFromAny(value)["results"]) {
        if strings.EqualFold(strings.TrimSpace(fmt.Sprint(mapFromAny(item)["reason"])), "same_fertilizer_type_already_used") {
            count++
        }
    }
    if count == 0 && isSameFertilizerTypeAlreadyUsed(value) {
        return fallback
    }
    return count
}
```

In `runOwnFertilizer`, parse the map once. Before the existing non-OK return, accept only `isSameFertilizerTypeAlreadyUsed(result)`, increment `skippedLandCount`, and continue without enabling linked harvest. Prefix both successful message bodies with:

```go
skipPrefix := ""
if skippedLandCount > 0 {
    skipPrefix = fmt.Sprintf("本季已施肥跳过 %d 块，", skippedLandCount)
}
```

All other non-OK responses retain existing failure handling.

- [ ] **Step 4: Run focused tests and verify they pass**

Run `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnFertilizer(TreatsAlreadyUsedBatchAsSkip|ReportsRuntimeNotOKResult)' -count=1`.

Expected: PASS; only the exact repeated-fertilizer reason becomes a skip.

- [ ] **Step 5: Commit the Go implementation**

Run `git add internal/farm/automation/runtime_helpers.go internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go` followed by `git commit -m "fix: treat repeated fertilizer response as skip"`.

### Task 4: Integrated Verification

**Files:**

- Verify: `scripts/test-fertilizer-repeat-skip.js`
- Verify: `resources/wmpf/button.js`
- Verify: `internal/farm/automation/runtime_own.go`

- [ ] **Step 1: Run focused checks**

Run `node scripts/test-fertilizer-repeat-skip.js` and `go test ./internal/farm/automation -count=1`.

Expected: the VM cache/diagnostics test and all automation tests pass.

- [ ] **Step 2: Run project verification**

Run `npm --prefix frontend test`, `npm --prefix frontend run build`, `go test ./... -count=1`, and `git diff --check`.

Expected: all frontend and Go checks pass, and the diff check is empty.

- [ ] **Step 3: Inspect branch state**

Run `git status --short` and `git log --oneline master..HEAD`.

Expected: only the spec, plan, Node test, runtime script, and automation files are on `codex/fertilizer-repeat-skip`.
