# Warehouse Jindoudou Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Hide the runtime warehouse system-currency item named `金豆豆` from the warehouse page.

**Architecture:** Keep the existing display-only name set in `AssetsLandView.tsx` as the single source of warehouse filtering. Add `金豆豆` to that set before the panel derives categories, selections, and rows. Extend the existing server-rendered warehouse regression fixture with the same name so it catches future name-list regressions without changing backend payloads or sale APIs.

**Tech Stack:** React 18, TypeScript, Vitest, react-test-renderer, Vite.

---

## File Structure

- `frontend/src/views/AssetsLandView.tsx`: owns the display-only set of system-resource names used at the warehouse panel data boundary.
- `frontend/src/views/AssetsLandView.test.tsx`: renders a warehouse snapshot and verifies hidden names are absent while an ordinary item remains present.

### Task 1: Add the `金豆豆` Warehouse Display Filter

**Files:**
- Modify: `frontend/src/views/AssetsLandView.test.tsx:1482-1485`
- Modify: `frontend/src/views/AssetsLandView.tsx:291-300`

- [ ] **Step 1: Extend the existing regression fixture with the runtime item name**

In `frontend/src/views/AssetsLandView.test.tsx`, add `金豆豆` immediately after `金豆` in the `hiddenNames` array:

```tsx
const hiddenNames = [
  '普通化肥容器', '有机化肥容器', '种植经验', '普通收藏点',
  '典藏收藏点', '金币', '点券', '金豆', '金豆豆',
];
```

The existing fixture maps every name in this array into a `tool` warehouse item and asserts each name is absent from the rendered HTML, so no additional test fixture or assertion is needed.

- [ ] **Step 2: Run the focused regression test and verify it fails**

Run from `frontend`:

```powershell
npm test -- --run src/views/AssetsLandView.test.tsx -t "hides system resources"
```

Expected: FAIL because the current name set excludes `金豆` but not `金豆豆`, so the rendered table contains `金豆豆`.

- [ ] **Step 3: Add the runtime name to the display-only filter set**

In `frontend/src/views/AssetsLandView.tsx`, add `金豆豆` after `金豆` in `hiddenWarehouseItemNames`:

```tsx
const hiddenWarehouseItemNames = new Set([
  '普通化肥容器',
  '有机化肥容器',
  '种植经验',
  '普通收藏点',
  '典藏收藏点',
  '金币',
  '点券',
  '金豆',
  '金豆豆',
]);
```

Do not alter `WarehousePanel`'s existing `items` derivation, backend snapshot model, or warehouse sale calls.

- [ ] **Step 4: Run the focused regression test and verify it passes**

Run from `frontend`:

```powershell
npm test -- --run src/views/AssetsLandView.test.tsx -t "hides system resources"
```

Expected: one passing test, zero failed tests. The rendered HTML excludes `金豆豆` and retains `白萝卜`.

- [ ] **Step 5: Run the complete frontend verification**

Run from `frontend`:

```powershell
npm test
npm run build
```

Expected: the full Vitest suite passes and the Vite production build exits with code `0`.

- [ ] **Step 6: Commit the implementation**

```powershell
git add frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx
git commit -m "fix: hide jindoudou in warehouse"
```
