# LAN Four-Column Land Cards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render four compact, actionable land cards per row in the remote mobile land-details view.

**Architecture:** Keep `LandDetailsPanel` data flow and handlers intact. Add a compact countdown presentation alongside the current detailed countdown, then let the mobile `.app-shell-remote` CSS select compact content, fixed art dimensions, and icon-only actions while desktop keeps its existing card layout.

**Tech Stack:** React 18, TypeScript, CSS, Vitest, Vite.

---

### Task 1: Specify Compact Card Behavior With Failing Tests

**Files:**
- Modify: `frontend/src/views/AssetsLandView.test.tsx:839-894`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Add the compact countdown and accessible-control expectations**

  Add this test beside the existing land-card render tests:

  ```ts
  it('renders compact mobile land countdowns and labelled icon actions', () => {
    const compactCountdown = (AssetsLandModule as Record<string, unknown>).landCompactCountdownText;

    expect(compactCountdown).toBeTypeOf('function');
    if (typeof compactCountdown !== 'function') return;
    expect(compactCountdown({ status: 'mature', canHarvest: true }, 0)).toBe('可收');
    expect(compactCountdown({ status: 'growing', matureInSec: 205 }, 0)).toBe('03:25');

    const html = renderToStaticMarkup(
      <LandDetailsPanel
        loading={false}
        error=""
        nowMs={0}
        onRefresh={() => undefined}
        payload={{
          status: 'runtime',
          lands: [{ id: '1', landId: 1, plantName: '鹭草', status: 'growing', statusLabel: '生长中', matureInSec: 205, canHarvest: false }],
          actions: [],
        }}
      />,
    );

    expect(html).toContain('land-countdown-compact');
    expect(html).toContain('aria-label="施用无机肥"');
    expect(html).toContain('aria-label="施用有机肥"');
    expect(html).toContain('aria-label="铲除作物"');
  });
  ```

- [ ] **Step 2: Replace the remote-grid expectation with the four-column compact contract**

  Add this assertion to the land-grid CSS test:

  ```ts
  expect(css).toMatch(/\.app-shell-remote \.land-grid\s*\{[\s\S]*grid-template-columns:\s*repeat\(4,\s*minmax\(0,\s*1fr\)\);/);
  expect(css).toMatch(/\.app-shell-remote \.land-tile\s*\{[\s\S]*min-height:\s*154px;/);
  expect(css).toMatch(/\.app-shell-remote \.land-visual\s*\{[\s\S]*height:\s*62px;/);
  ```

- [ ] **Step 3: Run the focused test and confirm it fails**

  Run: `npm test -- AssetsLandView.test.tsx`

  Expected: FAIL because `landCompactCountdownText`, the compact countdown markup, accessible icon labels, and four-column remote CSS do not exist yet.

