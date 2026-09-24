# Four-Grid Auto Fertilizer Anchor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make planting fertilizer and rush fertilizer target one real anchor per four-grid crop instead of non-anchor coverage lands.

**Architecture:** Keep the fix inside the Go automation boundary. Rush candidates normalize farm-status rows through `occupancyAnchorLandId`; planting fertilizer extracts anchors from the successful runtime `fourGridPlantDecision.groups` response and otherwise preserves the existing land list.

**Tech Stack:** Go 1.25, table-driven unit tests, existing `RuntimeFacade` fake runtime caller.

---

### Task 1: Reproduce Rush Fertilizer Anchor Duplication

**Files:**
- Modify: `internal/farm/automation/runtime_own_test.go`
- Modify: `internal/farm/automation/runtime_helpers.go`

- [x] **Step 1: Write the failing helper test**

Add a test that supplies four growing grid rows with land IDs `1,2,5,6`, the same `occupancyAnchorLandId: 5`, valid plant IDs, and `matureInSec: 60`. Assert `collectFertilizerRushLandIDs(status, 60)` equals `[]int{5}`. Include a normal growing land without an anchor in the same table and assert it keeps its own ID.

- [x] **Step 2: Run the focused test and verify RED**

Run:

```powershell
go test ./internal/farm/automation -run TestCollectFertilizerRushLandIDsUsesFourGridAnchor -count=1
```

Expected: FAIL because the current helper returns the four coverage IDs.

- [x] **Step 3: Implement minimal anchor normalization**

Change the candidate ID selection to:

```go
id := intFromAny(firstExistingAny(grid["occupancyAnchorLandId"], grid["landId"], grid["id"]))
```

Keep the existing positive-ID filter, set-based deduplication, and numeric sorting.

- [x] **Step 4: Run the focused test and verify GREEN**

Run the same command and expect PASS.

### Task 2: Reproduce Planting Fertilizer Using Coverage Lands

**Files:**
- Modify: `internal/farm/automation/runtime_own_test.go`
- Modify: `internal/farm/automation/runtime_own.go`

- [x] **Step 1: Write the failing facade test**

Add `TestRuntimeFacadeOwnPlantFertilizesFourGridAnchors` with:

```go
"gameCtl.autoPlant": map[string]any{
    "ok": true,
    "fourGridPlantDecision": map[string]any{
        "groups": []any{[]any{float64(5), float64(6), float64(1), float64(2)}},
    },
},
```

Configure four-grid backpack planting, enable the fertilizer master switch, and choose `normal` plant fertilizer. Assert the fertilizer call receives `landIds: []int{5}` rather than `[]int{1,2,5,6}`.

- [x] **Step 2: Run the focused test and verify RED**

```powershell
go test ./internal/farm/automation -run TestRuntimeFacadeOwnPlantFertilizesFourGridAnchors -count=1
```

Expected: FAIL with the current coverage-land payload.

- [x] **Step 3: Implement the result-based anchor helper**

Add a small helper that:

```go
func plantFertilizerLandIDs(plantOpts map[string]any, plantResult any) []int
```

It returns the original `emptyLandIds` for normal plans. For `plantSize >= 2`, it reads `fourGridPlantDecision.groups`, takes the first positive ID from each group, removes duplicates, and returns those anchors. If no valid group exists, it returns the original list.

Use this helper only after a successful `gameCtl.autoPlant` response and before calling `gameCtl.fertilizeLandsBatch`.

- [x] **Step 4: Run focused planting fertilizer tests and verify GREEN**

```powershell
go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnPlant(AppliesPlantFertilizerAfterSuccessfulPlant|FertilizesFourGridAnchors)' -count=1
```

Expected: both ordinary and four-grid tests PASS.

### Task 3: Verify and Integrate

**Files:**
- Review: `internal/farm/automation/runtime_own.go`
- Review: `internal/farm/automation/runtime_helpers.go`
- Review: `internal/farm/automation/runtime_own_test.go`

- [x] **Step 1: Format and run the automation suite**

```powershell
gofmt -w internal/farm/automation/runtime_own.go internal/farm/automation/runtime_helpers.go internal/farm/automation/runtime_own_test.go
go test ./internal/farm/automation -count=1
```

Expected: PASS.

- [x] **Step 2: Run full verification**

```powershell
go test ./... -count=1
Set-Location frontend
npm test -- --run
npm run build
```

Expected: all commands exit 0.

- [x] **Step 3: Commit the implementation**

```powershell
git add internal/farm/automation/runtime_own.go internal/farm/automation/runtime_helpers.go internal/farm/automation/runtime_own_test.go
git commit -m "fix: target four-grid fertilizer anchors"
```

- [x] **Step 4: Merge and verify master**

Merge `fix/four-grid-fertilizer-anchor` into `master`, rerun `go test ./... -count=1`, and only then remove the owned worktree and delete the merged feature branch.
