# Land Mutation Icon Stack Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render every available mutation icon in a land-details crop visual, stacked in runtime order from top to bottom.

**Architecture:** Keep `mutationTypes` as the primary source of icon URLs and add a view-local helper returning every usable URL in input order. Retain `mutationIconUrl` as a one-item fallback for older payloads. The visual maps those URLs into an overlay container and CSS controls the vertical stack.

**Tech Stack:** React 18, TypeScript, CSS, Vitest, React DOM server rendering.

---

## File Structure

- `frontend/src/views/AssetsLandView.tsx`: derive ordered mutation icon URLs and render the overlay.
- `frontend/src/style.css`: position the overlay and stack its fixed-size icon children.
- `frontend/src/views/AssetsLandView.test.tsx`: assert multiple sources render in input order.

### Task 1: Specify Multiple Mutation Icons

**Files:**
- Modify: `frontend/src/views/AssetsLandView.test.tsx` after the existing mutated-land-card test

- [x] **Step 1: Write the failing test**

```tsx
  it('renders every mutation icon in runtime order on a land card', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView initialTab="lands" initialLandDetails={{
        status: 'runtime', message: '土地详情已从游戏运行时读取。',
        lands: [{
          id: '16', landId: 16, plantName: '荷花', status: 'growing',
          statusLabel: '生长中', hasMutation: true,
          mutationTypes: [
            { typeId: 1, name: '黄金', iconUrl: 'data:image/png;base64,gold' },
            { typeId: 2, name: '神秘', iconUrl: 'data:image/png;base64,mystery' },
          ],
        } as any],
        actions: [],
      }} />,
    );

    const gold = html.indexOf('src="data:image/png;base64,gold"');
    const mystery = html.indexOf('src="data:image/png;base64,mystery"');
    expect(gold).toBeGreaterThan(-1);
    expect(mystery).toBeGreaterThan(gold);
  });
```

- [x] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/views/AssetsLandView.test.tsx` from `frontend`.

Expected: FAIL because `mutationTypes.find(...)` returns only the first icon, leaving the `mystery` source absent.

### Task 2: Render The Ordered Icon Stack

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx:1040-1057`
- Modify: `frontend/src/views/AssetsLandView.tsx:1749-1751`

- [x] **Step 1: Replace the singular icon URL in `LandVisual`**

```tsx
  const iconUrls = mutationIconUrls(land);
  const mutationIcons = iconUrls.length > 0 && (
    <div className="land-mutation-icons">
      {iconUrls.map((iconUrl, index) => (
        <FallbackImage
          key={`${iconUrl}-${index}`}
          className="land-mutation-icon"
          alt={land.mutationTypes?.[index]?.name || land.mutationLabel || '变异'}
          src={iconUrl}
        />
      ))}
    </div>
  );
```

Render `{mutationIcons}` in both `LandVisual` branches in place of the singular icon image.

- [x] **Step 2: Select all icon URLs with legacy fallback**

```tsx
function mutationIconUrls(land: LandDetailsItemLike) {
  const typeIconUrls = land.mutationTypes
    ?.map((item) => item.iconUrl)
    .filter((iconUrl): iconUrl is string => Boolean(iconUrl)) || [];
  return typeIconUrls.length > 0
    ? typeIconUrls
    : land.mutationIconUrl ? [land.mutationIconUrl] : [];
}
```

- [x] **Step 3: Run test to verify it passes**

Run: `npm test -- --run src/views/AssetsLandView.test.tsx` from `frontend`.

Expected: PASS, including the existing legacy `mutationIconUrl` assertion and the new ordered multi-icon assertion.

- [x] **Step 4: Commit the implementation**

```bash
git add frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx
git commit -m "fix: stack land mutation icons"
```

### Task 3: Stack Icons In The Visual Overlay

**Files:**
- Modify: `frontend/src/style.css:4626-4638`

- [x] **Step 1: Place the overlay and retain the 24px icon appearance**

```css
.land-visual .land-mutation-icons {
  position: absolute;
  top: 7px;
  right: 7px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.land-visual .land-mutation-icon {
  width: 24px;
  height: 24px;
  padding: 2px;
  border: 1px solid #e690f2;
  border-radius: 999px;
  background: #fffdf7;
  box-shadow: 0 4px 12px rgba(154, 21, 138, 0.18);
}
```

- [x] **Step 2: Run the focused test after styling**

Run: `npm test -- --run src/views/AssetsLandView.test.tsx` from `frontend`.

Expected: PASS.

- [x] **Step 3: Commit the style change**

```bash
git add frontend/src/style.css
git commit -m "style: stack land mutation icons vertically"
```

### Task 4: Verify The Frontend

**Files:**
- Verify: `frontend/src/views/AssetsLandView.test.tsx`
- Verify: `frontend/src/views/AssetsLandView.tsx`
- Verify: `frontend/src/style.css`

- [x] **Step 1: Run the frontend test suite**

Run: `npm test` from `frontend`.

Expected: PASS with no failing tests.

- [x] **Step 2: Build the frontend**

Run: `npm run build` from `frontend`.

Expected: TypeScript checking and Vite production build complete successfully.

- [x] **Step 3: Inspect the final diff**

Run: `git diff HEAD -- frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx frontend/src/style.css`.

Expected: Only the ordered URL helper, overlay markup, stacked icon CSS, and multiple-icon regression test are present.