### Task 2: Render Compact Land Card Content

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx:947-982`
- Modify: `frontend/src/views/AssetsLandView.tsx:2131-2168`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Add a compact countdown helper**

  Add these functions next to `landCountdownText`; do not change `landCountdownText`:

  ```ts
  export function landCompactCountdownText(land: LandDetailsItemLike, nowMs: number) {
    if (land.status === 'empty') return '空地';
    if (land.status === 'locked') return '未解锁';
    if (land.status === 'mature' || land.canHarvest) return '可收';
    const matureAtMs = Number(land.matureAtMs) || 0;
    if (matureAtMs > 0) {
      const seconds = Math.max(0, Math.ceil((matureAtMs - nowMs) / 1000));
      return seconds <= 0 ? '可收' : formatCompactCountdown(seconds);
    }
    const matureInSec = Number(land.matureInSec);
    if (Number.isFinite(matureInSec) && matureInSec > 0) return formatCompactCountdown(matureInSec);
    return land.statusLabel;
  }

  function formatCompactCountdown(seconds: number) {
    const total = Math.max(0, Math.floor(seconds));
    const hours = Math.floor(total / 3600);
    const minutes = Math.floor((total % 3600) / 60);
    const secs = total % 60;
    return hours > 0
      ? `${hours}:${String(minutes).padStart(2, '0')}`
      : `${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  }
  ```

- [ ] **Step 2: Render both countdown forms and label each action**

  In the `.land-pill-row`, change the existing full-countdown class to `land-info-pill land-info-blue land-countdown-full` and add:

  ```tsx
  <span className="land-info-pill land-info-blue land-countdown-compact">
    {landCompactCountdownText(land, nowMs)}
  </span>
  ```

  Add `aria-label` and `title` attributes to the existing three card buttons:

  ```tsx
  aria-label="施用无机肥" title="施用无机肥"
  aria-label="施用有机肥" title="施用有机肥"
  aria-label="铲除作物" title="铲除作物"
  ```

  Give each visible action text span the `land-card-action-label` class so mobile CSS can hide only the text. Keep the callbacks, `disabled`, and busy icon branches unchanged.

- [ ] **Step 3: Run the focused test after implementation**

  Run: `npm test -- AssetsLandView.test.tsx`

  Expected: PASS, including compact maturity text, elapsed-time formatting, compact markup, and the three accessible action labels.

### Task 3: Apply The Four-Column Remote Layout

**Files:**
- Modify: `frontend/src/style.css:9092-9162`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Add the default compact-countdown visibility rule**

  Add this rule outside the mobile media query:

  ```css
  .land-countdown-compact {
    display: none;
  }
  ```

- [ ] **Step 2: Replace the remote two-column land overrides**

  Under `@media (max-width: 760px)`, replace the remote land-card rules with:

  ```css
  .app-shell-remote .land-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
    grid-auto-rows: 154px;
    gap: 5px;
    overflow-x: hidden;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding-right: 1px;
  }

  .app-shell-remote .land-tile {
    grid-template-rows: 16px 62px minmax(0, 1fr) 24px;
    min-height: 154px;
    padding: 4px;
    gap: 3px;
    overflow: hidden;
  }

  .app-shell-remote .land-visual {
    height: 62px;
    min-height: 62px;
    aspect-ratio: 1 / 1;
    border-radius: 5px;
  }

  .app-shell-remote .land-visual-image,
  .app-shell-remote .land-visual img {
    width: 56px;
    height: 56px;
    max-width: 56px;
    max-height: 56px;
  }

  .app-shell-remote .land-tile-main {
    gap: 2px;
  }

  .app-shell-remote .land-tile-main strong {
    font-size: 10px;
    line-height: 1.2;
  }

  .app-shell-remote .land-countdown-full,
  .app-shell-remote .land-pill-row .land-info-pill:not(.land-info-blue),
  .app-shell-remote .land-tag-list,
  .app-shell-remote .land-card-action-label {
    display: none;
  }

  .app-shell-remote .land-countdown-compact {
    display: inline-flex;
  }

  .app-shell-remote .land-info-pill {
    min-height: 16px;
    padding: 2px 3px 0;
    font-size: 8px;
  }

  .app-shell-remote .land-card-actions {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    min-height: 24px;
    gap: 2px;
  }

  .app-shell-remote .land-card-action {
    min-height: 24px;
    padding: 0;
    border-radius: 4px;
  }
  ```

  Add these remote rules so header and mutation details remain inside their fixed rows:

  ```css
  .app-shell-remote .land-header-badges {
    gap: 2px;
  }

  .app-shell-remote .land-status-pill {
    max-width: 42px;
    height: 16px;
    padding: 2px 3px 0;
    font-size: 8px;
  }

  .app-shell-remote .land-visual .land-mutation-icons {
    top: 3px;
    right: 3px;
    gap: 2px;
  }

  .app-shell-remote .land-visual .land-mutation-icon {
    width: 14px;
    height: 14px;
    padding: 1px;
  }
  ```

- [ ] **Step 3: Run the focused test after the style change**

  Run: `npm test -- AssetsLandView.test.tsx`

  Expected: PASS with the remote four-column and fixed-card CSS assertions.

### Task 4: Build And Verify The LAN Delivery

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx`
- Modify: `frontend/src/style.css`
- Modify: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Run complete frontend verification**

  Run: `npm test`

  Expected: all Vitest suites pass.

  Run: `npm run build`

  Expected: `tsc` and `vite build` exit with code `0` and update `frontend/dist/assets`.

- [ ] **Step 2: Verify the running LAN source**

  Run:

  ```powershell
  $source = (Invoke-WebRequest -UseBasicParsing 'http://127.0.0.1:34115/src/views/AssetsLandView.tsx').Content
  $source.Contains('landCompactCountdownText')
  ```

  Expected: `True` while the Wails development server is running.

- [ ] **Step 3: Commit the implementation**

  Run: `git diff --check`

  Expected: no output.

  Run: `git add frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx frontend/src/style.css`

  Run: `git commit -m "feat: compact LAN land cards into four columns"`
