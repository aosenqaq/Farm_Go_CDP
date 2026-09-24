# Multi-Season Fertilizer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply the configured planting fertilizer to multi-season crops that continue growing after a confirmed harvest, while reusing existing post-harvest scans and preserving dead-crop cleanup first.

**Architecture:** Add a small pure post-harvest module that accepts protocol results and a refreshed farm-status payload, returns only confirmed successful harvest IDs and later-season multi-crop anchor IDs, and resolves the configuration-gated planting fertilizer mode. Reuse that module in automatic collection, automatic linked harvest, and manual linked harvest. Each path keeps its existing status read, executes dead-crop cleanup first, and dispatches a non-linked fertilizer batch only afterward.

**Tech Stack:** Go 1.24, Wails runtime caller, QQ mini-game protocol facade, Go standard-library tests, existing fake runtime caller.

---

## File Structure

- Create: `internal/farm/automation/post_harvest.go` - pure configuration and post-harvest protocol/status selectors shared by runtime automation and `main.App`.
- Create: `internal/farm/automation/post_harvest_test.go` - selector and configuration contract tests with no runtime calls.
- Modify: `internal/farm/automation/runtime_own.go` - return the existing settled post-harvest state, then reuse it for ordered shovel and multi-season fertilizer work in `own_collect` and linked `own_fertilizer`.
- Modify: `internal/farm/automation/runtime_own_test.go` - runtime call-order tests for automatic collection and linked harvest.
- Modify: `app.go` - extend manual `FarmLandRush` linked-harvest cleanup using its current status read, after dead cleanup succeeds.
- Modify: `app_test.go` - manual linked-harvest result, order, and failure tests.

### Task 1: Define Pure Post-Harvest Contracts

**Files:**
- Create: `internal/farm/automation/post_harvest.go`
- Create: `internal/farm/automation/post_harvest_test.go`

- [ ] **Step 1: Write failing selector and mode tests**

Add these cases to `internal/farm/automation/post_harvest_test.go`:

```go
func TestSuccessfulRuntimeLandIDsRequiresExplicitSuccess(t *testing.T) {
    got := SuccessfulRuntimeLandIDs(map[string]any{
        "results": []any{
            map[string]any{"ok": true, "landId": float64(8)},
            map[string]any{"ok": true, "land_id": "3"},
            map[string]any{"ok": false, "landId": float64(5)},
            map[string]any{"landId": float64(6)},
            map[string]any{"ok": true, "landId": float64(8)},
        },
    })
    if want := []int{3, 8}; !reflect.DeepEqual(got, want) {
        t.Fatalf("successful land IDs = %#v, want %#v", got, want)
    }
}

func TestMultiSeasonContinuationLandIDsUsesConfirmedAnchors(t *testing.T) {
    status := map[string]any{"grids": []any{
        map[string]any{"landId": 1, "occupancyAnchorLandId": 5, "hasPlant": true, "stageKind": "growing", "isMultiSeason": true, "currentSeason": 2, "totalSeason": 3},
        map[string]any{"landId": 5, "occupancyAnchorLandId": 5, "hasPlant": true, "stageKind": "growing", "isMultiSeason": true, "currentSeason": 2, "totalSeason": 3},
        map[string]any{"landId": 7, "hasPlant": true, "stageKind": "growing", "isMultiSeason": true, "currentSeason": 1, "totalSeason": 3},
        map[string]any{"landId": 9, "hasPlant": true, "stageKind": "growing", "isMultiSeason": false, "currentSeason": 2, "totalSeason": 3},
    }}
    if got, want := MultiSeasonContinuationLandIDs(status, []int{1, 7, 9}), []int{5}; !reflect.DeepEqual(got, want) {
        t.Fatalf("multi-season anchors = %#v, want %#v", got, want)
    }
}

func TestMultiSeasonFertilizerModeUsesPlantingStrategy(t *testing.T) {
    cases := []struct {
        config map[string]any
        want   string
    }{
        {map[string]any{"autoFarmFertilizerEnabled": true, "autoFarmFertilizerMultiSeason": true, "autoFarmPlantFertilizerMode": "normal"}, "normal"},
        {map[string]any{"autoFarmFertilizerEnabled": true, "autoFarmFertilizerMultiSeason": true, "autoFarmPlantFertilizerMode": "organic"}, "organic"},
        {map[string]any{"autoFarmFertilizerEnabled": true, "autoFarmFertilizerMultiSeason": false, "autoFarmPlantFertilizerMode": "normal"}, ""},
        {map[string]any{"autoFarmFertilizerEnabled": true, "autoFarmFertilizerMultiSeason": true, "autoFarmPlantFertilizerMode": "none"}, ""},
        {map[string]any{"autoFarmFertilizerEnabled": false, "autoFarmFertilizerMultiSeason": true, "autoFarmPlantFertilizerMode": "normal"}, ""},
    }
    for _, tc := range cases {
        if got := MultiSeasonFertilizerMode(tc.config); got != tc.want {
            t.Fatalf("mode = %q, want %q for %#v", got, tc.want, tc.config)
        }
    }
}
```

