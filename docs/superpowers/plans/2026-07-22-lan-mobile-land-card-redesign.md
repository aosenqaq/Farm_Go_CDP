# LAN Mobile Land Card Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rework LAN WebUI mobile land cards into a readable two-column layout and let each card select inorganic or organic fertilizer from a bottom sheet.

**Architecture:** Preserve the desktop card layout and all existing runtime commands. Extend the LAN land payload with the runtime's already-available `totalStages` so the mobile card can render an accurate growth indicator. `LandDetailsPanel` holds a temporary fertilizer target; its bottom sheet delegates the selected mode to the existing `runLandCardAction` command path.

**Tech Stack:** Go, React 18, TypeScript, CSS, Vitest, Vite.

---

## File Structure

- `internal/farm/gameconfig.go`: expose runtime `totalStages` in `LandDetailsItem` without changing the polling or command APIs.
- `internal/farm/gameconfig_test.go`: prove the runtime land builder retains current and total stage values.
- `frontend/src/views/AssetsLandView.tsx`: add the two-column card markup, mobile fertilizer chooser state/component, and stage-progress helper while preserving desktop actions.
- `frontend/src/views/AssetsLandView.test.tsx`: cover stage percentage, card markup, two-column CSS contract, chooser opening, normal/organic dispatch, close, and disabled behavior.
- `frontend/src/style.css`: scope the two-column visual redesign and bottom sheet to `.app-shell-remote` at the mobile breakpoint.

### Task 1: Expose Accurate Runtime Stage Count

**Files:**
- Modify: `internal/farm/gameconfig.go:277-313, 904-958`
- Modify: `internal/farm/gameconfig_test.go:394-420`
- Test: `internal/farm/gameconfig_test.go`

- [ ] **Step 1: Write the failing runtime payload test**

  Add this test after `TestRuntimeLandDetailsExposesLandRushAndBulkFertilizerActions`:

  ```go
  func TestRuntimeLandDetailsPreservesStageCount(t *testing.T) {
      payload := BuildRuntimeLandDetailsForRoot(map[string]any{
          "farmType": "own",
          "grids": []any{map[string]any{
              "landId":      1,
              "plantName":   "白萝卜",
              "stageKind":   "growing",
              "currentStage": 2,
              "totalStages": 5,
          }},
      }, filepath.Join("..", "..", "resources", "gameConfig"))

      if len(payload.Lands) != 1 {
          t.Fatalf("expected one land, got %#v", payload.Lands)
      }
      land := payload.Lands[0]
      if land.CurrentStage != 2 || land.TotalStages != 5 {
          t.Fatalf("expected runtime stage count, got %#v", land)
      }
  }
  ```

- [ ] **Step 2: Run the focused Go test and confirm the expected compile failure**

  Run: `go test ./internal/farm -run TestRuntimeLandDetailsPreservesStageCount -count=1`

  Expected: FAIL because `farm.LandDetailsItem` does not yet define `TotalStages`.

- [ ] **Step 3: Add the payload field and runtime mapping**

  Add the optional JSON field directly after `CurrentStage`:

  ```go
  CurrentStage int    `json:"currentStage,omitempty"`
  TotalStages  int    `json:"totalStages,omitempty"`
  PhaseName    string `json:"phaseName,omitempty"`
  ```

  Read the value from the runtime grid beside `currentStage` and assign it in `land`:

  ```go
  currentStage := intFromMap(item, "currentStage")
  totalStages := intFromMap(item, "totalStages")
  phaseName := stringFromMap(item, "phaseName")

  // In the LandDetailsItem literal:
  CurrentStage: currentStage,
  TotalStages:  totalStages,
  PhaseName:    phaseName,
  ```

  Do not infer a value from local config. A missing runtime value remains `0`, so the frontend can omit the progress indicator rather than display invented progress.

- [ ] **Step 4: Run the focused Go test and confirm it passes**

  Run: `go test ./internal/farm -run TestRuntimeLandDetailsPreservesStageCount -count=1`

  Expected: PASS.

- [ ] **Step 5: Commit the independent runtime payload change**

  ```powershell
  git add -- internal/farm/gameconfig.go internal/farm/gameconfig_test.go
  git commit -m "feat: expose runtime land stage counts"
  ```

### Task 2: Define The Failing Mobile Card And Fertilizer Chooser Contract

