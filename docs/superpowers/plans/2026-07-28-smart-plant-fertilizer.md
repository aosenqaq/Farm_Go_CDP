# Smart Plant Fertilizer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add smart normal and organic planting fertilizer strategies that defer eligible crops until their longest “大叶子” phase, while immediately fertilizing crops without that phase.

**Architecture:** `internal/farm` loads `Plant.json` into a small immutable index that classifies a crop as smart-target, fallback, or unknown. `internal/farm/automation` maps smart strategy values to the real fertilizer type, uses the index for planting and multi-season fallbacks, and merges smart patrol targets with existing rush targets without duplicate land IDs. The React settings surface only persists the two new strategy values.

**Tech Stack:** Go 1.x, existing `internal/farm` game-config reader, Go `testing`, React/TypeScript, Vitest.

---

## File Structure

- Modify: `internal/farm/gameconfig.go:377,2500` — introduce a public smart-fertilizer crop index backed by `Plant.json`.
- Modify: `internal/farm/gameconfig_test.go` — test phase classification and plant/seed lookup.
- Modify: `internal/farm/automation/runtime_own.go:326-359,442-471,491-500,1024-1176` — resolve smart strategy modes, defer eligible crops, and dispatch smart patrol targets.
- Modify: `internal/farm/automation/runtime_own_test.go` — cover planting, patrol, multi-season fallback, and de-duplication behavior.
- Modify: `frontend/src/views/AutomationView.tsx:1556-1560` — render the two new strategy choices.
- Modify: `frontend/src/views/AutomationView.test.tsx:1093-1129` — assert the new choices are rendered.

### Task 1: Add Crop Phase Classification

**Files:**
- Modify: `internal/farm/gameconfig.go:377-388,2500-2511`
- Test: `internal/farm/gameconfig_test.go`

- [ ] **Step 1: Write the failing tests**

Add a table-driven test that calls the new package-private phase helper and the exported index loader. It must assert that only a longest positive-duration “大叶子” is smart, ties are smart, absent/shorter “大叶子” is fallback, malformed/zero-duration data is fallback, and unmapped IDs are unknown.

~~~go
func TestSmartFertilizerCropClassFromGrowPhases(t *testing.T) {
    cases := []struct {
        phases string
        want   SmartFertilizerCropClass
    }{
        {"种子:4800;发芽:4800;小叶子:4800;大叶子:7200;花蕾:7200;盛开:0;", SmartFertilizerCropSmart},
        {"种子:4800;大叶子:7200;花蕾:9600;成熟:0;", SmartFertilizerCropFallback},
        {"种子:30;发芽:30;成熟:0;", SmartFertilizerCropFallback},
    }
    for _, tc := range cases {
        if got := smartFertilizerCropClassFromGrowPhases(tc.phases); got != tc.want {
            t.Fatalf("class(%q) = %v, want %v", tc.phases, got, tc.want)
        }
    }
}