- [ ] **Step 2: Run the new tests and verify they fail**

Run: `go test ./internal/farm/automation -run 'TestSuccessfulRuntimeLandIDsRequiresExplicitSuccess|TestMultiSeasonContinuationLandIDsUsesConfirmedAnchors|TestMultiSeasonFertilizerModeUsesPlantingStrategy' -count=1`

Expected: FAIL because `SuccessfulRuntimeLandIDs`, `MultiSeasonContinuationLandIDs`, and `MultiSeasonFertilizerMode` do not exist.

- [ ] **Step 3: Implement the pure selectors**

Create `internal/farm/automation/post_harvest.go` with these exported functions. Reuse existing package helpers such as `mapFromAny`, `sliceFromAny`, `intFromAny`, `firstExistingAny`, `boolFromAny`, `fertilizerMasterEnabled`, and `plantFertilizerMode`.

```go
package automation

import "sort"

func SuccessfulRuntimeLandIDs(value any) []int {
    result := mapFromAny(value)
    seen := map[int]bool{}
    for _, item := range sliceFromAny(result["results"]) {
        entry := mapFromAny(item)
        if !boolFromAny(entry["ok"]) {
            continue
        }
        if landID := intFromAny(firstExistingAny(entry["landId"], entry["land_id"])); landID > 0 {
            seen[landID] = true
        }
    }
    ids := make([]int, 0, len(seen))
    for landID := range seen {
        ids = append(ids, landID)
    }
    sort.Ints(ids)
    return ids
}

func MultiSeasonContinuationLandIDs(status map[string]any, harvestedLandIDs []int) []int {
    harvested := map[int]bool{}
    for _, landID := range harvestedLandIDs {
        if landID > 0 {
            harvested[landID] = true
        }
    }
    anchors := map[int]bool{}
    for _, item := range sliceFromAny(status["grids"]) {
        grid := mapFromAny(item)
        landID := intFromAny(firstExistingAny(grid["landId"], grid["id"]))
        anchorID := intFromAny(firstExistingAny(grid["occupancyAnchorLandId"], grid["landId"], grid["id"]))
        if anchorID <= 0 || (!harvested[landID] && !harvested[anchorID]) {
            continue
        }
        if !boolFromAny(grid["hasPlant"]) || grid["stageKind"] != "growing" || !boolFromAny(grid["isMultiSeason"]) {
            continue
        }
        if intFromAny(grid["totalSeason"]) <= 1 || intFromAny(grid["currentSeason"]) <= 1 {
            continue
        }
        anchors[anchorID] = true
    }
    ids := make([]int, 0, len(anchors))
    for landID := range anchors {
        ids = append(ids, landID)
    }
    sort.Ints(ids)
    return ids
}

func MultiSeasonFertilizerMode(config map[string]any) string {
    if !boolFromAny(config["autoFarmFertilizerMultiSeason"]) || !fertilizerMasterEnabled(config) {
        return ""
    }
    return plantFertilizerMode(config)
}
```

- [ ] **Step 4: Run the pure-contract tests and verify they pass**

