# Shared Land Fertilizer Revision Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove land-card progress bars and use the same `施肥` plus `铲除` action layout and fertilizer chooser on desktop and LAN mobile land cards.

**Architecture:** Keep `LandFertilizerChooser` and its existing callbacks. Remove the now-unused total-stage payload extension and all progress-related frontend code. Make the base land action row show the chooser trigger and shovel action; the remote breakpoint only changes card density and positions the chooser as a bottom sheet.

**Tech Stack:** Go, React 18, TypeScript, CSS, Vitest, Vite.

---

### Task 1: Remove The Unused Stage Payload With A Failing Regression Test

**Files:**
- Modify: `internal/farm/gameconfig.go:277-313, 910-945`
- Modify: `internal/farm/gameconfig_test.go:1-8, 421-447`
- Modify: `frontend/wailsjs/go/models.ts:1205-1256`
- Test: `internal/farm/gameconfig_test.go`

- [ ] **Step 1: Replace the stage-count test with an absence contract**

  Add `encoding/json` to the test imports and replace `TestRuntimeLandDetailsPreservesStageCount` with:

  ```go
  func TestRuntimeLandDetailsDoesNotExposeTotalStageCount(t *testing.T) {
      payload := BuildRuntimeLandDetailsForRoot(map[string]any{
          "farmType": "own",
          "grids": []any{map[string]any{
              "landId": 1, "plantName": "白萝卜", "stageKind": "growing",
              "currentStage": 2, "totalStages": 5,
          }},
      }, filepath.Join("..", "..", "resources", "gameConfig"))

      encoded, err := json.Marshal(payload.Lands[0])
      if err != nil {
          t.Fatalf("marshal land: %v", err)
      }
      if strings.Contains(string(encoded), `"totalStages"`) {
          t.Fatalf("land payload still exposes totalStages: %s", encoded)
      }
  }
  ```

- [ ] **Step 2: Run the focused Go test and confirm it fails**

  Run: `go test ./internal/farm -run TestRuntimeLandDetailsDoesNotExposeTotalStageCount -count=1`

  Expected: FAIL because the JSON payload still contains `totalStages`.

- [ ] **Step 3: Remove the field from the runtime and generated model**

  Delete the `TotalStages` member from `LandDetailsItem`, remove the `totalStages := intFromMap(...)` assignment, and remove `TotalStages: totalStages` from its literal. In `frontend/wailsjs/go/models.ts`, delete both generated `totalStages?: number;` and `this.totalStages = source["totalStages"];` lines.

- [ ] **Step 4: Re-run the focused Go test**

  Run: `go test ./internal/farm -run TestRuntimeLandDetailsDoesNotExposeTotalStageCount -count=1`

  Expected: PASS.

### Task 2: Specify The Shared Action Layout Before Changing The Component

**Files:**
- Modify: `frontend/src/views/AssetsLandView.test.tsx:864-1028`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Change existing card assertions to the shared actions**

  In `sorts land cards by land id and renders figure-style card actions`, replace the direct fertilizer text assertions with:

  ```tsx
  expect(html).toContain('land-card-actions');
  expect(html).toContain('>施肥</span>');
  expect(html).toContain('>铲除</span>');
  expect(html).not.toContain('>无机</span>');
  expect(html).not.toContain('>有机</span>');
  ```

  In `renders compact mobile land countdowns and labelled icon actions`, replace the two direct-fertilizer label assertions with:

  ```tsx
  expect(html).toContain('aria-label="选择肥料类型"');
  expect(html).toContain('aria-label="铲除作物"');
  expect(html).not.toContain('aria-label="施用无机肥"');
  expect(html).not.toContain('aria-label="施用有机肥"');
  ```

- [ ] **Step 2: Replace the progress test with a no-progress, shared-layout contract**

  Replace `renders a mobile fertilizer trigger and accurate stage progress when runtime provides both stages` with:

  ```tsx
  it('uses the shared fertilizer trigger without rendering a land progress bar', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');
    const html = renderToStaticMarkup(
      <LandDetailsPanel
        loading={false}
        error=""
        nowMs={0}
        onRefresh={() => undefined}
        payload={{
          status: 'runtime',
          lands: [{
            id: '1', landId: 1, plantName: '白萝卜', status: 'growing', statusLabel: '生长中',
            matureInSec: 205, currentSeason: 2, totalSeason: 3, landTypeLabel: '紫金土地', canHarvest: false,
          }],
          actions: [],
        }}
      />,
    );

    expect(html).toContain('aria-label="选择肥料类型"');
    expect(html).not.toContain('land-growth-track');
    expect(source).not.toContain('landGrowthPercent');
    expect(css).not.toContain('.land-growth-track');
  });
  ```