func TestLoadSmartFertilizerCropIndexUsesPlantAndSeedIDs(t *testing.T) {
    index, err := LoadSmartFertilizerCropIndex(filepath.Join("..", "..", "resources", "gameConfig"))
    if err != nil { t.Fatal(err) }
    if got := index.Classify(1020003, 0); got != SmartFertilizerCropSmart { t.Fatalf("carrot = %v", got) }
    if got := index.Classify(0, 20002); got != SmartFertilizerCropFallback { t.Fatalf("radish = %v", got) }
    if got := index.Classify(999999, 0); got != SmartFertilizerCropUnknown { t.Fatalf("unknown = %v", got) }
}
~~~

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/farm -run 'Test(SmartFertilizerCropClassFromGrowPhases|LoadSmartFertilizerCropIndexUsesPlantAndSeedIDs)$' -count=1`

Expected: compilation failure because `SmartFertilizerCropClass`, `LoadSmartFertilizerCropIndex`, and the phase helper do not exist.

- [ ] **Step 3: Implement the minimal crop index**

Add these types and functions beside `plantConfigItem`, reusing `readGameConfigJSON` so root resolution and JSON errors follow existing conventions:

~~~go
type SmartFertilizerCropClass int

const (
    SmartFertilizerCropUnknown SmartFertilizerCropClass = iota
    SmartFertilizerCropFallback
    SmartFertilizerCropSmart
)

type SmartFertilizerCropIndex struct {
    byPlantID map[int]SmartFertilizerCropClass
    bySeedID  map[int]SmartFertilizerCropClass
}

func (index SmartFertilizerCropIndex) Classify(plantID, seedID int) SmartFertilizerCropClass {
    if class, ok := index.byPlantID[plantID]; ok { return class }
    if class, ok := index.bySeedID[seedID]; ok { return class }
    return SmartFertilizerCropUnknown
}

func smartFertilizerCropClassFromGrowPhases(growPhases string) SmartFertilizerCropClass {
    maxDuration, largeLeafDuration := 0, -1
    for _, raw := range strings.Split(growPhases, ";") {
        parts := strings.SplitN(strings.TrimSpace(raw), ":", 2)
        if len(parts) != 2 { continue }
        duration, err := strconv.Atoi(strings.TrimSpace(parts[1]))
        if err != nil || duration <= 0 { continue }
        if duration > maxDuration { maxDuration = duration }
        if strings.TrimSpace(parts[0]) == "大叶子" { largeLeafDuration = duration }
    }
    if maxDuration > 0 && largeLeafDuration == maxDuration { return SmartFertilizerCropSmart }
    return SmartFertilizerCropFallback
}
~~~

`LoadSmartFertilizerCropIndex` reads only `Plant.json`, classifies every valid entry once, and stores the same class under both non-zero IDs.

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `go test ./internal/farm -run 'Test(SmartFertilizerCropClassFromGrowPhases|LoadSmartFertilizerCropIndexUsesPlantAndSeedIDs)$' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the crop classification**

~~~powershell
git add internal/farm/gameconfig.go internal/farm/gameconfig_test.go
git commit -m "feat: classify crops for smart fertilizer"
~~~

### Task 2: Expose Smart Strategy Choices

**Files:**
- Modify: `frontend/src/views/AutomationView.tsx:1556-1560`
- Test: `frontend/src/views/AutomationView.test.tsx:1093-1129`

- [ ] **Step 1: Write the failing UI assertion**

Replace the two current negative assertions with expectations for the final labels:

~~~tsx
expect(html).toContain('智能无机肥');
expect(html).toContain('智能有机肥');
~~~

- [ ] **Step 2: Run the test to verify it fails**

Run: `npm --prefix frontend test -- AutomationView.test.tsx`

Expected: FAIL because neither label is rendered.

- [ ] **Step 3: Add the two persisted option values**

Extend the existing `plantFertilizerOptions` array without changing the old values:

~~~tsx
const plantFertilizerOptions = [
  ['none', '不施肥'],
  ['normal', '普通化肥'],
  ['organic', '有机化肥'],
  ['smart_normal', '智能无机肥'],
  ['smart_organic', '智能有机肥'],
];
~~~

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `npm --prefix frontend test -- AutomationView.test.tsx`

Expected: PASS with the fertilizer settings render test covering both labels.

- [ ] **Step 5: Commit the strategy controls**

~~~powershell
git add frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx
git commit -m "feat: expose smart fertilizer strategies"
~~~

### Task 3: Apply Smart Strategy During Planting and Multi-Season Fallback

**Files:**
- Modify: `internal/farm/automation/runtime_own.go:326-359,442-471,491-500`
- Test: `internal/farm/automation/runtime_own_test.go:582-714,1567-1634`

- [ ] **Step 1: Write failing automation tests**

Add focused tests with the real game-config root. The planting test uses seed `20003` (carrot, smart target) and verifies no immediate batch call, then seed `20002` (white radish, fallback) and verifies one `normal` batch call. Add multi-season tests that make the post-harvest status contain either `plantId: 1020003` (deferred) or `plantId: 1020002` (immediate fallback), both under `smart_organic`.

