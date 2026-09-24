# New Crop Resource Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Import the new crop families so warehouse names, land-stage images, and steal-crop option thumbnails work, with activity crops ordered last.

**Architecture:** Game configuration and artwork remain the source of truth. A new activity-crop mapping below `plant_images/stages/_mappings` is read by the Go resolver for both image category lookup and the final steal-list sort group. The frontend continues using backend `imageUrl` and `sortGroup` without a separate catalog.

**Tech Stack:** Go 1.23, Go standard-library JSON/filesystem APIs, React 18, TypeScript, Vitest, Wails local image URLs.

---

### Task 1: Add Failing Regression Tests

**Files:**
- Modify: `internal/farm/gameconfig_test.go:50-145`
- Modify: `internal/farm/gameconfig_land_assets_test.go:17-180`
- Modify: `frontend/src/views/AutomationView.test.tsx:64-90`

- [ ] **Step 1: Test imported warehouse IDs and steal-list options**

Extend `TestBuildStealCropOptionsFromResources` with representative imported options, including the seedless activity crop:

```go
for _, expected := range []struct {
    name string
    plantID int
    seedID int
}{
    {"发财红包", 1020329, 20329},
    {"星语铃花", 1029003, 29003},
    {"蝶梦星铃", 1028003, 0},
} {
    option := findStealCropOption(payload.List, expected.name)
    if option == nil || option.PlantID != expected.plantID || option.SeedID != expected.seedID || option.ImageURL == "" {
        t.Fatalf("expected imported option %#v, got %#v", expected, option)
    }
    if option.SortGroup != 2 {
        t.Fatalf("expected activity-last option, got %#v", option)
    }
}
```

Add a `LoadItemInfoMap` assertion for `20329: 发财红包种子`, `40329: 发财红包`, `1049003: 黄金·星语铃花`, and `204006: 蝶梦星铃`.

- [ ] **Step 2: Test first-phase and missing-phase image fallback**

Use `writeLandAssetConfig` to create an activity crop and create the two relevant tiny images:

```go
writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "作物", "白萝卜", "白萝卜_01_种子.png"))
writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "活动果实", "星语铃花", "星语铃花_00_主图.png"))
```

Assert that phase one produces the generic local seed-stage URL and a later unavailable phase produces the activity crop's local main image, never the default thumbnail.

- [ ] **Step 3: Test crop-grid image rendering and order**

Pass a normal option and an activity option with local `imageUrl` values into `AutomationView`, open the friend crop list, and assert the visible labels and image nodes:

```tsx
expect(cropButtons.map((node) => node.findAllByType('span')[0]?.children.join('')))
  .toEqual(['白萝卜', '星语铃花']);
expect(cropButtons.every((node) => node.findAllByType('img').length === 1)).toBe(true);
```

- [ ] **Step 4: Verify RED**

Run:

```powershell
go test ./internal/farm -run 'TestBuildStealCropOptionsFromResources|TestImportedCropStageFallback' -count=1
Set-Location frontend; npm test -- --run AutomationView.test.tsx
```

Expected: FAIL because imported data, activity category resolution, and seedless crop support do not yet exist.

### Task 2: Import the Resource Delta

**Files:**
- Modify: `resources/gameConfig/Plant.json`
- Modify: `resources/gameConfig/ItemInfo.json`
- Create: `resources/gameConfig/plant_images/stages/_mappings/activity_crops.json`
- Create: `resources/gameConfig/plant_images/stages/活动果实/<crop>/<stage>.png`

- [ ] **Step 1: Merge only the source crop and item records**

Read the source Cocos `Plant` and `ItemInfo` payloads from the fetched miniapp and append only absent numeric IDs for: 发财红包, 帝王血, 铃兰, 月见草, 紫茉莉, 萱草, 月光花, 银星海棠, 紫薇, 梧桐, 勿忘我, 木槿, 星语铃花, 蝶梦星铃, and their golden variants.

Retain existing JSON records. The imported plant identities are:

```text
发财红包: 1020329 / 20329 / 40329
帝王血: 1020264 / 20264 / 40264
铃兰: 1020108 / 20108 / 40108
月见草: 1026032 / 26032 / 46032
紫茉莉: 1021251 / 21251 / 41251
萱草: 1021050 / 21050 / 41050
月光花: 1021404 / 21404 / 41404
银星海棠: 1021037 / 21037 / 41037
紫薇: 1021353 / 21353 / 41353
梧桐: 1021380 / 21380 / 41380
勿忘我: 1020129 / 20129 / 40129
木槿: 1020375 / 20375 / 40375
星语铃花: 1029003 / 29003 / 49003
蝶梦星铃: 1028003 / no seed / 204006
```

