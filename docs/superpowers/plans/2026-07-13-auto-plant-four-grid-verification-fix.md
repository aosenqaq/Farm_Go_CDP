# Auto Plant Four-Grid Verification Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allocate four-grid seeds to real 2x2 land groups and fall back to ordinary seeds when no complete group exists.

**Architecture:** Add an own-plant Go helper that orders empty land IDs by complete, non-overlapping 2x2 groups derived from `gridPos`. Preserve `no_complete_multi_land_group` in the injected JavaScript before post-dispatch verification so the existing Go fallback branch can handle it.

**Tech Stack:** Go 1.24, Go tests, injected JavaScript in `resources/wmpf/button.js`, Wails with WeChat CDP.

---

## File Structure

- Modify `internal/farm/automation/runtime_helpers.go`: pure grid-position grouping and ordering.
- Modify `internal/farm/automation/runtime_own.go`: opt into spatial ordering for four-grid planting.
- Modify `internal/farm/automation/runtime_own_test.go`: spatial allocation and fallback coverage.
- Modify `resources/wmpf/button.js`: preserve the structural four-grid failure reason.
- Create `auto_plant_button_test.go`: injected-script control-flow contract tests.

### Task 1: Spatial Four-Grid Allocation

**Files:**
- Modify: `internal/farm/automation/runtime_helpers.go`
- Modify: `internal/farm/automation/runtime_own.go`
- Test: `internal/farm/automation/runtime_own_test.go`

- [ ] **Step 1: Write failing spatial-order tests**

Add the complete fixture and tests below to `runtime_own_test.go`:

```go
func twoPlantingRowsStatus() map[string]any {
	grids := []any{}
	for x := 0; x < 4; x++ {
		grids = append(grids, map[string]any{
			"landId": x + 1, "stageKind": "empty", "interactable": true,
			"gridPos": map[string]any{"x": x, "y": 5},
		})
		grids = append(grids, map[string]any{
			"landId": x + 5, "stageKind": "empty", "interactable": true,
			"gridPos": map[string]any{"x": x, "y": 4},
		})
	}
	return map[string]any{"farmType": "own", "grids": grids}
}

func TestRuntimeFacadeOwnPlantAllocatesSpatialFourGridFromFullRows(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus":    twoPlantingRowsStatus(),
		"gameCtl.getSeedList": []any{
			map[string]any{"itemId": 21032, "count": 2, "name": "琉璃宝荷种子"},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmPlantPrimaryMode":     "backpack_first",
		"autoFarmFourGridPlantEnabled": true,
	})

	result := facade.RunTask(context.Background(), "own_plant")
	if !result.OK {
		t.Fatalf("own_plant should succeed: %#v", result)
	}
	payloads := collectAutoPlantPayloads(caller.calls)
	want := []int{1, 2, 5, 6, 3, 4, 7, 8}
	if len(payloads) != 1 || !reflect.DeepEqual(payloads[0]["emptyLandIds"], want) {
		t.Fatalf("four-grid payloads = %#v, want grouped lands %#v", payloads, want)
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

```powershell
go test ./internal/farm/automation -run TestRuntimeFacadeOwnPlantAllocatesSpatialFourGridFromFullRows -count=1
```

Expected: FAIL because the payload currently contains the ID-only order `[1 2 3 4 5 6 7 8]`.

- [ ] **Step 3: Implement a pure planting order helper**

Add to `runtime_helpers.go`:

```go
type plantingGridPosition struct {
	landID int
	x      int
	y      int
}

func collectPlantingEmptyLandIDs(status map[string]any, prioritizeFourGrid bool) []int {
	ids := collectEmptyLandIDs(status)
	if !prioritizeFourGrid || len(ids) < 4 {
		return ids
	}
	byLandID := map[int]plantingGridPosition{}
	byPosition := map[[2]int]int{}
	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		if grid["stageKind"] != "empty" || !boolFromAny(grid["interactable"]) {
			continue
		}
		landID := intFromAny(firstExistingAny(grid["landId"], grid["id"]))
		position := mapFromAny(grid["gridPos"])
		x, xOK := exactGridCoordinate(position["x"])
		y, yOK := exactGridCoordinate(position["y"])
		if landID <= 0 || !xOK || !yOK {
			continue
		}
		entry := plantingGridPosition{landID: landID, x: x, y: y}
		byLandID[landID] = entry
		byPosition[[2]int{x, y}] = landID
	}
	used := map[int]bool{}
	ordered := make([]int, 0, len(ids))
	for _, landID := range ids {
		entry, ok := byLandID[landID]
		if !ok || used[landID] {
			continue
		}
		for _, offset := range [][2]int{{0, -1}, {-1, -1}, {0, 0}, {-1, 0}} {
			minX, minY := entry.x+offset[0], entry.y+offset[1]
			group := []int{
				byPosition[[2]int{minX, minY}],
				byPosition[[2]int{minX + 1, minY}],
				byPosition[[2]int{minX, minY + 1}],
				byPosition[[2]int{minX + 1, minY + 1}],
			}
			if !completeUnusedLandGroup(group, used) {
				continue
			}
			sort.Ints(group)
			for _, groupedID := range group {
				used[groupedID] = true
				ordered = append(ordered, groupedID)
			}
			break
		}
	}
	for _, landID := range ids {
		if !used[landID] {
			ordered = append(ordered, landID)
		}
	}
	return ordered
}

func completeUnusedLandGroup(group []int, used map[int]bool) bool {
	seen := map[int]bool{}
	for _, landID := range group {
		if landID <= 0 || used[landID] || seen[landID] {
			return false
		}
		seen[landID] = true
	}
	return len(seen) == 4
}

