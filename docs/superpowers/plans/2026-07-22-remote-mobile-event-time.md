# Remote Mobile Event Time Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Display complete `HH:MM:SS` recent-event timestamps in the LAN WebUI at phone widths.

**Architecture:** Keep event formatting and markup unchanged. The remote-only `max-width: 760px` CSS grid reserves a sufficient fixed time column, while the existing `minmax(0, 1fr)` details column preserves current title truncation and result line clamping. A CSS-contract test protects the exact mobile grid allocation.

**Tech Stack:** React, TypeScript, Vitest, CSS Grid, Vite.

---

## File Structure

- Modify `frontend/src/views/OverviewView.test.tsx`: assert that the final remote-phone event grid reserves a 52px timestamp column.
- Modify `frontend/src/style.css`: increase the first remote-phone `.workbench-event` grid column from 44px to 52px.

### Task 1: Reserve a Complete Mobile Event Timestamp Column

**Files:**
- Modify: `frontend/src/views/OverviewView.test.tsx:123`
- Modify: `frontend/src/style.css:7360`

- [x] **Step 1: Change the existing CSS-contract expectation to 52px**

Replace the mobile event grid assertion with:

```tsx
expect(narrowEvent).toMatch(/grid-template-columns:\s*52px\s+minmax\(0,\s*1fr\);/);
```

- [x] **Step 2: Run the focused test and verify RED**

Run:

```powershell
Set-Location frontend
npm test -- OverviewView.test.tsx
```

Expected: the responsive layout test fails because `style.css` still declares `44px` for the first `.workbench-event` column.

- [x] **Step 3: Increase only the remote-phone time column**

In the existing remote `@media (max-width: 760px)` rule, replace the event grid declaration with:

```css
.app-shell-remote .workbench-event {
  grid-template-columns: 52px minmax(0, 1fr);
  grid-template-rows: auto auto;
  min-height: 56px;
  padding: 10px 0;
  gap: 4px 8px;
}
```

- [x] **Step 4: Run the focused test and verify GREEN**

Run:

```powershell
Set-Location frontend
npm test -- OverviewView.test.tsx
```

Expected: all `OverviewView` tests pass, including the updated 52px mobile timestamp-column assertion.

- [x] **Step 5: Run the production frontend build**

Run:

```powershell
Set-Location frontend
npm run build
```

Expected: TypeScript checking and the Vite production build exit with code 0.

- [ ] **Step 6: Inspect the remote-phone layout**

At a 360px-wide remote WebUI viewport, render an event timestamped `23:37:16` and verify all eight characters remain visible on one line. Confirm the adjacent task title and result remain inside the second grid column, without overlap or horizontal page scrolling.

- [ ] **Step 7: Commit the focused fix**

```powershell
git add -- frontend/src/style.css frontend/src/views/OverviewView.test.tsx
git commit -m "fix: show complete remote mobile event times"
```

## Self-Review

- Spec coverage: Task 1 covers the 52px remote-only layout adjustment, the CSS regression assertion, focused test, build, and phone-width visual acceptance condition.
- Placeholder scan: no deferred choices or unspecified commands remain.
- Consistency: the selector, breakpoint, expected CSS value, and test expectation all use the same 52px time column.
