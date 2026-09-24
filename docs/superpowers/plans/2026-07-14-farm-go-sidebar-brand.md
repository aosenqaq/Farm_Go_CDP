# Farm Go Sidebar Brand Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the sidebar diagnostic subtitle with the selected animated Farm Go sweeping-metal wordmark.

**Architecture:** Keep the brand as a text-only element in `AppShell`; the presentation is entirely local CSS in `style.css`. Pseudo-elements supply the decorative line and `//` suffix, while a reduced-motion override preserves the static wordmark for users who disable animation.

**Tech Stack:** React 18, TypeScript, Vitest, CSS, Vite.

---

## File structure

- Modify `frontend/src/components/AppShell.tsx`: render the single `Farm Go` brand name and remove the diagnostic subtitle node.
- Modify `frontend/src/components/AppShell.test.tsx`: assert the new brand text is rendered and the removed subtitle is absent.
- Modify `frontend/src/style.css`: add the selected wordmark treatment and reduced-motion behavior beside the current sidebar brand styles.

### Task 1: Lock the visible brand contract

**Files:**
- Modify: `frontend/src/components/AppShell.test.tsx`
- Modify: `frontend/src/components/AppShell.tsx:43-46`

- [ ] **Step 1: Write the failing test**

Add this assertion block to the existing `uses grouped farm feature navigation` test after rendering `html`:

```tsx
expect(html).toContain('Farm Go');
expect(html).not.toContain('Farm_Go');
expect(html).not.toContain('本机诊断站');
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `pnpm test -- AppShell.test.tsx`

Expected: the test fails because the rendered shell still contains `Farm_Go` and `本机诊断站`.

- [ ] **Step 3: Render the approved single-line wordmark**

Replace the brand block with:

```tsx
<div className="brand-block">
  <div className="brand-name">Farm Go</div>
</div>
```

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `pnpm test -- AppShell.test.tsx`

Expected: the AppShell test file passes with the new brand contract.

### Task 2: Apply the sweeping-metal presentation

**Files:**
- Modify: `frontend/src/style.css:177-190`

- [ ] **Step 1: Replace the brand CSS with the scoped visual treatment**

Replace the existing `.brand-name` and `.brand-subtitle` rules with:

```css
.brand-name {
  position: relative;
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  color: transparent;
  background: linear-gradient(105deg, #ffffff 8%, #b6cdb1 31%, #f8f3e8 52%, #d6ff67 66%, #f8f3e8 82%);
  background-size: 240% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  font-size: 18px;
  font-weight: 900;
  letter-spacing: 0;
  text-shadow: 0 1px 0 rgba(255, 255, 255, 0.08);
  animation: brand-metal-sweep 3.5s linear infinite;
}

.brand-name::before {
  position: absolute;
  top: -5px;
  left: 0;
  width: 94px;
  height: 1px;
  background: rgba(214, 255, 103, 0.6);
  box-shadow: 0 0 10px rgba(214, 255, 103, 0.5);
  content: "";
}

.brand-name::after {
  margin-left: 6px;
  color: #d6ff67;
  content: "//";
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 2px;
}

@keyframes brand-metal-sweep {
  to {
    background-position: -240% 0;
  }
}
```

- [ ] **Step 2: Add the reduced-motion override**

Append this selector to the existing `@media (prefers-reduced-motion: reduce)` block:

```css
.brand-name {
  animation: none;
}
```

- [ ] **Step 3: Run the focused test again**

Run: `pnpm test -- AppShell.test.tsx`

Expected: the test remains green because styling does not change text content.

### Task 3: Verify the production bundle

**Files:**
- Generated: `public/app/index.html`
- Generated: `public/app/assets/*`

- [ ] **Step 1: Run the complete frontend test suite**

Run: `pnpm test`

Expected: all Vitest tests pass with zero failures.

- [ ] **Step 2: Build the frontend bundle served by the desktop gateway**

Run: `pnpm run frontend:build`

Expected: TypeScript and Vite complete successfully, and `public/app/index.html` references the newly generated hashed asset files.

- [ ] **Step 3: Confirm generated output changed**

Run: `Get-ChildItem public\\app\\assets | Sort-Object LastWriteTime -Descending | Select-Object -First 5 Name,LastWriteTime`

Expected: the latest hashed assets have timestamps from this build.

- [ ] **Step 4: Commit the completed implementation**

Run:

```powershell
git add -- frontend/src/components/AppShell.tsx frontend/src/components/AppShell.test.tsx frontend/src/style.css public/app
git commit -m "feat: refresh Farm Go sidebar brand"
```

Expected: one commit contains only the sidebar brand source, test, and generated frontend bundle.