**Files:**
- Modify: `frontend/src/views/AssetsLandView.test.tsx:864-925`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Replace the four-column CSS expectation with the approved two-column contract**

  Replace `uses a fixed four-column land grid` with this test:

  ```tsx
  it('uses two readable columns for remote mobile land cards', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).toMatch(/\.app-shell-remote \.land-grid\s*\{[\s\S]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/);
    expect(css).toMatch(/\.app-shell-remote \.land-tile\s*\{[\s\S]*min-height:\s*270px;/);
    expect(css).toMatch(/\.app-shell-remote \.land-visual\s*\{[\s\S]*height:\s*82px;/);
    expect(css).toMatch(/\.app-shell-remote \.land-card-action-fertilize\s*\{[\s\S]*display:\s*inline-flex;/);
    expect(css).toContain('.app-shell-remote .land-card-action-mode');
  });
  ```

- [ ] **Step 2: Add failing card markup and progress tests**

  Add this test beside the existing compact-card tests:

  ```tsx
  it('renders a mobile fertilizer trigger and accurate stage progress when runtime provides both stages', () => {
    const landGrowthPercent = (AssetsLandModule as Record<string, unknown>).landGrowthPercent;

    expect(landGrowthPercent).toBeTypeOf('function');
    if (typeof landGrowthPercent !== 'function') return;
    expect(landGrowthPercent({ status: 'growing', currentStage: 2, totalStages: 5 })).toBe(25);
    expect(landGrowthPercent({ status: 'mature', canHarvest: true, currentStage: 5, totalStages: 5 })).toBe(100);
    expect(landGrowthPercent({ status: 'growing', currentStage: 2, totalStages: 0 })).toBeNull();

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
            matureInSec: 205, currentStage: 2, totalStages: 5, currentSeason: 2, totalSeason: 3,
            landTypeLabel: '紫金土地', canHarvest: false,
          }],
          actions: [],
        }}
      />,
    );

    expect(html).toContain('land-growth-track');
    expect(html).toContain('aria-label="生长进度 25%"');
    expect(html).toContain('aria-label="选择肥料类型"');
    expect(html).toContain('>施肥</span>');
    expect(html).toContain('>铲除</span>');
  });
  ```

- [ ] **Step 3: Add the failing chooser interaction test**

  Add this test after the markup test:

  ```tsx
  it('opens the fertilizer chooser and dispatches the selected fertilizer mode for its land', () => {
    const onLandCardAction = vi.fn();
    const renderer = create(
      <LandDetailsPanel
        loading={false}
        error=""
        nowMs={0}
        onRefresh={() => undefined}
        onLandCardAction={onLandCardAction}
        payload={{
          status: 'runtime',
          lands: [{ id: '1', landId: 1, plantName: '白萝卜', status: 'growing', statusLabel: '生长中', canHarvest: false }],
          actions: [],
        }}
      />,
    );

    act(() => {
      renderer.root.findByProps({ 'aria-label': '选择肥料类型' }).props.onClick();
    });
    expect(renderer.root.findByProps({ role: 'dialog' }).props['aria-label']).toBe('对 #1 白萝卜施肥');

    act(() => {
      renderer.root.findAllByType('button').find((node) => node.children.join('') === '取消')?.props.onClick();
    });
    expect(renderer.root.findAllByProps({ role: 'dialog' })).toHaveLength(0);

    act(() => {
      renderer.root.findByProps({ 'aria-label': '选择肥料类型' }).props.onClick();
    });

    act(() => {
      renderer.root.findByProps({ 'aria-label': '对 #1 白萝卜施用有机肥' }).props.onClick();
    });
    expect(onLandCardAction).toHaveBeenCalledWith(
      'fertilize',
      expect.objectContaining({ landId: 1 }),
      'organic',
    );
    expect(renderer.root.findAllByProps({ role: 'dialog' })).toHaveLength(0);
  });
  ```

- [ ] **Step 4: Add the failing disabled-trigger test**

  Add this test after the interaction test:

  ```tsx
  it('does not open the fertilizer chooser when land card actions are disabled', () => {
    const renderer = create(
      <LandDetailsPanel
        loading={false}
        error=""
        nowMs={0}
        cardActionsDisabled
        onRefresh={() => undefined}
        payload={{
          status: 'runtime',
          lands: [{ id: '1', landId: 1, plantName: '白萝卜', status: 'growing', statusLabel: '生长中', canHarvest: false }],
          actions: [],
        }}
      />,
    );

    const trigger = renderer.root.findByProps({ 'aria-label': '选择肥料类型' });
    expect(trigger.props.disabled).toBe(true);
    act(() => {
      trigger.props.onClick();
    });
    expect(renderer.root.findAllByProps({ role: 'dialog' })).toHaveLength(0);
  });
  ```