- [ ] **Step 3: Change the CSS regression contract to cover desktop too**

  In `uses two readable columns for remote mobile land cards`, retain the two-column remote assertions and replace its action assertions with:

  ```tsx
  expect(css).toMatch(/\.land-card-actions\s*\{[\s\S]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/);
  expect(css).toMatch(/\.land-card-action-fertilize\s*\{[\s\S]*display:\s*inline-flex;/);
  expect(css).not.toContain('land-card-action-mode');
  ```

- [ ] **Step 4: Run the focused frontend test and confirm it fails**

  Run: `pnpm --dir frontend test -- AssetsLandView.test.tsx`

  Expected: FAIL because progress markup and styles remain, and the normal page still exposes direct inorganic and organic buttons.

### Task 3: Remove Progress And Make The Chooser Trigger Universal

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx:219-236, 936-1032, 2287-2293`
- Modify: `frontend/src/style.css:5563-5576, 5656-5718, 9331-9348`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Delete frontend stage-only declarations and markup**

  Remove `currentStage`, `totalStages`, and `phaseName` from `LandDetailsItemLike`. Remove `landGrowthPercent`, the `growthPercent` local, and the complete `land-growth-track` progress-bar JSX. Do not remove the backend's pre-existing `CurrentStage` or `PhaseName` fields, which image resolution still needs.

- [ ] **Step 2: Replace direct card fertilizer buttons with the chooser trigger**

  Delete the two `land-card-action-mode` buttons for direct normal and organic fertilizer. Remove the now-unused `normalBusy` and `organicBusy` variables. Keep this single chooser trigger and the existing shovel button in every card:

  ```tsx
  <button
    type="button"
    aria-label="选择肥料类型"
    className="land-card-action land-card-action-fertilize"
    disabled={disabled}
    onClick={() => openFertilizerChooser(land)}
    title="选择肥料类型"
  >
    <Sprout size={13} />
    <span className="land-card-action-label">施肥</span>
  </button>
  ```

  Leave `LandFertilizerChooser`, its normal/organic selection callbacks, busy protections, Cancel control, and the shovel callback unchanged.

- [ ] **Step 3: Make the base action row the shared layout**

  Delete both `.land-growth-track` rules. Change the base styles to:

  ```css
  .land-card-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 6px;
    min-width: 0;
    padding-top: 8px;
    border-top: 1px solid #eee5d4;
  }

  .land-card-action-fertilize {
    display: inline-flex;
    border-color: #a8d7a9;
    color: #287039;
    background: #effbe9;
  }
  ```

  Delete the remote `.land-card-action-mode` rule and its remote fertilize visibility override. Keep the remote two-column action sizing and its red shovel styling, so the shared markup naturally renders in both contexts.

- [ ] **Step 4: Run the focused frontend suite**

  Run: `pnpm --dir frontend test -- AssetsLandView.test.tsx`

  Expected: PASS, including chooser selection, Cancel, disabled-state, no-progress, and shared desktop/mobile action assertions.

### Task 4: Verify The Revision

**Files:**
- Modify: `internal/farm/gameconfig.go`
- Modify: `internal/farm/gameconfig_test.go`
- Modify: `frontend/wailsjs/go/models.ts`
- Modify: `frontend/src/views/AssetsLandView.tsx`
- Modify: `frontend/src/views/AssetsLandView.test.tsx`
- Modify: `frontend/src/style.css`
- Generated: `frontend/dist/*` through the Vite build

- [ ] **Step 1: Run complete test coverage**

  Run: `go test ./internal/farm -count=1`

  Run: `pnpm --dir frontend test`

  Expected: both commands exit `0`.

- [ ] **Step 2: Build the Wails frontend**

  Run: `pnpm --dir frontend run build`

  Expected: TypeScript and Vite exit `0`, and `frontend/dist/index.html` references freshly emitted assets.

- [ ] **Step 3: Inspect the final diff**

  Run: `git diff --check`

  Expected: no output.
