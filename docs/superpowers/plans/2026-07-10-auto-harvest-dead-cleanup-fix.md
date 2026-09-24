# Auto Harvest Dead Crop Cleanup Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep one-click farming limited to care work, make auto harvest reliably shovel dead crops, and report confirmed harvest and shovel counts with the required log wording.

**Architecture:** `runOwnBase` will stop dispatching harvest requests. `runOwnCollect` remains the sole owner of own-farm harvesting and will derive counts from runtime result payloads. The JavaScript shovel batch will use the same `eraseDead` action set as farm status scanning so its `onlyDead` safety check does not contradict the scan that selected the lands.

**Tech Stack:** Go 1.25, JavaScript runtime bundle, Node.js assertion scripts, Wails runtime calls.

---

## File Structure

- Modify `internal/farm/automation/runtime_own_test.go`: one-click responsibility, auto-harvest ordering, and exact log-message tests.
- Modify `internal/farm/automation/runtime_own.go`: remove harvesting from `own_base`, preserve confirmed counts in `own_collect`, and return shovel success counts.
- Modify `internal/farm/automation/runtime_helpers.go`: add a runtime-result success counter.
- Create `scripts/test-shovel-dead-action-sets.js`: VM regression test for action-set-backed dead detection.
- Modify `resources/wmpf/button.js`: preserve `eraseDead` state through shovel validation.

### Task 1: Remove Harvesting From One-Click Farming

**Files:**
- Modify: `internal/farm/automation/runtime_own_test.go:9`
- Modify: `internal/farm/automation/runtime_own.go:11`

- [ ] **Step 1: Rewrite the one-click farming tests to reject harvest calls**

Keep collectable lands in the status fixture, remove the fake harvest response, and assert:

```go
wantMethods := []string{
	"gameCtl.getFarmOwnership",
	"gameCtl.getFarmStatus",
	"gameCtl.triggerOneClickOperation",
}
if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
	t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
}
if result.Message != "一键务农已执行：照料 2 项，清理金虫 0 处。" {
	t.Fatalf("message = %q", result.Message)
}
```

Update the legacy `landIds.collect` test the same way and assert its message contains no `收获`.

- [ ] **Step 2: Run the focused tests and verify RED**

```powershell
go test ./internal/farm/automation -run 'TestRuntimeFacadeRunsOwnBase' -count=1
```

Expected: FAIL because `runOwnBase` still calls `gameCtl.harvestLandsBatchByProtocol` and reports a harvest count.

- [ ] **Step 3: Remove the harvest branch and fix messages**

Delete the `harvestIDs` collection and harvest runtime call. Keep care and golden-bug operations unchanged:

```go
if actions == 0 {
	return ActionResult{
		OK:      true,
		Status:  StatusOK,
		TaskID:  taskID,
		Message: "没有检测到可执行的浇水、除草或杀虫任务，本轮一键务农跳过。",
	}
}

return ActionResult{
	OK:      true,
	Status:  StatusOK,
	TaskID:  taskID,
	Message: fmt.Sprintf("一键务农已执行：照料 %d 项，清理金虫 %d 处。", careCount, len(goldenBugIDs)),
}
```

- [ ] **Step 4: Run the focused tests and verify GREEN**

```powershell
go test ./internal/farm/automation -run 'TestRuntimeFacadeRunsOwnBase|TestRuntimeFacadeOwnBase' -count=1
```

Expected: PASS with no harvest call from `own_base`.

- [ ] **Step 5: Commit the responsibility fix**

```powershell
git add internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go
git commit -m "fix: keep one-click farming limited to care"
```

### Task 2: Report Confirmed Auto-Harvest And Shovel Counts

**Files:**
- Modify: `internal/farm/automation/runtime_own_test.go:198`
- Modify: `internal/farm/automation/runtime_helpers.go:498`
- Modify: `internal/farm/automation/runtime_own.go:180`

- [ ] **Step 1: Add failing tests for exact confirmed-count messages**

