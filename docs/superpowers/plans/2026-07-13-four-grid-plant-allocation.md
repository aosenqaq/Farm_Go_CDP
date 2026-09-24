# Four-Grid Plant Allocation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a backpack four-grid seed claim four candidate lands per seed, while preserving normal-seed fallback when no complete 2x2 group exists.

**Architecture:** The Go scheduler will calculate candidate-land capacity from `plantSize`, so one size-2 seed receives up to four land IDs instead of one. It reserves those lands from ordinary plans, then returns them to the next ordinary seed only if runtime coordinate validation reports no complete 2x2 group. The Go executor extracts nested runtime failure reasons and treats `no_complete_multi_land_group` as a skippable plan failure.

**Tech Stack:** Go 1.25, Go standard testing package, embedded JavaScript runtime patch.

---

### Task 1: Add Backend Allocation Regression Tests

**Files:**
- Modify: `internal/farm/automation/runtime_own_test.go`
- Test: `internal/farm/automation/runtime_own_test.go`

- [x] **Step 1: Write failing test for one four-grid seed**

```go
func TestRuntimeFacadeOwnPlantAllocatesFourLandsToOneFourGridSeed(t *testing.T) {
    // Four empty grid IDs and one size-2 seed must produce a payload containing all four IDs.
}
```

- [x] **Step 2: Run the focused test and verify failure**

Run: `go test ./internal/farm/automation -run TestRuntimeFacadeOwnPlantAllocatesFourLandsToOneFourGridSeed -count=1`

Expected: FAIL because the existing planner passes only one land ID for one seed.

- [x] **Step 3: Write failing fallback test**

```go
func TestRuntimeFacadeOwnPlantContinuesAfterNoCompleteFourGridGroup(t *testing.T) {
    // A no_complete_multi_land_group result must not prevent the later normal-seed plan.
}
```

- [x] **Step 4: Run the focused test and verify failure**

Run: `go test ./internal/farm/automation -run TestRuntimeFacadeOwnPlantContinuesAfterNoCompleteFourGridGroup -count=1`

Expected: FAIL because the executor returns immediately on every failed autoPlant result.

### Task 2: Allocate Multi-Tile Seed Capacity And Continue On Skippable Failures

**Files:**
- Modify: `internal/farm/automation/runtime_own.go:300-370`
- Modify: `internal/farm/automation/runtime_own.go:552-610`
- Test: `internal/farm/automation/runtime_own_test.go`

- [x] **Step 1: Calculate land capacity from `PlantSize`**

```go
landCount := option.BackpackCount
if option.PlantSize >= 2 {
    landCount *= option.PlantSize * option.PlantSize
}
landCount = min(landCount, len(remaining))
```

- [x] **Step 2: Preserve normal-seed fallback after an ungroupable four-grid plan**

Keep a later normal-seed plan eligible when a size-2 plan receives no complete 2x2 group. Treat only `no_complete_multi_land_group` as skippable; retain existing failures for transport, protocol, and other runtime errors.

- [x] **Step 3: Extract nested runtime reasons**

```go
reason := firstNonEmptyText(
    stringFromAny(result["reason"]),
    stringFromAny(mapFromAny(result["plantResult"])["reason"]),
    stringFromAny(mapFromAny(result["fourGridPlantDecision"])["reason"]),
)
```

- [x] **Step 4: Run focused tests**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwnPlant(AllocatesFourLandsToOneFourGridSeed|ContinuesAfterNoCompleteFourGridGroup)' -count=1`

Expected: PASS.

### Task 3: Preserve Runtime Failure Detail

**Files:**
- Modify: `internal/farm/automation/runtime_own.go:300-370`
- Test: `internal/farm/automation/runtime_own_test.go`

- [x] **Step 1: Extract the nested runtime reason in Go**

```go
reason := firstNonEmptyText(
    stringConfigAny(result["reason"]),
    stringConfigAny(mapFromAny(result["plantResult"])["reason"]),
)
```

- [x] **Step 2: Run focused Go regression tests**

Run: `go test ./internal/farm/automation -run TestRuntimeFacadeOwnPlant -count=1`

Expected: PASS.

### Task 4: Verify The Complete Change

**Files:**
- Verify: `internal/farm/automation/runtime_own.go`
- Verify: `internal/farm/automation/runtime_own_test.go`
- Verify: `resources/wmpf/button.js`

- [x] **Step 1: Format Go files**

Run: `gofmt -w internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go`

- [x] **Step 2: Run the affected package**

Run: `go test ./internal/farm/automation -count=1`

Expected: PASS.

- [x] **Step 3: Run the full Go suite**

Run: `go test ./...`

Expected: PASS.

- [x] **Step 4: Inspect the patch**

Run: `git diff --check && git diff -- internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go resources/wmpf/button.js`

Expected: no whitespace errors; only the planned allocation, fallback, error-detail, and test changes.