~~~go
func TestRuntimeFacadeOwnPlantSmartFertilizerDefersLargeLeafCrop(t *testing.T) {
    caller := &fakeRuntimeCaller{responses: map[string]any{
        "gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
        "gameCtl.getFarmStatus": map[string]any{"farmType": "own", "grids": []any{
            map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true},
        }},
        "gameCtl.autoPlant": map[string]any{"ok": true},
    }}
    facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
        "autoFarmFertilizerEnabled": true, "autoFarmPlantPrimaryMode": "specified_seed",
        "autoFarmPlantSeedId": 20003, "autoFarmPlantFertilizerMode": "smart_normal",
    })
    if result := facade.RunTask(context.Background(), "own_plant"); !result.OK { t.Fatal(result) }
    want := []string{"gameCtl.getFarmOwnership", "gameCtl.getFarmStatus", "gameCtl.autoPlant"}
    if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, want) { t.Fatalf("methods = %#v, want %#v", got, want) }
}

func TestRuntimeFacadeOwnPlantSmartFertilizerFallsBackWithoutLargeLeaf(t *testing.T) {
    caller := &fakeRuntimeCaller{responses: map[string]any{
        "gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
        "gameCtl.getFarmStatus": map[string]any{"farmType": "own", "grids": []any{
            map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true},
        }},
        "gameCtl.autoPlant": map[string]any{"ok": true}, "gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
    }}
    facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
        "autoFarmFertilizerEnabled": true, "autoFarmPlantPrimaryMode": "specified_seed",
        "autoFarmPlantSeedId": 20002, "autoFarmPlantFertilizerMode": "smart_normal",
    })
    if result := facade.RunTask(context.Background(), "own_plant"); !result.OK { t.Fatal(result) }
    payload := mapFromAny(caller.calls[len(caller.calls)-1].args[0])
    if payload["type"] != "normal" || !reflect.DeepEqual(payload["landIds"], []int{1}) { t.Fatalf("payload = %#v", payload) }
}

func TestRuntimeFacadeOwnFertilizerMultiSeasonSmartStrategyDefersLargeLeafCrop(t *testing.T) {
    caller := &fakeRuntimeCaller{}
    facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
        "autoFarmFertilizerEnabled": true, "autoFarmFertilizerMultiSeason": true,
        "autoFarmPlantFertilizerMode": "smart_organic",
    })
    status := map[string]any{"grids": []any{map[string]any{
        "landId": float64(4), "plantId": float64(1020003), "hasPlant": true,
        "stageKind": "growing", "isMultiSeason": true, "currentSeason": float64(2), "totalSeason": float64(3),
    }}}
    count, failed := facade.fertilizeMultiSeasonAfterHarvest(context.Background(), "own_fertilizer", status, []int{4}, "test")
    if failed != nil || count != 0 || len(caller.calls) != 0 { t.Fatalf("count=%d failed=%#v calls=%#v", count, failed, caller.calls) }
}

func TestRuntimeFacadeOwnFertilizerMultiSeasonSmartStrategyFallsBackWithoutLargeLeaf(t *testing.T) {
    caller := &fakeRuntimeCaller{responses: map[string]any{"gameCtl.fertilizeLandsBatch": map[string]any{"ok": true, "successCount": float64(1)}}}
    facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
        "autoFarmFertilizerEnabled": true, "autoFarmFertilizerMultiSeason": true,
        "autoFarmPlantFertilizerMode": "smart_organic",
    })
    status := map[string]any{"grids": []any{map[string]any{
        "landId": float64(4), "plantId": float64(1020002), "hasPlant": true,
        "stageKind": "growing", "isMultiSeason": true, "currentSeason": float64(2), "totalSeason": float64(3),
    }}}
    count, failed := facade.fertilizeMultiSeasonAfterHarvest(context.Background(), "own_fertilizer", status, []int{4}, "test")
    if failed != nil || count != 1 { t.Fatalf("count=%d failed=%#v", count, failed) }
    if got := mapFromAny(caller.calls[0].args[0])["type"]; got != "organic" { t.Fatalf("type = %#v", got) }
}
~~~

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwn(PlantSmartFertilizer|FertilizerMultiSeasonSmartStrategy)' -count=1`

Expected: the large-leaf planting case currently receives an immediate batch call, and the fallback/multi-season cases cannot distinguish smart values.

- [ ] **Step 3: Implement normalized smart strategy resolution and fallback dispatch**