Run: `go test ./internal/farm/automation -run 'TestSuccessfulRuntimeLandIDsRequiresExplicitSuccess|TestMultiSeasonContinuationLandIDsUsesConfirmedAnchors|TestMultiSeasonFertilizerModeUsesPlantingStrategy' -count=1`

Expected: PASS with all three tests green.

- [ ] **Step 5: Commit the pure contract**

```powershell
git add internal/farm/automation/post_harvest.go internal/farm/automation/post_harvest_test.go
git commit -m "feat: add multi-season post-harvest selectors"
```

### Task 2: Reuse Automatic Collection's Existing Scan

**Files:**
- Modify: `internal/farm/automation/runtime_own.go:131-309`
- Modify: `internal/farm/automation/runtime_own_test.go`

- [ ] **Step 1: Write failing collection order tests**

Add a test in `runtime_own_test.go` whose successful harvest result contains land `5`; its existing post-harvest status contains both:

```go
map[string]any{"landId": 5, "hasPlant": true, "stageKind": "growing", "isMultiSeason": true, "currentSeason": 2, "totalSeason": 3},
map[string]any{"landId": 9, "stageKind": "dead"},
```

Construct the facade with:

```go
map[string]any{
    "autoFarmFertilizerEnabled": true,
    "autoFarmFertilizerMultiSeason": true,
    "autoFarmPlantFertilizerMode": "normal",
}
```

Assert this exact suffix of runtime methods:

```go
[]string{
    "gameCtl.getFarmStatus",
    "gameCtl.harvestLandsBatchByProtocol",
    "gameCtl.getFarmStatus",
    "gameCtl.shovelLandsBatch",
    "gameCtl.fertilizeLandsBatch",
}
```

Assert that the final fertilizer payload uses `landIds: []int{5}`, `type: "normal"`, `linkedHarvestAfterFertilize: false`, and source `farm_go_auto_collect_multi_season`.

Add a second test with `gameCtl.shovelLandsBatch` returning `{"ok": false, "reason": "shovel_failed"}`. Assert the task fails and no second `gameCtl.fertilizeLandsBatch` call is made.

- [ ] **Step 2: Run the collection tests and verify they fail**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnCollect.*MultiSeason' -count=1`

Expected: FAIL because collection currently only extracts dead IDs from the rescan and has no multi-season batch call.

- [ ] **Step 3: Return status from the existing collection rescan and add ordered follow-up**

Replace `scanDeadOwnLandIDsAfterHarvest` with a `scanOwnFarmStatusAfterHarvest` helper returning `map[string]any`. Keep its `300ms` settle, `forceRefresh: true`, and conditional `350ms` retry exactly as today: return the first status when `collectDeadLandIDs(firstStatus)` is non-empty; otherwise wait 350ms, fetch once more, and return the second status map.

Add this runtime method near `shovelDeadOwnLands`:

```go
func (r RuntimeFacade) fertilizeMultiSeasonAfterHarvest(ctx context.Context, taskID string, status map[string]any, harvestedLandIDs []int, source string) (int, *ActionResult) {
    mode := MultiSeasonFertilizerMode(r.config)
    landIDs := MultiSeasonContinuationLandIDs(status, harvestedLandIDs)
    if mode == "" || len(landIDs) == 0 {
        return 0, nil
    }
    value, err := r.caller.Call(ctx, "gameCtl.fertilizeLandsBatch", []any{map[string]any{
        "landIds":                     landIDs,
        "type":                        mode,
        "mode":                        mode,
        "dryRun":                      false,
        "cleanupUi":                   true,
        "linkedHarvestAfterFertilize": false,
        "silent":                      true,
        "source":                      source,
    }}, 45*time.Second)
    if err != nil {
        return 0, &ActionResult{OK: false, Status: StatusFailed, TaskID: taskID, Message: "多季补肥调用失败：" + err.Error()}
    }
    if failed, reason := runtimeResultFailed(value); failed {
        return 0, &ActionResult{OK: false, Status: StatusFailed, TaskID: taskID, Message: "多季补肥返回失败：" + reason}
    }
    return runtimeSuccessfulOperationCount(value), nil
}
```

