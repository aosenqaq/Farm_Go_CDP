# Dream Butterfly Mutation Implementation Plan

> **For agentic workers:** Execute this plan inline with test-first changes; each task is independently verifiable.

**Goal:** Display the new `梦蝶` mutation type and the `黄金·蝶梦星铃` result in runtime land details.

**Architecture:** Keep the existing static mutation-type registry as the source of runtime labels and IDs. Copy the new icon into the existing mutation-icon convention, map runtime output ID `1128003` to the already imported activity crop, and extend mutation image lookup to reuse that crop image.

**Tech Stack:** Go 1.23, standard-library filesystem APIs, existing local game asset URLs.

---

### Task 1: Define the Regression

**Files:**
- Modify: `internal/farm/gameconfig_land_assets_test.go`

- [x] **Step 1: Write the failing runtime land-details test**

Add a case for `plantName: "蝶梦星铃"` and `activeMutantTypes: []any{float64(5), float64(11)}`. Create local files for the `梦蝶` icon and `黄金·蝶梦星铃` main image. Assert:

```go
if land.MutationLabel != "黄金、梦蝶" {
	t.Fatalf("unexpected mutation label %q", land.MutationLabel)
}
if land.DisplayPlantName != "黄金·蝶梦星铃" {
	t.Fatalf("unexpected mutation output %q", land.DisplayPlantName)
}
assertLocalLandImage(t, land.MutationIconURL)
assertLocalLandImage(t, land.MutationImageURL)
```

- [x] **Step 2: Verify the failure**

Run:

```powershell
go test ./internal/farm -run TestBuildRuntimeLandDetailsResolvesDreamButterflyMutation -count=1
```

Expected: fail because type `11` has no display name or icon and runtime ID `1128003` is unmapped.

### Task 2: Register the Mutation and Resource

**Files:**
- Modify: `internal/farm/gameconfig.go`
- Create: `resources/gameConfig/plant_images/stages/变异/变异宝典/梦蝶/梦蝶_00_变异图标.png`

- [x] **Step 1: Register the type and golden output**

Add `11: "梦蝶"` to both ID/name maps and append `梦蝶` to the display order. Add `1128003: "黄金·蝶梦星铃"` to `mutationPlantNamesByRuntimeID`.

- [x] **Step 2: Reuse activity crop imagery**

At the end of `resolveMutationPlantImage`, look up the supplied name through `activityCrops`; when present, return `resolveActivityCropMainImagePath` as a local image URL.

- [x] **Step 3: Copy the icon**

Copy source `新增资源整理/变异类型/蝶梦.png` to the exact icon path used by `resolveMutationIcon`.

- [x] **Step 4: Verify the focused test**

Run the Task 1 command. Expected: PASS.

### Task 3: Regression Verification

**Files:**
- Verify only: `internal/farm/gameconfig.go`
- Verify only: `internal/farm/gameconfig_land_assets_test.go`

- [x] **Step 1: Format and run Go tests**

Run:

```powershell
gofmt -w internal/farm/gameconfig.go internal/farm/gameconfig_land_assets_test.go
go test ./...
```

Expected: PASS.

- [x] **Step 2: Verify frontend compatibility**

Run:

```powershell
Set-Location frontend
npm test -- --run
npm run build
```

Expected: all tests and TypeScript production build pass.

- [x] **Step 3: Inspect scope**

Run `git diff --check` and `git status --short`. Expected: only the mutation registry, test, icon, and approved documentation changes are added on top of the existing crop work.