Use a harvest response with duplicate successful attempts and one failed attempt:

```go
"gameCtl.harvestLandsBatchByProtocol": map[string]any{
	"ok": true,
	"results": []any{
		map[string]any{"ok": true, "landId": float64(1)},
		map[string]any{"ok": true, "landId": float64(2)},
		map[string]any{"ok": true, "landId": float64(2)},
		map[string]any{"ok": false, "landId": float64(3)},
	},
},
```

Assert these exact messages:

```go
"一键收获 2 块，清理枯萎 0 块。"
"一键收获 2 块，清理枯萎 1 块。"
"一键收获 0 块，清理枯萎 1 块。"
```

The three cases are harvest-only, harvest-plus-cleanup, and a later dead-only round.

Add a direct helper test so duplicate land attempts and skipped entries cannot inflate the log:

```go
func TestRuntimeSuccessfulOperationCountDeduplicatesLandResults(t *testing.T) {
	got := runtimeSuccessfulOperationCount(map[string]any{
		"results": []any{
			map[string]any{"ok": true, "landId": float64(1)},
			map[string]any{"ok": true, "landId": float64(1)},
			map[string]any{"ok": true, "action": "skipped", "landId": float64(2)},
			map[string]any{"ok": false, "landId": float64(3)},
		},
	})
	if got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

```powershell
go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnCollect' -count=1
```

Expected: FAIL because messages use requested land counts and the cleanup helper discards `successCount`.

- [ ] **Step 3: Add the runtime success counter**

Add next to `runtimeResultFailed`:

```go
func runtimeSuccessfulOperationCount(value any) int {
	result := mapFromAny(value)
	for _, key := range []string{"successCount", "claimedCount", "boughtCount"} {
		if _, exists := result[key]; exists {
			count := intFromAny(result[key])
			if count > 0 {
				return count
			}
			return 0
		}
	}

	seenLandIDs := map[int]bool{}
	successesWithoutLandID := 0
	for _, item := range sliceFromAny(result["results"]) {
		entry := mapFromAny(item)
		if len(entry) == 0 || entry["ok"] == false ||
			strings.EqualFold(strings.TrimSpace(fmt.Sprint(entry["action"])), "skipped") {
			continue
		}
		if landID := intFromAny(firstExistingAny(entry["landId"], entry["land_id"])); landID > 0 {
			seenLandIDs[landID] = true
		} else {
			successesWithoutLandID++
		}
	}
	return len(seenLandIDs) + successesWithoutLandID
}
```

- [ ] **Step 4: Preserve counts through `runOwnCollect`**

Initialize `harvestCount` and `shovelCount` to zero. Set `harvestCount` from the harvest result. Change the cleanup helper to return `(int, *ActionResult)`, returning:

```go
return runtimeSuccessfulOperationCount(shovelResult), nil
```

Emit:

```go
Message: fmt.Sprintf("一键收获 %d 块，清理枯萎 %d 块。", harvestCount, shovelCount),
```

- [ ] **Step 5: Run the focused tests and verify GREEN**

```powershell
go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnCollect|TestRuntimeSuccessfulOperationCount' -count=1
```

Expected: PASS with exact confirmed-count messages.

- [ ] **Step 6: Commit confirmed-count logging**

```powershell
git add internal/farm/automation/runtime_helpers.go internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go
git commit -m "fix: report confirmed auto harvest counts"
```

### Task 3: Keep Dead Detection Consistent In The JS Shovel Batch

**Files:**
- Create: `scripts/test-shovel-dead-action-sets.js`
- Modify: `resources/wmpf/button.js:23390`

- [ ] **Step 1: Write the failing JavaScript regression script**

Follow the existing VM script pattern and expose the planned predicate before `G.gameCtl`:

```js
source = source.replace(
  '  G.gameCtl = {',
  '  G.__testIsDeadShovelTarget = isDeadShovelTarget;\n  G.gameCtl = {',
);
```

Assert action-set membership supplies the missing dead signal:

```js
const isDeadShovelTarget = context.__testIsDeadShovelTarget;
assert.strictEqual(typeof isDeadShovelTarget, 'function');
const actionSets = { eraseDead: new Set([8]) };
assert.strictEqual(
  isDeadShovelTarget({ stageKind: 'growing', isDead: false, canEraseDead: false }, actionSets, 8),
  true,
);
assert.strictEqual(
  isDeadShovelTarget({ stageKind: 'growing', isDead: false, canEraseDead: false }, actionSets, 9),
  false,
);
```

Extract `shovelLandsBatch` text and assert it calls `getFarmWorkSummary`, carries `actionSets`, and calls `isDeadShovelTarget`.

- [ ] **Step 2: Run the script and verify RED**

```powershell
node scripts/test-shovel-dead-action-sets.js
```

Expected: FAIL because the predicate and action-set propagation do not exist.

- [ ] **Step 3: Add action-set-aware dead validation**

Add:

```js
function isDeadShovelTarget(state, actionSets, landId) {
  if (!state) return false;
  if (
    state.stageKind === 'dead'
    || state.isDead === true
    || state.canEraseDead === true
    || state.needsEraseDead === true
    || state.needEraseDead === true
  ) {
    return true;
  }
  return !!(actionSets && actionSets.eraseDead && actionSets.eraseDead.has(Number(landId)));
}
```

Resolve one work summary at the start of `shovelLandsBatch`:

```js
const root = findGridOrigin(opts.root || opts.path);
if (!root) throw new Error('GridOrigin not found');
const workSummary = getFarmWorkSummary({ path: root, farmType: 'own', silent: true });
const actionSets = workSummary && workSummary.sets ? workSummary.sets : null;
```

Pass `actionSets` into normalization and every shovel `getGridState` read. Replace the inline dead condition with:

```js
else if (onlyDead && !isDeadShovelTarget(before, actionSets, landId)) {
  skipReason = 'not_dead';
}
```

- [ ] **Step 4: Run JS regression checks and verify GREEN**

```powershell
node scripts/test-shovel-dead-action-sets.js
node scripts/test-fertilizer-linked-harvest.js
node --check resources/wmpf/button.js
```

Expected: all commands exit 0.

- [ ] **Step 5: Commit the runtime state fix**

```powershell
git add resources/wmpf/button.js scripts/test-shovel-dead-action-sets.js
git commit -m "fix: preserve dead crop state during shovel"
```

### Task 4: Full Regression Verification

**Files:**
- Verify: `internal/farm/automation/runtime_own.go`
- Verify: `internal/farm/automation/runtime_helpers.go`
- Verify: `resources/wmpf/button.js`

- [ ] **Step 1: Format changed Go files**

```powershell
gofmt -w internal/farm/automation/runtime_own.go internal/farm/automation/runtime_helpers.go internal/farm/automation/runtime_own_test.go
```

- [ ] **Step 2: Run the automation package**

```powershell
go test ./internal/farm/automation -count=1
```

Expected: PASS.

- [ ] **Step 3: Run all Go tests**

```powershell
go test ./... -count=1
```

Expected: PASS with zero failed packages.

- [ ] **Step 4: Run relevant JavaScript tests**

```powershell
node scripts/test-shovel-dead-action-sets.js
node scripts/test-fertilizer-linked-harvest.js
node scripts/test-protocol-failure-text.js
node --check resources/wmpf/button.js
```

Expected: all commands exit 0.

- [ ] **Step 5: Check the final diff**

```powershell
git diff --check
git status --short
```

Confirm:

- `own_base` cannot call `harvestLandsBatchByProtocol`.
- a dead-only `own_collect` round still calls shovel.
- `onlyDead` accepts `workSummary.sets.eraseDead` membership.
- logs use `一键收获 X 块，清理枯萎 Y 块。` with confirmed counts.