- [ ] **Step 5: Run the focused frontend test and confirm it fails**

  Run: `pnpm --dir frontend test -- AssetsLandView.test.tsx`

  Expected: FAIL because the new two-column selector, `landGrowthPercent`, `land-growth-track`, mobile fertilizer trigger, and chooser dialog do not exist yet.

### Task 3: Implement The Card State And Fertilizer Selection Flow

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx:1, 160, 823-1000, 2130-2180`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Extend the frontend land type and add the progress helper**

  Add the fields already returned by the LAN payload:

  ```ts
  currentStage?: number;
  totalStages?: number;
  phaseName?: string;
  ```

  Add this exported helper next to `landCountdownText`:

  ```ts
  export function landGrowthPercent(land: LandDetailsItemLike) {
    if (land.status === 'mature' || land.canHarvest) return 100;
    const currentStage = Number(land.currentStage);
    const totalStages = Number(land.totalStages);
    if (!Number.isFinite(currentStage) || !Number.isFinite(totalStages) || currentStage < 1 || totalStages < 2) return null;
    return Math.round(Math.min(1, Math.max(0, (currentStage - 1) / (totalStages - 1))) * 100);
  }
  ```

- [ ] **Step 2: Add the chooser state and component**

  Inside `LandDetailsPanel`, declare the selected target and use an explicit open guard:

  ```tsx
  const [fertilizerTarget, setFertilizerTarget] = useState<LandDetailsItemLike | null>(null);

  function openFertilizerChooser(land: LandDetailsItemLike) {
    if (cardActionsDisabled || !landRushLandId(land) || landActionBusyKey) return;
    setFertilizerTarget(land);
  }

  function selectFertilizer(mode: LandCardFertilizerMode) {
    const target = fertilizerTarget;
    if (!target || cardActionsDisabled || landActionBusyKey) return;
    setFertilizerTarget(null);
    onLandCardAction?.('fertilize', target, mode);
  }
  ```

  Wrap the existing section and the chooser in a fragment, then render:

  ```tsx
  <LandFertilizerChooser
    land={fertilizerTarget}
    busy={Boolean(landActionBusyKey)}
    open={Boolean(fertilizerTarget)}
    onClose={() => setFertilizerTarget(null)}
    onSelect={selectFertilizer}
  />
  ```

  Add `LandFertilizerChooser` before `LandRushDialog`. It must return `null` while closed and otherwise provide the target-specific label, two explicit mode choices, and Cancel:

  ```tsx
  function LandFertilizerChooser({
    open, land, busy, onClose, onSelect,
  }: {
    open: boolean;
    land: LandDetailsItemLike | null;
    busy: boolean;
    onClose: () => void;
    onSelect: (mode: LandCardFertilizerMode) => void;
  }) {
    if (!open || !land) return null;
    const title = `对 #${land.landId} ${landDisplayName(land)}施肥`;
    return (
      <div className="land-fertilizer-drawer-backdrop" role="presentation">
        <aside className="land-fertilizer-drawer" role="dialog" aria-modal="true" aria-label={title}>
          <header className="land-fertilizer-header">
            <div><h2>选择肥料类型</h2><p>{title}</p></div>
            <button type="button" aria-label="关闭施肥选择" onClick={onClose}><X size={17} /></button>
          </header>
          <div className="land-fertilizer-options">
            <button type="button" disabled={busy} aria-label={`对 #${land.landId} ${landDisplayName(land)}施用无机肥`} onClick={() => onSelect('normal')}><Sprout size={18} /><strong>无机肥</strong><span>立即施用</span></button>
            <button type="button" disabled={busy} aria-label={`对 #${land.landId} ${landDisplayName(land)}施用有机肥`} onClick={() => onSelect('organic')}><Droplet size={18} /><strong>有机肥</strong><span>立即施用</span></button>
          </div>
          <footer><button type="button" disabled={busy} onClick={onClose}>取消</button></footer>
        </aside>
      </div>
    );
  }
  ```

- [ ] **Step 3: Preserve desktop actions and add the mobile-only fertilizer trigger**

  Keep the existing normal, organic, and shovel callbacks for desktop. Give the two existing fertilizer buttons the additional `land-card-action-mode` class. Before them, add this trigger:

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

  After the crop name in `.land-tile-main`, conditionally render accurate progress only when both stage values are usable:

  ```tsx
  {landGrowthPercent(land) != null && (
    <div
      aria-label={`生长进度 ${landGrowthPercent(land)}%`}
      aria-valuemax={100}
      aria-valuemin={0}
      aria-valuenow={landGrowthPercent(land) || 0}
      className="land-growth-track"
      role="progressbar"
    >
      <span style={{ width: `${landGrowthPercent(land)}%` }} />
    </div>
  )}
  ```

  Store `const growthPercent = landGrowthPercent(land);` once per mapped land and use that constant in the conditional, label, value, and style instead of calculating it four times.

- [ ] **Step 4: Run the focused frontend test and confirm it passes**

  Run: `pnpm --dir frontend test -- AssetsLandView.test.tsx`

  Expected: PASS, including the new normal/organic chooser dispatch and the existing land action tests.

### Task 4: Apply The Scoped Two-Column Visual Design

**Files:**
- Modify: `frontend/src/style.css:5388-5699, 9115-9218, 9247-9285`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Add shared action, progress, and chooser styles**

  Add these rules beside the existing land action styles. The trigger is hidden outside the mobile LAN shell, so desktop retains its existing three actions:

  ```css
  .land-growth-track {
    width: min(100%, 132px);
    height: 5px;
    overflow: hidden;
    border-radius: 999px;
    background: #e5e4dc;
  }

  .land-growth-track span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: linear-gradient(90deg, #61bd72, #e9c35d, #eb8a58);
  }

  .land-card-action-fertilize { display: none; }

  .land-fertilizer-drawer-backdrop {
    position: fixed;
    inset: 0;
    z-index: 70;
    display: flex;
    justify-content: flex-end;
    background: rgba(31, 34, 28, 0.32);
  }

  .land-fertilizer-drawer {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr) auto;
    width: min(390px, 100vw);
    border-left: 1px solid #d8c7a8;
    background: #fffdf7;
    box-shadow: -18px 0 44px rgba(55, 44, 24, 0.22);
  }
  ```

  Add the complete header, option-grid, choice-button, and footer rules:

  ```css
  .land-fertilizer-header,
  .land-fertilizer-drawer footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 14px;
  }

  .land-fertilizer-header { border-bottom: 1px solid #eadfc8; }
  .land-fertilizer-header h2 { margin: 0; color: #20271f; font-size: 16px; font-weight: 950; }
  .land-fertilizer-header p { margin: 4px 0 0; color: #756b59; font-size: 12px; font-weight: 800; }
  .land-fertilizer-header button {
    display: grid;
    place-items: center;
    width: 32px;
    height: 32px;
    border: 1px solid #d6c9ae;
    border-radius: 7px;
    color: #3d3a30;
    background: #fff7e8;
  }

  .land-fertilizer-options {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
    padding: 14px;
  }

  .land-fertilizer-options button {
    display: grid;
    place-items: center;
    align-content: center;
    gap: 3px;
    min-width: 0;
    min-height: 76px;
    border: 1px solid #a8d7a9;
    border-radius: 8px;
    color: #287039;
    background: #effbe9;
    font: inherit;
    cursor: pointer;
  }

  .land-fertilizer-options button:nth-child(2) { border-color: #8fc7ff; color: #0076df; background: #eef8ff; }
  .land-fertilizer-options button strong { font-size: 13px; }
  .land-fertilizer-options button span { color: #607563; font-size: 10px; font-weight: 800; }
  .land-fertilizer-options button:disabled,
  .land-fertilizer-drawer footer button:disabled { cursor: wait; opacity: 0.62; }

  .land-fertilizer-drawer footer { border-top: 1px solid #eadfc8; }
  .land-fertilizer-drawer footer button {
    width: 100%;
    height: 34px;
    border: 0;
    border-radius: 6px;
    color: #586259;
    background: #eef0eb;
    font: inherit;
    font-size: 12px;
    font-weight: 900;
    cursor: pointer;
  }
  ```

- [ ] **Step 2: Replace only the remote mobile card overrides**

  Inside the existing `@media (max-width: 760px)` block, replace only `.app-shell-remote` land-card declarations with:

  ```css
  .app-shell-remote .land-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    grid-auto-rows: minmax(270px, auto);
    gap: 8px;
    overflow-x: hidden;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding-right: 1px;
  }

  .app-shell-remote .land-tile {
    grid-template-rows: 18px 82px minmax(54px, auto) minmax(36px, 1fr) 30px;
    min-height: 270px;
    padding: 8px;
    gap: 5px;
    overflow: hidden;
  }

  .app-shell-remote .land-visual {
    height: 82px;
    min-height: 82px;
    border-radius: 6px;
  }

  .app-shell-remote .land-visual-image,
  .app-shell-remote .land-visual img {
    width: 76px;
    height: 76px;
    max-width: 76px;
    max-height: 76px;
  }

  .app-shell-remote .land-tile-main { gap: 4px; }
  .app-shell-remote .land-tile-main strong { font-size: 13px; line-height: 1.25; }
  .app-shell-remote .land-info-pill { min-height: 19px; padding: 3px 6px 0; font-size: 9px; }
  .app-shell-remote .land-tag-list { display: flex; gap: 4px; }
  .app-shell-remote .land-tag { height: 20px; padding: 3px 5px 0; font-size: 9px; }

  .app-shell-remote .land-card-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    min-height: 30px;
    gap: 5px;
    padding-top: 0;
    border-top: 0;
  }

  .app-shell-remote .land-card-action { min-height: 30px; padding: 0 4px; border-radius: 5px; }
  .app-shell-remote .land-card-action-mode { display: none; }
  .app-shell-remote .land-card-action-fertilize { display: inline-flex; border-color: #a8d7a9; color: #287039; background: #effbe9; }
  .app-shell-remote .land-card-action-orange { border-color: #f0abab; color: #ce4a3c; background: #fff0ed; }
  ```

  Keep the existing remote mutations, compact status-pill handling, vertical scroll containment, and no-horizontal-scroll rules. Remove the old remote selectors that hide `.land-tag-list` or `.land-card-action-label`; the approved two-column design displays both.

- [ ] **Step 3: Make the chooser a remote bottom sheet**

  Add the chooser to the existing remote bottom-sheet selector and give it the same bounded surface behavior:

  ```css
  .app-shell-remote .land-fertilizer-drawer-backdrop {
    place-items: end stretch;
    padding: 0 8px;
  }

  .app-shell-remote .land-fertilizer-drawer {
    align-self: end;
    justify-self: stretch;
    width: 100%;
    min-height: 0;
    height: auto;
    max-height: 82dvh;
    border-radius: 14px 14px 0 0;
    box-shadow: 0 -16px 42px rgba(55, 44, 24, 0.2);
  }
  ```

- [ ] **Step 4: Run the focused frontend test and confirm it passes**

  Run: `pnpm --dir frontend test -- AssetsLandView.test.tsx`

  Expected: PASS, including the two-column CSS assertion and all chooser behavior.

### Task 5: Verify The Delivered LAN Frontend

**Files:**
- Modify: `internal/farm/gameconfig.go`
- Modify: `internal/farm/gameconfig_test.go`
- Modify: `frontend/src/views/AssetsLandView.tsx`
- Modify: `frontend/src/views/AssetsLandView.test.tsx`
- Modify: `frontend/src/style.css`
- Generated: `frontend/dist/*` through the Vite build

- [ ] **Step 1: Run full affected test suites**

  Run: `go test ./internal/farm -count=1`

  Expected: PASS.

  Run: `pnpm --dir frontend test -- AssetsLandView.test.tsx`

  Expected: PASS.

  Run: `pnpm --dir frontend test`

  Expected: all Vitest suites PASS.

- [ ] **Step 2: Build the frontend bundle that Wails embeds**

  Run: `pnpm --dir frontend run build`

  Expected: TypeScript and Vite exit `0`, and `frontend/dist/index.html` references freshly emitted hashed assets. This repository's Wails configuration builds `frontend/dist`; it has no root `package.json` script or active `public/app` deployment surface.

  Run:

  ```powershell
  Get-Content frontend/dist/index.html
  Get-ChildItem frontend/dist/assets | Sort-Object LastWriteTime -Descending | Select-Object -First 5 Name, LastWriteTime
  ```

  Expected: the index references the newest asset filename and the asset timestamps correspond to the build.

- [ ] **Step 3: Visually verify the LAN page at a mobile viewport**

  Start or reuse the LAN WebUI, open `资产与土地` then `土地详情` at a viewport no wider than `760px`, and verify:

  - Two cards fit per row without horizontal page scrolling.
  - Card number, crop, maturity text, type/season tags, and `施肥`/`铲除` remain readable.
  - Stage progress appears only when runtime sent both positive stage values.
  - `施肥` opens the bottom sheet; selecting either mode starts the matching existing action and closes the sheet.
  - Existing desktop land cards still show their three direct per-land actions.

- [ ] **Step 4: Inspect and commit the complete implementation**

  Run: `git diff --check`

  Expected: no output.

  ```powershell
  git add -- internal/farm/gameconfig.go internal/farm/gameconfig_test.go frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx frontend/src/style.css frontend/dist
  git commit -m "feat: redesign mobile LAN land cards"
  ```
