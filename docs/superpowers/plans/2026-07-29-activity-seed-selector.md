# Activity Seed Selector Implementation Plan

> **For agentic workers:** Execute the checked steps in order. Tests must fail before their implementation or generated-resource change.

**Goal:** Keep every runtime-verified activity seed visible to automatic backpack planting, retain non-seed protection, and synchronize the embedded game-config archive.

**Architecture:** The runtime seed bridge emits normalized records with type 5 and interaction type plant after its own non-seed gate. The Go selector accepts these explicit fields only for live backpack entries, while force-priority rows remain static-ID-only. The bundle test extracts Plant.json and ItemInfo.json from the ZIP into a temporary config root, then exercises the same selector contract for every current activity seed.

**Tech Stack:** Go, standard-library testing, ZIP resources, PowerShell.

---

### Task 1: Accept Runtime-Verified Activity Seeds

**Files:**
- Modify: internal/farm/gameconfig.go
- Modify: internal/farm/gameconfig_test.go

- [ ] **Step 1: Add the failing unknown-activity-seed regression**

Add the following test after TestBuildBackpackSeedOptionsDropsNonSeedBagItems:

~~~
func TestBuildBackpackSeedOptionsAcceptsRuntimeVerifiedUnknownSeed(t *testing.T) {
	payload := BuildBackpackSeedOptions([]any{
		map[string]any{
			"itemId":          float64(99999123),
			"name":            "未来活动种子",
			"count":           float64(4),
			"type":            float64(5),
			"interactionType": "plant",
		},
	}, BackpackSeedOptionsRequest{})

	if !payload.OK {
		t.Fatalf("payload not ok: %#v", payload)
	}
	if len(payload.List) != 1 {
		t.Fatalf("option count = %d, want 1: %#v", len(payload.List), payload.List)
	}
	if option := payload.List[0]; option.SeedID != 99999123 || option.Name != "未来活动" || option.BackpackCount != 4 {
		t.Fatalf("unexpected runtime-verified activity seed: %#v", option)
	}
}
~~~

- [ ] **Step 2: Verify the regression is red**

Run: go test ./internal/farm -run TestBuildBackpackSeedOptionsAcceptsRuntimeVerifiedUnknownSeed -count=1

Expected: FAIL because the ID is absent from the static Plant.json and ItemInfo.json maps.

- [ ] **Step 3: Use runtime proof only for live entries**

Replace the live-list filter in BuildBackpackSeedOptions with:

~~~
if !isPlantableBackpackSeedItem(seedID, item, itemMap, plantBySeedID) {
	continue
}
~~~

Add this helper immediately before isPlantableBackpackSeedID:

~~~
func isPlantableBackpackSeedItem(seedID int, item map[string]any, itemMap map[int]itemInfoConfigItem, plantBySeedID map[int]plantConfigItem) bool {
	if isPlantableBackpackSeedID(seedID, itemMap, plantBySeedID) {
		return true
	}
	return intFromMap(item, "type") == 5 && strings.EqualFold(
		strings.TrimSpace(firstNonEmptyString(
			stringFromMap(item, "interactionType"),
			stringFromMap(item, "interaction_type"),
		)),
		"plant",
	)
}
~~~

Do not alter the force-priority loop: it must keep calling isPlantableBackpackSeedID because absent IDs have no runtime proof.

- [ ] **Step 4: Verify both activity and non-seed paths**

Run: go test ./internal/farm -run 'TestBuildBackpackSeedOptions(AcceptsRuntimeVerifiedUnknownSeed|DropsNonSeedBagItems)' -count=1

Expected: PASS. The unknown activity seed is retained, while dog food, gift packs, and football remain excluded.

### Task 2: Synchronize and Verify All Activity Seed Resources

**Files:**
- Modify: internal/farm/gameconfig_resources_test.go
- Modify: resources/gameConfig.bundle.zip

- [ ] **Step 1: Add the failing archive-selector test**

Add archive/zip and io to the imports. Add this test and helper:

~~~
func TestBundledGameConfigRecognizesAllActivitySeeds(t *testing.T) {
	archive, err := zip.OpenReader(filepath.Join("..", "..", "resources", "gameConfig.bundle.zip"))
	if err != nil {
		t.Fatalf("open bundled game config: %v", err)
	}
	t.Cleanup(func() { archive.Close() })

	root := t.TempDir()
	for _, name := range []string{"Plant.json", "ItemInfo.json"} {
		copyBundledGameConfigFile(t, archive, root, name)
	}
	SetGameConfigRoot(root)
	t.Cleanup(func() { SetGameConfigRoot("") })

	wantBySeedID := map[int]string{
		20329: "发财红包", 20264: "帝王血", 20108: "铃兰", 26032: "月见草",
		21251: "紫茉莉", 21050: "萱草", 21404: "月光花", 21037: "银星海棠",
		21353: "紫薇", 21380: "梧桐", 20129: "勿忘我", 20375: "木槿",
		29003: "星语铃花",
	}
	seedList := make([]any, 0, len(wantBySeedID))
	for seedID := range wantBySeedID {
		seedList = append(seedList, map[string]any{"itemId": seedID, "count": 1})
	}
	payload := BuildBackpackSeedOptions(seedList, BackpackSeedOptionsRequest{})
	if !payload.OK || len(payload.List) != len(wantBySeedID) {
		t.Fatalf("bundled activity seed options = %#v", payload)
	}
	for _, option := range payload.List {
		wantName, ok := wantBySeedID[option.SeedID]
		if !ok || option.Name != wantName || option.BackpackCount != 1 {
			t.Fatalf("bundled activity seed option = %#v, want name %q", option, wantName)
		}
	}
}

func copyBundledGameConfigFile(t *testing.T, archive *zip.ReadCloser, root string, name string) {
	t.Helper()
	for _, entry := range archive.File {
		if entry.Name != name {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatalf("open bundled %s: %v", name, err)
		}
		defer reader.Close()
		output, err := os.Create(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("create extracted %s: %v", name, err)
		}
		if _, err := io.Copy(output, reader); err != nil {
			output.Close()
			t.Fatalf("extract %s: %v", name, err)
		}
		if err := output.Close(); err != nil {
			t.Fatalf("close extracted %s: %v", name, err)
		}
		return
	}
	t.Fatalf("bundled game config is missing %s", name)
}
~~~

- [ ] **Step 2: Verify the archive regression is red**

Run: go test ./internal/farm -run TestBundledGameConfigRecognizesAllActivitySeeds -count=1

Expected: FAIL because the previous ZIP predates the current activity Plant.json records.

- [ ] **Step 3: Rebuild the embedded game-config archive**

Run: powershell -ExecutionPolicy Bypass -File scripts/build-resource-bundle.ps1

Expected: a regenerated resources/gameConfig.bundle.zip with all current resources and a matching resource manifest.

- [ ] **Step 4: Verify every bundled activity seed**

Run: go test ./internal/farm -run TestBundledGameConfigRecognizesAllActivitySeeds -count=1

Expected: PASS. All 13 seed IDs resolve through the extracted bundle to the correct plant names and a backpack count of one.

### Task 3: Regression Verification and Commit

**Files:**
- Verify: internal/farm/gameconfig.go
- Verify: internal/farm/gameconfig_test.go
- Verify: internal/farm/gameconfig_resources_test.go
- Verify: resources/gameConfig.bundle.zip

- [ ] **Step 1: Format and test affected packages**

Run: gofmt -w internal/farm/gameconfig.go internal/farm/gameconfig_test.go internal/farm/gameconfig_resources_test.go resource_bundle_test.go

Run: go test ./internal/farm -count=1

Run: go test ./internal/... -count=1

Expected: PASS.

- [ ] **Step 2: Check the root-package test environment**

Run: go test . -run TestInstallBundledGameConfigCopiesConfigAndImages -count=1

Expected: PASS when the Wails-generated Farm_Go-res.syso file is present. If that generated file is absent, report the pre-existing linker error without replacing or fabricating the Windows resource object.

- [ ] **Step 3: Inspect and commit the finished change**

Run: git diff --check

Run: git add internal/farm/gameconfig.go internal/farm/gameconfig_test.go internal/farm/gameconfig_resources_test.go resource_bundle_test.go resources/gameConfig.bundle.zip docs/superpowers/plans/2026-07-29-activity-seed-selector.md

Run: git commit -m "fix: retain activity seeds in auto planting"

Expected: a clean working tree with the runtime fallback, current activity archive, tests, and implementation plan committed together.