Replace the narrow `plantFertilizerMode` check with a helper that preserves the actual batch mode and separately reports whether it is smart:

~~~go
func plantFertilizerStrategy(config map[string]any) (mode string, smart bool) {
    if !fertilizerMasterEnabled(config) { return "", false }
    switch strings.ToLower(strings.TrimSpace(stringConfigAny(config["autoFarmPlantFertilizerMode"]))) {
    case "normal":
        return "normal", false
    case "organic":
        return "organic", false
    case "smart_normal":
        return "normal", true
    case "smart_organic":
        return "organic", true
    default:
        return "", false
    }
}
~~~

For a smart planting plan, load the crop index with `farm.DefaultGameConfigRoot()` and use `plantOpts["seedId"]`; submit the existing batch payload only when its class is `SmartFertilizerCropFallback`. In `fertilizeMultiSeasonAfterHarvest`, use each continuation grid’s `plantId` with the same index; immediately dispatch only fallback IDs, while unknown and smart IDs are deferred. Keep `normal` and `organic` flows unchanged.

- [ ] **Step 4: Run the focused automation tests to verify they pass**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwn(Plant(AppliesPlantFertilizerAfterSuccessfulPlant|SmartFertilizer)|Fertilizer(MultiSeasonSmartStrategy|LinkedHarvestFertilizesMultiSeasonAfterCleanup))' -count=1`

Expected: PASS, including unchanged direct normal/organic behavior.

- [ ] **Step 5: Commit the planting and multi-season behavior**

~~~powershell
git add internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go
git commit -m "feat: defer smart fertilizer until large leaf phase"
~~~

### Task 4: Run Smart Patrol Alongside Existing Rush Logic

**Files:**
- Modify: `internal/farm/automation/runtime_own.go:1024-1176`
- Test: `internal/farm/automation/runtime_own_test.go:1335-1465,1728-1827`

- [ ] **Step 1: Write failing patrol tests**

Add `farm "Farm_Go/internal/farm"` to the test import block. Then add tests that pass farm-status grids with `plantId`, `phaseName`, and multi-tile anchor fields:

~~~go
func TestRuntimeFacadeOwnFertilizerSmartPatrolRunsWhenRushDisabled(t *testing.T) {
    caller := &fakeRuntimeCaller{responses: map[string]any{
        "gameCtl.getFarmStatus": map[string]any{"farmType": "own", "grids": []any{map[string]any{
            "landId": float64(5), "plantId": float64(1020003), "hasPlant": true, "stageKind": "growing", "phaseName": "大叶子",
        }}},
        "gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
    }}
    facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
        "autoFarmFertilizerEnabled": true, "autoFarmPlantFertilizerMode": "smart_normal", "autoFarmRushFertilizerMode": "none",
    })
    if result := facade.RunTask(context.Background(), "own_fertilizer"); !result.OK { t.Fatal(result) }
    payload := mapFromAny(caller.calls[1].args[0])
    if payload["type"] != "normal" || !reflect.DeepEqual(payload["landIds"], []int{5}) { t.Fatalf("payload = %#v", payload) }
}

func TestRuntimeFacadeOwnFertilizerSmartPatrolSkipsOtherPhasesAndUnknownCrops(t *testing.T) {
    index, err := farm.LoadSmartFertilizerCropIndex(farm.DefaultGameConfigRoot())
    if err != nil { t.Fatal(err) }
    status := map[string]any{"grids": []any{
        map[string]any{"landId": float64(1), "plantId": float64(1020003), "hasPlant": true, "stageKind": "growing", "phaseName": "小叶子"},
        map[string]any{"landId": float64(2), "plantId": float64(999999), "hasPlant": true, "stageKind": "growing", "phaseName": "大叶子"},
        map[string]any{"landId": float64(3), "plantId": float64(1020003), "hasPlant": true, "stageKind": "growing", "phaseName": "大叶子"},
    }}
    if got := collectSmartFertilizerLandIDs(status, index); !reflect.DeepEqual(got, []int{3}) { t.Fatalf("ids = %#v", got) }
}