func exactGridCoordinate(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case float64:
		integer := int(number)
		return integer, number == float64(integer)
	default:
		return 0, false
	}
}
```

Reuse an existing exact numeric helper if one already exists.

- [ ] **Step 4: Use the helper from `loadOwnPlantEmptyLandIDs`**

Replace the final return with:

```go
return collectPlantingEmptyLandIDs(
	status,
	boolConfigAny(r.config["autoFarmFourGridPlantEnabled"]),
), true, nil
```

- [ ] **Step 5: Run focused tests and verify GREEN**

```powershell
go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnPlantAllocatesSpatialFourGridFromFullRows|TestRuntimeFacadeOwnPlantAllocatesFourLandsToOneFourGridSeed' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/farm/automation/runtime_helpers.go internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go
git commit -m "fix: allocate spatial four-grid planting groups"
```

### Task 2: Preserve Four-Grid Failure Reasons

**Files:**
- Modify: `resources/wmpf/button.js`
- Create: `auto_plant_button_test.go`

- [ ] **Step 1: Write failing script contract tests**

Create `auto_plant_button_test.go` using the existing `readWMPFButtonScript` helper:

```go
package main

import (
	"strings"
	"testing"
)

func TestAutoPlantPreservesIncompleteFourGridReasonBeforeVerification(t *testing.T) {
	script := readWMPFButtonScript(t)
	start := strings.Index(script, "async function plantSeedsOnLandsVerified")
	end := strings.Index(script[start:], "async function autoPlant")
	if start < 0 || end < 0 {
		t.Fatalf("expected verified planting block")
	}
	block := script[start : start+end]
	guard := strings.Index(block, "plantResult.reason === 'no_complete_multi_land_group'")
	waitAfterDispatch := strings.Index(block, "await wait(waitAfterPlantMs)")
	if guard < 0 || waitAfterDispatch < 0 || guard > waitAfterDispatch {
		t.Fatalf("incomplete four-grid reason must return before verification")
	}
	if !strings.Contains(block[guard:waitAfterDispatch], "reason: plantResult.reason") {
		t.Fatalf("expected structural planting reason to be preserved")
	}
}

func TestAutoPlantStillReportsPostDispatchVerificationFailure(t *testing.T) {
	if !strings.Contains(readWMPFButtonScript(t), "reason: 'plant_verify_failed'") {
		t.Fatalf("expected post-dispatch verification failure to remain")
	}
}
```

- [ ] **Step 2: Run the tests and verify RED**

```powershell
go test . -run 'TestAutoPlantPreservesIncompleteFourGridReasonBeforeVerification|TestAutoPlantStillReportsPostDispatchVerificationFailure' -count=1
```

Expected: the first test FAILS because the guard is absent; the second passes.

- [ ] **Step 3: Return structural failure before waiting**

Immediately after the `plantSeedsOnLands` call/catch and before `await wait(waitAfterPlantMs)`, add:

```javascript
if (!dispatchError && plantResult && plantResult.planted === false &&
    plantResult.reason === 'no_complete_multi_land_group') {
  const afterEmptyIds = getEmptyLandIds();
  attempts.push({
    candidateSeedId: candidate,
    plantResult: plantResult,
    dispatchError: null,
    requestedLandIds: requestedLandIds,
    skippedLandIds: skippedLandIds,
    beforeEmptyIds: beforeEmptyIds,
    afterEmptyIds: afterEmptyIds,
    plantedCount: 0,
    fourGridPlantDecision: plantResult.fourGridPlantDecision || null,
    ok: false,
  });
  return {
    ok: false,
    reason: plantResult.reason,
    attempts: attempts,
    requestedLandIds: requestedLandIds,
    skippedLandIds: skippedLandIds,
    beforeEmptyIds: beforeEmptyIds,
    afterEmptyIds: afterEmptyIds,
    fourGridPlantDecision: plantResult.fourGridPlantDecision || null,
  };
}
```

- [ ] **Step 4: Run contract tests and verify GREEN**

```powershell
go test . -run 'TestAutoPlantPreservesIncompleteFourGridReasonBeforeVerification|TestAutoPlantStillReportsPostDispatchVerificationFailure' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add resources/wmpf/button.js auto_plant_button_test.go
git commit -m "fix: preserve four-grid planting failure reason"
```

### Task 3: Regression And Live Verification

**Files:**
- Test: `internal/farm/automation/runtime_own_test.go`
- Test: `auto_plant_button_test.go`

- [ ] **Step 1: Run own-plant regression tests**

```powershell
go test ./internal/farm/automation -run OwnPlant -count=1
```

Expected: PASS, including `TestRuntimeFacadeOwnPlantContinuesAfterNoCompleteFourGridGroup`.

- [ ] **Step 2: Format and run the complete suite**

```powershell
gofmt -w internal/farm/automation/runtime_helpers.go internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go auto_plant_button_test.go
go test ./... -count=1
git diff --check
```

Expected: all tests pass and `git diff --check` exits 0 without output.

- [ ] **Step 3: Perform live CDP verification**

Use the established `ws://127.0.0.1:62000/` CDP connection to read `gameCtl.getFarmStatus`. After a
complete 2x2 becomes empty, run `own_plant` once from Wails. Verify the 2x2 receives the four-grid
seed. For a later incomplete empty layout, verify ordinary seeds fill the lands. Query the latest
`runtime_events` rows and confirm the task does not report `plant_verify_failed` for
`no_complete_multi_land_group`.

- [ ] **Step 4: Commit only necessary final test adjustments**

```powershell
git add internal/farm/automation/runtime_own_test.go auto_plant_button_test.go
git commit -m "test: cover automatic planting fallback"
```

Skip this commit when live verification requires no test adjustment.
