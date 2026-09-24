# Remove Golden-Bug Direct Protocols Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Remove friend-placement and per-land golden-bug protocols while retaining land-state detection and generic one-click own_base handling.

**Architecture:** Golden-bug state remains in the runtime status payload and the existing land-details UI path. own_base counts a detected golden bug as generic care work, then calls only gameCtl.triggerOneClickOperation("FARM_WORK"); all CleanSocialItems and PutSocialItem golden-bug-specific helpers and exports are removed.

**Tech Stack:** Go 1.25, Wails v2, React/TypeScript, Vitest, runtime JavaScript resource.

---

## File Structure

- internal/farm/automation/runtime_own.go: Remove the dedicated golden-bug protocol branch and simplify its success message.
- internal/farm/automation/runtime_helpers.go: Keep gridHasGoldenBug, use it in generic care counting, and remove ID collection for direct cleanup.
- internal/farm/automation/runtime_own_test.go: Prove golden-bug-only farms use generic one-click care.
- resources/wmpf/button.js: Retain status fields, remove direct clean/friend-place protocol builders, methods, and exports.
- resource_bundle_test.go: Guard removal of the runtime script APIs while preserving detection fields.
- internal/farm/automation/config.go: Remove unused friend golden-bug defaults.
- internal/farm/automation/catalog_test.go: Guard absence of those defaults.
- internal/farm/automation/runtime_cache.go: Remove the obsolete golden-bug mutation-name exception.
- internal/farm/automation/runtime_cache_test.go: Prove the retired method name has no special cache invalidation behavior.

Do not modify resources/gameConfig/ItemInfo.json, resources/gameConfig.bundle.zip, internal/farm/gameconfig.go, frontend/wailsjs/go/models.ts, or frontend/src/views/AssetsLandView.tsx. They retain golden-bug item data and display state.

### Task 1: Route Golden-Bug Work Through One-Click Care

**Files:**
- Modify: internal/farm/automation/runtime_own_test.go:16-146
- Modify: internal/farm/automation/runtime_own.go:70-150
- Modify: internal/farm/automation/runtime_helpers.go:247-321

- [ ] **Step 1: Replace the direct-clean test with a failing one-click regression test.**

~~~go
func TestRuntimeFacadeOwnBaseUsesOneClickForGoldenBugs(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{"farmType": "own", "grids": []any{
			map[string]any{"landId": float64(8), "needGoldenBug": true},
			map[string]any{"landId": float64(3), "socialItemIds": []any{float64(301101)}},
		}},
		"gameCtl.triggerOneClickOperation": map[string]any{"ok": true, "type": "FARM_WORK"},
	}}

	result := NewRuntimeFacade(caller).RunTask(context.Background(), "own_base")
	if !result.OK || result.Message != "一键务农已执行：照料 2 项。" {
		t.Fatalf("unexpected result: %#v", result)
	}
	wantMethods := []string{
		"gameCtl.getFarmOwnership",
		"gameCtl.getFarmStatus",
		"gameCtl.triggerOneClickOperation",
	}
	if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", got, wantMethods)
	}
}
~~~

Update TestRuntimeFacadeRunsOwnBaseThroughRuntimeActions to expect 一键务农已执行：照料 2 项。.

- [ ] **Step 2: Run the targeted test to verify it fails.**

Run: go test ./internal/farm/automation -run 'TestRuntimeFacade(OwnBaseUsesOneClickForGoldenBugs|RunsOwnBaseThroughRuntimeActions)$' -count=1

Expected: FAIL because the current implementation invokes gameCtl.cleanGoldenBugsByProtocol and does not count a golden-bug-only grid as generic care.

- [ ] **Step 3: Write the minimal implementation.**

Inside the existing grid loop in collectCareWorkCount, after the normal-bug condition, add:

~~~go
if gridHasGoldenBug(grid) {
	total++
}
~~~