- [ ] **Step 2: Create the explicit activity mapping**

Create `activity_crops.json` with every imported family in `items`. Each record has `crop_id`, optional `seed_id`, `fruit_id`, `name`, and `image_category: "活动果实"`. Do not infer this category from names or ID ranges.

- [ ] **Step 3: Copy and normalize image names**

Copy only the 115 stage images and 27 seed images from `新增资源整理`. Place them under `活动果实/<display name>/` and rename them to the parser convention:

```text
铃兰_02_发芽.png
铃兰_03_小叶子.png
铃兰_04_大叶子.png
铃兰_05_初熟.png
铃兰_06_成熟.png
黄金·铃兰_02_发芽.png
铃兰_00_主图.png
```

Do not copy the text-only unavailable-stage notices. 星语铃花 and 蝶梦星铃 receive only their supplied seed/main images.

- [ ] **Step 4: Verify resource-only state**

Run:

```powershell
go test ./internal/farm -run 'TestBuildStealCropOptionsFromResources|TestImportedCropStageFallback' -count=1
```

Expected: still FAIL only because the resolver has not yet read `活动果实` or applied the specified fallback rules.

### Task 3: Implement Resolver and Sort-Group Support

**Files:**
- Modify: `internal/farm/gameconfig.go:418-481`
- Modify: `internal/farm/gameconfig.go:1328-1377`
- Modify: `internal/farm/gameconfig.go:1574-1620`
- Modify: `internal/farm/gameconfig.go:2869-2896`

- [ ] **Step 1: Load activity metadata into reusable ID/name maps**

Add `activityCropMetaIndex` with `byCropID`, `bySeedID`, `byFruitID`, and `byName` maps. It reads `activity_crops.json` and allows zero seed IDs for 蝶梦星铃.

```go
type activityCropMeta struct {
    CropID int `json:"crop_id"`
    SeedID int `json:"seed_id"`
    FruitID int `json:"fruit_id"`
    Name string `json:"name"`
    ImageCategory string `json:"image_category"`
}
```

- [ ] **Step 2: Resolve activity stage and main images**

Change `resolvePlantStageImagePath` and `resolvePlantMainImagePath` to search both `作物/<name>` and the metadata-declared `活动果实/<name>` directory. A missing first stage returns the deterministic existing seed-stage image. A missing later phase for an activity crop returns its own `*_00_主图.png`, then the normal project default only when no supplied image exists.

- [ ] **Step 3: Generate final activity sort groups**

Update `BuildStealCropOptions` to permit any positive plant ID and name, even if `SeedID == 0`. Determine `SortGroup` through the activity metadata:

```go
sortGroup := 1
if activityCrops.contains(plant.ID, plant.SeedID, plant.Fruit.ID, name) {
    sortGroup = 2
} else if isShopEligiblePlant(plant, itemMap) {
    sortGroup = 0
}
```

Use the same branch for standard config and atlas-fallback options.

- [ ] **Step 4: Verify GREEN**

Run:

```powershell
go test ./internal/farm -run 'TestBuildStealCropOptionsFromResources|TestImportedCropStageFallback' -count=1
```

Expected: PASS. Imported activity options have local image URLs and are placed after ordinary crops.

### Task 4: Confirm Frontend Contract Preservation

**Files:**
- Modify: `frontend/src/views/AutomationView.test.tsx:64-90`
- Modify: `frontend/src/views/AutomationView.tsx:1438-1475` only if the test exposes a normalization issue

- [ ] **Step 1: Keep backend ordering and resource URLs intact**

Do not add a frontend activity list. Keep `sortGroup`, `sortOrder`, and `imageUrl` in `stealCropOptionsFromConfig`; only change the existing comparator if it fails to preserve a backend-provided group.

- [ ] **Step 2: Verify the frontend test**

Run:

```powershell
Set-Location frontend
npm test -- --run AutomationView.test.tsx
```

Expected: PASS with normal-before-activity labels and image nodes.

### Task 5: Full Verification

**Files:**
- Verify only: `internal/farm/gameconfig.go`
- Verify only: `resources/gameConfig/**`
- Verify only: `frontend/src/views/AutomationView.tsx`

- [ ] **Step 1: Run all Go tests**

Run `go test ./...` from the project root. Expected: PASS.

- [ ] **Step 2: Run all frontend tests and the production build**

Run `npm test` and `npm run build` from `frontend`. Expected: PASS with no TypeScript errors.

- [ ] **Step 3: Inspect final scope**

Inspect the Git working tree and whitespace check. Expected: only the crop configuration/resources, resolver/tests, frontend regression coverage when necessary, and the approved documentation have changed.
