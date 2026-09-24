# Land Artwork Geometric Centering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Center the visible outline of mobile LAN land-card crop artwork without clipping it.

**Architecture:** Keep the existing mobile-only `LandStageImage` canvas workflow. Change `landArtworkDrawPlacement` to derive its anchor and scale constraint from the alpha bounds' geometric midpoint rather than its alpha-weighted centroid. No card CSS, server code, asset path, or desktop fallback changes.

**Tech Stack:** React 18, TypeScript, Vitest, Vite.

---

### Task 1: Add The Failing Geometry Regression

**Files:**
- Modify: `frontend/src/views/AssetsLandView.test.tsx:794-821`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Replace the alpha-weight centering test with the visible-bounds test**

  Replace `centers the opaque artwork weight without clipping its alpha bounds` with:

  ```ts
  it('centers visible alpha bounds for asymmetric artwork without clipping them', () => {
    const drawPlacement = (AssetsLandModule as Record<string, unknown>).landArtworkDrawPlacement;

    expect(drawPlacement).toBeTypeOf('function');
    if (typeof drawPlacement !== 'function') return;

    const bounds = {
      width: 200,
      height: 200,
      minX: 20,
      maxX: 180,
      minY: 20,
      maxY: 180,
      alphaCenterX: 50,
      alphaCenterY: 140,
    };
    const crop = { x: 4, y: 4, width: 192, height: 192 };
    const placement = drawPlacement(bounds, crop, 148);

    expect(placement).toBeTruthy();
    if (!placement) return;
    const scale = placement.width / crop.width;
    const visibleCenterX = placement.x + (((bounds.minX + bounds.maxX) / 2 - crop.x) * scale);
    const visibleCenterY = placement.y + (((bounds.minY + bounds.maxY) / 2 - crop.y) * scale);
    expect(visibleCenterX).toBeCloseTo(74, 5);
    expect(visibleCenterY).toBeCloseTo(74, 5);
    expect(placement.x + (bounds.minX - crop.x) * scale).toBeGreaterThanOrEqual(4);
    expect(placement.x + (bounds.maxX - crop.x) * scale).toBeLessThanOrEqual(144);
    expect(placement.y + (bounds.minY - crop.y) * scale).toBeGreaterThanOrEqual(4);
    expect(placement.y + (bounds.maxY - crop.y) * scale).toBeLessThanOrEqual(144);
  });
  ```

- [ ] **Step 2: Run the test and confirm the current algorithm fails**

  Run: `npm test -- AssetsLandView.test.tsx`

  Expected: FAIL in `centers visible alpha bounds for asymmetric artwork without clipping them`, because `landArtworkDrawPlacement` still uses `alphaCenterX` and `alphaCenterY` instead of the visible bounds midpoint.

### Task 2: Center The Visible Crop Bounds

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx:1223-1367`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Remove the weighted-centroid fields and accumulation**

  Remove `alphaCenterX` and `alphaCenterY` from `LandArtworkBounds`. In `alphaBounds`, remove `alphaWeight`, `weightedX`, `weightedY`, and their per-pixel updates. Return only `width`, `height`, `minX`, `maxX`, `minY`, and `maxY`.

- [ ] **Step 2: Anchor placement on the alpha-bound midpoint**

  In `landArtworkDrawPlacement`, replace the alpha-center calculations with:

  ```ts
  const centerX = (bounds.minX + bounds.maxX) / 2;
  const centerY = (bounds.minY + bounds.maxY) / 2;
  const baseScale = Math.min(size / crop.width, size / crop.height);
  const safeRadius = Math.max(0, size / 2 - Math.max(4, Math.round(size * 0.027)));
  const horizontalReach = Math.max(centerX - bounds.minX, bounds.maxX - centerX);
  const verticalReach = Math.max(centerY - bounds.minY, bounds.maxY - centerY);
  const visibleScale = Math.min(
    horizontalReach > 0 ? safeRadius / horizontalReach : Infinity,
    verticalReach > 0 ? safeRadius / verticalReach : Infinity,
  );
  ```

  Return the same `width` and `height` calculation, with the origin changed to:

  ```ts
  x: size / 2 - (centerX - crop.x) * scale,
  y: size / 2 - (centerY - crop.y) * scale,
  ```

- [ ] **Step 3: Run the focused test after the minimal implementation**

  Run: `npm test -- AssetsLandView.test.tsx`

  Expected: PASS, including the new asymmetric visible-bounds regression.

### Task 3: Validate The Delivered Mobile Frontend

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx`
- Modify: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Run source and production checks**

  Run: `npm test -- AssetsLandView.test.tsx`

  Expected: PASS with no failed tests.

  Run: `npm run build`

  Expected: TypeScript compilation and Vite production build complete with exit code `0`.

- [ ] **Step 2: Verify the LAN page at a mobile viewport**

  Use the currently running LAN WebUI, or start it through the existing Wails development workflow. Open `资产与土地` then `土地详情` at a viewport no wider than 760px. Confirm the opaque crop outline, not only its 74px image element, crosses the visual area's horizontal and vertical midpoint while all leaves, soil, and effects remain visible.

- [ ] **Step 3: Inspect the final change and commit it**

  Run: `git diff --check`

  Expected: no output.

  Run: `git add frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx`

  Run: `git commit -m "fix: center visible mobile land artwork"`