Delete the complete collectGoldenBugLandIDs function. In runOwnBase, delete the complete goldenBugIDs block that calls gameCtl.cleanGoldenBugsByProtocol, including its two dedicated error results. Set the final message to:

~~~go
Message: fmt.Sprintf("一键务农已执行：照料 %d 项。", careCount),
~~~

- [ ] **Step 4: Run the targeted test to verify it passes.**

Run: go test ./internal/farm/automation -run 'TestRuntimeFacade(OwnBaseUsesOneClickForGoldenBugs|RunsOwnBaseThroughRuntimeActions)$' -count=1

Expected: PASS; both tests observe only FARM_WORK after ownership and status reads.

- [ ] **Step 5: Commit the automation change.**

~~~powershell
git add internal/farm/automation/runtime_own.go internal/farm/automation/runtime_helpers.go internal/farm/automation/runtime_own_test.go
git commit -m "refactor: route golden bug care through one click"
~~~

### Task 2: Remove Direct Runtime Protocol APIs

**Files:**
- Modify: resource_bundle_test.go:3-22
- Modify: resources/wmpf/button.js:20141-20184,21620-21705,23818-23868,26711-26712

- [ ] **Step 1: Add a failing runtime-resource regression test.**

~~~go
func TestRuntimeScriptRemovesGoldenBugDirectProtocols(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("resources", "wmpf", "button.js"))
	if err != nil {
		t.Fatalf("read runtime script: %v", err)
	}
	text := string(script)
	for _, symbol := range []string{
		"buildFriendGoldenBugRequestBytes",
		"buildGoldenBugCleanRequestBytes",
		"findGoldenBugLandIds",
		"cleanGoldenBugsByProtocol",
		"putFriendGoldenBugsByProtocol",
	} {
		if strings.Contains(text, symbol) {
			t.Errorf("retired golden-bug protocol symbol remains: %s", symbol)
		}
	}
	for _, stateField := range []string{
		"hasGoldenBug: hasGoldenBugRuntime",
		"needGoldenBug: hasGoldenBugRuntime",
		"needsGoldenBug: hasGoldenBugRuntime",
	} {
		if !strings.Contains(text, stateField) {
			t.Errorf("golden-bug state field is missing: %s", stateField)
		}
	}
}
~~~

- [ ] **Step 2: Run the resource regression test to verify it fails.**

Run: go test . -run TestRuntimeScriptRemovesGoldenBugDirectProtocols -count=1

Expected: FAIL, reporting the five current direct-protocol symbols.

- [ ] **Step 3: Delete every direct golden-bug protocol implementation and export.**

Delete the complete function declarations named below from resources/wmpf/button.js:

~~~text
buildFriendGoldenBugRequestBytes
buildGoldenBugCleanRequestBytes
findGoldenBugLandIds
cleanGoldenBugsByProtocol
putFriendGoldenBugsByProtocol
~~~

Remove the two entries from the final gameCtl return object:

~~~diff
-    cleanGoldenBugsByProtocol,
-    putFriendGoldenBugsByProtocol,
~~~

Leave the hasGoldenBugRuntime calculation and all three state properties in getGridState and getFarmStatus unchanged.

- [ ] **Step 4: Run the resource regression test to verify it passes.**

Run: go test . -run TestRuntimeScriptRemovesGoldenBugDirectProtocols -count=1

Expected: PASS; no direct protocol symbol remains and all three state fields remain.

- [ ] **Step 5: Commit the runtime resource change.**

~~~powershell
git add resources/wmpf/button.js resource_bundle_test.go
git commit -m "refactor: remove golden bug direct protocols"
~~~

### Task 3: Remove Configuration and Cache Residue

**Files:**
- Modify: internal/farm/automation/catalog_test.go:37-55
- Modify: internal/farm/automation/config.go:28-36
- Modify: internal/farm/automation/runtime_cache_test.go:1-20
- Modify: internal/farm/automation/runtime_cache.go:251-256

- [ ] **Step 1: Add failing tests for retired defaults and cache behavior.**

