# Warehouse System Resource Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Hide the eight specified system-resource rows from the warehouse page without changing the backend warehouse snapshot or sale APIs.

**Architecture:** Keep a read-only set of hidden Chinese item names adjacent to the warehouse UI constants. Derive the panel's `items` array once from the runtime payload by excluding those names, so category tabs, select-all behavior, selected sell keys, table rendering, and the empty state all consume the same filtered data.

**Tech Stack:** React 18, TypeScript, Vitest, react-test-renderer.

---

## File Structure

- `frontend/src/views/AssetsLandView.tsx`: Defines the warehouse display-name set and applies it at the `WarehousePanel` data boundary.
- `frontend/src/views/AssetsLandView.test.tsx`: Adds an HTML-rendering regression test for hidden system resources and a retained ordinary warehouse item.

### Task 1: Warehouse System Resource Display Filter

**Files:**
- Modify: `frontend/src/views/AssetsLandView.test.tsx:865` (add a warehouse rendering regression test after the existing runtime snapshot test)
- Modify: `frontend/src/views/AssetsLandView.tsx:268-281` (add the display-only hidden-name set)
- Modify: `frontend/src/views/AssetsLandView.tsx:1122-1123` (filter warehouse panel items before downstream use)

- [ ] **Step 1: Write the failing warehouse rendering test**

Add this test after `renders runtime warehouse items when backend returns a snapshot`:

```tsx
  it('hides system resources from the warehouse while retaining ordinary items', () => {
    const hiddenNames = [
      '普通化肥容器', '有机化肥容器', '种植经验', '普通收藏点',
      '典藏收藏点', '金币', '点券', '金豆',
    ];
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="warehouse"
        initialWarehouse={{
          status: 'runtime',
          message: '仓库已从游戏运行时读取。',
          items: [
            ...hiddenNames.map((name, index) => ({
              id: `system:${index}`,
              itemId: index + 1,
              name,
              count: 1,
              category: 'tool',
              categoryLabel: '道具',
              canSell: false,
              locked: false,
              estimatedSellPrice: 0,
            })),
            {
              id: '40002:0', itemId: 40002, name: '白萝卜', count: 5,
              category: 'fruit', categoryLabel: '果实', canSell: true,
              locked: false, estimatedSellPrice: 10,
            },
          ],
        }}
      />,
    );

    for (const name of hiddenNames) expect(html).not.toContain(name);
    expect(html).toContain('白萝卜');
  });
```

- [ ] **Step 2: Run the focused test and verify it fails for the missing filter**

Working directory: `frontend`

Run: `npm test -- --run src/views/AssetsLandView.test.tsx -t "hides system resources"`

Expected: FAIL because the current warehouse table includes one or more of the specified system-resource names.

- [ ] **Step 3: Add the minimal display-name set and filter items once in the panel**

After `warehouseCategoryOptions`, add:

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
]);
```

Replace the current item assignment in `WarehousePanel`:

```tsx
  const items = payload?.items || [];
```

with:

```tsx
  const items = (payload?.items || []).filter((item) => !hiddenWarehouseItemNames.has(item.name));
```

This intentionally leaves the payload itself unchanged while causing the existing category merge, tabs, selection logic, and table to operate on the visible items only.

- [ ] **Step 4: Run the focused regression test and verify it passes**

Working directory: `frontend`

Run: `npm test -- --run src/views/AssetsLandView.test.tsx -t "hides system resources"`

Expected: PASS with one passing test and no failures.

- [ ] **Step 5: Run the full frontend test suite and production build**

Working directory: `frontend`

Run: `npm test`

Expected: PASS with all frontend tests passing.

Run: `npm run build`

Expected: TypeScript completes and Vite writes the production build with exit code 0.

- [ ] **Step 6: Commit the feature**

```bash
git add frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx
git commit -m "feat: hide system resources in warehouse"
```
