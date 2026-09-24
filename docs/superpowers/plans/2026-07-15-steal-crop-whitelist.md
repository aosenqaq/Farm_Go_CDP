# Friend Steal Crop Whitelist Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce selected crops in friend-steal whitelist mode and expose the missing 红云飞片 and 艾草 crop options.

**Architecture:** Keep all crop-mode behavior in `internal/farm/stealrules.ApplyCropRules`, used by automated and manual friend stealing. Build crop choices from existing `Plant.json` entries plus the bundled crop-level mapping, preserving IDs and image lookup through the existing resolver.

**Tech Stack:** Go, existing `go test` suites under `internal/farm/automation` and `internal/farm`.

---

### Task 1: Whitelist Rule Regression Coverage

**Files:**
- Modify: `internal/farm/automation/runtime_friend_test.go`

- [ ] **Step 1: Write failing whitelist strategy tests**

Add tests that configure `autoFarmFriendStealPlantListMode: "whitelist"` and `autoFarmFriendStealPlantWhitelist: []int{2002}` against an inspected farm containing plants `2001` and `2002`. Assert strategy `1` skips the farm and strategy `2` harvests only the `2002` land. Add an enabled empty-whitelist case that asserts no harvest request is made.

- [ ] **Step 2: Run the focused regression tests**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeFriendSteal.*Whitelist' -count=1`

Expected: FAIL because `ApplyCropRules` currently returns unfiltered lands for `whitelist` mode.

- [ ] **Step 3: Implement whitelist filtering**

Modify `internal/farm/stealrules/crop_rules.go` so the active list depends on `autoFarmFriendStealPlantListMode`. For whitelist mode, mark detectable plant IDs not present in `autoFarmFriendStealPlantWhitelist` as blocked, applying existing strategy `1` and `2` semantics. Return `SkipFarm: true` when an enabled whitelist is empty.

- [ ] **Step 4: Verify the focused regression tests**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeFriendSteal.*Whitelist' -count=1`

Expected: PASS.

### Task 2: Complete Crop Option Coverage

**Files:**
- Modify: `internal/farm/gameconfig_test.go`
- Modify: `internal/farm/gameconfig.go`

- [ ] **Step 1: Write a failing crop-option test**

Extend `TestBuildStealCropOptionsFromResources` to find `红云飞片` and `艾草`, asserting IDs `1020193` and `1021135`, seed IDs `20193` and `21135`, and non-empty image URLs.

- [ ] **Step 2: Run the focused option test**

Run: `go test ./internal/farm -run TestBuildStealCropOptionsFromResources -count=1`

Expected: FAIL because both crops are absent from `Plant.json` and therefore omitted by `BuildStealCropOptions`.

- [ ] **Step 3: Supplement primary crop entries from the existing mapping**

Modify `BuildStealCropOptions` to merge `loadCropAtlasMetaIndex(root).byCropID` entries after `Plant.json`, skip duplicate plant IDs, preserve mapping order as `SortOrder`, and reuse `resolvePlantMainImage` for local image data URLs.

- [ ] **Step 4: Verify the focused option test**

Run: `go test ./internal/farm -run TestBuildStealCropOptionsFromResources -count=1`

Expected: PASS.

### Task 3: Full Verification

**Files:**
- Verify only

- [ ] **Step 1: Run the affected package suites**

Run: `go test ./internal/farm/automation ./internal/farm/social ./internal/farm -count=1`

Expected: PASS with no failures.

- [ ] **Step 2: Inspect the final diff**

Run: `git diff --check; git diff -- internal/farm/stealrules/crop_rules.go internal/farm/automation/runtime_friend_test.go internal/farm/gameconfig.go internal/farm/gameconfig_test.go`

Expected: no whitespace errors; only whitelist behavior, crop option supplementation, and regression tests are changed.
