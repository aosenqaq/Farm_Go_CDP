# Warehouse Sell Records Close Button Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the warehouse sell-records dialog's top-right close button consistently visible and clickable on desktop and narrow remote layouts.

**Architecture:** Keep the existing dialog markup, Lucide icon, and `onClose` behavior. Constrain the records dialog grid column so the table cannot push its header beyond the clipped viewport, then add a dialog-owned class with an explicit visual/layout contract for the existing button.

**Tech Stack:** React 18, TypeScript, Lucide React, CSS, Vitest, Vite

---

## File Map

- Modify `frontend/src/views/AssetsLandView.test.tsx`: add the regression contract before production changes.
- Modify `frontend/src/views/AssetsLandView.tsx`: assign the dedicated class to the existing close button.
- Modify `frontend/src/style.css`: define the stable visible close-button presentation.
- Rebuild `frontend/dist/index.html` and hashed assets through the existing Vite build.

### Task 1: Add the Sell-Records Close-Button Contract

**Files:**
- Test: `frontend/src/views/AssetsLandView.test.tsx`
- Modify: `frontend/src/views/AssetsLandView.tsx:1781`
- Modify: `frontend/src/style.css:5475`

- [ ] **Step 1: Write the failing regression test**

Extend the existing `renders warehouse sell record controls and sell result toast copy` test with source and CSS assertions:

```tsx
const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');
expect(source).toContain('className="icon-button warehouse-records-close"');
expect(source).toContain('aria-label="关闭出售记录"');
expect(css).toMatch(/\.warehouse-records-dialog\s*\{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\);/);
expect(css).toMatch(/\.warehouse-records-close\s*\{[^}]*width:\s*34px;[^}]*height:\s*34px;[^}]*min-width:\s*34px;[^}]*min-height:\s*34px;/);
expect(css).toMatch(/\.warehouse-records-close\s*\{[^}]*color:\s*#20271f;[^}]*background:\s*#d6ff67;[^}]*visibility:\s*visible;[^}]*opacity:\s*1;/);
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
npm test -- --run src/views/AssetsLandView.test.tsx
```

Run from `frontend`. Expected: FAIL because `warehouse-records-close` does not yet exist in the component or stylesheet.

- [ ] **Step 3: Assign the dedicated class to the existing close button**

Change only the sell-records close control in `WarehouseSellRecordsDialog`:

```tsx
<button
  className="icon-button warehouse-records-close"
  type="button"
  aria-label="关闭出售记录"
  onClick={onClose}
>
  <X size={18} />
</button>
```

- [ ] **Step 4: Add the minimal dialog-owned CSS contract**

Place these rules beside the warehouse dialog/header styles in `frontend/src/style.css`:

```css
.warehouse-records-dialog {
  grid-template-columns: minmax(0, 1fr);
}

.warehouse-records-close {
  position: relative;
  z-index: 1;
  justify-self: end;
  flex: 0 0 34px;
  width: 34px;
  height: 34px;
  min-width: 34px;
  min-height: 34px;
  color: #20271f;
  background: #d6ff67;
  visibility: visible;
  opacity: 1;
}

.warehouse-records-close svg {
  display: block;
  flex: none;
}
```

- [ ] **Step 5: Run the focused test and verify GREEN**

Run from `frontend`:

```powershell
npm test -- --run src/views/AssetsLandView.test.tsx
```

Expected: the `AssetsLandView` test file passes with no warnings or errors.

- [ ] **Step 6: Run the complete frontend test suite**

Run from `frontend`:

```powershell
npm test
```

Expected: all frontend tests pass.

- [ ] **Step 7: Commit the tested source fix**

```powershell
git add -- frontend/src/views/AssetsLandView.test.tsx frontend/src/views/AssetsLandView.tsx frontend/src/style.css
git commit -m "fix: show warehouse records close button"
```

### Task 2: Rebuild and Verify the Delivered UI

**Files:**
- Modify: `frontend/dist/index.html`
- Modify/Create: `frontend/dist/assets/index-*.css`
- Modify/Create: `frontend/dist/assets/index-*.js`

- [ ] **Step 1: Record the current generated asset references**

Run from the repository root:

```powershell
Get-Content -Raw -LiteralPath 'frontend/dist/index.html'
Get-Item -LiteralPath 'frontend/dist/index.html' | Select-Object LastWriteTime
```

Expected: output includes the current hashed JavaScript and CSS asset names and the pre-build timestamp.

- [ ] **Step 2: Build the frontend**

Run from `frontend`:

```powershell
npm run build
```

Expected: TypeScript and Vite complete successfully and write `frontend/dist`.

- [ ] **Step 3: Confirm the generated asset changed and contains the fix**

Run from the repository root:

```powershell
Get-Content -Raw -LiteralPath 'frontend/dist/index.html'
Get-Item -LiteralPath 'frontend/dist/index.html' | Select-Object LastWriteTime
rg -l -F 'warehouse-records-close' frontend/dist/assets
```

Expected: `index.html` has a newer timestamp, references the newly generated hashed assets, and at least one generated CSS/JS asset contains `warehouse-records-close`.

- [ ] **Step 4: Verify the dialog visually and interactively**

Start the existing frontend development server on an unused local port, open the warehouse page, and open `出售记录`. Verify at a desktop viewport and a viewport narrower than 760 CSS pixels:

```text
- A lime square X button is visible in the dialog header's top-right corner.
- The button does not overlap the title, date filter, or table/list content.
- Clicking the button closes the dialog.
- Reopening the dialog restores the same visible button.
```

- [ ] **Step 5: Commit generated frontend assets if tracked**

```powershell
git add -- frontend/dist
git commit -m "build: refresh frontend assets"
```

If `frontend/dist` is ignored or unchanged in Git, record that fact and do not create an empty commit.
