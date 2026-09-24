# Centered Desktop Fertilizer Modal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Present the normal desktop land fertilizer chooser as a centered, appropriately sized modal while retaining the LAN mobile bottom sheet.

**Architecture:** Keep `LandFertilizerChooser` markup, action callbacks, and the remote mobile overrides unchanged. Change only the default chooser backdrop and surface CSS; the existing `.app-shell-remote` media-query rules continue to override the default with its bottom-sheet behavior.

**Tech Stack:** React 18, CSS, Vitest, Vite.

---

### Task 1: Add The Failing Desktop Modal CSS Contract

**Files:**
- Modify: `frontend/src/views/AssetsLandView.test.tsx:917-955`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Add a desktop-modal assertion after the shared action-layout test**

  ```tsx
  it('centers the desktop fertilizer chooser while retaining the remote bottom-sheet override', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).toMatch(/\.land-fertilizer-drawer-backdrop\s*\{[\s\S]*display:\s*grid;[\s\S]*place-items:\s*center;/);
    expect(css).toMatch(/\.land-fertilizer-drawer\s*\{[\s\S]*width:\s*min\(440px,\s*calc\(100vw - 32px\)\);/);
    expect(css).toMatch(/\.app-shell-remote \.land-fertilizer-drawer-backdrop\s*\{[\s\S]*place-items:\s*end stretch;/);
  });
  ```

- [ ] **Step 2: Run the focused test and confirm it fails**

  Run: `pnpm --dir frontend test -- AssetsLandView.test.tsx`

  Expected: FAIL because the default backdrop is still a flex right-side drawer and the surface width is still `390px`.

### Task 2: Center The Desktop Fertilizer Chooser

**Files:**
- Modify: `frontend/src/style.css:5735-5858, 9380-9411`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Replace default drawer positioning with a centered modal surface**

  Replace the default backdrop and surface declarations with:

  ```css
  .land-fertilizer-drawer-backdrop {
    position: fixed;
    inset: 0;
    z-index: 70;
    display: grid;
    place-items: center;
    padding: 16px;
    background: rgba(31, 34, 28, 0.32);
  }

  .land-fertilizer-drawer {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr) auto;
    width: min(440px, calc(100vw - 32px));
    max-height: calc(100dvh - 32px);
    overflow: auto;
    border: 1px solid #d8c7a8;
    border-radius: 10px;
    background: #fffdf7;
    box-shadow: 0 28px 80px rgba(55, 44, 24, 0.28);
  }
  ```

  Do not modify `.land-fertilizer-options`, its two fertilizer cards, dialog semantics, or callbacks.

- [ ] **Step 2: Retain the existing LAN mobile bottom-sheet override**

  Keep the current remote declarations that set the chooser backdrop to `place-items: end stretch` with `padding: 0 8px`, and keep the remote surface's full width, `max-height: 82dvh`, and top-only rounded corners. Its later `height: auto` override remains required for the compact choice content.

- [ ] **Step 3: Run the focused test and confirm it passes**

  Run: `pnpm --dir frontend test -- AssetsLandView.test.tsx`

  Expected: PASS, including prior shared-action and chooser interaction tests.

### Task 3: Verify The Delivered Frontend

**Files:**
- Modify: `frontend/src/style.css`
- Modify: `frontend/src/views/AssetsLandView.test.tsx`
- Generated: `frontend/dist/*` through the Vite build

- [ ] **Step 1: Run all frontend tests**

  Run: `pnpm --dir frontend test`

  Expected: all Vitest test files pass.

- [ ] **Step 2: Build the Wails-embedded frontend**

  Run: `pnpm --dir frontend run build`

  Expected: TypeScript and Vite exit `0`; `frontend/dist/index.html` references new hashed assets.

- [ ] **Step 3: Inspect the diff**

  Run: `git diff --check`

  Expected: no output.