In `runOwnCollect`, derive `successfulHarvestLandIDs := SuccessfulRuntimeLandIDs(harvestResult)` before the existing scan. Merge pre-harvest dead IDs with `collectDeadLandIDs(postHarvestStatus)`, call `shovelDeadOwnLands` first, and only then call `fertilizeMultiSeasonAfterHarvest` with source `farm_go_auto_collect_multi_season`. Preserve the existing scan count, harvest count, and normal no-op behavior.

- [ ] **Step 4: Run collection tests and the existing collection suite**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnCollect' -count=1`

Expected: PASS, including the existing delayed-dead-crop retry test and the new shovel-before-fertilizer tests.

- [ ] **Step 5: Commit automatic collection support**

```powershell
git add internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go
git commit -m "feat: fertilize multi-season crops after collection"
```

### Task 3: Reuse Automatic Linked-Harvest Scan

**Files:**
- Modify: `internal/farm/automation/runtime_own.go:974-1115`
- Modify: `internal/farm/automation/runtime_own_test.go`

- [ ] **Step 1: Write a failing linked-harvest test**

Add a test where `own_fertilizer` has `autoFarmFertilizerHarvestLinkEnabled: true`, `autoFarmFertilizerMultiSeason: true`, and `autoFarmPlantFertilizerMode: "organic"`. Its initial rush target is land `4`; the outer fertilizer response includes:

```go
"linkedHarvest": map[string]any{"ok": true, "results": []any{
    map[string]any{"ok": true, "landId": float64(4)},
    map[string]any{"ok": false, "landId": float64(8)},
}},
```

Its existing linked-harvest status response contains a dead land `2` and a growing later-season multi-season crop anchored at `4`. Assert the runtime order is outer fertilizer, existing status read, shovel, then a second fertilizer call. Assert the second call has organic mode, land `4`, `linkedHarvestAfterFertilize: false`, and source `farm_go_auto_fertilizer_multi_season`.

- [ ] **Step 2: Run the linked-harvest test and verify it fails**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnFertilizerLinkedHarvest.*MultiSeason' -count=1`

Expected: FAIL because linked `own_fertilizer` currently reads dead IDs, shovels, and returns without reusing that status for multi-season fertilizer.

- [ ] **Step 3: Add linked-harvest follow-up after shovel success**

In `runOwnFertilizer`, retain the one existing `gameCtl.getFarmStatus` call after linked harvest. Derive:

```go
linkedHarvestResult := mapFromAny(mapFromAny(fertilizerResult)["linkedHarvest"])
successfulHarvestLandIDs := SuccessfulRuntimeLandIDs(linkedHarvestResult)
deadLandIDs := collectDeadLandIDs(mapFromAny(deadStatusValue))
```

Keep the current shovel payload and failure handling first. Immediately after a successful shovel or when `deadLandIDs` is empty, call:

```go
_, multiSeasonFailure := r.fertilizeMultiSeasonAfterHarvest(
    ctx,
    taskID,
    mapFromAny(deadStatusValue),
    successfulHarvestLandIDs,
    "farm_go_auto_fertilizer_multi_season",
)
if multiSeasonFailure != nil {
    return *multiSeasonFailure
}
```

Do not change the outer rush fertilizer mode, threshold, or its linked-harvest flag.

- [ ] **Step 4: Run linked-harvest and fertilizer regression tests**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnFertilizer' -count=1`

Expected: PASS. The original linked cleanup test remains green, and the new test proves the status scan is still called once and is reused.

- [ ] **Step 5: Commit automatic linked-harvest support**

```powershell
git add internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go
git commit -m "feat: fertilize multi-season crops after linked harvest"
```

### Task 4: Reuse Manual Linked-Harvest Scan

**Files:**
- Modify: `app.go:2395-2420,2467-2545`
- Modify: `app_test.go:160-260`

- [ ] **Step 1: Write failing manual flow tests**

Add a `TestFarmLandRushFertilizesMultiSeasonAfterDeadCleanup` case. Configure `app.FarmAutomationState()` with:

