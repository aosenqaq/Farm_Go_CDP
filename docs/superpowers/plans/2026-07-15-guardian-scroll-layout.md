# Guardian Service Scroll Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let guardian service content scroll within the fixed application viewport and keep expanded settings cards in normal document flow.

**Architecture:** `GuardView` will place its command panel and worker-card grid in a guardian-specific scroll wrapper. Guardian CSS will make that wrapper the remaining-height grid row with `overflow-y: auto`; the capability grid will size naturally inside it.

**Tech Stack:** React 18, TypeScript, CSS Grid, Vitest.

---

### Task 1: Add the guardian layout regression test

**Files:**
- Modify: `frontend/src/views/GuardView.test.tsx`
- Test: `frontend/src/views/GuardView.test.tsx`

- [ ] **Step 1: Write the failing test**

```tsx
// @ts-expect-error The frontend tsconfig intentionally omits Node types.
import { readFileSync } from 'node:fs';

it('places guardian cards inside a dedicated scroll region', () => {
  const html = renderToString(<GuardView /* existing minimal props */ />);
  expect(html).toContain('guard-scroll');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir frontend test GuardView.test.tsx`

Expected: FAIL because the rendered markup does not include `guard-scroll`.

- [ ] **Step 3: Add the CSS contract assertion**

```tsx
const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
expect(css).toMatch(/\.guard-scroll\s*\{[^}]*overflow-y:\s*auto;/);
expect(css).toMatch(/\.guard-stack\s*\{[^}]*grid-template-rows:\s*auto\s+minmax\(0,\s*1fr\);/);
```

- [ ] **Step 4: Run test to verify it still fails for the missing production layout**

Run: `pnpm --dir frontend test GuardView.test.tsx`

Expected: FAIL because neither the wrapper nor CSS contract exists.

### Task 2: Implement guardian-only scrolling

**Files:**
- Modify: `frontend/src/views/GuardView.tsx:110-222`
- Modify: `frontend/src/style.css:6653-6690`
- Test: `frontend/src/views/GuardView.test.tsx`

- [ ] **Step 1: Add the scroll wrapper in `GuardView`**

```tsx
<div className="guard-scroll">
  <div className="guard-hero">...</div>
  <section className="guard-capability-grid">...</section>
</div>
```

- [ ] **Step 2: Make the wrapper the guardian content row**

```css
.guard-stack {
  grid-template-rows: auto minmax(0, 1fr);
}

.guard-scroll {
  display: grid;
  align-content: start;
  gap: 14px;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  padding-right: 4px;
}
```

- [ ] **Step 3: Run the focused test to verify it passes**

Run: `pnpm --dir frontend test GuardView.test.tsx`

Expected: PASS with the guard view test file reporting zero failures.

### Task 3: Verify the shipped frontend artifact

**Files:**
- Generated: `public/app/*`

- [ ] **Step 1: Run all frontend tests**

Run: `pnpm --dir frontend test`

Expected: PASS with zero failing tests.

- [ ] **Step 2: Build the frontend bundle**

Run: `pnpm run frontend:build`

Expected: PASS and emit an updated JavaScript asset under `public/app/assets`.

- [ ] **Step 3: Check generated asset reference**

Run: `Get-Content public/app/index.html`

Expected: the page references the newly emitted hashed asset.