func TestRuntimeFacadeOwnFertilizerSmartPatrolExcludesSmartLandFromRushBatch(t *testing.T) {
    caller := &fakeRuntimeCaller{responses: map[string]any{
        "gameCtl.getFarmStatus": map[string]any{"farmType": "own", "grids": []any{map[string]any{
            "landId": float64(5), "plantId": float64(1020003), "hasPlant": true, "stageKind": "growing", "phaseName": "大叶子", "matureInSec": float64(30),
        }}},
        "gameCtl.fertilizeLandsBatch": map[string]any{"ok": true},
    }}
    facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
        "autoFarmFertilizerEnabled": true, "autoFarmPlantFertilizerMode": "smart_normal",
        "autoFarmRushFertilizerMode": "organic", "autoFarmFertilizerRushThresholdSec": 60,
    })
    if result := facade.RunTask(context.Background(), "own_fertilizer"); !result.OK { t.Fatal(result) }
    if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, []string{"gameCtl.getFarmStatus", "gameCtl.fertilizeLandsBatch"}) { t.Fatalf("methods = %#v", got) }
    if got := mapFromAny(caller.calls[1].args[0])["type"]; got != "normal" { t.Fatalf("type = %#v", got) }
}
~~~

- [ ] **Step 2: Run the new patrol tests to verify they fail**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnFertilizerSmartPatrol' -count=1`

Expected: the first test skips before reading farm status because rush is disabled.

- [ ] **Step 3: Implement smart target collection and ordered batch dispatch**

Add a helper that returns unique anchor land IDs where all conditions hold: `hasPlant`, `stageKind == "growing"`, `phaseName == "大叶子"`, and `SmartFertilizerCropIndex.Classify(plantId, seedId) == SmartFertilizerCropSmart`.

In `runOwnFertilizer`:

~~~go
smartMode, smartEnabled := plantFertilizerStrategy(r.config)
rushMode := normalizedRushFertilizerMode(r.config)
if !smartEnabled && rushMode == "" {
    return ActionResult{OK: true, Status: StatusOK, TaskID: taskID, Message: "催熟策略已关闭，本轮自动施肥跳过。"}
}
status := mapFromAny(value)
smartLandIDs := collectSmartFertilizerLandIDs(status, cropIndex)
rushLandIDs := removeLandIDs(collectFertilizerRushLandIDs(status, rushThresholdSec), smartLandIDs)
~~~

Dispatch the smart batch before the existing rush batch. When both use the same fertilizer type, append unique IDs and make one batch call. When they differ, send the smart batch first and the remaining rush batch second. Preserve the existing timeout, payload keys, linked-harvest cleanup, and failure handling for every dispatched batch.

- [ ] **Step 4: Run focused patrol and regression tests**

Run: `go test ./internal/farm/automation -run 'Test(RuntimeFacadeOwnFertilizer|CollectFertilizerRushLandIDs)' -count=1`

Expected: PASS, including the disabled-rush legacy skip when neither smart strategy is selected.

- [ ] **Step 5: Commit the smart patrol**

~~~powershell
git add internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go
git commit -m "feat: patrol large leaf crops with smart fertilizer"
~~~

### Task 5: Full Verification

**Files:**
- Verify: `internal/farm/gameconfig.go`
- Verify: `internal/farm/automation/runtime_own.go`
- Verify: `frontend/src/views/AutomationView.tsx`

- [ ] **Step 1: Format the Go files**

Run: `gofmt -w internal/farm/gameconfig.go internal/farm/gameconfig_test.go internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go`

- [ ] **Step 2: Run backend tests**

Run: `go test ./internal/farm/... -count=1`

Expected: PASS with zero failures.

- [ ] **Step 3: Run frontend tests and production build**

Run: `npm --prefix frontend test`

Expected: PASS with zero failed tests.

Run: `npm --prefix frontend run build`

Expected: TypeScript compilation and Vite build both exit with code 0.

- [ ] **Step 4: Run full Go regression tests**

Run: `go test ./... -count=1`

Expected: PASS with zero failures.

- [ ] **Step 5: Review the final diff and commit verification state**

Run: `git diff --check HEAD~3..HEAD`

Expected: no whitespace errors.

Run:

~~~powershell
git status --short
git log --oneline -3
~~~

Expected: only the documented smart-fertilizer commits are present and the worktree is clean.