```go
state.Config["autoFarmFertilizerEnabled"] = true
state.Config["autoFarmFertilizerMultiSeason"] = true
state.Config["autoFarmPlantFertilizerMode"] = "normal"
```

Return a linked-harvest result with successful land `4`, return one status response containing dead land `2` and continuing multi-season land `4`, and assert:

```go
[]string{
    "gameCtl.fertilizeLandsBatch",
    "gameCtl.getFarmStatus",
    "gameCtl.shovelLandsBatch",
    "gameCtl.fertilizeLandsBatch",
}
```

Assert the final fertilizer uses normal mode, `landIds: []int{4}`, `linkedHarvestAfterFertilize: false`, and source `farm_go_land_rush_multi_season`. Add a shovel-failure variant asserting the final fertilizer call does not occur and the returned map has `ok: false`.

- [ ] **Step 2: Run the manual flow tests and verify they fail**

Run: `go test . -run 'TestFarmLandRush.*MultiSeason' -count=1`

Expected: FAIL because `attachDeadCleanupAfterHarvest` only adds `deadCleanup` and returns before any multi-season action.

- [ ] **Step 3: Extend the existing manual cleanup helper without adding a scan**

Change the call site to pass the current account configuration into the existing helper:

```go
return a.attachDeadCleanupAfterHarvest(
    result,
    "farm_go_land_rush_dead_cleanup",
    a.farmAutomationStateForAccount(a.accountKey()).Config,
)
```

After the helper's existing `gameCtl.getFarmStatus` call and successful shovel branch, derive mode and targets from the already-read result:

```go
linkedHarvest := mapFromAny(result["linkedHarvest"])
mode := automation.MultiSeasonFertilizerMode(config)
landIDs := automation.MultiSeasonContinuationLandIDs(
    mapFromAny(statusValue),
    automation.SuccessfulRuntimeLandIDs(linkedHarvest),
)
```

When `mode != ""` and targets are non-empty, call `gameCtl.fertilizeLandsBatch` with the same payload contract as Task 2, source `farm_go_land_rush_multi_season`, and `linkedHarvestAfterFertilize: false`. Store a bounded `multiSeasonFertilizer` summary in `result`. On its error or explicit `ok: false`, set `result["ok"] = false` and return only after the already-required dead cleanup has completed.

- [ ] **Step 4: Run manual rush tests**

Run: `go test . -run 'TestFarmLandRush' -count=1`

Expected: PASS, including existing linked-harvest cleanup tests and the new ordered multi-season cases.

- [ ] **Step 5: Commit manual linked-harvest support**

```powershell
git add app.go app_test.go
git commit -m "feat: support multi-season manual linked harvest"
```

### Task 5: Verify the Complete Feature

**Files:**
- Modify: none

- [ ] **Step 1: Run all focused multi-season tests**

Run:

```powershell
go test ./internal/farm/automation -run 'Test(SuccessfulRuntimeLandIDsRequiresExplicitSuccess|MultiSeasonContinuationLandIDsUsesConfirmedAnchors|MultiSeasonFertilizerModeUsesPlantingStrategy|RuntimeFacadeOwnCollect.*MultiSeason|RuntimeFacadeOwnFertilizerLinkedHarvest.*MultiSeason)' -count=1
go test . -run 'TestFarmLandRush.*MultiSeason' -count=1
```

Expected: PASS with no skipped focused tests.

- [ ] **Step 2: Run complete regression suites and static checks**

Run:

```powershell
node scripts/test-fertilizer-linked-harvest.js
go test ./... -count=1
npm run test -- AutomationView.test.tsx
git diff --check
git status --short
```

Run the npm command from `frontend/`. Expected: all Go packages pass, the runtime script passes, the focused frontend suite passes, `git diff --check` is silent, and only the expected implementation changes remain before the final commit.

- [ ] **Step 3: Commit the verified feature**

```powershell
git add internal/farm/automation/post_harvest.go internal/farm/automation/post_harvest_test.go internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go app.go app_test.go
git commit -m "feat: automate multi-season fertilizer follow-up"
```
