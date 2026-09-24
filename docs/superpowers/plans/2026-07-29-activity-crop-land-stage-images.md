# Activity Crop Land Stage Images Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve local activity-crop stage images in land details when a runtime grid provides only `displayPlantName`.

**Architecture:** Keep image selection in `internal/farm/gameconfig.go`. Derive a lookup name once from `plantName` and `displayPlantName`, then pass it through the existing plant and activity-crop resolvers. The existing resolver preserves its stage-image, activity-main-image, seed-image, and global fallback order.

**Tech Stack:** Go, standard library filesystem paths, Go testing package.

---

## File Structure

- Modify: `internal/farm/gameconfig.go` - derive the image lookup name from both runtime name fields before resolving land artwork.
- Modify: `internal/farm/gameconfig_land_assets_test.go` - reproduce an activity crop payload that lacks `plantName` but has a display name and a mature-stage image.

### Task 1: Prove The Runtime Display-Name Regression

**Files:**
- Modify: `internal/farm/gameconfig_land_assets_test.go`
- Test: `internal/farm/gameconfig_land_assets_test.go`

- [x] **Step 1: Add the failing regression test after `TestBuildRuntimeLandDetailsUsesImportedActivityCropFallbacks`**

```go
func TestBuildRuntimeLandDetailsUsesDisplayNameForActivityCropStage(t *testing.T) {
	root := writeLandAssetConfig(t)
	stagePath := filepath.Join(root, "plant_images", "stages", "活动果实", "紫薇", "紫薇_06_成熟.png")
	writeTinyPNG(t, stagePath)
	if err := os.MkdirAll(filepath.Join(root, "plant_images", "stages", "_mappings"), 0o755); err != nil {
		t.Fatalf("mkdir activity mapping: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "plant_images", "stages", "_mappings", "activity_crops.json"), []byte(`{"items":[{"crop_id":1021353,"seed_id":21353,"fruit_id":41353,"name":"紫薇","image_category":"活动果实"}]}`), 0o644); err != nil {
		t.Fatalf("write activity mapping: %v", err)
	}

	payload := BuildRuntimeLandDetailsForRoot(map[string]any{
		"grids": []any{map[string]any{
			"landId":           float64(11),
			"displayPlantName": "紫薇",
			"stageKind":        "mature",
			"currentStage":     float64(6),
			"phaseName":        "成熟",
		}},
	}, root)

	if len(payload.Lands) != 1 {
		t.Fatalf("expected one activity land, got %#v", payload.Lands)
	}
	if got, want := payload.Lands[0].ImageURL, gameConfigLocalImageURL(root, stagePath); got != want {
		t.Fatalf("activity stage image = %q, want %q", got, want)
	}
}
```

- [x] **Step 2: Run test to verify it fails before stage resolution**

Run:

```powershell
go test ./internal/farm -run '^TestBuildRuntimeLandDetailsUsesDisplayNameForActivityCropStage$' -count=1
```

Observed: `FAIL` because `ImageURL` is empty in the isolated fixture rather than `紫薇_06_成熟.png`; in the packaged runtime configuration this unresolved path reaches the default image.

### Task 2: Use The Display Name In Backend Resource Resolution

**Files:**
- Modify: `internal/farm/gameconfig.go:957-1006`
- Test: `internal/farm/gameconfig_land_assets_test.go`

- [x] **Step 1: Derive a lookup name before identifying the plant**

Immediately after reading `plantName` and `displayPlantName`, add:

```go
imageLookupName := firstNonEmptyString(plantName, displayPlantName)
```

Replace the plant lookup with:

```go
plant := resolver.plantByRuntimeIdentity(plantID, seedID, imageLookupName)
```

When a resolved plant supplies the missing primary name, keep the lookup name synchronized:

```go
if plantName == "" && plant != nil {
	plantName = strings.TrimSpace(plant.Name)
	imageLookupName = firstNonEmptyString(plantName, displayPlantName)
	displayPlantName = firstNonEmptyString(displayPlantName, plantName)
}
```

- [x] **Step 2: Resolve the stage image with that lookup name**

Replace the land artwork call with:

```go
imageURL := resolver.resolvePlantStageImage(seedID, imageLookupName, currentStage, phaseName)
```

Do not change `resolvePlantStageImagePath`, its fallback order, or the frontend image component.

- [x] **Step 3: Run the focused regression test and verify it passes**

Run:

```powershell
go test ./internal/farm -run '^TestBuildRuntimeLandDetailsUsesDisplayNameForActivityCropStage$' -count=1
```

Expected: `ok   Farm_Go/internal/farm`.

### Task 3: Verify Existing Resolution Behavior

**Files:**
- Test: `internal/farm/gameconfig_land_assets_test.go`

- [x] **Step 1: Run all land-image resolver tests**

Run:

```powershell
go test ./internal/farm -run '^TestBuildRuntimeLandDetails' -count=1
```

Expected: `ok   Farm_Go/internal/farm`; ordinary crop, existing activity-main-image, mutation, and the new display-name-stage tests all pass.

- [x] **Step 2: Run the complete farm package test suite**

Run:

```powershell
go test ./internal/farm -count=1
```

Expected: `ok   Farm_Go/internal/farm`.

- [x] **Step 3: Inspect the scoped diff before committing**

Run:

```powershell
git diff --check -- internal/farm/gameconfig.go internal/farm/gameconfig_land_assets_test.go
git diff -- internal/farm/gameconfig.go internal/farm/gameconfig_land_assets_test.go
```

Expected: no whitespace errors and only the lookup-name fallback plus its regression test. Do not stage or commit any pre-existing worktree changes.
