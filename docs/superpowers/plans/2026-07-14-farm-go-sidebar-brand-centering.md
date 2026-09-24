# Farm Go Sidebar Brand Centering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Center the complete Farm Go wordmark horizontally inside the desktop sidebar brand area.

**Architecture:** The brand container owns horizontal alignment through flexbox. The existing wordmark and its pseudo-elements remain unchanged, so the decorative line and suffix stay attached to the text while the current height, margin, responsive hiding, and animation are preserved.

**Tech Stack:** React 18, TypeScript, Vitest, CSS, Vite.

---

## File structure

- Modify `frontend/src/App.test.tsx`: add a CSS source contract for the desktop brand container alignment.
- Modify `frontend/src/style.css`: make `.brand-block` a horizontally centered flex container.

### Task 1: Lock the centering contract

**Files:**
- Modify: `frontend/src/App.test.tsx`
- Modify: `frontend/src/style.css:177-180`

- [ ] **Step 1: Write the failing CSS contract test**

Add this test after `does not preserve a transform on app views that contain fixed dialogs`:

```tsx
it('centers the desktop sidebar brand as one wordmark', () => {
  const css = readFileSync(new URL('./style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');

  expect(css).toMatch(/\.brand-block \{[^}]*display: flex;[^}]*justify-content: center;/);
});
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `pnpm test -- App.test.tsx`

Expected: the new test fails because `.brand-block` has no flex display or horizontal centering rule.

- [ ] **Step 3: Add the minimal container alignment**

Update the existing selector to:

```css
.brand-block {
  display: flex;
  justify-content: center;
  min-height: 34px;
  margin-bottom: 22px;
}
```

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `pnpm test -- App.test.tsx`

Expected: all App tests pass, including the centered-brand CSS contract.

### Task 2: Verify shipped output

**Files:**
- Generated: `frontend/dist/index.html`
- Generated: `frontend/dist/assets/*`

- [ ] **Step 1: Run the complete frontend test suite**

Run: `pnpm test`

Expected: all Vitest tests pass with zero failures.

- [ ] **Step 2: Run the production frontend build**

Run: `pnpm run build`

Expected: TypeScript and Vite complete successfully, and the generated CSS includes the `.brand-block` centering rules.

- [ ] **Step 3: Verify the generated CSS**

Run: `rg -l '\.brand-block\{justify-content:center' dist/assets`

Expected: the latest hashed CSS asset is reported.

- [ ] **Step 4: Commit the implementation**

Run:

```powershell
git add -- frontend/src/App.test.tsx frontend/src/style.css docs/superpowers/plans/2026-07-14-farm-go-sidebar-brand-centering.md
git commit -m "style: center Farm Go sidebar brand"
```

Expected: one commit contains only the centering source, test, and implementation plan.