~~~go
func TestDefaultConfigExcludesFriendGoldenBugSettings(t *testing.T) {
	config := DefaultConfig()
	for _, key := range []string{
		"autoFarmFriendGoldenBugEnabled",
		"autoFarmFriendGoldenBugSpecifiedEnabled",
		"autoFarmFriendGoldenBugFriendGids",
	} {
		if _, exists := config[key]; exists {
			t.Fatalf("retired golden-bug setting remains: %s", key)
		}
	}
}

func TestRuntimeMutationGroupsDoesNotSpecialCaseGoldenBug(t *testing.T) {
	if got := runtimeMutationGroups("gameCtl.cleanGoldenBugsByProtocol"); len(got) != 0 {
		t.Fatalf("golden-bug method groups = %#v, want none", got)
	}
}
~~~

- [ ] **Step 2: Run the new tests to verify they fail.**

Run: go test ./internal/farm/automation -run 'Test(DefaultConfigExcludesFriendGoldenBugSettings|RuntimeMutationGroupsDoesNotSpecialCaseGoldenBug)$' -count=1

Expected: FAIL because all three defaults exist and goldenbug still invalidates farm-related cache groups.

- [ ] **Step 3: Write the minimal cleanup.**

Delete these entries from DefaultConfig:

~~~go
"autoFarmFriendGoldenBugEnabled":          false,
"autoFarmFriendGoldenBugSpecifiedEnabled": false,
"autoFarmFriendGoldenBugFriendGids":       []int{},
~~~

Remove || strings.Contains(name, "goldenbug") from the runtimeMutationGroups farm-mutation condition. Do not add a migration for saved unknown keys; they have no consumer once the defaults are removed.

- [ ] **Step 4: Run the new tests to verify they pass.**

Run: go test ./internal/farm/automation -run 'Test(DefaultConfigExcludesFriendGoldenBugSettings|RuntimeMutationGroupsDoesNotSpecialCaseGoldenBug)$' -count=1

Expected: PASS.

- [ ] **Step 5: Commit configuration and cache cleanup.**

~~~powershell
git add internal/farm/automation/config.go internal/farm/automation/catalog_test.go internal/farm/automation/runtime_cache.go internal/farm/automation/runtime_cache_test.go
git commit -m "chore: remove golden bug configuration residue"
~~~

### Task 4: Verify Retained Detection and Full Regression Suite

**Files:**
- Verify only: internal/farm/gameconfig.go, frontend/wailsjs/go/models.ts, frontend/src/views/AssetsLandView.tsx, frontend/src/views/AssetsLandView.test.tsx

- [ ] **Step 1: Verify the retained status and card contracts remain present.**

~~~powershell
rg -n -S 'NeedGoldenBug|needGoldenBug|金虫' internal/farm/gameconfig.go frontend/wailsjs/go/models.ts frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx
~~~

Expected: the backend field, Wails field, 金虫 card tags, and existing UI test fixture are found.

- [ ] **Step 2: Verify source removal without touching retained configuration or history.**

~~~powershell
rg -n -S --glob '!**/*_test.go' --glob '!graphify-out/**' 'buildFriendGoldenBugRequestBytes|buildGoldenBugCleanRequestBytes|findGoldenBugLandIds|cleanGoldenBugsByProtocol|putFriendGoldenBugsByProtocol|autoFarmFriendGoldenBug' internal resources/wmpf/button.js
~~~

Expected: no production-code matches. Regression tests intentionally retain the removed names as assertions.

- [ ] **Step 3: Run all automated checks.**

~~~powershell
go test ./...
Set-Location frontend; npm test; npm run build
~~~

Expected: all Go packages pass, all Vitest tests pass, and TypeScript/Vite build succeeds.

- [ ] **Step 4: Inspect the final diff and worktree state.**

~~~powershell
git diff master...HEAD --check
git status --short
git log --oneline master..HEAD
~~~

Expected: only the planned protocol-removal commits and the approved design/plan documentation are present; no game-config item data is modified.
